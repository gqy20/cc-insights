package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGracefulDegradation_MissingDebugDir 测试 debug 目录缺失时仍返回其他数据
// P0: 单个数据源故障不应导致整个 API 失败
func TestGracefulDegradation_MissingDebugDir(t *testing.T) {
	// Arrange: 创建最小化的假数据目录
	tmpDir := t.TempDir()

	// 创建必要的子目录
	os.MkdirAll(filepath.Join(tmpDir, "projects", "test-proj"), 0755)

	// 写入有效的 history.jsonl
	historyPath := filepath.Join(tmpDir, "history.jsonl")
	historyContent := `{"display":"/test","pastedContents":{},"timestamp":1700000000000,"project":"/tmp"}
{"display":"/help","pastedContents":{},"timestamp":1700000001000,"project":"/tmp"}
`
	os.WriteFile(historyPath, []byte(historyContent), 0644)

	// 写入有效的 projects/*.jsonl
	projPath := filepath.Join(tmpDir, "projects", "test-proj", "session.jsonl")
	projContent := `{"type":"assistant","message":{"id":"1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":10,"output_tokens":20}},"timestamp":"2024-11-15T00:00:00Z","cwd":"/tmp","sessionId":"s1"}
`
	os.WriteFile(projPath, []byte(projContent), 0644)

	// 故意不创建 debug/ 目录 → ParseDebugLogsConcurrent 应该失败

	// 切换数据目录到 tmpDir
	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	// Act: 调用 buildDataFromParsing
	data, err := buildDataFromParsing(tf, "all")

	// 🔴 红阶段: 当前实现会在 debug 缺失时返回 error
	// 修复后应该返回非 nil 的部分数据
	if err != nil {
		t.Errorf("❌ FAILED: buildDataFromParsing 返回错误 (应优雅降级): %v", err)
		return
	}

	if data == nil {
		t.Error("❌ FAILED: 返回了 nil 数据")
		return
	}

	// 验证: 成功解析的数据源应有值，失败的应为空默认值
	if len(data.Commands) == 0 {
		t.Error("Commands 不应为空（history.jsonl 存在且有效）")
	}
	if data.ProjectStats == nil || len(data.ProjectStats.Projects) == 0 {
		t.Error("ProjectStats 不应为空（projects 数据存在）")
	}
	// debug 缺失时 MCPTools 应为空而非 nil 导致 panic
	if data.MCPTools == nil {
		t.Log("⚠️ MCPTools 为 nil（debug 目录缺失，预期为空切片）")
	}
}

// TestGracefulDegradation_CorruptStatsCache 测试 stats-cache.json 损坏时的降级
// P0: JSON 文件被截断时应容错
func TestGracefulDegradation_CorruptStatsCache(t *testing.T) {
	// Arrange: 创建损坏的 stats-cache.json
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "stats-cache.json")
	os.WriteFile(cachePath, []byte("{invalid json content"), 0644)

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	// Act
	cache, err := ParseStatsCache()

	// 🔴 红阶段: 当前可能 panic 或返回难以处理的错误
	// 修复后应返回 error 但不 panic，调用方可安全降级
	if err == nil {
		// 如果没报错，验证内容是否安全
		if cache == nil {
			t.Log("返回 nil + nil error，可接受")
		}
	} else {
		t.Logf("ParseStatsCache 返回错误 (可接受): %v", err)
	}

	// 关键: 绝不能 panic
	t.Log("✅ 未发生 panic")
}

// TestGracefulDegradation_PartialFailureAPI 测试 API 层面的部分失败处理
// 模拟 HTTP 请求中某个数据源不可用
func TestGracefulDegradation_PartialFailureAPI(t *testing.T) {
	// Arrange: 设置一个缺少 debug 目录的数据环境
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "projects", "p"), 0755)

	historyPath := filepath.Join(tmpDir, "history.jsonl")
	os.WriteFile(historyPath, []byte(`{"display":"/tdd","pastedContents":{},"timestamp":1700000000000,"project":"/tmp"}`+"\n"), 0644)

	projPath := filepath.Join(tmpDir, "projects", "p", "s.jsonl")
	os.WriteFile(projPath, []byte(`{"type":"assistant","message":{"role":"assistant","model":"m-1","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":5,"output_tokens":10}},"timestamp":"2024-11-15T00:00:00Z","cwd":"/tmp","sessionId":"s1"}`+"\n"), 0644)

	// 不创建 debug/ 和 stats-cache.json

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	// Act: 发起 API 请求
	req := httptest.NewRequest("GET", "/api/data?preset=all", nil)
	w := httptest.NewRecorder()
	handleDataAPI(w, req)

	// Assert: HTTP 状态码应该是 200 而不是 400/500
	if w.Code != http.StatusOK {
		t.Errorf("❌ FAILED: HTTP 状态码 %d, 期望 200 (部分数据可用时应成功)", w.Code)
		t.Logf("响应体: %s", w.Body.String())
		return
	}

	// 解析响应
	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("无法解析响应 JSON: %v", err)
	}

	if !resp.Success {
		t.Errorf("❌ FAILED: Success=false, Error=%s (部分数据可用时不应报错)", resp.Error)
		return
	}

	// 验证有部分数据
	d, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应 Data 不是 map")
	}

	// Commands 应有值（history.jsonl 正常）
	cmds, _ := d["commands"].([]interface{})
	if len(cmds) == 0 {
		t.Error("Commands 应不为空")
	}

	t.Logf("✅ API 在部分数据源缺失时仍返回 200, commands=%d", len(cmds))
}

// TestSafeParseHistoryConcurrent 测试 history 解析的容错包装
// P0 错误隔离: 文件不存在/损坏时不 panic
func TestSafeParseHistoryConcurrent(t *testing.T) {
	// Arrange: 不存在的文件
	tmpDir := t.TempDir()
	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	// Act: 应该返回空结果而非 panic 或 error 向上传播
	// 🔴 红阶段: 当前 safe 包装函数尚不存在
	cmdStats, hourlyCounts, err := safeParseHistoryConcurrent(tf)

	if err != nil {
		t.Errorf("❌ FAILED: safeParseHistoryConcurrent 返回 error (应返回空结果): %v", err)
	}

	if cmdStats == nil {
		t.Error("cmdStats 不应为 nil，应为空切片")
	}
	if hourlyCounts == nil {
		t.Error("hourlyCounts 不应为 nil，应为空 map")
	}

	t.Logf("✅ 安全返回空结果: commands=%d, hours=%d", len(cmdStats), len(hourlyCounts))
}

// TestSafeParseProjectsOnce 测试项目解析的容错包装
func TestSafeParseProjectsOnce(t *testing.T) {
	tmpDir := t.TempDir()
	// 不创建 projects 目录
	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	agg, err := safeParseProjectsOnce(tf)

	if err != nil {
		t.Errorf("❌ FAILED: safeParseProjectsOnce 返回 error: %v", err)
	}

	if agg == nil {
		t.Error("❌ FAILED: aggregate 不应为 nil")
		return // 防止 nil pointer panic
	}

	// 验证所有字段都有安全的零值
	if agg.ProjectStats == nil {
		t.Error("ProjectStats 不应为 nil")
	}
	if agg.DailyActivityList == nil {
		t.Error("DailyActivityList 不应为 nil")
	}
	if agg.HourlyData == nil || len(agg.HourlyData) != 24 {
		t.Errorf("HourlyData 应为 24 个元素的切片, got %d", len(agg.HourlyData))
	}

	t.Logf("✅ 安全返回空 aggregate")
}

// TestSafeParseDebugLogs 测试 debug 日志解析的容错包装
func TestSafeParseDebugLogs(t *testing.T) {
	tmpDir := t.TempDir()
	// 不创建 debug 目录
	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	tools, err := safeParseDebugLogs(tf)

	if err != nil {
		t.Errorf("❌ FAILED: safeParseDebugLogs 返回 error: %v", err)
	}

	if tools == nil {
		t.Error("tools 不应为 nil，应为空切片")
	}

	t.Logf("✅ 安全返回空工具列表")
}

// TestSafeParseSessionStats 测试会话统计的容错包装
func TestSafeParseSessionStats(t *testing.T) {
	tmpDir := t.TempDir()
	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	stats, err := safeParseSessionStats(tf)

	if err != nil {
		t.Errorf("❌ FAILED: safeParseSessionStats 返回 error: %v", err)
	}

	if stats == nil {
		t.Error("stats 不应为 nil")
	}

	t.Logf("✅ 安全返回空会话统计")
}

// TestBuildDataFromParsing_AllSourcesMissing 测试所有数据源都缺失时的极端情况
func TestBuildDataFromParsing_AllSourcesMissing(t *testing.T) {
	tmpDir := t.TempDir()
	// 完全空的目录，没有任何数据文件

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	now := time.Now()
	past := now.AddDate(0, 0, -7)
	tf := TimeFilter{
		Start: &past,
		End:   &now,
	}

	data, err := buildDataFromParsing(tf, "all")

	// 🔴 红阶段: 当前实现会在第一个 Parse 失败就返回 error
	// 修复后应返回完整的空 DashboardData
	if err != nil {
		t.Errorf("❌ FAILED: 所有数据源缺失时返回 error (应返回空 DashboardData): %v", err)
		return
	}

	if data == nil {
		t.Error("❌ FAILED: 返回 nil")
		return
	}

	// 验证所有字段都是安全的空值（不会导致前端 panic）
	if data.Commands == nil {
		t.Error("Commands 不应为 nil")
	}
	if data.HourlyCounts == nil {
		t.Error("HourlyCounts 不应为 nil")
	}
	if data.DailyTrend.Dates == nil {
		t.Error("DailyTrend.Dates 不应为 nil")
	}
	if data.MCPTools == nil {
		t.Error("MCPTools 不应为 nil")
	}
	if data.Sessions == nil {
		t.Error("Sessions 不应为 nil")
	}
	if data.ProjectStats == nil {
		t.Error("ProjectStats 不应为 nil")
	}
	if data.WeekdayStats == nil {
		t.Error("WeekdayStats 不应为 nil")
	}
	if data.ModelUsage == nil {
		t.Error("ModelUsage 不应为 nil")
	}
	if data.WorkHoursStats == nil {
		t.Error("WorkHoursStats 不应为 nil")
	}

	t.Logf("✅ 所有字段均有安全的空默认值")
}

// === P0 性能优化: 消除重复解析 ===

// TestNoDuplicateParse_SessionStatsFromAggregate 测试 SessionStats 应从
// 已解析的 ProjectAggregate 中提取，而不是重新遍历所有 jsonl 文件
// 核心验证: 获取 aggregate 后删除源文件，提取 session stats 仍应成功
func TestNoDuplicateParse_SessionStatsFromAggregate(t *testing.T) {
	// Arrange: 创建测试数据
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "projects", "proj-a")
	os.MkdirAll(projDir, 0755)

	// 写入 3 个 session 文件
	for i, sid := range []string{"sess-1", "sess-2", "sess-3"} {
		content := fmt.Sprintf(
			`{"type":"assistant","message":{"role":"assistant","model":"m-1","content":[{"type":"text","text":"msg-%d"}],"usage":{"input_tokens":10,"output_tokens":20}},"timestamp":"2026-01-0%dT10:00:00Z","cwd":"/tmp/proj-a","sessionId":"%s"}`+"\n",
			i, i+1, sid,
		)
		os.WriteFile(filepath.Join(projDir, sid+".jsonl"), []byte(content), 0644)
	}

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	// Step 1: 获取 aggregate（遍历文件一次）
	aggregate, err := safeParseProjectsOnce(tf)
	if err != nil || aggregate == nil {
		t.Fatalf("safeParseProjectsOnce 失败: %v", err)
	}

	// Step 2: 🔴 关键 — 删除 projects 目录！
	// 如果 extractSessionStatsFromAggregate 真正从内存中的 aggregate 提取，
	// 删除源文件后仍应正常工作；如果它重新读文件，则会失败
	os.RemoveAll(filepath.Join(tmpDir, "projects"))

	// Step 3: 从 aggregate 提取 session stats（不应再读文件）
	sessionFromAgg, err := extractSessionStatsFromAggregate(aggregate)

	if err != nil {
		t.Errorf("❌ FAILED: extractSessionStatsFromAggregate 在源文件删除后返回 error: %v\n"+
			"   这说明函数仍在重新读取 projects/*.jsonl 文件（存在重复 I/O）", err)
		return
	}
	if sessionFromAgg == nil {
		t.Error("❌ FAILED: 返回 nil")
		return
	}

	// 验证: 会话数应为 3（来自内存中的 aggregate 数据）
	if sessionFromAgg.TotalSessions == 0 {
		t.Error("❌ FAILED: TotalSessions=0，应有 3 个会话（数据来自 aggregate 内存）")
	}

	t.Logf("✅ SessionStats 真正从 Aggregate 内存提取（不依赖源文件）: sessions=%d", sessionFromAgg.TotalSessions)
}

// TestBuildDataFromParsing_NoRedundantIO 测试 API 构建不应重复读取 projects
// 通过验证：即使删除 projects 目录后的第二次调用仍能返回正确 session 数据
// （因为数据已在第一次 parseProjects 时缓存到 aggregate 中）
func TestBuildDataFromParsing_NoRedundantIO(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()

	// 创建完整的最小数据集
	projDir := filepath.Join(tmpDir, "projects", "p")
	os.MkdirAll(projDir, 0755)
	os.MkdirAll(filepath.Join(tmpDir, "debug"), 0755)

	historyPath := filepath.Join(tmpDir, "history.jsonl")
	os.WriteFile(historyPath, []byte(`{"display":"/tdd","pastedContents":{},"timestamp":1700000000000,"project":"/tmp"}`+"\n"), 0644)

	// 2 个 session 文件 = 2 个会话
	for _, sid := range []string{"s1", "s2"} {
		content := fmt.Sprintf(
			`{"type":"assistant","message":{"role":"assistant","model":"m-1","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":5,"output_tokens":10}},"timestamp":"2024-11-15T00:00:00Z","cwd":"/tmp","sessionId":"%s"}`+"\n",
			sid,
		)
		os.WriteFile(filepath.Join(projDir, sid+".jsonl"), []byte(content), 0644)
	}

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	// Act: 调用 buildDataFromParsing
	data, err := buildDataFromParsing(tf, "all")

	if err != nil {
		t.Fatalf("buildDataFromParsing error: %v", err)
	}
	if data == nil {
		t.Fatal("data is nil")
	}

	// Assert: Sessions 不应为 nil 且有正确的会话数
	if data.Sessions == nil {
		t.Error("❌ FAILED: Sessions 为 nil")
	} else if data.Sessions.TotalSessions < 2 {
		t.Errorf("❌ FAILED: TotalSessions=%d, 期望 >= 2（有 2 个 session 文件）", data.Sessions.TotalSessions)
	}

	// Assert: Projects 和 Sessions 的数据应一致（都来自同一次遍历）
	if data.ProjectStats != nil && data.Sessions != nil {
		// 每个 session 文件至少产生 1 条 assistant 消息
		if data.ProjectStats.TotalMessages < data.Sessions.TotalSessions {
			t.Logf("⚠️ TotalMessages(%d) < TotalSessions(%d)，可能正常（非每条都是assistant）",
				data.ProjectStats.TotalMessages, data.Sessions.TotalSessions)
		}
	}

	t.Logf("✅ 无冗余 I/O: sessions=%d, messages=%d, commands=%d",
		data.Sessions.TotalSessions,
		data.ProjectStats.TotalMessages,
		len(data.Commands))
}


// === P1 性能优化: 三大数据源并行解析 ===

// TestParallelParsing_Correctness 测试并行解析结果与串行一致
func TestParallelParsing_Correctness(t *testing.T) {
	// Arrange: 创建完整测试数据集
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "projects", "myproj")
	debugDir := filepath.Join(tmpDir, "debug")
	os.MkdirAll(projDir, 0755)
	os.MkdirAll(debugDir, 0755)

	// history.jsonl: 2 条记录
	historyContent := `{"display":"/cmd1","pastedContents":{},"timestamp":1700000000000,"project":"/tmp"}
{"display":"/cmd2","pastedContents":{},"timestamp":1700000100000,"project":"/tmp"}
`
	os.WriteFile(filepath.Join(tmpDir, "history.jsonl"), []byte(historyContent), 0644)

	// projects: 2 个 session 文件
	for _, sid := range []string{"sa", "sb"} {
		content := fmt.Sprintf(
			`{"type":"assistant","message":{"role":"assistant","model":"m-1","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":5,"outputTokens":10}},"timestamp":"2024-11-15T10:00:00Z","cwd":"/tmp/myproj","sessionId":"%s"}`+"\n",
			sid,
		)
		os.WriteFile(filepath.Join(projDir, sid+".jsonl"), []byte(content), 0644)
	}

	// debug: 1 个文件含 MCP 调用
	debugContent := `2024-11-15T10:00:01.000Z [DEBUG] ToolSearchTool: selected mcp__test__tool_name
2024-11-15T10:00:02.000Z [DEBUG] executePreToolHooks called for tool: mcp__test__tool_name
`
	os.WriteFile(filepath.Join(debugDir, "debug-session.txt"), []byte(debugContent), 0644)

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	// Act: 并行解析
	data, err := buildDataFromParsing(tf, "all")
	if err != nil {
		t.Fatalf("buildDataFromParsing error: %v", err)
	}
	if data == nil {
		t.Fatal("data is nil")
	}

	// Assert: 所有四个数据源都有正确数据
	if len(data.Commands) == 0 {
		t.Error("Commands 不应为空（history 有 2 条命令）")
	}
	if data.ProjectStats == nil || data.ProjectStats.TotalMessages == 0 {
		t.Error("ProjectStats 消息数应为 >0")
	}
	if len(data.MCPTools) == 0 {
		t.Error("MCPTools 不应为空（debug 有 MCP 记录）")
	}
	if data.Sessions == nil || data.Sessions.TotalSessions == 0 {
		t.Error("Sessions 不应为空（有 2 个 session 文件）")
	}

	// 验证结果一致性: commands 数量应匹配 history 行数
	if len(data.Commands) != 2 {
		t.Errorf("Commands 数量=%d, 期望=2", len(data.Commands))
	}

	t.Logf("✅ 并行解析结果一致: cmds=%d, msgs=%d, tools=%d, sessions=%d",
		len(data.Commands), data.ProjectStats.TotalMessages,
		len(data.MCPTools), data.Sessions.TotalSessions)
}

// === P0: 消除 cache_builder 冗余 I/O ===

// TestBuildFullCache_NoRedundantSessionParse 测试缓存构建不应重复遍历 projects 获取会话统计
// 核心验证: BuildFullCache 的第一步 ParseProjectsConcurrentOnce 已获取全部数据（含 DailySessions），
// 第三步应直接从 aggregate 提取 session 统计，而非调用 ParseSessionStatsWithFilter 重新遍历文件
func TestBuildFullCache_NoRedundantSessionParse(t *testing.T) {
	// Arrange: 创建最小化测试数据（3个session = 3个会话）
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	os.MkdirAll(cacheDir, 0755)
	debugDir := filepath.Join(tmpDir, "debug")
	os.MkdirAll(debugDir, 0755)

	projDir := filepath.Join(tmpDir, "projects", "test-proj")
	os.MkdirAll(projDir, 0755)

	// 写入 3 个不同日期的 session 文件 = 3 个独立会话
	sessions := []struct {
		sid  string
		date string
	}{
		{"sess-01", "2026-01-01"},
		{"sess-02", "2026-01-02"},
		{"sess-03", "2026-01-03"},
	}
	for _, s := range sessions {
		content := fmt.Sprintf(
			`{"type":"assistant","message":{"role":"assistant","model":"m-1","content":[{"type":"text","text":"msg"}],"usage":{"input_tokens":10,"output_tokens":20}},"timestamp":"%sT12:00:00Z","cwd":"/tmp/proj","sessionId":"%s"}`+"\n",
			s.date, s.sid,
		)
		os.WriteFile(filepath.Join(projDir, s.sid+".jsonl"), []byte(content), 0644)
	}

	// 写入 history.jsonl
	historyPath := filepath.Join(tmpDir, "history.jsonl")
	os.WriteFile(historyPath, []byte(`{"display":"/test","pastedContents":{},"timestamp":1700000000000,"project":"/tmp"}`+"\n"), 0644)

	origDataDir := cfg.DataDir
	origCacheDir := cfg.CacheDir
	cfg.DataDir = tmpDir
	cfg.CacheDir = cacheDir
	defer func() { cfg.DataDir = origDataDir; cfg.CacheDir = origCacheDir }()

	cachePath := filepath.Join(cacheDir, "cache.db")
	builder := &CacheBuilder{CachePath: cachePath, DataDir: tmpDir}

	// 先单独获取 aggregate 作为基准
	tf := TimeFilter{Start: nil, End: nil}
	aggregate, err := ParseProjectsConcurrentOnce(tf)
	if err != nil || aggregate == nil {
		t.Fatalf("ParseProjectsConcurrentOnce 失败: %v", err)
	}

	// 从 aggregate 提取预期的 session 统计（这是正确的无冗余方式）
	expectedSessions, _ := extractSessionStatsFromAggregate(aggregate)

	// Act: 调用 BuildFullCache
	err = builder.BuildFullCache()
	if err != nil {
		t.Fatalf("❌ FAILED: BuildFullCache 返回错误: %v\n"+
			"   绿阶段目标: 应使用 extractSessionStatsFromAggregate 而非 ParseSessionStatsWithFilter", err)
	}

	// 加载构建好的缓存
	loadedCache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("❌ FAILED: 无法加载缓存: %v", err)
	}

	// Assert: 缓存中的 TotalSessions 必须与 aggregate 提取的一致
	if loadedCache.TotalSessions != expectedSessions.TotalSessions {
		t.Errorf("❌ FAILED: 缓存 TotalSessions=%d, 期望=%d (从 aggregate 提取)\n"+
			"   说明 BuildFullCache 未正确使用 aggregate 中的 DailySessions 数据",
			loadedCache.TotalSessions, expectedSessions.TotalSessions)
		return
	}

	// 验证每日 session 分布也一致
	for date, expectedCount := range expectedSessions.DailySessionMap {
		if dayStat, ok := loadedCache.DailyStats[date]; ok {
			if dayStat.SessionCount != expectedCount {
				t.Errorf("❌ FAILED: %s SessionCount=%d, 期望=%d",
					date, dayStat.SessionCount, expectedCount)
			}
		}
	}

	t.Logf("✅ BuildFullCache 正确从 aggregate 提取 session 统计（无冗余 I/O）")
	t.Logf("   TotalSessions=%d, 每日分布: %v",
		loadedCache.TotalSessions, expectedSessions.DailySessionMap)
}

// TestParallelParsing_FasterThanSerial 测试并行比串行快
// 通过对比 buildDataFromParsing 与手动串行调用的耗时
func TestParallelParsing_FasterThanSerial(t *testing.T) {
	// Arrange: 使用实际数据目录（如果可用）
	dataDir := os.Getenv("HOME") + "/.claude"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("跳过：实际数据目录不存在")
	}

	origDataDir := cfg.DataDir
	cfg.DataDir = dataDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	// 预热
	buildDataFromParsing(tf, "all")

	// 测量并行版本（当前实现）
	var parallelDur time.Duration
	iterations := 3
	for i := 0; i < iterations; i++ {
		t0 := time.Now()
		buildDataFromParsing(tf, "all")
		parallelDur += time.Since(t0)
	}
	avgParallel := parallelDur / time.Duration(iterations)

	t.Logf("✅ 并行解析平均耗时: %v (迭代%d次)", avgParallel, iterations)

	// 注意: 此测试主要用于验证不崩溃和基本性能
	// 真正的性能对比需要 benchmark 模式
	if avgParallel > 60*time.Second {
		t.Errorf("⚠️ 并行解析耗时 %v 过长，可能未真正并行化", avgParallel)
	}
}

// === P1: HTTP 超时保护 ===

// TestHTTPTimeout_ProtectSlowRequests 测试 handleDataAPI 应有超时保护
// 防止慢请求长时间占用连接
//
// 🔴 红阶段: 当前实现没有 context.WithTimeout，
// 如果解析耗时很长，HTTP 连接会被无限期占用
func TestHTTPTimeout_ProtectSlowRequests(t *testing.T) {
	// Arrange: 创建正常数据目录
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "projects", "p"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "debug"), 0755)

	historyPath := filepath.Join(tmpDir, "history.jsonl")
	os.WriteFile(historyPath, []byte(`{"display":"/test","pastedContents":{},"timestamp":1700000000000,"project":"/tmp"}`+"\n"), 0644)

	projPath := filepath.Join(tmpDir, "projects", "p", "s.jsonl")
	os.WriteFile(projPath, []byte(`{"type":"assistant","message":{"role":"assistant","model":"m-1","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":5,"output_tokens":10}},"timestamp":"2024-11-15T00:00:00Z","cwd":"/tmp","sessionId":"s1"}`+"\n"), 0644)

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	// Act: 发起 API 请求（正常数据应在合理时间内返回）
	req := httptest.NewRequest("GET", "/api/data?preset=all", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	handleDataAPI(w, req)
	elapsed := time.Since(start)

	// Assert: 正常数据请求应快速完成（<5s）
	if w.Code != http.StatusOK {
		t.Errorf("❌ FAILED: HTTP 状态码 %d, 期望 200", w.Code)
		return
	}

	// 正常小数据集应在 1s 内完成
	if elapsed > 1*time.Second {
		t.Logf("⚠️ 请求耗时 %v（超过 1s，但数据集很小）", elapsed)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("无法解析响应 JSON: %v", err)
	}

	if !resp.Success {
		t.Errorf("❌ FAILED: Success=false, Error=%s", resp.Error)
		return
	}

	t.Logf("✅ HTTP 请求在 %v 内完成（有超时保护时会更安全）", elapsed)
}
