package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCLIPromptReportFiltersNoiseAndToolResults(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	jsonl := filepath.Join(projectDir, "session.jsonl")
	content := `{"type":"user","timestamp":"2026-06-10T10:00:00Z","cwd":"/repo/demo","sessionId":"s1","message":{"role":"user","content":"先分析这个前端，截图核对后开始优化"}}
{"type":"user","timestamp":"2026-06-10T10:01:00Z","cwd":"/repo/demo","sessionId":"s1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}
{"type":"user","timestamp":"2026-06-10T10:02:00Z","cwd":"/repo/demo","sessionId":"s1","message":{"role":"user","content":"This session is being continued from a previous conversation that ran out of context."}}
{"type":"user","timestamp":"2026-06-10T10:03:00Z","cwd":"/repo/demo","sessionId":"s1","message":{"role":"user","content":"提交更改"}}
{"type":"assistant","timestamp":"2026-06-10T10:04:00Z","cwd":"/repo/demo","sessionId":"s1","message":{"role":"assistant","content":[]}}
`
	if err := os.WriteFile(jsonl, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldCfg := cfg
	cfg = defaultConfig()
	cfg.DataDir = dir
	defer func() { cfg = oldCfg }()

	report, err := buildCLIPromptReport(TimeFilter{}, "all", cliOptions{Limit: 10})
	if err != nil {
		t.Fatalf("buildCLIPromptReport: %v", err)
	}
	if report.RawPrompts != 3 {
		t.Fatalf("RawPrompts = %d, want 3", report.RawPrompts)
	}
	if report.CleanPrompts != 2 {
		t.Fatalf("CleanPrompts = %d, want 2", report.CleanPrompts)
	}
	if report.NoisePrompts != 1 {
		t.Fatalf("NoisePrompts = %d, want 1", report.NoisePrompts)
	}
	if report.ToolResultRecords != 1 {
		t.Fatalf("ToolResultRecords = %d, want 1", report.ToolResultRecords)
	}
	if !hasPromptCategory(report.ByCategory, "先说明/规划") || !hasPromptCategory(report.ByCategory, "验证核对") || !hasPromptCategory(report.ByCategory, "开始实现/推进") {
		t.Fatalf("expected multi-label prompt categories, got %#v", report.ByCategory)
	}
	if !hasNameCount(report.TopShortPrompts, "提交更改") {
		t.Fatalf("TopShortPrompts = %#v, want 提交更改", report.TopShortPrompts)
	}
}

func TestBuildCLIPromptReportProjectFilter(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	jsonl := filepath.Join(projectDir, "session.jsonl")
	content := `{"type":"user","timestamp":"2026-06-10T10:00:00Z","cwd":"/repo/demo","sessionId":"s1","message":{"role":"user","content":"开始实现"}}
{"type":"user","timestamp":"2026-06-10T10:01:00Z","cwd":"/repo/other","sessionId":"s2","message":{"role":"user","content":"继续"}}
`
	if err := os.WriteFile(jsonl, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	oldCfg := cfg
	cfg = defaultConfig()
	cfg.DataDir = dir
	defer func() { cfg = oldCfg }()

	report, err := buildCLIPromptReport(TimeFilter{}, "all", cliOptions{Limit: 10, Project: "demo"})
	if err != nil {
		t.Fatalf("buildCLIPromptReport: %v", err)
	}
	if report.CleanPrompts != 1 {
		t.Fatalf("CleanPrompts = %d, want 1", report.CleanPrompts)
	}
	if report.Filter.Project != "demo" {
		t.Fatalf("Filter.Project = %q, want demo", report.Filter.Project)
	}
}

func TestPromptPreviewExtractsCommandArgsAndKeepsRunes(t *testing.T) {
	text := "<command-message>x</command-message><command-name>/x</command-name><command-args>看一下当前界面是否太挤</command-args>"
	got := promptPreview(text, 6)
	if got != "看一下当前界" {
		t.Fatalf("promptPreview = %q, want rune-safe command args preview", got)
	}
}

func hasPromptCategory(items []promptCategoryStat, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func hasNameCount(items []nameCount, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}
