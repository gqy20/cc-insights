package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Qoder transcript 是 Claude 兼容 schema，但官方不下发 token：usage 里的
// input_tokens/output_tokens/cache_* 实测恒为 0（transcript 与运行日志都是 0），
// 消耗改用 credits，上下文压力用 context_usage_ratio。一次 API 响应会被拆成多条
// assistant 记录，Claude 还会在每条上重复同一份 usage，所以记账必须按 message.id 去重。

const qoderSessionFixture = `{"type":"runtime-config","sessionId":"sess-q1","model":"qfmodel","contextWindow":null,"timestamp":1770013200000}
{"type":"user","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:01.000Z","isSidechain":false,"message":{"role":"user","content":[{"type":"text","text":"跑一下测试"}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:02.000Z","isSidechain":false,"message":{"id":"resp-1","role":"assistant","model":"qfmodel","stop_reason":null,"content":[{"type":"thinking","thinking":"先看测试入口"}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:02.100Z","isSidechain":false,"message":{"id":"resp-1","role":"assistant","model":"qfmodel","stop_reason":"tool_use","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"credits":0.5,"context_usage_ratio":0.3},"content":[{"type":"tool_use","id":"call-1","name":"Bash","input":{"command":"go test ./..."}}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:02.200Z","isSidechain":false,"message":{"id":"resp-1","role":"assistant","model":"qfmodel","stop_reason":null,"content":[{"type":"text","text":"执行中"}]}}
{"type":"user","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:05.000Z","isSidechain":false,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-1","is_error":true,"content":"exit status 1"}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:06.000Z","isSidechain":false,"message":{"id":"resp-2","role":"assistant","model":"qfmodel","stop_reason":"tool_use","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"credits":0.25,"context_usage_ratio":0.55},"content":[{"type":"tool_use","id":"call-2","name":"Read","input":{"file_path":"/tmp/x"}}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:06.100Z","isSidechain":false,"message":{"id":"resp-2","role":"assistant","model":"qfmodel","stop_reason":"tool_use","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"credits":0.25,"context_usage_ratio":0.55},"content":[{"type":"text","text":"读完了"}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T10:00:07.000Z","isSidechain":false,"message":{"id":"resp-3","role":"assistant","model":"qfmodel","stop_reason":"end_turn","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"credits":0.1,"context_usage_ratio":0.2},"content":[{"type":"text","text":"完成"}]}}
`

func writeQoderFixture(t *testing.T, dir string) string {
	t.Helper()
	projectDir := filepath.Join(dir, "projects", "T-tmp-qoder-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	path := filepath.Join(projectDir, "sess-q1.jsonl")
	if err := os.WriteFile(path, []byte(qoderSessionFixture), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	return path
}

func TestQoderUsageAccountingDedupsSplitRecords(t *testing.T) {
	path := writeQoderFixture(t, t.TempDir())

	agg := newProjectAggregate()
	parseProjectFileAggregate(path, TimeFilter{}, agg)
	agg.finalize()

	// 9 条 assistant 记录属于 3 次 API 响应，请求数必须是 3。
	stat := agg.CostModelStats["qfmodel"]
	if stat == nil {
		t.Fatalf("CostModelStats 缺少 qfmodel: %+v", agg.CostModelStats)
	}
	if stat.RequestCount != 3 {
		t.Fatalf("RequestCount=%d, want 3（9 条拆分记录去重成 3 次响应）", stat.RequestCount)
	}
	// resp-2 的 usage 被重复写在两条记录上，积分只能计一次。
	wantCredits := 0.85
	if stat.Credits < wantCredits-1e-9 || stat.Credits > wantCredits+1e-9 {
		t.Fatalf("Credits=%v, want %v", stat.Credits, wantCredits)
	}
	if agg.CostAnalysis == nil || agg.CostAnalysis.Totals.Credits < wantCredits-1e-9 || agg.CostAnalysis.Totals.Credits > wantCredits+1e-9 {
		t.Fatalf("Totals.Credits=%+v, want %v", agg.CostAnalysis, wantCredits)
	}
	if got := agg.CostAnalysis.Totals.RequestCount; got != 3 {
		t.Fatalf("Totals.RequestCount=%d, want 3", got)
	}
	if agg.ModelUsage["qfmodel"] == nil || agg.ModelUsage["qfmodel"].Count != 3 {
		t.Fatalf("ModelUsage=%+v, want count=3", agg.ModelUsage["qfmodel"])
	}
	if got := agg.DailyModelCounts["2026-07-03"]["qfmodel"]; got != 3 {
		t.Fatalf("DailyModelCounts=%d, want 3", got)
	}

	// 工具记账不受去重影响：两次 tool_use 都要计到。
	if bash := agg.ToolStats["Bash"]; bash == nil || bash.CallCount != 1 || bash.FailureCount != 1 {
		t.Fatalf("Bash stat=%+v, want calls=1 failure=1", bash)
	}
	if read := agg.ToolStats["Read"]; read == nil || read.CallCount != 1 {
		t.Fatalf("Read stat=%+v, want calls=1", read)
	}
}

func TestQoderContextRatioRecordsPeakOnly(t *testing.T) {
	path := writeQoderFixture(t, t.TempDir())

	agg := newProjectAggregate()
	parseProjectFileAggregate(path, TimeFilter{}, agg)
	agg.finalize()

	context := agg.ContextStats
	if context == nil {
		t.Fatalf("ContextStats 为空，积分/上下文口径没有落地")
	}
	if context.PeakContextRatio < 0.549 || context.PeakContextRatio > 0.551 {
		t.Fatalf("PeakContextRatio=%v, want 0.55", context.PeakContextRatio)
	}
	// 压缩计数只认 system/compact_boundary，不再按 ratio 回落推断（两种口径会打架）。
	if context.ContextCompactions != 0 {
		t.Fatalf("ContextCompactions=%d, want 0（比例回落不该计数）", context.ContextCompactions)
	}
}

func TestCompactBoundaryRecordedFromTranscript(t *testing.T) {
	const record = `{"type":"user","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T11:00:01.000Z","message":{"role":"user","content":[{"type":"text","text":"继续"}]}}
{"type":"system","cwd":"/tmp/qoder-project","sessionId":"sess-q1","timestamp":"2026-07-03T11:00:02.000Z","subtype":"compact_boundary","isCompactSummary":true,"compactMetadata":{"trigger":"auto","preTokens":167588,"postTokens":6122,"messagesSummarized":372,"durationMs":112331}}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sess-q2.jsonl")
	if err := os.WriteFile(path, []byte(record), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	agg := newProjectAggregate()
	parseProjectFileAggregate(path, TimeFilter{}, agg)
	agg.finalize()

	context := agg.ContextStats
	if context == nil {
		t.Fatalf("compact_boundary 没有落到 ContextStats")
	}
	if context.ContextCompactions != 1 {
		t.Fatalf("ContextCompactions=%d, want 1", context.ContextCompactions)
	}
	if context.CompactDroppedTokens != 167588-6122 {
		t.Fatalf("CompactDroppedTokens=%d, want %d", context.CompactDroppedTokens, 167588-6122)
	}
	if context.CompactDurationMs != 112331 {
		t.Fatalf("CompactDurationMs=%d, want 112331", context.CompactDurationMs)
	}
	if context.CompactMessagesSummary != 372 {
		t.Fatalf("CompactMessagesSummary=%d, want 372", context.CompactMessagesSummary)
	}
	if context.CompactMaxPreTokens != 167588 {
		t.Fatalf("CompactMaxPreTokens=%d, want 167588", context.CompactMaxPreTokens)
	}
	// 按天快照同步，时间过滤查询才不会丢压缩代价。
	snapshot := agg.DailyRuntime["2026-07-03"]
	if snapshot == nil || snapshot.ContextStats == nil || snapshot.ContextStats.ContextCompactions != 1 {
		t.Fatalf("按天快照缺少压缩计数: %+v", snapshot)
	}
}

func TestCompactBoundaryWithoutMetadataStillCounts(t *testing.T) {
	const record = `{"type":"system","cwd":"/tmp/p","sessionId":"s","timestamp":"2026-07-03T11:00:02.000Z","subtype":"compact_boundary"}`
	dir := t.TempDir()
	path := filepath.Join(dir, "sess-q3.jsonl")
	if err := os.WriteFile(path, []byte(record), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	agg := newProjectAggregate()
	parseProjectFileAggregate(path, TimeFilter{}, agg)
	if agg.ContextStats == nil || agg.ContextStats.ContextCompactions != 1 {
		t.Fatalf("缺 compactMetadata 的压缩边界仍应计一次: %+v", agg.ContextStats)
	}
	if agg.ContextStats.CompactDroppedTokens != 0 {
		t.Fatalf("元数据缺失时不该造出 token 数: %+v", agg.ContextStats)
	}
}

func TestShouldAccountResponseKeepsLegacyFallbackWithoutID(t *testing.T) {
	seen := make(map[string]bool)
	usage := MessageUsage{Credits: 1}
	if !usage.hasAccounting() {
		t.Fatalf("hasAccounting=false, want true when credits present")
	}
	// 无 message.id：无法去重，逐条记账（保持旧 provider 数据的行为）。
	for i := 0; i < 3; i++ {
		if !shouldAccountResponse("", usage, seen) {
			t.Fatalf("第 %d 次无 id 记账被拒", i+1)
		}
	}
	if len(seen) != 0 {
		t.Fatalf("无 id 不应写入去重表: %v", seen)
	}

	// 无用量拆分记录不能占用记账名额，否则会把真正带 usage 的那条挡在门外。
	if shouldAccountResponse("resp-9", MessageUsage{}, seen) {
		t.Fatalf("无用量记录不应记账")
	}
	if !shouldAccountResponse("resp-9", usage, seen) {
		t.Fatalf("同 id 的无用量记录先出现，带 usage 的那条应仍可记账")
	}
	if shouldAccountResponse("resp-9", usage, seen) {
		t.Fatalf("同 id 的重复 usage 应被去重")
	}
}

func TestQoderProviderRegistrationAndDetection(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "projects"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if looksLikeQoderDataDir(dir) {
		t.Fatalf("缺 installation_id 不应判定为 qoder")
	}
	if resolveProvider(ProviderAuto, dir) != ProviderClaude {
		t.Fatalf("只有 projects/ 的目录应仍解析为 claude")
	}
	if err := os.WriteFile(filepath.Join(dir, "installation_id"), []byte("id"), 0644); err != nil {
		t.Fatalf("write installation_id failed: %v", err)
	}
	if !looksLikeQoderDataDir(dir) {
		t.Fatalf("installation_id + projects/ 应判定为 qoder")
	}
	// 关键回归：~/.qoder 也有 projects/，qoder 判断必须早于 claude。
	if got := resolveProvider(ProviderAuto, dir); got != ProviderQoder {
		t.Fatalf("resolveProvider(auto)=%s, want qoder", got)
	}

	var provider Provider
	if err := provider.Set("qoder"); err != nil {
		t.Fatalf("provider.Set(qoder) 失败: %v", err)
	}
	if provider != ProviderQoder {
		t.Fatalf("provider=%s, want qoder", provider)
	}
	if err := provider.Set("nope"); err == nil {
		t.Fatalf("未知 provider 应报错")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir 失败: %v", err)
	}
	if got := defaultDataDirForProvider(ProviderQoder); got != filepath.Join(homeDir, ".qoder") {
		t.Fatalf("默认数据目录=%s, want ~/.qoder", got)
	}
	if providerCacheDir("/cache", ProviderQoder) != filepath.Join("/cache", "qoder") {
		t.Fatalf("qoder 缓存目录未按 provider 隔离")
	}
	for _, listed := range allMergeProviderList {
		if listed == ProviderQoder {
			return
		}
	}
	t.Fatalf("allMergeProviderList 未包含 qoder: %v", allMergeProviderList)
}

const qoderSubagentFixture = `{"type":"user","cwd":"/tmp/qoder-project","sessionId":"sess-q1","agentId":"aExplore-30c9817e20fe643f","timestamp":"2026-07-03T10:10:01.000Z","isSidechain":true,"message":{"role":"user","content":[{"type":"text","text":"找预测集成点"}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","agentId":"aExplore-30c9817e20fe643f","timestamp":"2026-07-03T10:10:02.000Z","isSidechain":true,"message":{"id":"sub-1","role":"assistant","model":"smodel","stop_reason":"tool_use","usage":{"input_tokens":0,"output_tokens":0,"credits":0.2,"context_usage_ratio":0.1},"content":[{"type":"tool_use","id":"sc-1","name":"Grep","input":{"pattern":"predict"}}]}}
{"type":"user","cwd":"/tmp/qoder-project","sessionId":"sess-q1","agentId":"aExplore-30c9817e20fe643f","timestamp":"2026-07-03T10:10:03.000Z","isSidechain":true,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"sc-1","is_error":false,"content":"flows/predict.py:12"}]}}
{"type":"assistant","cwd":"/tmp/qoder-project","sessionId":"sess-q1","agentId":"aExplore-30c9817e20fe643f","timestamp":"2026-07-03T10:10:04.000Z","isSidechain":true,"message":{"id":"sub-2","role":"assistant","model":"smodel","stop_reason":"end_turn","usage":{"input_tokens":0,"output_tokens":0,"credits":0.3,"context_usage_ratio":0.2},"content":[{"type":"text","text":"找到了"}]}}
`

// writeQoderSubagentFile 造出 Qoder 的真实布局：
// projects/<slug>/<parentSession>/subagents/agent-<agentID>.jsonl（+ 可选 meta.json）。
func writeQoderSubagentFile(t *testing.T, dir, agentID string, withMeta bool) string {
	t.Helper()
	subDir := filepath.Join(dir, "projects", "T-tmp-qoder-project", "sess-q1", "subagents")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	path := filepath.Join(subDir, "agent-"+agentID+".jsonl")
	if err := os.WriteFile(path, []byte(qoderSubagentFixture), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if withMeta {
		meta := `{"agentType":"Explore","toolUseId":"fc-1","description":"Find prediction integration points","invocationName":"Explore"}`
		if err := os.WriteFile(strings.TrimSuffix(path, ".jsonl")+".meta.json", []byte(meta), 0644); err != nil {
			t.Fatalf("write meta failed: %v", err)
		}
	}
	return path
}

func TestQoderSubagentNamePrefersMetaJSON(t *testing.T) {
	path := writeQoderSubagentFile(t, t.TempDir(), "aExplore-30c9817e20fe643f", true)
	if got := qoderSubagentName(path); got != "Explore" {
		t.Fatalf("name=%q, want Explore", got)
	}
}

func TestQoderSubagentNameFallsBackToFileName(t *testing.T) {
	dir := t.TempDir()
	// meta.json 缺失（旧版本或写入中断）时退回文件名推导：只切末段 hash，再去掉前缀 a。
	path := writeQoderSubagentFile(t, dir, "ageneral-purpose-3260704d90fe3f9e", false)
	if got := qoderSubagentName(path); got != "general-purpose" {
		t.Fatalf("name=%q, want general-purpose", got)
	}
	if got := qoderAgentTypeFromID("aExplore-30c9817e20fe643f"); got != "Explore" {
		t.Fatalf("qoderAgentTypeFromID=%q, want Explore", got)
	}
}

func TestQoderSubagentNameIgnoresParentSessionFile(t *testing.T) {
	path := writeQoderFixture(t, t.TempDir())
	if got := qoderSubagentName(path); got != "" {
		t.Fatalf("父会话文件不应推导出子代理名字: %q", got)
	}
}

func TestParseQoderSessionFileNamesSubagentAgent(t *testing.T) {
	path := writeQoderSubagentFile(t, t.TempDir(), "aExplore-30c9817e20fe643f", true)

	agg := newProjectAggregate()
	parseQoderSessionFile(path, TimeFilter{}, agg)
	agg.finalize()

	// agentId 由记录自带（ProjectRecord 的 tag 就是 camelCase agentId），
	// 这里补的是可读名字，不能改动桶的划分。
	const agentID = "aExplore-30c9817e20fe643f"
	agent := agg.AgentStats[agentID]
	if agent == nil {
		t.Fatalf("子代理没有按 agentId 归因，AgentStats=%+v", agg.AgentStats)
	}
	if agent.AgentName != "Explore" {
		t.Fatalf("AgentName=%q, want Explore", agent.AgentName)
	}
	if !agent.IsSidechain {
		t.Fatalf("AgentName=%q IsSidechain=false, want true", agent.AgentName)
	}
	if agent.ToolCallCount != 1 {
		t.Fatalf("ToolCallCount=%d, want 1", agent.ToolCallCount)
	}
	// 时间过滤查询会从按天快照重建 agent 分析，名字必须同步落到快照里，
	// 否则 -p 7d 与 all 会给出不同的 agent 展示。
	snapshot := agg.DailySessionRuntime["2026-07-03"]["sess-q1"]
	if snapshot == nil {
		t.Fatalf("缺少按 session 的按天快照: %+v", agg.DailySessionRuntime)
	}
	if stat := snapshot.AgentStats[agentID]; stat == nil || stat.AgentName != "Explore" {
		t.Fatalf("按天快照里的 agent 名字缺失: %+v", stat)
	}

	// 子代理自带模型（smodel）与积分，都要归到该 agent 身上。
	cost := agg.CostAgentStats[agentID]
	if cost == nil || cost.RequestCount != 2 {
		t.Fatalf("CostAgentStats=%+v, want %s request=2", agg.CostAgentStats, agentID)
	}
	if want := 0.5; cost.Credits < want-1e-9 || cost.Credits > want+1e-9 {
		t.Fatalf("Credits=%v, want %v", cost.Credits, want)
	}
	// 子代理记录带父 sessionId，会话归属不能被拆开。
	if item := agg.SessionStatsMap["sess-q1"]; item == nil {
		t.Fatalf("子代理记录没有并入父会话: %+v", agg.SessionStatsMap)
	}
}
