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
			Samples: []ToolFailureSample{
				{Tool: "Bash", Category: "runtime", Reason: "timeout", Project: "/tmp/project", SessionID: "session-1", ContentPreview: "Command timed out after running full test suite"},
				{Tool: "Read", Category: "file", Reason: "not_found", Project: "/tmp/project", SessionID: "session-1", ContentPreview: "Error: no such file or directory"},
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

	report := buildCLIRecommendationReport(data, cliOptions{Limit: 20})
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
	if !hasFindingWithDiagnosticDetails(report.Recommendations) {
		t.Fatalf("expected diagnostic details: %+v", report.Recommendations)
	}
	if !hasFindingWithExamplesAndActions(report.Recommendations) {
		t.Fatalf("expected examples and actions: %+v", report.Recommendations)
	}
}

func TestBuildCLIRecommendationReportFiltersByID(t *testing.T) {
	data := &DashboardData{
		TimeRange: TimeRangeInfo{Preset: "7d"},
		ToolPerformance: &ToolPerformanceData{
			SlowestCalls: []ToolSlowCallItem{
				{Tool: "Bash", Category: "Bash:test", DurationMs: 30000, Project: "/tmp/project", SessionID: "session-1"},
			},
		},
	}

	report := buildCLIRecommendationReport(data, cliOptions{Limit: 20, ID: "performance.slowest_call", Detail: true})
	if !report.Detail {
		t.Fatal("Detail = false, want true")
	}
	if report.FilterID != "performance.slowest_call" {
		t.Fatalf("FilterID = %q, want performance.slowest_call", report.FilterID)
	}
	if report.TotalFindings != 1 {
		t.Fatalf("TotalFindings = %d, want 1", report.TotalFindings)
	}
	if report.Recommendations[0].ID != "performance.slowest_call" {
		t.Fatalf("ID = %q, want performance.slowest_call", report.Recommendations[0].ID)
	}
}

func TestDiagnosticRulesLoad(t *testing.T) {
	rule, ok := diagnosticRuleForID("performance.slowest_call")
	if !ok {
		t.Fatal("expected performance.slowest_call diagnostic rule")
	}
	if rule.Metric != "slowest_call_duration" {
		t.Fatalf("Metric = %q, want slowest_call_duration", rule.Metric)
	}
}

func TestDetailedFailureRootCauseUsesExamples(t *testing.T) {
	finding := diagnosticFinding{
		Category:   "failure",
		Confidence: "high",
		Evidence: []diagnosticEvidence{
			{Label: "原因", Value: "mcp/explicit_error"},
			{Label: "工具", Value: "mcp__crawl-mcp__crawl_single"},
		},
	}
	causes := buildDiagnosticRootCauses(finding, []diagnosticExample{
		{Tool: "mcp__crawl-mcp__crawl_single", Category: "mcp", Reason: "explicit_error", ContentPreview: "Executable doesn't exist at /home/user/.cache/ms-playwright/chromium/chrome"},
	})
	if len(causes) == 0 {
		t.Fatal("expected root cause")
	}
	if causes[0].Type != "browser_missing" {
		t.Fatalf("Type = %q, want browser_missing", causes[0].Type)
	}
}

func TestResolveCLICommandRecognizesRec(t *testing.T) {
	resolved := resolveCLICommand([]string{"rec", "-p", "7d"})
	if resolved.Name != "rec" {
		t.Fatalf("Name = %q, want rec", resolved.Name)
	}
	if len(resolved.Args) != 2 || resolved.Args[0] != "-p" || resolved.Args[1] != "7d" {
		t.Fatalf("Args = %#v, want [-p 7d]", resolved.Args)
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

func hasFindingWithDiagnosticDetails(items []diagnosticFinding) bool {
	for _, item := range items {
		if item.Trigger != nil && len(item.RootCauses) > 0 && len(item.Targets) > 0 {
			return true
		}
	}
	return false
}

func hasFindingWithExamplesAndActions(items []diagnosticFinding) bool {
	for _, item := range items {
		if len(item.Examples) > 0 && len(item.Actions) > 0 {
			return true
		}
	}
	return false
}
