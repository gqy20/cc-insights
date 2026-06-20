package main

import (
	"sort"
	"time"
)

// maxSlowTurns 慢回合 Top-N 上限，与 SlowestCalls 的 maxSlowCalls 对齐。
const maxSlowTurns = 20

// recordTurnDurationLocked 累加一条 turn_duration 事件的耗时到 agg（project/daily/session 各层）。
// 来自 system/turn_duration 行的官方口径：用户发起一轮到 AI 回复完成的总时长（含工具执行）。
// model 留空：turn_duration 行不携带 model 字段，诊断时按 session 关联 PrimaryModel。
func recordTurnDurationLocked(agg *ProjectAggregate, durationMs int64, messageCount int, sessionID, projectName string, timestamp time.Time) {
	if durationMs <= 0 {
		return
	}
	agg.TurnCount++
	agg.TurnTotalDurationMs += durationMs
	if durationMs > agg.TurnMaxDurationMs {
		agg.TurnMaxDurationMs = durationMs
	}
	agg.SlowTurns = mergeSlowTurn(agg.SlowTurns, TurnSlowItem{
		SessionID:    sessionID,
		Project:      projectName,
		DurationMs:   durationMs,
		MessageCount: messageCount,
		Timestamp:    timestamp.Format(time.RFC3339),
	})
}

// mergeSlowTurn 维护按 DurationMs 降序的 Top-N 慢回合列表（与 SlowestCalls 的维护方式一致）。
func mergeSlowTurn(items []TurnSlowItem, item TurnSlowItem) []TurnSlowItem {
	if len(items) < maxSlowTurns {
		items = append(items, item)
		sortSlowTurnsDesc(items)
		return items
	}
	if item.DurationMs > items[len(items)-1].DurationMs {
		items[len(items)-1] = item
		sortSlowTurnsDesc(items)
	}
	return items
}

func sortSlowTurnsDesc(items []TurnSlowItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].DurationMs > items[j].DurationMs
	})
}

// mergeTurnStats 把 src 的 turn 耗时聚合合并到 dst（mergeProjectAggregate 调用）。
func mergeTurnStats(dst, src *ProjectAggregate) {
	dst.TurnCount += src.TurnCount
	dst.TurnTotalDurationMs += src.TurnTotalDurationMs
	if src.TurnMaxDurationMs > dst.TurnMaxDurationMs {
		dst.TurnMaxDurationMs = src.TurnMaxDurationMs
	}
	for _, st := range src.SlowTurns {
		dst.SlowTurns = mergeSlowTurn(dst.SlowTurns, st)
	}
}

// finalizeTurnDuration 把 turn 耗时中间聚合输出到 ToolPerformanceData。
// 独立于 finalizeToolPerformance：后者在无工具数据时提前 return，turn 数据需单独保证输出。
func (agg *ProjectAggregate) finalizeTurnDuration() {
	if agg.TurnCount == 0 {
		return
	}
	if agg.ToolPerformance == nil {
		agg.ToolPerformance = &ToolPerformanceData{}
	}
	out := agg.ToolPerformance
	out.TurnCount = agg.TurnCount
	out.AvgTurnDurationMs = float64(agg.TurnTotalDurationMs) / float64(agg.TurnCount)
	out.MaxTurnDurationMs = agg.TurnMaxDurationMs
	out.SlowTurns = append([]TurnSlowItem(nil), agg.SlowTurns...)
}
