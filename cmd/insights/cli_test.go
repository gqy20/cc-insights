package main

import "testing"

func TestNormalizeCLICommandAliases(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantArgs []string
	}{
		{name: "default", args: nil, wantName: "sum", wantArgs: nil},
		{name: "default with flags", args: []string{"-p", "7d"}, wantName: "sum", wantArgs: []string{"-p", "7d"}},
		{name: "summary", args: []string{"sum", "-j"}, wantName: "sum", wantArgs: []string{"sum", "-j"}},
		{name: "failures", args: []string{"err", "-p", "7d"}, wantName: "err", wantArgs: []string{"err", "-p", "7d"}},
		{name: "why", args: []string{"why", "--reason", "error_text"}, wantName: "why", wantArgs: []string{"why", "--reason", "error_text"}},
		{name: "cost", args: []string{"tok", "-p", "30d"}, wantName: "tok", wantArgs: []string{"tok", "-p", "30d"}},
		{name: "web", args: []string{"web", "--addr", ":8932"}, wantName: "web", wantArgs: []string{"web", "--addr", ":8932"}},
		{name: "unknown long form", args: []string{"failures", "-j"}, wantName: "failures", wantArgs: []string{"failures", "-j"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCLICommand(tt.args)
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
	opts, err := parseCLIOptions("err", []string{"-p", "7d", "-j", "-n", "5"}, true)
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
	opts, err := parseCLIOptions("why", []string{"-p", "7d", "-n", "3", "--reason", "error_text", "-m"}, true)
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

func TestRunCLIRejectsLegacyLongCommands(t *testing.T) {
	for _, command := range []string{"summary", "failures", "cost", "inspect"} {
		t.Run(command, func(t *testing.T) {
			if err := runCLI([]string{command}); err == nil {
				t.Fatalf("runCLI(%q) returned nil error", command)
			}
		})
	}
}
