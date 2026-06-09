package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseProjectsConcurrentOnce_ToolAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "tool-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	content := attachmentRecord("/tmp/tool-project", "session-tools", base.Add(-4*time.Second), `{"type":"invoked_skills","skills":[{"name":"playwright-cli","path":"userSettings:playwright-cli"}]}`) + "\n" +
		attachmentRecord("/tmp/tool-project", "session-tools", base.Add(-3*time.Second), `{"type":"hook_non_blocking_error","hookName":"Stop","hookEvent":"Stop","stderr":"Hook evaluator API error","exitCode":1,"command":"bash ~/.claude/notify.sh","durationMs":1508}`) + "\n" +
		attachmentRecord("/tmp/tool-project", "session-tools", base.Add(-2*time.Second), `{"type":"plan_mode","reminderType":"full","isSubAgent":false}`) + "\n" +
		attachmentRecord("/tmp/tool-project", "session-tools", base.Add(-1*time.Second), `{"type":"opened_file_in_ide","filename":"/tmp/tool-project/main.go"}`) + "\n" +
		permissionModeRecord("/tmp/tool-project", "session-tools", base.Add(-500*time.Millisecond), "acceptEdits") + "\n" +
		toolUseRecordWithInput("/tmp/tool-project", "session-tools", base, "call-ok", "Bash", "claude-sonnet-4.5", false, "", `{"command":"sudo rm -rf /tmp/demo","description":"cleanup"}`) + "\n" +
		toolResultRecord("/tmp/tool-project", "session-tools", base.Add(time.Second), "call-ok", "command completed") + "\n" +
		toolUseRecordWithInput("/tmp/tool-project", "session-tools", base.Add(2*time.Second), "call-fail", "Read", "glm-5v-turbo", true, "agent-reader", `{"file_path":"/tmp/tool-project/missing.go"}`) + "\n" +
		toolResultRecord("/tmp/tool-project", "session-tools", base.Add(3*time.Second), "call-fail", "Error: no such file or directory") + "\n" +
		toolUseRecordWithModel("/tmp/tool-project", "session-tools", base.Add(4*time.Second), "call-missing", "Edit", "claude-sonnet-4.5") + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}

	if agg.ToolAnalysis == nil {
		t.Fatal("ToolAnalysis should not be nil")
	}
	if agg.ToolAnalysis.TotalCalls != 3 {
		t.Fatalf("TotalCalls=%d, want 3", agg.ToolAnalysis.TotalCalls)
	}
	if agg.ToolAnalysis.TotalFailures != 1 {
		t.Fatalf("TotalFailures=%d, want 1", agg.ToolAnalysis.TotalFailures)
	}
	if agg.ToolAnalysis.MissingResults != 1 {
		t.Fatalf("MissingResults=%d, want 1", agg.ToolAnalysis.MissingResults)
	}

	stats := make(map[string]ToolStatItem)
	for _, item := range agg.ToolAnalysis.Tools {
		stats[item.Tool] = item
	}
	if stats["Bash"].SuccessCount != 1 {
		t.Errorf("Bash SuccessCount=%d, want 1", stats["Bash"].SuccessCount)
	}
	if stats["Read"].FailureCount != 1 {
		t.Errorf("Read FailureCount=%d, want 1", stats["Read"].FailureCount)
	}
	if stats["Edit"].MissingResultCount != 1 {
		t.Errorf("Edit MissingResultCount=%d, want 1", stats["Edit"].MissingResultCount)
	}

	if len(agg.ToolAnalysis.FailureKinds) == 0 || agg.ToolAnalysis.FailureKinds[0].Kind != "not_found" {
		t.Fatalf("FailureKinds=%+v, want first kind not_found", agg.ToolAnalysis.FailureKinds)
	}
	if len(agg.ToolAnalysis.FailureSamples) != 1 {
		t.Fatalf("FailureSamples length=%d, want 1", len(agg.ToolAnalysis.FailureSamples))
	}
	if agg.ToolAnalysis.FailureSamples[0].Model != "glm-5v-turbo" {
		t.Fatalf("Failure sample model=%q, want glm-5v-turbo", agg.ToolAnalysis.FailureSamples[0].Model)
	}

	byModel := make(map[string]ToolModelStatItem)
	for _, item := range agg.ToolAnalysis.ByModel {
		byModel[item.Model+"::"+item.Tool] = item
	}
	if byModel["claude-sonnet-4.5::Bash"].SuccessCount != 1 {
		t.Errorf("claude-sonnet-4.5::Bash SuccessCount=%d, want 1", byModel["claude-sonnet-4.5::Bash"].SuccessCount)
	}
	if byModel["glm-5v-turbo::Read"].FailureCount != 1 {
		t.Errorf("glm-5v-turbo::Read FailureCount=%d, want 1", byModel["glm-5v-turbo::Read"].FailureCount)
	}
	if byModel["claude-sonnet-4.5::Edit"].MissingResultCount != 1 {
		t.Errorf("claude-sonnet-4.5::Edit MissingResultCount=%d, want 1", byModel["claude-sonnet-4.5::Edit"].MissingResultCount)
	}

	if agg.EventAnalysis == nil {
		t.Fatal("EventAnalysis should not be nil")
	}
	if agg.EventAnalysis.PlanModeCount != 1 {
		t.Fatalf("PlanModeCount=%d, want 1", agg.EventAnalysis.PlanModeCount)
	}
	if len(agg.EventAnalysis.Hooks) != 1 || agg.EventAnalysis.Hooks[0].ErrorCount != 1 {
		t.Fatalf("Hooks=%+v, want one hook error", agg.EventAnalysis.Hooks)
	}
	if len(agg.EventAnalysis.Skills) != 1 || agg.EventAnalysis.Skills[0].Name != "playwright-cli" {
		t.Fatalf("Skills=%+v, want playwright-cli", agg.EventAnalysis.Skills)
	}
	if len(agg.EventAnalysis.PermissionModes) != 1 || agg.EventAnalysis.PermissionModes[0].Mode != "acceptEdits" {
		t.Fatalf("PermissionModes=%+v, want acceptEdits", agg.EventAnalysis.PermissionModes)
	}
	if len(agg.EventAnalysis.OpenedFiles) != 1 || agg.EventAnalysis.OpenedFiles[0].Path != "/tmp/tool-project/main.go" {
		t.Fatalf("OpenedFiles=%+v, want main.go", agg.EventAnalysis.OpenedFiles)
	}

	if agg.AgentAnalysis == nil {
		t.Fatal("AgentAnalysis should not be nil")
	}
	if agg.AgentAnalysis.SidechainToolCalls != 1 {
		t.Fatalf("SidechainToolCalls=%d, want 1", agg.AgentAnalysis.SidechainToolCalls)
	}

	if agg.CommandAnalysis == nil {
		t.Fatal("CommandAnalysis should not be nil")
	}
	if len(agg.CommandAnalysis.RiskyCommands) != 1 || agg.CommandAnalysis.RiskyCommands[0].CommandName != "sudo rm" {
		t.Fatalf("RiskyCommands=%+v, want sudo rm", agg.CommandAnalysis.RiskyCommands)
	}
	if len(agg.CommandAnalysis.FileOperations) != 1 || agg.CommandAnalysis.FileOperations[0].FailureCount != 1 {
		t.Fatalf("FileOperations=%+v, want one failed Read", agg.CommandAnalysis.FileOperations)
	}
}

func toolUseRecord(cwd, sessionID string, ts time.Time, callID, tool string) string {
	return toolUseRecordWithModel(cwd, sessionID, ts, callID, tool, "claude-sonnet-4.5")
}

func toolUseRecordWithModel(cwd, sessionID string, ts time.Time, callID, tool, model string) string {
	return toolUseRecordWithInput(cwd, sessionID, ts, callID, tool, model, false, "", `{}`)
}

func toolUseRecordWithInput(cwd, sessionID string, ts time.Time, callID, tool, model string, isSidechain bool, agentID string, input string) string {
	sidechain := "false"
	if isSidechain {
		sidechain = "true"
	}
	agentField := ""
	if agentID != "" {
		agentField = `,"agentId":"` + agentID + `"`
	}
	return `{"type":"assistant","cwd":"` + cwd + `","sessionId":"` + sessionID + `","isSidechain":` + sidechain + agentField + `,"timestamp":"` + ts.Format(time.RFC3339Nano) + `","message":{"model":"` + model + `","content":[{"type":"tool_use","id":"` + callID + `","name":"` + tool + `","input":` + input + `}],"usage":{"input_tokens":1,"output_tokens":1}}}`
}

func toolResultRecord(cwd, sessionID string, ts time.Time, callID, content string) string {
	return `{"type":"user","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"` + callID + `","content":"` + content + `"}]}}`
}

func attachmentRecord(cwd, sessionID string, ts time.Time, attachment string) string {
	return `{"type":"attachment","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `","attachment":` + attachment + `}`
}

func permissionModeRecord(cwd, sessionID string, ts time.Time, mode string) string {
	return `{"type":"permission-mode","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `","permissionMode":"` + mode + `"}`
}
