package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDashboardDataConsistencyFromScopedCache(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "consistency-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Create project dir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "debug"), 0755); err != nil {
		t.Fatalf("Create debug dir failed: %v", err)
	}

	oldTime := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 2, 11, 0, 0, 0, time.UTC)
	content := projectRecordJSON("/tmp/consistency-old", "old-session", oldTime) + "\n" +
		toolUseRecordWithInput("/tmp/consistency-new", "new-session", newTime, "new-bash", "Bash", "new-model", false, "", `{"command":"go test ./..."}`) + "\n" +
		toolResultRecord("/tmp/consistency-new", "new-session", newTime.Add(time.Second), "new-bash", "ok") + "\n" +
		toolUseRecordWithInput("/tmp/consistency-new", "new-session", newTime.Add(2*time.Second), "new-mcp", "mcp__crawl__extract_url", "new-model", false, "", `{"url":"https://example.com"}`) + "\n" +
		toolResultRecord("/tmp/consistency-new", "new-session", newTime.Add(3*time.Second), "new-mcp", "ok") + "\n"
	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("Write project jsonl failed: %v", err)
	}

	historyContent := `{"display":"/new","timestamp":` + formatUnixMilli(newTime) + `,"project":"consistency-project"}` + "\n"
	if err := os.WriteFile(filepath.Join(dataDir, "history.jsonl"), []byte(historyContent), 0644); err != nil {
		t.Fatalf("Write history failed: %v", err)
	}

	cachePath := filepath.Join(tmpDir, "cache.db")
	builder := &CacheBuilder{CachePath: cachePath, DataDir: dataDir}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("BuildFullCache failed: %v", err)
	}
	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("LoadCacheFile failed: %v", err)
	}

	originalCache := globalCache
	originalDataDir := cfg.DataDir
	globalCache = cache
	cfg.DataDir = dataDir
	defer func() {
		globalCache = originalCache
		cfg.DataDir = originalDataDir
	}()

	start := newTime.Format("2006-01-02")
	end := newTime.Format("2006-01-02")
	tf, err := NewTimeFilterCustom(start, end)
	if err != nil {
		t.Fatalf("NewTimeFilterCustom failed: %v", err)
	}
	data, err := buildDataFromCache(tf, "custom")
	if err != nil {
		t.Fatalf("buildDataFromCache failed: %v", err)
	}

	report := buildDashboardConsistencyReport(data)
	if len(report.Issues) > 0 {
		t.Fatalf("Consistency issues: %v; checks=%+v", report.Issues, report.Checks)
	}
	if report.Checks["total_messages"] != 2 {
		t.Fatalf("total_messages=%d, want 2; checks=%+v", report.Checks["total_messages"], report.Checks)
	}
	if report.Checks["tool_total_calls"] != 2 {
		t.Fatalf("tool_total_calls=%d, want 2; checks=%+v", report.Checks["tool_total_calls"], report.Checks)
	}
	if report.Checks["runtime_tool_sum"] != 1 {
		t.Fatalf("runtime_tool_sum=%d, want 1; checks=%+v", report.Checks["runtime_tool_sum"], report.Checks)
	}
	if report.Checks["cost_request_count"] != 2 {
		t.Fatalf("cost_request_count=%d, want 2; checks=%+v", report.Checks["cost_request_count"], report.Checks)
	}
}

func TestDashboardDataConsistencyDetectsMismatchedTotals(t *testing.T) {
	data := &DashboardData{
		DailyTrend:   DailyTrendData{Counts: []int{2}},
		HourlyCounts: map[string]int{"10": 5},
		ProjectStats: &ProjectStatsData{
			TotalMessages: 2,
			Projects: []ProjectStatItem{
				{Project: "demo", MessageCount: 2},
			},
		},
		ModelUsage: []ModelUsageItem{{Model: "model", Count: 2}},
		WeekdayStats: &WeekdayStats{WeekdayData: []WeekdayItem{
			{Weekday: 0, MessageCount: 2},
		}},
		WorkHoursStats: &WorkHoursStats{
			HourlyData:     []HourlyItem{{Hour: 10, Count: 2}},
			WorkHoursCount: 2,
		},
		CostAnalysis: &CostAnalysisData{
			Totals:  TokenUsageBreakdown{RequestCount: 2},
			ByModel: []CostModelStat{{Model: "model", RequestCount: 2}},
		},
	}

	issues := validateDashboardDataConsistency(data)
	if len(issues) == 0 {
		t.Fatal("Expected consistency issues, got none")
	}
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "hourly_sum=5 != total_messages=2") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected hourly mismatch issue, got %v", issues)
	}
}

func formatUnixMilli(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}
