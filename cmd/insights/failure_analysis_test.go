package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestReadToolResultTextDoesNotCreateFailure(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "read-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := toolUseRecordWithInput("/tmp/read-project", "session-read", base, "read-ok", "Read", "claude-sonnet-4.5", false, "", `{"file_path":"README.md"}`) + "\n" +
		toolResultRecord("/tmp/read-project", "session-read", base.Add(time.Second), "read-ok", "This document mentions error handling, failed retries, and permission denied examples.") + "\n"

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
	if agg.FailureAnalysis.TotalFailures != 0 {
		t.Fatalf("TotalFailures=%d, want 0; failures=%+v", agg.FailureAnalysis.TotalFailures, agg.FailureAnalysis.ByReason)
	}
}

func TestBashReportTextDoesNotCreateFailure(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "bash-report-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := toolUseRecordWithInput("/tmp/bash-report-project", "session-report", base, "bash-report", "Bash", "claude-sonnet-4.5", false, "", `{"command":"python report.py"}`) + "\n" +
		toolResultRecord("/tmp/bash-report-project", "session-report", base.Add(time.Second), "bash-report", "整体: 10篇 成功7 失败3 成本$14.12 error_text coverage report") + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}
	if agg.FailureAnalysis.TotalFailures != 0 {
		t.Fatalf("TotalFailures=%d, want 0; failures=%+v", agg.FailureAnalysis.TotalFailures, agg.FailureAnalysis.ByReason)
	}
}

func TestBashStrongErrorTextStillCreatesFailure(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "bash-error-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := toolUseRecordWithInput("/tmp/bash-error-project", "session-error", base, "bash-error", "Bash", "claude-sonnet-4.5", false, "", `{"command":"node broken.js"}`) + "\n" +
		toolResultRecord("/tmp/bash-error-project", "session-error", base.Add(time.Second), "bash-error", "SyntaxError: Unexpected token ';'") + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}
	if agg.FailureAnalysis.TotalFailures != 1 {
		t.Fatalf("TotalFailures=%d, want 1", agg.FailureAnalysis.TotalFailures)
	}
	reason := agg.FailureAnalysis.ByReason[0]
	if reason.Category != "bash" || reason.Reason != "error_text" {
		t.Fatalf("reason=%+v, want bash/error_text", reason)
	}
}

func TestFailureSamplesKeepBeyondFirstThirty(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "many-failures")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	lines := make([]string, 0, 80)
	for i := 0; i < 40; i++ {
		callID := "bash-fail-" + formatInt(i)
		lines = append(lines,
			toolUseRecordWithInput("/tmp/many-failures", "session-many", base.Add(time.Duration(i*2)*time.Second), callID, "Bash", "claude-sonnet-4.5", false, "", `{"command":"npm test"}`),
			toolResultRecord("/tmp/many-failures", "session-many", base.Add(time.Duration(i*2+1)*time.Second), callID, "Command failed with exit code 1: npm test"),
		)
	}

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}
	if got := len(agg.FailureAnalysis.Samples); got != 40 {
		t.Fatalf("samples=%d, want 40", got)
	}
	report := buildCLIInspectFailuresReport(&DashboardData{
		TimeRange:       TimeRangeInfo{Preset: "all"},
		FailureAnalysis: agg.FailureAnalysis,
	}, cliOptions{Category: "bash", Reason: "exit_code_1", Tool: "Bash", Limit: 5, Samples: 5})
	if report.Summary.MatchedSamples != 40 {
		t.Fatalf("MatchedSamples=%d, want 40", report.Summary.MatchedSamples)
	}
	if len(report.Samples) != 5 {
		t.Fatalf("Samples=%d, want 5", len(report.Samples))
	}
}
