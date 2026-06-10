package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseProjectsConcurrentOnce_FailureAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "failure-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := toolUseRecordWithInput("/tmp/failure-project", "session-failures", base, "bash-exit", "Bash", "claude-sonnet-4.5", false, "", `{"command":"npm test"}`) + "\n" +
		toolResultRecord("/tmp/failure-project", "session-failures", base.Add(time.Second), "bash-exit", "Command failed with exit code 127: npm test") + "\n" +
		toolUseRecordWithInput("/tmp/failure-project", "session-failures", base.Add(2*time.Second), "edit-mismatch", "Edit", "claude-sonnet-4.5", false, "", `{"file_path":"/tmp/failure-project/main.go","old_string":"missing","new_string":"fixed"}`) + "\n" +
		toolResultRecord("/tmp/failure-project", "session-failures", base.Add(3*time.Second), "edit-mismatch", "Error: old_string not found in file") + "\n" +
		toolUseRecordWithInput("/tmp/failure-project", "session-failures", base.Add(4*time.Second), "mcp-rate", "mcp__server__search", "glm-5v-turbo", false, "", `{}`) + "\n" +
		toolResultRecord("/tmp/failure-project", "session-failures", base.Add(5*time.Second), "mcp-rate", "Error: HTTP 429 too many requests rate limit") + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}
	if agg.FailureAnalysis == nil {
		t.Fatal("FailureAnalysis should not be nil")
	}
	if agg.FailureAnalysis.TotalFailures != 3 {
		t.Fatalf("TotalFailures=%d, want 3", agg.FailureAnalysis.TotalFailures)
	}

	reasons := make(map[string]int)
	for _, item := range agg.FailureAnalysis.ByReason {
		reasons[item.Category+"/"+item.Reason] = item.Count
	}
	for _, key := range []string{"bash/exit_code_127", "edit/old_string_mismatch", "rate_limit/rate_limit_429"} {
		if reasons[key] != 1 {
			t.Fatalf("reason %q count=%d, want 1; all=%+v", key, reasons[key], agg.FailureAnalysis.ByReason)
		}
	}

	toolReasons := make(map[string]FailureToolReasonStat)
	for _, item := range agg.FailureAnalysis.ByToolReason {
		toolReasons[item.Tool+"/"+item.Category+"/"+item.Reason] = item
	}
	if toolReasons["Bash/bash/exit_code_127"].Count != 1 {
		t.Fatalf("Bash exit_code_127 stat=%+v, want count 1", toolReasons["Bash/bash/exit_code_127"])
	}
	if toolReasons["Edit/edit/old_string_mismatch"].Count != 1 {
		t.Fatalf("Edit old_string_mismatch stat=%+v, want count 1", toolReasons["Edit/edit/old_string_mismatch"])
	}
	if toolReasons["mcp__server__search/rate_limit/rate_limit_429"].Count != 1 {
		t.Fatalf("MCP rate_limit stat=%+v, want count 1", toolReasons["mcp__server__search/rate_limit/rate_limit_429"])
	}

	modelReasons := make(map[string]int)
	for _, item := range agg.FailureAnalysis.ByModelReason {
		modelReasons[item.Model+"/"+item.Category+"/"+item.Reason] = item.Count
	}
	if modelReasons["claude-sonnet-4.5/bash/exit_code_127"] != 1 {
		t.Fatalf("model reason map=%+v, want claude bash exit_code_127", modelReasons)
	}
	if modelReasons["glm-5v-turbo/rate_limit/rate_limit_429"] != 1 {
		t.Fatalf("model reason map=%+v, want glm rate limit", modelReasons)
	}

	if len(agg.FailureAnalysis.Samples) != 3 {
		t.Fatalf("samples=%d, want 3", len(agg.FailureAnalysis.Samples))
	}
	if agg.FailureAnalysis.Samples[0].Category == "" || agg.FailureAnalysis.Samples[0].Reason == "" {
		t.Fatalf("sample missing category/reason: %+v", agg.FailureAnalysis.Samples[0])
	}
}
