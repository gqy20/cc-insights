package main

import (
	"os"
	"path/filepath"
	"testing"
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
	} else if model.InputTokens != 20 || model.OutputTokens != 10 || model.ReasoningOutputTokens != 12 {
		t.Fatalf("ModelUsage token breakdown = %+v, want input=20 output=10 reasoning=12", model)
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
{"timestamp":"2026-07-03T10:00:03Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-success","arguments":"{\"cmd\":\"go test ./...\"}"}}
{"timestamp":"2026-07-03T10:00:04Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-success","output":"Process exited with code 1"}}
{"timestamp":"2026-07-03T10:00:05Z","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-success","command":"go test ./...","status":"completed","exit_code":0,"stdout":"ok","stderr":"","duration":2000}}
{"timestamp":"2026-07-03T10:00:06Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-fail","arguments":"{\"cmd\":\"false\"}"}}
{"timestamp":"2026-07-03T10:00:07Z","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-fail","command":"false","status":"failed","exit_code":1,"stdout":"","stderr":"boom","duration":100}}
{"timestamp":"2026-07-03T10:00:08Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-missing","arguments":"{\"cmd\":\"pwd\"}"}}
{"timestamp":"2026-07-03T10:00:09Z","type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","call_id":"call-patch","input":"*** Begin Patch"}}
{"timestamp":"2026-07-03T10:00:10Z","type":"event_msg","payload":{"type":"patch_apply_end","call_id":"call-patch","success":true,"status":"success","stdout":"done","stderr":""}}
{"timestamp":"2026-07-03T10:00:11Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":20,"output_tokens":10,"reasoning_output_tokens":12,"total_tokens":42},"total_token_usage":{"total_tokens":999}}}}
`
	if err := os.WriteFile(sessionPath, []byte(content), 0644); err != nil {
		t.Fatalf("Write session fixture failed: %v", err)
	}
	return sessionPath
}
