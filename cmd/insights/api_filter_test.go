package main

import "testing"

func TestApplyDashboardFilterNarrowsDashboardData(t *testing.T) {
	data := &DashboardData{
		ProjectStats: &ProjectStatsData{
			Projects: []ProjectStatItem{
				{Project: "/repo/alpha", MessageCount: 10, SessionCount: 2},
				{Project: "/repo/beta", MessageCount: 30, SessionCount: 3},
			},
			TotalMessages: 40,
			TotalSessions: 5,
		},
		ModelUsage: []ModelUsageItem{
			{Model: "sonnet", Count: 10, Tokens: 1000},
			{Model: "opus", Count: 3, Tokens: 300},
		},
		ToolAnalysis: &ToolAnalysisData{
			TotalCalls:    12,
			TotalFailures: 5,
			Tools: []ToolStatItem{
				{Tool: "Bash", CallCount: 7, FailureCount: 4},
				{Tool: "Read", CallCount: 5, FailureCount: 1},
			},
			ByModel: []ToolModelStatItem{
				{Model: "sonnet", Tool: "Bash", CallCount: 7, FailureCount: 4},
				{Model: "opus", Tool: "Read", CallCount: 5, FailureCount: 1},
			},
		},
		FailureAnalysis: &FailureAnalysisData{
			TotalFailures: 5,
			ByReason: []FailureReasonStat{
				{Category: "bash", Reason: "exit_code_1", Count: 4},
				{Category: "file", Reason: "not_found", Count: 1},
			},
			ByToolReason: []FailureToolReasonStat{
				{Tool: "Bash", Category: "bash", Reason: "exit_code_1", Count: 4},
				{Tool: "Read", Category: "file", Reason: "not_found", Count: 1},
			},
			ByModelReason: []FailureModelReasonStat{
				{Model: "sonnet", Category: "bash", Reason: "exit_code_1", Count: 4},
				{Model: "opus", Category: "file", Reason: "not_found", Count: 1},
			},
			Samples: []ToolFailureSample{
				{Tool: "Bash", Model: "sonnet", Category: "bash", Reason: "exit_code_1", Project: "/repo/alpha", SessionID: "s1"},
				{Tool: "Read", Model: "opus", Category: "file", Reason: "not_found", Project: "/repo/beta", SessionID: "s2"},
			},
		},
		CostAnalysis: &CostAnalysisData{
			ByModel: []CostModelStat{
				{Model: "sonnet", RequestCount: 2, TotalTokens: 1000},
				{Model: "opus", RequestCount: 1, TotalTokens: 300},
			},
			ByProject: []CostProjectStat{
				{Project: "/repo/alpha", RequestCount: 2, TotalTokens: 1000},
				{Project: "/repo/beta", RequestCount: 1, TotalTokens: 300},
			},
			BySession: []CostSessionStat{
				{SessionID: "s1", Project: "/repo/alpha", Model: "sonnet", RequestCount: 2, TotalTokens: 1000},
				{SessionID: "s2", Project: "/repo/beta", Model: "opus", RequestCount: 1, TotalTokens: 300},
			},
		},
		SessionAnalysis: &SessionAnalysisData{
			Sessions: []SessionAnalysisItem{
				{SessionID: "s1", Project: "/repo/alpha"},
				{SessionID: "s2", Project: "/repo/beta"},
			},
			LongRunning: []SessionAnalysisItem{
				{SessionID: "s1", Project: "/repo/alpha"},
				{SessionID: "s2", Project: "/repo/beta"},
			},
		},
	}

	applyDashboardFilter(data, AnalysisFilter{
		Project: "/repo/alpha",
		Session: "s1",
		Tool:    "Bash",
		Model:   "sonnet",
		Reason:  "exit_code_1",
	})

	if data.ProjectStats != nil {
		t.Fatalf("project stats should be empty for tool/reason/session combined filter, got %+v", data.ProjectStats)
	}
	if len(data.ModelUsage) != 0 {
		t.Fatalf("model usage should be empty for tool/reason/session combined filter, got %+v", data.ModelUsage)
	}
	if data.ToolAnalysis != nil {
		t.Fatalf("tool analysis should be empty for project/session/reason combined filter, got %+v", data.ToolAnalysis)
	}
	if got := len(data.FailureAnalysis.Samples); got != 1 || data.FailureAnalysis.Samples[0].SessionID != "s1" {
		t.Fatalf("filtered failure samples = %+v, want s1 only", data.FailureAnalysis.Samples)
	}
	if data.CostAnalysis != nil {
		t.Fatalf("cost analysis should be empty for tool filter, got %+v", data.CostAnalysis)
	}
	if data.SessionAnalysis != nil {
		t.Fatalf("session analysis should be empty for tool/model/reason filter, got %+v", data.SessionAnalysis)
	}
}

func TestApplyDashboardFilterKeepsPreciseToolData(t *testing.T) {
	origCache := globalCache
	globalCache = &CacheFile{
		DailyStats: map[string]*DayAggregate{
			"2026-06-01": {Date: "2026-06-01"},
		},
		DailyRuntime: map[string]ProjectFileAggregate{
			"2026-06-01": {
				ToolStats: map[string]ToolStatItem{
					"Bash": {Tool: "Bash", CallCount: 7},
					"Read": {Tool: "Read", CallCount: 5},
				},
			},
		},
	}
	defer func() { globalCache = origCache }()

	data := &DashboardData{
		DailyTrend: DailyTrendData{Dates: []string{"2026-06-01"}, Counts: []int{10}},
		Commands:   []CommandStats{{Command: "/resume", Count: 2}},
		MCPTools: []MCPToolStats{
			{Server: "crawl", Tool: "search", Count: 5},
			{Server: "other", Tool: "read", Count: 3},
		},
		ToolAnalysis: &ToolAnalysisData{
			Tools: []ToolStatItem{
				{Tool: "Bash", CallCount: 7, FailureCount: 4},
				{Tool: "Read", CallCount: 5, FailureCount: 1},
			},
			ByModel: []ToolModelStatItem{
				{Model: "sonnet", Tool: "Bash", CallCount: 7, FailureCount: 4},
				{Model: "sonnet", Tool: "Read", CallCount: 5, FailureCount: 1},
			},
		},
	}

	applyDashboardFilter(data, AnalysisFilter{Tool: "Bash"})

	if len(data.DailyTrend.Counts) != 1 || data.DailyTrend.Counts[0] != 7 {
		t.Fatalf("daily trend = %+v, want Bash count 7", data.DailyTrend)
	}
	if len(data.Commands) != 0 {
		t.Fatalf("slash commands should be empty for tool filter, got %+v", data.Commands)
	}
	if data.ToolAnalysis == nil || len(data.ToolAnalysis.Tools) != 1 || data.ToolAnalysis.Tools[0].Tool != "Bash" {
		t.Fatalf("tool analysis = %+v, want Bash only", data.ToolAnalysis)
	}
	if data.ToolAnalysis.TotalCalls != 7 || data.ToolAnalysis.TotalFailures != 4 {
		t.Fatalf("tool totals = calls %d failures %d, want 7/4", data.ToolAnalysis.TotalCalls, data.ToolAnalysis.TotalFailures)
	}
}

func TestApplyDashboardFilterRebuildsReasonTrend(t *testing.T) {
	origCache := globalCache
	globalCache = &CacheFile{
		DailyStats: map[string]*DayAggregate{
			"2026-06-01": {Date: "2026-06-01"},
			"2026-06-02": {Date: "2026-06-02"},
		},
		DailyRuntime: map[string]ProjectFileAggregate{
			"2026-06-01": {
				FailureReasons: map[string]FailureReasonStat{
					"bash\x00exit_code_1": {Category: "bash", Reason: "exit_code_1", Count: 3},
					"file\x00not_found":   {Category: "file", Reason: "not_found", Count: 2},
				},
			},
			"2026-06-02": {
				FailureReasons: map[string]FailureReasonStat{
					"bash\x00exit_code_1": {Category: "bash", Reason: "exit_code_1", Count: 4},
				},
			},
		},
	}
	defer func() { globalCache = origCache }()

	data := &DashboardData{
		DailyTrend: DailyTrendData{Dates: []string{"2026-06-01", "2026-06-02"}, Counts: []int{10, 20}},
		FailureAnalysis: &FailureAnalysisData{
			ByReason: []FailureReasonStat{
				{Category: "bash", Reason: "exit_code_1", Count: 7},
				{Category: "file", Reason: "not_found", Count: 2},
			},
			Samples: []ToolFailureSample{{Category: "bash", Reason: "exit_code_1"}},
		},
	}

	applyDashboardFilter(data, AnalysisFilter{Category: "bash", Reason: "exit_code_1"})

	if len(data.DailyTrend.Counts) != 2 || data.DailyTrend.Counts[0] != 3 || data.DailyTrend.Counts[1] != 4 {
		t.Fatalf("daily trend = %+v, want [3 4]", data.DailyTrend)
	}
	if data.WeekdayStats == nil {
		t.Fatal("weekday stats should be rebuilt for reason filter")
	}
	if data.FailureAnalysis == nil || data.FailureAnalysis.TotalFailures != 7 {
		t.Fatalf("failure total = %+v, want reason aggregate total 7", data.FailureAnalysis)
	}
}

func TestApplyDashboardFilterRebuildsFailureTotalsFromToolReasons(t *testing.T) {
	data := &DashboardData{
		FailureAnalysis: &FailureAnalysisData{
			ByReason: []FailureReasonStat{
				{Category: "bash", Reason: "exit_code_1", Count: 9},
				{Category: "file", Reason: "not_found", Count: 3},
			},
			ByToolReason: []FailureToolReasonStat{
				{Tool: "Bash", Category: "bash", Reason: "exit_code_1", Count: 6},
				{Tool: "Bash", Category: "bash", Reason: "timeout", Count: 2},
				{Tool: "Read", Category: "file", Reason: "not_found", Count: 3},
			},
			Samples: []ToolFailureSample{{Tool: "Bash", Category: "bash", Reason: "exit_code_1"}},
		},
	}

	applyDashboardFilter(data, AnalysisFilter{Tool: "Bash"})

	if data.FailureAnalysis == nil {
		t.Fatal("failure analysis should remain available for tool filter")
	}
	if data.FailureAnalysis.TotalFailures != 8 {
		t.Fatalf("failure total = %d, want 8 from Bash by_tool_reason aggregate", data.FailureAnalysis.TotalFailures)
	}
	if len(data.FailureAnalysis.ByReason) != 2 {
		t.Fatalf("by reason = %+v, want two Bash reasons", data.FailureAnalysis.ByReason)
	}
}
