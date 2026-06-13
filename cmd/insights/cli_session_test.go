package main

import "testing"

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
