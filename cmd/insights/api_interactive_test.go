package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseAnalysisFilter(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/diagnostics?preset=7d&limit=5&samples=2&detail=true&id=performance.slowest_call&project=demo&target=tool", nil)
	filter, err := parseAnalysisFilter(req)
	if err != nil {
		t.Fatalf("parseAnalysisFilter returned error: %v", err)
	}
	if filter.Preset != "7d" {
		t.Fatalf("Preset=%q, want 7d", filter.Preset)
	}
	if filter.Limit != 5 || filter.Samples != 2 {
		t.Fatalf("Limit/Samples=%d/%d, want 5/2", filter.Limit, filter.Samples)
	}
	if !filter.Detail {
		t.Fatal("Detail=false, want true")
	}
	if filter.ID != "performance.slowest_call" || filter.Project != "demo" || filter.Target != "tool" {
		t.Fatalf("unexpected filter: %+v", filter)
	}
}

func TestFilterDiagnosticFindings(t *testing.T) {
	items := []diagnosticFinding{
		{ID: "a", Severity: "high", Targets: []string{"tool"}, Evidence: []diagnosticEvidence{{Label: "项目", Value: "/tmp/demo"}}},
		{ID: "b", Severity: "low", Targets: []string{"workflow"}, Evidence: []diagnosticEvidence{{Label: "项目", Value: "/tmp/other"}}},
	}
	filtered := filterDiagnosticFindings(items, AnalysisFilter{Severity: "high", Target: "tool", Project: "demo"})
	if len(filtered) != 1 || filtered[0].ID != "a" {
		t.Fatalf("filtered=%+v, want only a", filtered)
	}
}

func TestBuildTimelineData(t *testing.T) {
	data := &DashboardData{
		DailyTrend: DailyTrendData{
			Dates:  []string{"2026-06-12", "2026-06-13"},
			Counts: []int{10, 20},
		},
		Sessions: &SessionStats{DailySessionMap: map[string]int{"2026-06-12": 1, "2026-06-13": 2}},
	}
	oldCache := globalCache
	globalCache = nil
	t.Cleanup(func() { globalCache = oldCache })

	timeline := buildTimelineData(data)
	if timeline.Start != "2026-06-12" || timeline.End != "2026-06-13" {
		t.Fatalf("range=%s..%s, want 2026-06-12..2026-06-13", timeline.Start, timeline.End)
	}
	if len(timeline.Days) != 2 || timeline.Days[1].Messages != 20 || timeline.Days[1].Sessions != 2 {
		t.Fatalf("timeline=%+v", timeline)
	}
}

func TestBuildOverviewDataUsesSessionAnalysisCount(t *testing.T) {
	data := &DashboardData{
		ProjectStats: &ProjectStatsData{
			TotalMessages: 100,
			TotalSessions: 3,
			Projects:      []ProjectStatItem{{Project: "/repo", MessageCount: 100}},
		},
		Sessions: &SessionStats{TotalSessions: 4},
		SessionAnalysis: &SessionAnalysisData{
			Sessions: []SessionAnalysisItem{
				{SessionID: "s1"},
				{SessionID: "s2"},
			},
		},
	}

	overview := buildOverviewData(data, cliRecommendationReport{}, AnalysisFilter{})
	if overview.Summary.Sessions != 2 {
		t.Fatalf("overview sessions=%d, want session_analysis count 2", overview.Summary.Sessions)
	}
}

func TestSendInteractiveJSONIncludesCoverageInMeta(t *testing.T) {
	rec := httptest.NewRecorder()
	filter := AnalysisFilter{Project: "/repo", Tool: "Bash"}

	sendInteractiveJSON(rec, map[string]string{"ok": "yes"}, "cache", TimeRangeInfo{Preset: "7d"}, filter, time.Now())

	var body interactiveAPIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success {
		t.Fatalf("success=false, body=%s", rec.Body.String())
	}
	if body.Meta.Coverage["toolModelFailureChart"].Status != "unavailable" {
		t.Fatalf("coverage=%+v, want tool model unavailable", body.Meta.Coverage["toolModelFailureChart"])
	}
	if body.Meta.Coverage["failureReasonChart"].Status != "sample" {
		t.Fatalf("coverage=%+v, want failure reason sample", body.Meta.Coverage["failureReasonChart"])
	}
}
