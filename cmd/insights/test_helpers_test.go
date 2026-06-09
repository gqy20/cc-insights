package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func createTestDataDir(t *testing.T, parent string) string {
	t.Helper()

	dataDir := filepath.Join(parent, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "projects", "test-project"), 0755); err != nil {
		t.Fatalf("创建 projects 测试目录失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "debug"), 0755); err != nil {
		t.Fatalf("创建 debug 测试目录失败: %v", err)
	}

	now := time.Now()
	historyContent := `{"display":"test","timestamp":` + strconv.FormatInt(now.UnixMilli(), 10) + `,"project":"test-project"}
{"display":"/help","timestamp":` + strconv.FormatInt(now.Add(time.Second).UnixMilli(), 10) + `,"project":"test-project"}
`
	if err := os.WriteFile(filepath.Join(dataDir, "history.jsonl"), []byte(historyContent), 0644); err != nil {
		t.Fatalf("创建 history.jsonl 失败: %v", err)
	}

	projectContent := projectRecordJSON("/tmp/test-project", "session-1", now) + "\n" +
		projectRecordJSON("/tmp/test-project", "session-2", now.Add(time.Hour)) + "\n"
	if err := os.WriteFile(filepath.Join(dataDir, "projects", "test-project", "session.jsonl"), []byte(projectContent), 0644); err != nil {
		t.Fatalf("创建 projects jsonl 失败: %v", err)
	}

	debugContent := now.UTC().Format("2006-01-02T15:04:05.000Z") + " [DEBUG] tool call mcp__github__get_file_contents\n"
	if err := os.WriteFile(filepath.Join(dataDir, "debug", "debug.txt"), []byte(debugContent), 0644); err != nil {
		t.Fatalf("创建 debug 日志失败: %v", err)
	}

	return dataDir
}

func appendProjectRecord(t *testing.T, dataDir string, cwd string, sessionID string, ts time.Time) {
	t.Helper()

	path := filepath.Join(dataDir, "projects", "test-project", "session.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("打开 projects jsonl 失败: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString(projectRecordJSON(cwd, sessionID, ts) + "\n"); err != nil {
		t.Fatalf("追加 projects jsonl 失败: %v", err)
	}
}

func projectRecordJSON(cwd string, sessionID string, ts time.Time) string {
	return `{"type":"assistant","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` + ts.UTC().Format(time.RFC3339Nano) + `","message":{"model":"claude-sonnet-4.5","usage":{"input_tokens":10,"output_tokens":5}}}`
}
