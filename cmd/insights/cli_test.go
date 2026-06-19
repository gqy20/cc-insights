package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestResolveCLICommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantArgs []string
	}{
		{name: "default", args: nil, wantName: "sum", wantArgs: nil},
		{name: "default with flags", args: []string{"-p", "7d"}, wantName: "sum", wantArgs: []string{"-p", "7d"}},
		{name: "summary", args: []string{"sum", "-j"}, wantName: "sum", wantArgs: []string{"-j"}},
		{name: "failures", args: []string{"err", "-p", "7d"}, wantName: "err", wantArgs: []string{"-p", "7d"}},
		{name: "why", args: []string{"why", "--reason", "error_text"}, wantName: "why", wantArgs: []string{"--reason", "error_text"}},
		{name: "cost", args: []string{"tok", "-p", "30d"}, wantName: "tok", wantArgs: []string{"-p", "30d"}},
		{name: "sessions", args: []string{"ses", "-p", "7d"}, wantName: "ses", wantArgs: []string{"-p", "7d"}},
		{name: "web", args: []string{"web", "--addr", ":8932"}, wantName: "web", wantArgs: []string{"--addr", ":8932"}},
		{name: "unknown long form", args: []string{"failures", "-j"}, wantName: "failures", wantArgs: []string{"-j"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCLICommand(tt.args)
			if got.Name != tt.wantName {
				t.Fatalf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if len(got.Args) != len(tt.wantArgs) {
				t.Fatalf("Args length = %d, want %d: %#v", len(got.Args), len(tt.wantArgs), got.Args)
			}
			for i := range got.Args {
				if got.Args[i] != tt.wantArgs[i] {
					t.Fatalf("Args[%d] = %q, want %q", i, got.Args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestParseCLIOptionsShortFlags(t *testing.T) {
	opts, err := parseCLIOptions(cmdErr, []string{"-p", "7d", "-j", "-n", "5"})
	if err != nil {
		t.Fatalf("parseCLIOptions returned error: %v", err)
	}
	if opts.Preset != "7d" {
		t.Fatalf("Preset = %q, want 7d", opts.Preset)
	}
	if opts.Format != "json" {
		t.Fatalf("Format = %q, want json", opts.Format)
	}
	if opts.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", opts.Limit)
	}
}

func TestParseCLIOptionsInspectUsesLimitAsSamples(t *testing.T) {
	opts, err := parseCLIOptions(cmdWhy, []string{"-p", "7d", "-n", "3", "--reason", "error_text", "-m"})
	if err != nil {
		t.Fatalf("parseCLIOptions returned error: %v", err)
	}
	if opts.Samples != 3 {
		t.Fatalf("Samples = %d, want 3", opts.Samples)
	}
	if opts.Reason != "error_text" {
		t.Fatalf("Reason = %q, want error_text", opts.Reason)
	}
	if opts.Format != "markdown" {
		t.Fatalf("Format = %q, want markdown", opts.Format)
	}
}

func TestParseCLIOptionsRecommendationDetail(t *testing.T) {
	opts, err := parseCLIOptions(cmdRec, []string{"-p", "7d", "--detail", "--id", "performance.slowest_call"})
	if err != nil {
		t.Fatalf("parseCLIOptions returned error: %v", err)
	}
	if !opts.Detail {
		t.Fatal("Detail = false, want true")
	}
	if opts.ID != "performance.slowest_call" {
		t.Fatalf("ID = %q, want performance.slowest_call", opts.ID)
	}
}

func TestParseCLIOptionsPromptProfile(t *testing.T) {
	opts, err := parseCLIOptions(cmdRec, []string{"-p", "30d", "--prompts", "--project", "cc-insights"})
	if err != nil {
		t.Fatalf("parseCLIOptions returned error: %v", err)
	}
	if !opts.Prompts {
		t.Fatal("Prompts = false, want true")
	}
	if opts.Project != "cc-insights" {
		t.Fatalf("Project = %q, want cc-insights", opts.Project)
	}
}

func TestRunCLIRejectsLegacyLongCommands(t *testing.T) {
	for _, command := range []string{"summary", "failures", "cost", "inspect"} {
		t.Run(command, func(t *testing.T) {
			if err := runCLI([]string{command}); err == nil {
				t.Fatalf("runCLI(%q) returned nil error", command)
			}
		})
	}
}

// ============================================================
// Help 系统（registry 派生）锚定测试，防止命令/flag 描述漂移
// ============================================================

func TestGlobalHelpListsAllCommands(t *testing.T) {
	var buf bytes.Buffer
	printGlobalHelp(&buf)
	out := buf.String()
	for _, name := range []string{"sum", "err", "why", "tok", "cmd", "ses", "rec", "web"} {
		if !strings.Contains(out, name) {
			t.Errorf("global help missing command %q", name)
		}
	}
	if !strings.Contains(out, "我想看") {
		t.Error("global help missing Chinese navigation section")
	}
	// --rules 此前隐藏在旧 help，registry 后作为通用 flag 可见
	for _, f := range []string{"--data", "--cache", "--rules"} {
		if !strings.Contains(out, f) {
			t.Errorf("global help missing common config flag %q", f)
		}
	}
	// --addr/--base 是 web 专属，不应出现在全局 help
	for _, f := range []string{"--addr", "--base"} {
		if strings.Contains(out, f) {
			t.Errorf("global help should not list web-only flag %q", f)
		}
	}
}

func TestSubcommandHelpWorks(t *testing.T) {
	var buf bytes.Buffer
	fs := flag.NewFlagSet("why", flag.ContinueOnError)
	cmdWhy.Flags(fs, &cliOptions{})
	printCommandHelp(cmdWhy, fs, &buf)
	out := buf.String()
	for _, want := range []string{"--reason", "--category", "--tool", "--model"} {
		if !strings.Contains(out, want) {
			t.Errorf("why help missing filter flag %q", want)
		}
	}
	if !strings.Contains(out, "失败样例") {
		t.Error("why help missing Chinese long description")
	}
}

func TestSubcommandHelpForRecShowsDetailAndPrompts(t *testing.T) {
	var buf bytes.Buffer
	fs := flag.NewFlagSet("rec", flag.ContinueOnError)
	cmdRec.Flags(fs, &cliOptions{})
	printCommandHelp(cmdRec, fs, &buf)
	out := buf.String()
	for _, want := range []string{"--detail", "--id", "--prompts"} {
		if !strings.Contains(out, want) {
			t.Errorf("rec help missing flag %q", want)
		}
	}
}

func TestSubcommandHelpForWebShowsOnlyConfigFlags(t *testing.T) {
	var buf bytes.Buffer
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	opts := cliOptions{Config: defaultConfig()}
	registerConfigFlags(fs, &opts.Config)
	cmdWeb.Flags(fs, &opts)
	printCommandHelp(cmdWeb, fs, &buf)
	out := buf.String()
	for _, want := range []string{"--data", "--cache", "--addr", "--base", "--rules"} {
		if !strings.Contains(out, want) {
			t.Errorf("web help missing config flag %q", want)
		}
	}
	for _, unwanted := range []string{"--preset", "--reason", "--detail"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("web help should not show analysis flag %q", unwanted)
		}
	}
}

func TestSubcommandHelpHTriggersHelpExit(t *testing.T) {
	// cc-insights why -h 应静默退出（errHelp），不再泄露 "flag: help requested"
	if err := runCLI([]string{"why", "-h"}); err != nil {
		t.Fatalf("runCLI why -h returned error: %v", err)
	}
}

func TestFlagRestriction_SumRejectsReason(t *testing.T) {
	// sum 不归属 --reason，应被 flag 包拒绝（专属 flag 按命令归属限制的行为变化锚定）
	if err := runCLI([]string{"sum", "--reason", "x"}); err == nil {
		t.Fatal("runCLI sum --reason should error: reason is not a sum flag")
	}
}
