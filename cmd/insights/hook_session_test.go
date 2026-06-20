package main

import (
	"encoding/json"
	"testing"
)

// recordHookEventLocked + recordSessionHookLocked 后，hook→session（HookStats.SessionIDs）
// 与 session→hook（SessionStatsMap.HookBreakdown）双向数据都建立。
func TestHookSessionAssociationRecorded(t *testing.T) {
	agg := newProjectAggregate()
	recordHookEventLocked(agg, "hook_non_blocking_error", "Stop", "Stop", "bash x", "boom", 1500, "sess-1")
	recordSessionHookLocked(agg, "sess-1", "/p", "hook_non_blocking_error", "Stop", "Stop", "boom", 1500)
	recordHookEventLocked(agg, "hook_non_blocking_error", "Stop", "Stop", "bash x", "boom2", 2000, "sess-2")
	recordSessionHookLocked(agg, "sess-2", "/p", "hook_non_blocking_error", "Stop", "Stop", "boom2", 2000)
	recordHookEventLocked(agg, "hook_success", "PreToolUse", "PreToolUse", "", "", 10, "sess-1")
	recordSessionHookLocked(agg, "sess-1", "/p", "hook_success", "PreToolUse", "PreToolUse", "", 10)

	stopKey := "Stop\x00Stop"
	stopStat := agg.HookStats[stopKey]
	if stopStat == nil {
		t.Fatal("Stop hook 未记录")
	}
	if len(stopStat.SessionIDs) != 2 {
		t.Fatalf("Stop SessionIDs len=%d, want 2（hook→session）", len(stopStat.SessionIDs))
	}
	if stopStat.ErrorCount != 2 || stopStat.TotalCount != 2 {
		t.Fatalf("Stop count=(err=%d,total=%d), want (2,2)", stopStat.ErrorCount, stopStat.TotalCount)
	}

	sess1 := agg.SessionStatsMap["sess-1"]
	if sess1 == nil {
		t.Fatal("sess-1 未建立 session 统计")
	}
	stopBD := sess1.HookBreakdown["Stop"]
	if stopBD == nil || stopBD.ErrorCount != 1 {
		t.Fatalf("sess-1 Stop breakdown=%+v, want error=1（session→hook）", stopBD)
	}
	preBD := sess1.HookBreakdown["PreToolUse"]
	if preBD == nil || preBD.SuccessCount != 1 {
		t.Fatalf("sess-1 PreToolUse breakdown=%+v, want success=1", preBD)
	}
}

// 跨 aggregate 合并：两份各 record 不同 session 的同一 hook，合并后 SessionIDs 应去重合并。
// 覆盖 mergeProjectAggregate 的 nil 分支（深拷贝）与已存在分支（add）。
func TestHookSessionMergeAcrossAggregates(t *testing.T) {
	a := newProjectAggregate()
	b := newProjectAggregate()
	recordHookEventLocked(a, "hook_non_blocking_error", "Stop", "Stop", "", "", 100, "sess-1")
	recordHookEventLocked(b, "hook_non_blocking_error", "Stop", "Stop", "", "", 200, "sess-2")
	// b 再 record sess-1，验证合并去重不重复计数
	recordHookEventLocked(b, "hook_non_blocking_error", "Stop", "Stop", "", "", 300, "sess-1")

	mergeProjectAggregate(a, b)
	key := "Stop\x00Stop"
	merged := a.HookStats[key]
	if merged == nil {
		t.Fatal("合并后 Stop hook 丢失")
	}
	if len(merged.SessionIDs) != 2 {
		t.Fatalf("合并后 SessionIDs len=%d, want 2（去重后 sess-1+sess-2）", len(merged.SessionIDs))
	}
	if merged.ErrorCount != 3 {
		t.Fatalf("合并后 ErrorCount=%d, want 3", merged.ErrorCount)
	}
}

// 关键：JSON 快照往返（模拟缓存存盘→读盘）。SessionIDs/HookBreakdown 必须随序列化存活，
// 否则缓存路径（web /api/data、CLI ses 走 buildRecommendationDashboardData）字段全空、单测却绿。
func TestHookSessionSurvivesJSONSnapshotRoundTrip(t *testing.T) {
	agg := newProjectAggregate()
	recordHookEventLocked(agg, "hook_non_blocking_error", "Stop", "Stop", "bash x", "boom", 1500, "sess-1")
	recordSessionHookLocked(agg, "sess-1", "/p", "hook_non_blocking_error", "Stop", "Stop", "boom", 1500)
	recordHookEventLocked(agg, "hook_non_blocking_error", "Stop", "Stop", "bash x", "boom2", 2000, "sess-2")
	recordSessionHookLocked(agg, "sess-2", "/p", "hook_non_blocking_error", "Stop", "Stop", "boom2", 2000)

	snapshot := aggregateToProjectFileAggregate(agg)
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var restored ProjectFileAggregate
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	agg2 := projectFileAggregateToAggregate(restored)
	agg2.finalizeEventAnalysis()
	agg2.finalizeSessionAnalysis()

	// hook→session：Stop 涉及 2 个 session，且完整集合不随 EventAnalysis 输出。
	var stopStat *HookStatItem
	for i := range agg2.EventAnalysis.Hooks {
		if agg2.EventAnalysis.Hooks[i].HookName == "Stop" {
			stopStat = &agg2.EventAnalysis.Hooks[i]
		}
	}
	if stopStat == nil {
		t.Fatal("往返后 Stop hook 丢失（缓存路径丢字段）")
	}
	if stopStat.SessionCount != 2 {
		t.Fatalf("往返后 SessionCount=%d, want 2", stopStat.SessionCount)
	}
	if stopStat.ErrorCount != 2 || stopStat.FailureRate != 100 {
		t.Fatalf("往返后 Stop=(err=%d,rate=%v), want (2,100)", stopStat.ErrorCount, stopStat.FailureRate)
	}
	if len(stopStat.SampleSession) != 2 {
		t.Fatalf("往返后 SampleSession len=%d, want 2", len(stopStat.SampleSession))
	}
	if stopStat.SessionIDs != nil {
		t.Fatalf("SessionIDs 应在 finalize 后置 nil（不随输出），got %v", stopStat.SessionIDs)
	}

	// session→hook：sess-1 的 HookBreakdown 存活且 FailureRate 已算。
	var sess1 *SessionAnalysisItem
	for i := range agg2.SessionAnalysis.Sessions {
		if agg2.SessionAnalysis.Sessions[i].SessionID == "sess-1" {
			sess1 = &agg2.SessionAnalysis.Sessions[i]
		}
	}
	if sess1 == nil {
		t.Fatal("往返后 sess-1 丢失")
	}
	bd := sess1.HookBreakdown["Stop"]
	if bd == nil {
		t.Fatal("往返后 sess-1 HookBreakdown.Stop 丢失（缓存路径丢字段）")
	}
	if bd.ErrorCount != 1 || bd.FailureRate != 100 {
		t.Fatalf("往返后 sess-1 Stop=(err=%d,rate=%v), want (1,100)", bd.ErrorCount, bd.FailureRate)
	}
}
