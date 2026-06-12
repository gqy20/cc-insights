package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyBashCommandFamily(t *testing.T) {
	tests := []struct {
		name string
		stat BashCommandStat
		want string
	}{
		{name: "test", stat: BashCommandStat{CommandName: "python3", SampleCommand: "python3 -m pytest -q"}, want: "test"},
		{name: "build", stat: BashCommandStat{CommandName: "pnpm", SampleCommand: "pnpm build"}, want: "build"},
		{name: "dependency", stat: BashCommandStat{CommandName: "uv", SampleCommand: "uv sync --group dev"}, want: "dependency"},
		{name: "uv run", stat: BashCommandStat{CommandName: "uv", SampleCommand: "uv run python agent.py"}, want: "dependency"},
		{name: "git", stat: BashCommandStat{CommandName: "git", SampleCommand: "git commit -m test"}, want: "git"},
		{name: "network", stat: BashCommandStat{CommandName: "curl", SampleCommand: "curl https://example.com"}, want: "network"},
		{name: "python", stat: BashCommandStat{CommandName: "python3", SampleCommand: "python3 -c 'print(1)'"}, want: "python"},
		{name: "ripgrep", stat: BashCommandStat{CommandName: "rg", SampleCommand: "rg -n TODO ."}, want: "file/text"},
		{name: "xargs", stat: BashCommandStat{CommandName: "xargs", SampleCommand: "xargs -0 grep -n pattern"}, want: "file/text"},
		{name: "cleanup", stat: BashCommandStat{CommandName: "rm", SampleCommand: "rm -rf /tmp/demo"}, want: "cleanup"},
		{name: "shell", stat: BashCommandStat{CommandName: "echo", SampleCommand: "echo hello | cargo run --quiet"}, want: "shell"},
		{name: "timeout", stat: BashCommandStat{CommandName: "timeout", SampleCommand: "timeout 10s cargo test"}, want: "shell"},
		{name: "gh", stat: BashCommandStat{CommandName: "gh", SampleCommand: "gh auth status 2>&1"}, want: "git"},
		{name: "rust", stat: BashCommandStat{CommandName: "cargo", SampleCommand: "cargo run --quiet"}, want: "rust"},
		{name: "wrapper", stat: BashCommandStat{CommandName: "brun", SampleCommand: "brun run -- bash script.sh"}, want: "wrapper"},
		{name: "javascript npx", stat: BashCommandStat{CommandName: "npx", SampleCommand: "npx tsc --noEmit"}, want: "javascript"},
		{name: "container", stat: BashCommandStat{CommandName: "docker", SampleCommand: "docker ps"}, want: "container"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyBashCommandFamily(tt.stat); got != tt.want {
				t.Fatalf("classifyBashCommandFamily() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyBashCommandFamilyFromCustomRules(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })

	path := filepath.Join(t.TempDir(), "bash.yml")
	content := []byte("version: 1\nfamilies:\n  - name: custom\n    commands:\n      - foo\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write rules failed: %v", err)
	}
	cfg.RulesPath = path

	got := classifyBashCommandFamily(BashCommandStat{CommandName: "foo", SampleCommand: "foo run"})
	if got != "custom" {
		t.Fatalf("classifyBashCommandFamily() = %q, want custom", got)
	}
}

func TestFinalizeCommandAnalysisBuildsFamilies(t *testing.T) {
	agg := newProjectAggregate()
	agg.BashCommandStats["python3"] = &BashCommandStat{
		CommandName:   "python3",
		CallCount:     10,
		SuccessCount:  7,
		FailureCount:  3,
		SampleCommand: "python3 -m pytest -q",
	}
	agg.BashCommandStats["pnpm"] = &BashCommandStat{
		CommandName:   "pnpm",
		CallCount:     5,
		SuccessCount:  3,
		FailureCount:  2,
		SampleCommand: "pnpm build",
	}
	agg.BashCommandStats["git"] = &BashCommandStat{
		CommandName:   "git",
		CallCount:     4,
		SuccessCount:  4,
		SampleCommand: "git status",
	}

	agg.finalizeCommandAnalysis()

	if agg.CommandAnalysis == nil {
		t.Fatal("CommandAnalysis should not be nil")
	}
	if len(agg.CommandAnalysis.BashFamilies) != 3 {
		t.Fatalf("BashFamilies length = %d, want 3", len(agg.CommandAnalysis.BashFamilies))
	}
	if agg.CommandAnalysis.BashFamilies[0].Family != "test" {
		t.Fatalf("Top family = %q, want test", agg.CommandAnalysis.BashFamilies[0].Family)
	}

	report := buildCLICommandReport(&DashboardData{
		TimeRange:       TimeRangeInfo{Preset: "30d"},
		CommandAnalysis: agg.CommandAnalysis,
	}, 2)
	if report.TotalCommands != 3 {
		t.Fatalf("TotalCommands = %d, want 3", report.TotalCommands)
	}
	if report.TotalCalls != 19 {
		t.Fatalf("TotalCalls = %d, want 19", report.TotalCalls)
	}
	if len(report.ByFamily) != 2 {
		t.Fatalf("ByFamily length = %d, want 2", len(report.ByFamily))
	}
}
