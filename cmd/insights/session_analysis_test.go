package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestParseProjectsConcurrentOnce_SessionAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "session-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 11, 0, 0, 0, time.UTC)
	content := modeRecord("/tmp/session-project", "session-life", base, "normal") + "\n" +
		permissionModeRecord("/tmp/session-project", "session-life", base.Add(time.Second), "bypassPermissions") + "\n" +
		queueOperationRecord("session-life", base.Add(2*time.Second), "enqueue", "run tests and fix failures") + "\n" +
		titleRecord("ai-title", "session-life", "Investigate failing tests") + "\n" +
		attachmentRecord("/tmp/session-project", "session-life", base.Add(3*time.Second), `{"type":"plan_mode"}`) + "\n" +
		toolUseRecordWithUsage("/tmp/session-project", "session-life", base.Add(4*time.Second), "call-fail", "Bash", "claude-sonnet-4.5", `{"command":"go test ./..."}`, 10, 4) + "\n" +
		toolResultRecord("/tmp/session-project", "session-life", base.Add(5*time.Second), "call-fail", "Command failed with exit code 1") + "\n" +
		lastPromptRecord("/tmp/session-project", "session-life", base.Add(6*time.Second), "请运行测试并修复失败") + "\n" +
		systemTurnDurationRecord("/tmp/session-project", "session-life", base.Add(7*time.Second), 7000, 5) + "\n" +
		systemStopHookRecord("/tmp/session-project", "session-life", base.Add(8*time.Second), 1, 1, true, "hook error") + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}
	if agg.SessionAnalysis == nil {
		t.Fatal("SessionAnalysis should not be nil")
	}
	if len(agg.SessionAnalysis.Sessions) != 1 {
		t.Fatalf("sessions=%+v, want one session", agg.SessionAnalysis.Sessions)
	}
	session := agg.SessionAnalysis.Sessions[0]
	if session.SessionID != "session-life" {
		t.Fatalf("SessionID=%q, want session-life", session.SessionID)
	}
	if session.Title != "Investigate failing tests" || session.TitleSource != "ai-title" {
		t.Fatalf("title=(%q,%q), want ai-title", session.Title, session.TitleSource)
	}
	if session.LastPromptPreview == "" {
		t.Fatal("LastPromptPreview should not be empty")
	}
	if session.ToolCallCount != 1 || session.ToolFailureCount != 1 {
		t.Fatalf("tool counts=(%d,%d), want (1,1)", session.ToolCallCount, session.ToolFailureCount)
	}
	if session.TotalTokens != 14 || session.InputTokens != 10 || session.OutputTokens != 4 {
		t.Fatalf("token stats=(%d,%d,%d), want (14,10,4)", session.TotalTokens, session.InputTokens, session.OutputTokens)
	}
	if session.PermissionModeChanges != 1 || session.LastPermissionMode != "bypassPermissions" {
		t.Fatalf("permission stats=(%d,%q), want bypassPermissions", session.PermissionModeChanges, session.LastPermissionMode)
	}
	if session.PlanModeCount != 1 || session.QueueOperationCount != 1 {
		t.Fatalf("plan/queue=(%d,%d), want (1,1)", session.PlanModeCount, session.QueueOperationCount)
	}
	if session.HookCount != 1 || session.HookErrorCount != 1 || !session.PreventedContinuation {
		t.Fatalf("hook stats=(%d,%d,%v), want (1,1,true)", session.HookCount, session.HookErrorCount, session.PreventedContinuation)
	}
	if session.Outcome != "interrupted" {
		t.Fatalf("Outcome=%q, want interrupted", session.Outcome)
	}
	if len(agg.SessionAnalysis.TopFailures) != 1 || agg.SessionAnalysis.TopFailures[0].SessionID != "session-life" {
		t.Fatalf("TopFailures=%+v, want session-life", agg.SessionAnalysis.TopFailures)
	}
	if len(agg.SessionAnalysis.QueueOperations) != 1 || agg.SessionAnalysis.QueueOperations[0].Operation != "enqueue" {
		t.Fatalf("QueueOperations=%+v, want enqueue", agg.SessionAnalysis.QueueOperations)
	}
}

func modeRecord(cwd, sessionID string, ts time.Time, mode string) string {
	return `{"type":"mode","mode":"` + mode + `","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `"}`
}

func queueOperationRecord(sessionID string, ts time.Time, operation, content string) string {
	return `{"type":"queue-operation","operation":"` + operation + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `","sessionId":"` + sessionID + `","content":"` + content + `"}`
}

func titleRecord(recordType, sessionID, title string) string {
	field := "aiTitle"
	if recordType == "custom-title" {
		field = "customTitle"
	}
	return `{"type":"` + recordType + `","` + field + `":"` + title + `","sessionId":"` + sessionID + `"}`
}

func lastPromptRecord(cwd, sessionID string, ts time.Time, prompt string) string {
	return `{"type":"last-prompt","lastPrompt":"` + prompt + `","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `"}`
}

func systemTurnDurationRecord(cwd, sessionID string, ts time.Time, durationMs, messageCount int) string {
	return `{"type":"system","subtype":"turn_duration","durationMs":` + strconv.Itoa(durationMs) + `,"messageCount":` + strconv.Itoa(messageCount) + `,"cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `"}`
}

func systemStopHookRecord(cwd, sessionID string, ts time.Time, hookCount, hookErrors int, prevented bool, stopReason string) string {
	preventedValue := "false"
	if prevented {
		preventedValue = "true"
	}
	errors := "[]"
	if hookErrors > 0 {
		errors = `[{"message":"failed"}]`
	}
	return `{"type":"system","subtype":"stop_hook_summary","hookCount":` + strconv.Itoa(hookCount) + `,"hookErrors":` + errors + `,"preventedContinuation":` + preventedValue + `,"stopReason":"` + stopReason + `","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `"}`
}

func toolUseRecordWithUsage(cwd, sessionID string, ts time.Time, callID, tool, model, input string, inputTokens, outputTokens int) string {
	return `{"type":"assistant","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `","message":{"model":"` + model + `","content":[{"type":"tool_use","id":"` + callID + `","name":"` + tool + `","input":` + input + `}],"usage":{"input_tokens":` + strconv.Itoa(inputTokens) + `,"output_tokens":` + strconv.Itoa(outputTokens) + `}}}`
}
