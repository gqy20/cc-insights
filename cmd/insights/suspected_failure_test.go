package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// 通用文本命中失败关键词应降级为疑似失败，不计入失败率。
func TestClassifyGenericErrorTextIsSuspected(t *testing.T) {
	classification := classifyToolResult("WebFetch", AssistantContent{
		Type:    "tool_result",
		Content: json.RawMessage(strconv.Quote("page content mentions exception and failed retries")),
	})
	if !classification.Failed || !classification.Suspected {
		t.Fatalf("classification=%+v, want suspected failure", classification)
	}
	if classification.Reason != "suspect_error_text" {
		t.Fatalf("reason=%q, want suspect_error_text", classification.Reason)
	}
	if classification.Basis != "regex_text" {
		t.Fatalf("basis=%q, want regex_text", classification.Basis)
	}
}

// 结构化确认的失败（is_error）保持确认分类，不带 suspect_ 前缀。
func TestClassifyExplicitErrorStaysConfirmed(t *testing.T) {
	classification := classifyToolResult("WebFetch", AssistantContent{
		Type:    "tool_result",
		IsError: true,
		Content: json.RawMessage(strconv.Quote("fetch failed")),
	})
	if !classification.Failed || classification.Suspected {
		t.Fatalf("classification=%+v, want confirmed failure", classification)
	}
	if isSuspectedFailureReason(classification.Reason) {
		t.Fatalf("reason=%q should not be suspected", classification.Reason)
	}
}

// 疑似失败按成功记账，但保留 suspect_ 原因统计与带判定依据的样例。
func TestSuspectedFailureNotCountedInFailureRate(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "suspect-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := toolUseRecordWithInput("/tmp/suspect-project", "session-suspect", base, "fetch-1", "WebFetch", "claude-sonnet-4.5", false, "", `{"url":"https://example.com/logs"}`) + "\n" +
		toolResultRecord("/tmp/suspect-project", "session-suspect", base.Add(time.Second), "fetch-1", "page content mentions exception and failed retries") + "\n"

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
		t.Fatalf("TotalFailures=%d, want 0; byReason=%+v", agg.FailureAnalysis.TotalFailures, agg.FailureAnalysis.ByReason)
	}
	if agg.FailureAnalysis.TotalSuspected != 1 {
		t.Fatalf("TotalSuspected=%d, want 1; byReason=%+v", agg.FailureAnalysis.TotalSuspected, agg.FailureAnalysis.ByReason)
	}
	if stat := agg.ToolStats["WebFetch"]; stat == nil || stat.FailureCount != 0 || stat.SuccessCount != 1 {
		t.Fatalf("WebFetch stat=%+v, want success 1 / failure 0", stat)
	}
	foundSuspectSample := false
	for _, sample := range agg.FailureAnalysis.Samples {
		if sample.Reason == "suspect_error_text" {
			foundSuspectSample = true
			if sample.Basis != "regex_text" {
				t.Fatalf("sample basis=%q, want regex_text", sample.Basis)
			}
		}
	}
	if !foundSuspectSample {
		t.Fatalf("no suspect_error_text sample in %+v", agg.FailureAnalysis.Samples)
	}
}

// Codex 弱结果文本命中失败关键词应降级为疑似失败。
func TestCodexWeakClassificationIsSuspected(t *testing.T) {
	pending := &codexPendingTool{
		pendingToolCall: pendingToolCall{Tool: "shell"},
		WeakFailed:      true,
		WeakPreview:     "output mentions error and failed",
	}
	classification := codexWeakClassification(pending)
	if !classification.Suspected {
		t.Fatalf("classification=%+v, want suspected", classification)
	}
	if classification.Reason != "suspect_error_text" || classification.Basis != "regex_text" {
		t.Fatalf("classification=%+v, want suspect_error_text/regex_text", classification)
	}

	exitPending := &codexPendingTool{
		pendingToolCall: pendingToolCall{Tool: "shell"},
		WeakFailed:      true,
		WeakPreview:     "process exited with code 1",
	}
	if c := codexWeakClassification(exitPending); c.Basis != "exit_code_text" {
		t.Fatalf("basis=%q, want exit_code_text", c.Basis)
	}
}

// Codex MCP 结果：结构化失败字段保持确认，裸 "error" 子串降级为疑似。
func TestCodexMCPResultErrorSplit(t *testing.T) {
	if !codexResultExplicitError(json.RawMessage(`{"is_error":true}`)) {
		t.Fatal("is_error:true should be explicit error")
	}
	if !codexResultExplicitError(json.RawMessage(`{"success":false}`)) {
		t.Fatal("success:false should be explicit error")
	}
	if codexResultExplicitError(json.RawMessage(`{"error":null}`)) {
		t.Fatal("bare error field should not be explicit error")
	}
	if !codexResultHasError(json.RawMessage(`{"error":"boom"}`)) {
		t.Fatal("bare error substring should still be weak-detected")
	}
}

// finalizeFailureAnalysis 应把 suspect_ 原因计入 TotalSuspected 而非 TotalFailures。
func TestFinalizeFailureAnalysisSeparatesSuspected(t *testing.T) {
	agg := newProjectAggregate()
	call := pendingToolCall{Tool: "WebFetch", Model: "m", Project: "p", SessionID: "s", Timestamp: time.Now()}
	addToolResultLocked(agg, call, newSuspectedFailureClassification("error_text", "tool", "error_text", "regex_text"), "preview", time.Now(), 0, 7)
	confirmed := pendingToolCall{Tool: "Bash", Model: "m", Project: "p", SessionID: "s", Timestamp: time.Now()}
	addToolResultLocked(agg, confirmed, newFailureClassification("command_failed", "bash", "exit_code_1"), "preview", time.Now(), 0, 7)

	agg.finalizeFailureAnalysis()
	if agg.FailureAnalysis.TotalFailures != 1 {
		t.Fatalf("TotalFailures=%d, want 1", agg.FailureAnalysis.TotalFailures)
	}
	if agg.FailureAnalysis.TotalSuspected != 1 {
		t.Fatalf("TotalSuspected=%d, want 1", agg.FailureAnalysis.TotalSuspected)
	}
}
