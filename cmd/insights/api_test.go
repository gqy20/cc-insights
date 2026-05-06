package main

import (
	"encoding/json"
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
