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
