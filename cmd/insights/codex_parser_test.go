package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseCodexSessionFileAggregate(t *testing.T) {
	sessionPath := writeCodexSessionFixture(t, t.TempDir())

	agg := newProjectAggregate()
	parseCodexSessionFileAggregate(sessionPath, TimeFilter{}, agg)
	agg.finalize()

	if got := agg.DailyActivity["2026-07-03"]; got != 1 {
		t.Fatalf("DailyActivity = %d, want 1", got)
	}
	if got := agg.DailySessions["2026-07-03"]["sess-codex"]; !got {
		t.Fatalf("DailySessions missing sess-codex: %+v", agg.DailySessions)
	}
	if stat := agg.ProjectStats["/tmp/codex-project"]; stat == nil || stat.MessageCount != 1 {
		t.Fatalf("ProjectStats = %+v, want one assistant message", stat)
	}
	if model := agg.ModelUsage["gpt-5.5"]; model == nil || model.Count != 1 || model.Tokens != 42 {
		t.Fatalf("ModelUsage[gpt-5.5] = %+v, want count=1 tokens=42", model)
	} else if model.InputTokens != 20 || model.CachedInputTokens != 8 || model.OutputTokens != 10 || model.ReasoningOutputTokens != 12 {
		t.Fatalf("ModelUsage token breakdown = %+v, want input=20 cached=8 output=10 reasoning=12", model)
	}
	if agg.CostAnalysis == nil || len(agg.CostAnalysis.ByModel) == 0 {
		t.Fatalf("CostAnalysis missing: %+v", agg.CostAnalysis)
	}
	cost := agg.CostAnalysis.ByModel[0]
	if cost.CostUSD < 0.000723 || cost.CostUSD > 0.000725 {
		t.Fatalf("Codex CostUSD = %.9f, want about 0.000724", cost.CostUSD)
	}
	if cost.InputCostUSD < 0.000059 || cost.InputCostUSD > 0.000061 {
		t.Fatalf("Codex InputCostUSD = %.9f, want uncached input only", cost.InputCostUSD)
	}

	execStat := agg.ToolStats["exec_command"]
	if execStat == nil {
		t.Fatal("missing exec_command tool stat")
	}
	if execStat.CallCount != 3 || execStat.SuccessCount != 1 || execStat.FailureCount != 1 || execStat.MissingResultCount != 1 {
		t.Fatalf("exec_command stat = %+v, want calls=3 success=1 failure=1 missing=1", execStat)
	}
	if patchStat := agg.ToolStats["apply_patch"]; patchStat == nil || patchStat.SuccessCount != 1 {
		t.Fatalf("apply_patch stat = %+v, want success=1", patchStat)
	}
	if cmd := agg.BashCommandStats["go"]; cmd == nil || cmd.CallCount != 1 || cmd.SuccessCount != 1 || cmd.FailureCount != 0 {
		t.Fatalf("go command stat = %+v, want one successful call", cmd)
	}
	if cmd := agg.BashCommandStats["false"]; cmd == nil || cmd.FailureCount != 1 {
		t.Fatalf("false command stat = %+v, want one failed call", cmd)
	}
	if agg.TaskPlanAnalysis == nil || agg.TaskPlanAnalysis.PlanLifecycle.EntryCount != 1 || agg.TaskPlanAnalysis.PlanLifecycle.ExitCount != 1 {
		t.Fatalf("TaskPlanAnalysis lifecycle = %+v, want one codex update_plan entry and task_complete exit", agg.TaskPlanAnalysis)
	}
	if agg.EventAnalysis == nil || agg.EventAnalysis.PlanModeCount != 1 || agg.EventAnalysis.PlanModeExitCount != 1 {
		t.Fatalf("EventAnalysis plan counts = %+v, want one in/out", agg.EventAnalysis)
	}
	if agg.ToolPerformance == nil || agg.ToolPerformance.TurnCount != 1 || agg.ToolPerformance.MaxTurnDurationMs != 12345 {
		t.Fatalf("ToolPerformance turn stats = %+v, want one 12345ms codex task_complete", agg.ToolPerformance)
	}
	if agg.FileAnalysis == nil || agg.FileAnalysis.Totals.TotalEdits != 1 || agg.FileAnalysis.Totals.TotalWrites != 1 {
		t.Fatalf("FileAnalysis totals = %+v, want one edit and one write from patch changes", agg.FileAnalysis)
	}
}

func TestBuildFullCacheCodexProvider(t *testing.T) {
	dataDir := t.TempDir()
	writeCodexSessionFixture(t, dataDir)
	cachePath := filepath.Join(t.TempDir(), "cache.db")

	builder := &CacheBuilder{CachePath: cachePath, DataDir: dataDir, Provider: ProviderCodex}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("BuildFullCache() failed: %v", err)
	}
	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("LoadCacheFile() failed: %v", err)
	}
	if cache.Provider != string(ProviderCodex) {
		t.Fatalf("Provider = %q, want %q", cache.Provider, ProviderCodex)
	}
	if cache.TotalMessages != 1 {
		t.Fatalf("TotalMessages = %d, want 1", cache.TotalMessages)
	}
	if stat := cache.ToolStats["exec_command"]; stat == nil || stat.CallCount != 3 {
		t.Fatalf("cache exec_command stat = %+v, want 3 calls", stat)
	}
	filtered := cache.QueryByTimeRange(
		time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC),
	)
	if filtered.TaskPlanAnalysis == nil || filtered.TaskPlanAnalysis.PlanLifecycle.SessionsWithPlan != 1 {
		t.Fatalf("filtered plan sessions = %+v, want one session preserved through cache snapshot", filtered.TaskPlanAnalysis)
	}
}

func writeCodexSessionFixture(t *testing.T, dataDir string) string {
	t.Helper()
	sessionPath := filepath.Join(dataDir, "sessions", "2026", "07", "03", "rollout-test.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		t.Fatalf("Create sessions dir failed: %v", err)
	}
	content := `{"timestamp":"2026-07-03T10:00:00Z","type":"session_meta","payload":{"id":"sess-codex","timestamp":"2026-07-03T10:00:00Z","cwd":"/tmp/codex-project"}}
{"timestamp":"2026-07-03T10:00:01Z","type":"turn_context","payload":{"cwd":"/tmp/codex-project","model":"gpt-5.5"}}
{"timestamp":"2026-07-03T10:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ready"}]}}
{"timestamp":"2026-07-03T10:00:02.500Z","type":"response_item","payload":{"type":"function_call","name":"update_plan","call_id":"call-plan","arguments":"{\"plan\":[{\"step\":\"Inspect\",\"status\":\"in_progress\"}]}"}}
{"timestamp":"2026-07-03T10:00:02.600Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-plan","output":"ok"}}
{"timestamp":"2026-07-03T10:00:03Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-success","arguments":"{\"cmd\":\"go test ./...\"}"}}
{"timestamp":"2026-07-03T10:00:04Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-success","output":"Process exited with code 1"}}
{"timestamp":"2026-07-03T10:00:05Z","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-success","command":"go test ./...","status":"completed","exit_code":0,"stdout":"ok","stderr":"","duration":2000}}
{"timestamp":"2026-07-03T10:00:06Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-fail","arguments":"{\"cmd\":\"false\"}"}}
{"timestamp":"2026-07-03T10:00:07Z","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-fail","command":"false","status":"failed","exit_code":1,"stdout":"","stderr":"boom","duration":100}}
{"timestamp":"2026-07-03T10:00:08Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-missing","arguments":"{\"cmd\":\"pwd\"}"}}
{"timestamp":"2026-07-03T10:00:09Z","type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","call_id":"call-patch","input":"*** Begin Patch"}}
{"timestamp":"2026-07-03T10:00:10Z","type":"event_msg","payload":{"type":"patch_apply_end","call_id":"call-patch","success":true,"status":"success","stdout":"done","stderr":"","changes":{"/tmp/codex-project/main.go":{"type":"update","unified_diff":"@@ -1 +1 @@\n-old\n+new\n"},"/tmp/codex-project/new.go":{"type":"add","content":"package main\n"}}}}
{"timestamp":"2026-07-03T10:00:11Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"cached_input_tokens":8,"output_tokens":10,"reasoning_output_tokens":12,"total_tokens":42},"total_token_usage":{"total_tokens":999}}}}
{"timestamp":"2026-07-03T10:00:12Z","type":"event_msg","payload":{"type":"task_complete","duration_ms":12345}}
`
	if err := os.WriteFile(sessionPath, []byte(content), 0644); err != nil {
		t.Fatalf("Write session fixture failed: %v", err)
	}
	return sessionPath
}
