package main

import "testing"

func TestBuildCLIRecommendationReport(t *testing.T) {
	data := &DashboardData{
		TimeRange: TimeRangeInfo{Preset: "30d"},
		CommandAnalysis: &CommandAnalysisData{
			BashFamilies: []BashCommandFamilyStat{
				{Family: "test", CallCount: 100, FailureCount: 40, FailureRate: 40, TopCommand: "playwright-cli"},
				{Family: "other", CallCount: 30, FailureCount: 5, FailureRate: 16.7, TopCommand: "custom-tool"},
			},
			RiskyCommands: []BashCommandStat{
				{CommandName: "rm", CallCount: 3, RiskLevel: "high", RiskReason: "recursive delete"},
			},
		},
		FailureAnalysis: &FailureAnalysisData{
			TotalFailures: 100,
			ByReason: []FailureReasonStat{
				{Category: "runtime", Reason: "timeout", Count: 30},
			},
			ByToolReason: []FailureToolReasonStat{
				{Tool: "Bash", Category: "runtime", Reason: "timeout", Count: 25},
			},
		},
		ToolPerformance: &ToolPerformanceData{
			ByCategory: []ToolPerfCategoryItem{
				{Category: "Bash:test", BaseTool: "Bash", CallCount: 20, TotalDurationMs: 120000, AvgDurationMs: 6000, MaxDurationMs: 30000, ErrorRate: 35},
			},
			SlowestCalls: []ToolSlowCallItem{
				{Tool: "Bash", Category: "Bash:test", DurationMs: 30000, Project: "/tmp/project", SessionID: "session-1"},
			},
		},
		CostAnalysis: &CostAnalysisData{
			Totals: TokenUsageBreakdown{TotalTokens: 300000},
			ByProject: []CostProjectStat{
				{Project: "/tmp/project", TotalTokens: 150000},
			},
			BySession: []CostSessionStat{
				{SessionID: "session-1", Project: "/tmp/project", TotalTokens: 220000},
			},
		},
		SessionAnalysis: &SessionAnalysisData{
			LongRunning: []SessionAnalysisItem{
				{SessionID: "session-1", Project: "/tmp/project", DurationMs: 45 * 60 * 1000, ToolFailureCount: 12},
			},
		},
	}

	report := buildCLIRecommendationReport(data, 20)
	if report.TotalFindings == 0 {
		t.Fatal("expected findings")
	}
	if !hasFindingCategory(report.Recommendations, "command") {
		t.Fatalf("expected command finding: %+v", report.Recommendations)
	}
	if !hasFindingCategory(report.Recommendations, "performance") {
		t.Fatalf("expected performance finding: %+v", report.Recommendations)
	}
	if !hasFindingCategory(report.Recommendations, "context") {
		t.Fatalf("expected context finding: %+v", report.Recommendations)
	}
	if !hasFindingCategory(report.Recommendations, "workflow") {
		t.Fatalf("expected workflow finding: %+v", report.Recommendations)
	}
	if !hasFindingWithDrilldown(report.Recommendations) {
		t.Fatalf("expected drilldown commands: %+v", report.Recommendations)
	}
}

func TestNormalizeCLICommandRecognizesRec(t *testing.T) {
	normalized := normalizeCLICommand([]string{"rec", "-p", "7d"})
	if normalized.Name != "rec" {
		t.Fatalf("Name = %q, want rec", normalized.Name)
	}
}

func hasFindingCategory(items []diagnosticFinding, category string) bool {
	for _, item := range items {
		if item.Category == category {
			return true
		}
	}
	return false
}

func hasFindingWithDrilldown(items []diagnosticFinding) bool {
	for _, item := range items {
		if len(item.Drilldowns) > 0 && item.Drilldowns[0].Command != "" {
			return true
		}
	}
	return false
}
