package main

import (
	"strings"
	"testing"
)

func TestBuildCLISessionReportFiltersSessionAndProject(t *testing.T) {
	data := &DashboardData{
		TimeRange: TimeRangeInfo{Preset: "7d"},
		SessionAnalysis: &SessionAnalysisData{
			Sessions: []SessionAnalysisItem{
				{SessionID: "session-1", Project: "/tmp/project-a", DurationMs: 60_000, ToolFailureCount: 2},
				{SessionID: "session-2", Project: "/tmp/project-b", DurationMs: 30_000, ToolFailureCount: 0},
			},
			LongRunning: []SessionAnalysisItem{
				{SessionID: "session-1", Project: "/tmp/project-a", DurationMs: 60_000, ToolFailureCount: 2},
				{SessionID: "session-2", Project: "/tmp/project-b", DurationMs: 30_000, ToolFailureCount: 0},
			},
			TopFailures: []SessionAnalysisItem{
				{SessionID: "session-1", Project: "/tmp/project-a", DurationMs: 60_000, ToolFailureCount: 2},
			},
		},
		FailureAnalysis: &FailureAnalysisData{
			Samples: []ToolFailureSample{
				{SessionID: "session-1", Project: "/tmp/project-a", Tool: "Bash", Category: "runtime", Reason: "timeout"},
				{SessionID: "session-2", Project: "/tmp/project-b", Tool: "Read", Category: "filesystem", Reason: "missing_file"},
			},
		},
	}

	report := buildCLISessionReport(data, cliOptions{Limit: 10, Samples: 10, Session: "session-1", Project: "project-a"})
	if report.TotalSessions != 1 {
		t.Fatalf("TotalSessions = %d, want 1", report.TotalSessions)
	}
	if len(report.LongRunning) != 1 || report.LongRunning[0].SessionID != "session-1" {
		t.Fatalf("LongRunning = %+v, want session-1 only", report.LongRunning)
	}
	if len(report.FailureSamples) != 1 || report.FailureSamples[0].SessionID != "session-1" {
		t.Fatalf("FailureSamples = %+v, want session-1 only", report.FailureSamples)
	}
}

func TestBuildCLISessionReportIncludesHookStats(t *testing.T) {
	data := &DashboardData{
		TimeRange: TimeRangeInfo{Preset: "7d"},
		EventAnalysis: &EventAnalysisData{
			Hooks: []HookStatItem{
				{HookName: "Stop", HookEvent: "Stop", SuccessCount: 3, ErrorCount: 2, TotalCount: 5, FailureRate: 40.0, AvgDurationMs: 1500},
				{HookName: "PreToolUse", HookEvent: "PreToolUse", SuccessCount: 10, TotalCount: 10, FailureRate: 0, AvgDurationMs: 12},
			},
		},
	}

	report := buildCLISessionReport(data, cliOptions{Limit: 10})
	if len(report.Hooks) != 2 {
		t.Fatalf("Hooks len = %d, want 2", len(report.Hooks))
	}
	if report.Hooks[0].HookName != "Stop" {
		t.Fatalf("first hook = %s, want Stop", report.Hooks[0].HookName)
	}
	var found bool
	for _, ins := range report.Insights {
		if strings.Contains(ins, "Stop") && strings.Contains(ins, "失败") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("insights %v 缺少 Stop hook 失败洞察", report.Insights)
	}

	limitReport := buildCLISessionReport(data, cliOptions{Limit: 1})
	if len(limitReport.Hooks) != 1 {
		t.Fatalf("limited Hooks len = %d, want 1", len(limitReport.Hooks))
	}
}

func TestBuildCLISessionReportExplainsAnomalousSessions(t *testing.T) {
	data := &DashboardData{
		TimeRange: TimeRangeInfo{Preset: "7d"},
		SessionAnalysis: &SessionAnalysisData{
			Sessions: []SessionAnalysisItem{
				{SessionID: "session-hot", Project: "/repo", DurationMs: 3_600_000, ToolCallCount: 20, ToolFailureCount: 8, TotalTokens: 1_200_000, PrimaryModel: "model-a", PlanModeCount: 1},
			},
			LongRunning: []SessionAnalysisItem{
				{SessionID: "session-hot", Project: "/repo", DurationMs: 3_600_000, ToolCallCount: 20, ToolFailureCount: 8, TotalTokens: 1_200_000, PrimaryModel: "model-a", PlanModeCount: 1},
			},
			TopFailures: []SessionAnalysisItem{
				{SessionID: "session-hot", Project: "/repo", DurationMs: 3_600_000, ToolCallCount: 20, ToolFailureCount: 8, TotalTokens: 1_200_000, PrimaryModel: "model-a", PlanModeCount: 1},
			},
		},
		FailureAnalysis: &FailureAnalysisData{
			Samples: []ToolFailureSample{
				{SessionID: "session-hot", Tool: "Bash", Category: "bash", Reason: "exit_code_1"},
				{SessionID: "session-hot", Tool: "Bash", Category: "bash", Reason: "exit_code_1"},
				{SessionID: "session-hot", Tool: "Edit", Category: "edit", Reason: "old_string_mismatch"},
			},
		},
		ToolPerformance: &ToolPerformanceData{
			SlowestCalls: []ToolSlowCallItem{
				{SessionID: "session-hot", Tool: "Bash", Category: "Bash:test", DurationMs: 120_000},
			},
		},
	}

	report := buildCLISessionReport(data, cliOptions{Limit: 10, Samples: 10})
	if len(report.SessionDiagnostics) != 1 {
		t.Fatalf("SessionDiagnostics len=%d, want 1", len(report.SessionDiagnostics))
	}
	diag := report.SessionDiagnostics[0]
	if diag.SessionID != "session-hot" {
		t.Fatalf("diag session=%s, want session-hot", diag.SessionID)
	}
	if len(diag.Reasons) == 0 || diag.Reasons[0].Name != "bash/exit_code_1" || diag.Reasons[0].Count != 2 {
		t.Fatalf("reasons=%+v, want bash/exit_code_1 count 2", diag.Reasons)
	}
	if len(diag.Tools) == 0 || diag.Tools[0].Name != "Bash" || diag.Tools[0].Count != 2 {
		t.Fatalf("tools=%+v, want Bash count 2", diag.Tools)
	}
	if diag.SlowestCall == nil || diag.SlowestCall.Category != "Bash:test" {
		t.Fatalf("slowest=%+v, want Bash:test", diag.SlowestCall)
	}
	if !containsString(diag.Signals, "long_session") || !containsString(diag.Signals, "high_failure_rate") || !containsString(diag.Signals, "plan_lifecycle_unbalanced") {
		t.Fatalf("signals=%+v, want long/high/plan", diag.Signals)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
