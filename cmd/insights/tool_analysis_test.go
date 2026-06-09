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
	content := toolUseRecord("/tmp/tool-project", "session-tools", base, "call-ok", "Bash") + "\n" +
		toolResultRecord("/tmp/tool-project", "session-tools", base.Add(time.Second), "call-ok", "command completed") + "\n" +
		toolUseRecord("/tmp/tool-project", "session-tools", base.Add(2*time.Second), "call-fail", "Read") + "\n" +
		toolResultRecord("/tmp/tool-project", "session-tools", base.Add(3*time.Second), "call-fail", "Error: no such file or directory") + "\n" +
		toolUseRecord("/tmp/tool-project", "session-tools", base.Add(4*time.Second), "call-missing", "Edit") + "\n"

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
}

func toolUseRecord(cwd, sessionID string, ts time.Time, callID, tool string) string {
	return `{"type":"assistant","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `","message":{"model":"claude-sonnet-4.5","content":[{"type":"tool_use","id":"` + callID + `","name":"` + tool + `","input":{}}],"usage":{"input_tokens":1,"output_tokens":1}}}`
}

func toolResultRecord(cwd, sessionID string, ts time.Time, callID, content string) string {
	return `{"type":"user","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.Format(time.RFC3339Nano) + `","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"` + callID + `","content":"` + content + `"}]}}`
}
