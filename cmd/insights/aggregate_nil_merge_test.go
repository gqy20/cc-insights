package main

import (
	"testing"
	"time"
)

// TestMergeRestoredAggregateWithNilProneMaps 回归：projectFileAggregateToAggregate
// 曾把空 map 直接置 nil（copyIntMap 对空输入返回 nil），mergeProjectAggregate
// 再对 nil map 赋值会 panic。跨 provider 合并日期快照时一边有事件、一边为空即触发。
func TestMergeRestoredAggregateWithNilProneMaps(t *testing.T) {
	// dst：空快照（所有可 nil 字段为空），经序列化-还原路径。
	dst := projectFileAggregateToAggregate(ProjectFileAggregate{})

	// src：带 EventTypes / DailyActivity / DailyHourlyCounts 的正常聚合。
	src := newProjectAggregate()
	src.EventTypes["zcode:destructive_tool"] = 3
	src.DailyActivity["2026-06-15"] = 5
	src.SessionQueueOps["queue"] = 1
	dailyHourly := src.DailyHourlyCounts["2026-06-15"]
	dailyHourly[10] = 5
	src.DailyHourlyCounts["2026-06-15"] = dailyHourly

	mergeProjectAggregate(dst, src)

	if dst.EventTypes["zcode:destructive_tool"] != 3 {
		t.Fatalf("EventTypes = %v, want merged 3", dst.EventTypes)
	}
	if dst.DailyActivity["2026-06-15"] != 5 {
		t.Fatalf("DailyActivity = %v, want merged 5", dst.DailyActivity)
	}
	if dst.DailyHourlyCounts["2026-06-15"][10] != 5 {
		t.Fatalf("DailyHourlyCounts = %v, want merged 5", dst.DailyHourlyCounts)
	}
	if dst.SessionQueueOps["queue"] != 1 {
		t.Fatalf("SessionQueueOps = %v, want merged 1", dst.SessionQueueOps)
	}
}

// TestQueryByTimeRangeRestoresSessionTitles 标题是时间不变量：按天快照不携带，
// QueryByTimeRange 须从缓存存储的会话数据（SessionAnalysis + ProjectFiles）回填。
func TestQueryByTimeRangeRestoresSessionTitles(t *testing.T) {
	cf := &CacheFile{
		Version: CacheVersion,
		DailyStats: map[string]*DayAggregate{
			"2026-06-15": {Date: "2026-06-15", MessageCount: 2, SessionCount: 1},
		},
		DailyRuntime: map[string]ProjectFileAggregate{
			"2026-06-15": {
				DailyActivity: map[string]int{"2026-06-15": 2},
				SessionStats: map[string]SessionAnalysisItem{
					"s1": {SessionID: "s1", Project: "/repo", MessageCount: 2},
				},
			},
		},
		ProjectFiles: map[string]*ProjectFileCache{
			"projects/s1.jsonl": {
				Path: "projects/s1.jsonl",
				Aggregate: ProjectFileAggregate{
					SessionStats: map[string]SessionAnalysisItem{
						"s1": {SessionID: "s1", Title: "fix login bug", TitleSource: "custom-title"},
					},
				},
			},
		},
	}

	result := cf.QueryByTimeRange(time.Time{}, time.Now())
	if result.SessionAnalysis == nil {
		t.Fatal("SessionAnalysis should not be nil")
	}
	var found *SessionAnalysisItem
	for i := range result.SessionAnalysis.Sessions {
		if result.SessionAnalysis.Sessions[i].SessionID == "s1" {
			found = &result.SessionAnalysis.Sessions[i]
		}
	}
	if found == nil {
		t.Fatalf("session s1 missing from rebuilt sessions: %+v", result.SessionAnalysis.Sessions)
	}
	if found.Title != "fix login bug" || found.TitleSource != "custom-title" {
		t.Fatalf("title = %q/%q, want restored 'fix login bug'/'custom-title'", found.Title, found.TitleSource)
	}
}
