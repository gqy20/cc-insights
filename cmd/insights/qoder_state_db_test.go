package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// createQoderStateDB 造一个最小 Qoder 状态库：只建解析层真正读到的两张表。
func createQoderStateDB(t *testing.T, dir string) string {
	t.Helper()

	dbPath := filepath.Join(dir, qoderStateDBName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("打开测试状态库失败: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table chat_sessions (session_id text primary key, cwd text, title text)`,
		`create table chat_session_messages (session_id text, turn_id text, role text, status text, created_at integer, payload_json text)`,
		`create table chat_session_context_usage (session_id text, snapshot_json text, updated_at integer)`,
		`insert into chat_sessions values ('sess-q1', '/tmp/qoder-project', '补足qoder支持')`,
		`insert into chat_session_messages values ('sess-q1', 'turn-1', 'assistant', 'completed', 1770013200000, '{"durationMs":45000,"turnMetrics":{"durationMs":45000,"turnCount":2}}')`,
		`insert into chat_session_messages values ('sess-q1', 'turn-2', 'assistant', 'failed', 1770013300000, '{"durationMs":90000,"turnMetrics":{"durationMs":0,"turnCount":1}}')`,
		`insert into chat_session_messages values ('sess-q1', 'turn-3', 'assistant', 'interrupted', 1770013400000, '{"durationMs":0}')`,
		// durationMs 缺失时退回 turnMetrics.durationMs；两者皆无则不计回合。
		`insert into chat_session_messages values ('sess-q1', 'turn-4', 'assistant', 'completed', 1770013500000, '{"durationMs":0,"turnMetrics":{"durationMs":12000,"turnCount":3}}')`,
		`insert into chat_session_context_usage values ('sess-q1', '{"totalTokens":0,"categories":[{"name":"system_prompt","percentage":0.031},{"name":"messages","percentage":0.297},{"name":"skills","percentage":0.066},{"name":"free_space","percentage":0.382}]}', 1770013200000)`,
		`insert into chat_session_context_usage values ('sess-q2', '{"categories":[{"name":"messages","percentage":0.41},{"name":"system_prompt","percentage":0.02}]}', 1770013300000)`,
		// 空 categories（detailsPending 状态）与非法 JSON 都不能把整体读取带崩。
		`insert into chat_session_context_usage values ('sess-q3', '{"categories":[]}', 1770013400000)`,
		`insert into chat_session_context_usage values ('sess-q4', 'not-json', 1770013500000)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("初始化测试状态库失败: %v", err)
		}
	}
	return dbPath
}

func TestApplyQoderStateDBContextWindowsFillsGaps(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	storage := filepath.Join(appData, qoderAppStorageName)
	if err := os.MkdirAll(storage, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(storage, qoderStateDBName))
	if err != nil {
		t.Fatalf("打开测试状态库失败: %v", err)
	}
	for _, statement := range []string{
		`create table chat_model_preferences (model_key text primary key, visible integer, context_window integer, reasoning_effort text, updated_at integer)`,
		`insert into chat_model_preferences values ('qfmodel', 1, 400000, null, 0)`,
		`insert into chat_model_preferences values ('ultimate', 1, 1000000, null, 0)`,
		// 日志已给出 smodel=180000，DB 的 999999 不得覆盖（日志是请求时实际配置）。
		`insert into chat_model_preferences values ('smodel', 1, 999999, null, 0)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("初始化 chat_model_preferences 失败: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关闭测试状态库失败: %v", err)
	}

	catalog := map[string]qoderModelInfo{
		"smodel":   {Name: "Sonus", MaxInputTokens: 180000},
		"ultimate": {Name: "Ultimate"},
	}
	applyQoderStateDBContextWindows(catalog)
	if got := catalog["smodel"].MaxInputTokens; got != 180000 {
		t.Fatalf("smodel.MaxInputTokens = %d, want 180000（日志优先，DB 不覆盖）", got)
	}
	if got := catalog["ultimate"].MaxInputTokens; got != 1000000 {
		t.Fatalf("ultimate.MaxInputTokens = %d, want 1000000（DB 兜底）", got)
	}
	// catalog 里没有的别名（日志被清理过）也允许 DB 单独补窗口：真名缺席但估算仍可用。
	if got := catalog["qfmodel"]; got.MaxInputTokens != 400000 || got.Name != "" {
		t.Fatalf("qfmodel = %+v, want DB 兜底窗口 400000 且不带真名", got)
	}

	// 表缺失（旧版状态库）：静默跳过，不 panic、不改 catalog。
	legacyAppData := t.TempDir()
	t.Setenv("APPDATA", legacyAppData)
	legacyStorage := filepath.Join(legacyAppData, qoderAppStorageName)
	if err := os.MkdirAll(legacyStorage, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	createQoderStateDB(t, legacyStorage) // 没有 chat_model_preferences 表
	missing := map[string]qoderModelInfo{"smodel": {Name: "Sonus"}}
	applyQoderStateDBContextWindows(missing)
	if missing["smodel"].MaxInputTokens != 0 {
		t.Fatalf("无 chat_model_preferences 表时不应写入窗口: %+v", missing["smodel"])
	}
}

func TestRecordQoderStateDBTurnSignals(t *testing.T) {
	dbPath := createQoderStateDB(t, t.TempDir())
	agg := newProjectAggregate()
	if err := recordQoderStateDBLocked(agg, dbPath, TimeFilter{}); err != nil {
		t.Fatalf("读取状态库失败: %v", err)
	}

	// turn-1/2/4 有耗时，turn-3 两者皆 0 不计。
	if agg.TurnCount != 3 {
		t.Fatalf("TurnCount=%d, want 3", agg.TurnCount)
	}
	if agg.TurnTotalDurationMs != 45000+90000+12000 {
		t.Fatalf("TurnTotalDurationMs=%d, want %d", agg.TurnTotalDurationMs, 147000)
	}
	if agg.TurnMaxDurationMs != 90000 {
		t.Fatalf("TurnMaxDurationMs=%d, want 90000", agg.TurnMaxDurationMs)
	}
	if len(agg.SlowTurns) == 0 || agg.SlowTurns[0].Project != "/tmp/qoder-project" {
		t.Fatalf("慢回合项目归因缺失: %+v", agg.SlowTurns)
	}

	// turn 结局只有状态库知道：transcript 里没有 failed/interrupted。
	if agg.EventTypes["qoder_turn_failed"] != 1 {
		t.Fatalf("qoder_turn_failed=%v, want 1", agg.EventTypes["qoder_turn_failed"])
	}
	if agg.EventTypes["qoder_turn_interrupted"] != 1 {
		t.Fatalf("qoder_turn_interrupted=%v, want 1", agg.EventTypes["qoder_turn_interrupted"])
	}
	if agg.EventTypes["qoder_turn_completed"] != 0 {
		t.Fatalf("completed 不应记为异常事件: %v", agg.EventTypes)
	}
	// 按天快照必须同步，否则时间过滤查询会丢信号。
	dateKey := time.UnixMilli(1770013200000).Local().Format("2006-01-02")
	snapshot := agg.DailyRuntime[dateKey]
	if snapshot == nil || snapshot.EventTypes["qoder_turn_failed"] != 1 {
		t.Fatalf("按天快照缺少 turn 结局: dateKey=%s snapshot=%+v", dateKey, snapshot)
	}
	if snapshot == nil || snapshot.TurnCount == 0 {
		t.Fatalf("按天快照缺少回合耗时: %+v", snapshot)
	}
}

func TestRecordQoderContextCategoriesTakesMaxPerCategory(t *testing.T) {
	dbPath := createQoderStateDB(t, t.TempDir())
	agg := newProjectAggregate()
	if err := recordQoderContextCategories(agg, dbPath, TimeFilter{}); err != nil {
		t.Fatalf("读取上下文构成失败: %v", err)
	}

	shares := agg.ContextStats.CategoryShares
	if len(shares) == 0 {
		t.Fatalf("CategoryShares 为空，占比没有落地")
	}
	// messages: 0.297 与 0.41 取最大；system_prompt: 0.031 与 0.02 取最大。
	if shares["messages"] < 0.409 || shares["messages"] > 0.411 {
		t.Fatalf("messages 占比=%v, want 0.41", shares["messages"])
	}
	if shares["system_prompt"] < 0.030 || shares["system_prompt"] > 0.032 {
		t.Fatalf("system_prompt 占比=%v, want 0.031", shares["system_prompt"])
	}
	if shares["skills"] < 0.065 || shares["skills"] > 0.067 {
		t.Fatalf("skills 占比=%v, want 0.066", shares["skills"])
	}
	if got := formatContextCategoryShares(shares, 2); got == "" {
		t.Fatalf("formatContextCategoryShares 返回空串")
	} else if !strings.Contains(got, "messages 41%") || !strings.Contains(got, "free_space 38%") {
		t.Fatalf("Top-2 应按占比降序: %q", got)
	}
	if got := formatContextCategoryShares(shares, 99); strings.Count(got, " · ") != len(shares)-1 {
		t.Fatalf("全量渲染类别数不对: %q (%d 类)", got, len(shares))
	}

	// 按天快照同样要有这份数据：时间过滤查询从快照重建，只写顶层会让带 -p 的查询读空。
	snapshot := agg.DailyRuntime[time.UnixMilli(1770013200000).Local().Format("2006-01-02")]
	if snapshot == nil || snapshot.ContextStats == nil || len(snapshot.ContextStats.CategoryShares) == 0 {
		t.Fatalf("按天快照缺少上下文构成: %+v", snapshot)
	}
}

func TestRecordQoderContextCategoriesHonorsTimeFilter(t *testing.T) {
	dbPath := createQoderStateDB(t, t.TempDir())
	agg := newProjectAggregate()
	// 窗口恰好只覆盖第一条快照（1770013200000），把 1770013300000 的 messages=0.41 排除在外。
	start := time.UnixMilli(1770013200000)
	end := time.UnixMilli(1770013299999)
	if err := recordQoderContextCategories(agg, dbPath, TimeFilter{Start: &start, End: &end}); err != nil {
		t.Fatalf("读取上下文构成失败: %v", err)
	}

	// 窗口内只有第一条快照：messages 应取 0.297 而不是全量的 0.41。
	shares := agg.ContextStats.CategoryShares
	if shares["messages"] < 0.296 || shares["messages"] > 0.298 {
		t.Fatalf("窗口外快照混入顶层: messages=%v, want 0.297", shares["messages"])
	}
	// 窗口外的行连按天快照也不该产生。
	if len(agg.DailyRuntime) != 1 {
		t.Fatalf("按天快照数=%d, want 1: %v", len(agg.DailyRuntime), agg.DailyRuntime)
	}
}

func TestQoderStateDBMissingIsSilent(t *testing.T) {
	agg := newProjectAggregate()
	if err := recordQoderStateDBLocked(agg, "", TimeFilter{}); err != nil {
		t.Fatalf("空路径应静默跳过: %v", err)
	}
	if err := recordQoderStateDBLocked(agg, filepath.Join(t.TempDir(), "absent.sqlite"), TimeFilter{}); err != nil {
		t.Fatalf("状态库不存在应静默跳过: %v", err)
	}
	if err := recordQoderContextCategories(agg, filepath.Join(t.TempDir(), "absent.sqlite"), TimeFilter{}); err == nil {
		t.Fatalf("状态库不存在时读取应报错，由调用方降级为 Warn")
	}
	if agg.TurnCount != 0 || len(agg.EventTypes) != 0 {
		t.Fatalf("降级路径不应写入任何统计: turns=%d events=%v", agg.TurnCount, agg.EventTypes)
	}
}

func TestMergeContextStatsKeepsCategorySharesAndPeak(t *testing.T) {
	dst := newProjectAggregate()
	dst.ContextStats = &ContextAnalysisData{
		PeakContextRatio: 0.5,
		CategoryShares:   map[string]float64{"messages": 0.3},
	}
	src := newProjectAggregate()
	src.ContextStats = &ContextAnalysisData{
		PeakContextRatio:   0.9,
		ContextCompactions: 2,
		CategoryShares:     map[string]float64{"messages": 0.2, "skills": 0.07},
	}
	mergeContextStats(dst, src.ContextStats)

	if dst.ContextStats.PeakContextRatio != 0.9 {
		t.Fatalf("PeakContextRatio=%v, want 0.9", dst.ContextStats.PeakContextRatio)
	}
	if dst.ContextStats.ContextCompactions != 2 {
		t.Fatalf("ContextCompactions=%d, want 2", dst.ContextStats.ContextCompactions)
	}
	if dst.ContextStats.CategoryShares["messages"] != 0.3 {
		t.Fatalf("较低占比覆盖了较高占比: messages=%v", dst.ContextStats.CategoryShares["messages"])
	}
	if dst.ContextStats.CategoryShares["skills"] != 0.07 {
		t.Fatalf("新类别没有合并: skills=%v", dst.ContextStats.CategoryShares["skills"])
	}
}
