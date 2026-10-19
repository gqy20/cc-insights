package main

import (
	"testing"
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
