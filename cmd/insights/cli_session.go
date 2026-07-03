package main

import (
	"fmt"
	"sort"
)

func buildCLISessionReport(data *DashboardData, opts cliOptions) cliSessionReport {
	report := cliSessionReport{
		TimeRange: data.TimeRange,
		Filter: cliSessionReportFilter{
			Session: opts.Session,
			Project: opts.Project,
		},
	}
	if data.SessionAnalysis != nil {
		sessions := filterSessionItems(data.SessionAnalysis.Sessions, report.Filter)
		report.TotalSessions = len(sessions)
		report.LongRunning = limitSessionItems(filterSessionItems(data.SessionAnalysis.LongRunning, report.Filter), opts.Limit)
		report.TopFailures = limitSessionItems(filterSessionItems(data.SessionAnalysis.TopFailures, report.Filter), opts.Limit)
		report.SessionDiagnostics = buildSessionDiagnostics(data, report.LongRunning, report.TopFailures, report.Filter, opts.Limit)
		report.Outcomes = data.SessionAnalysis.Outcomes
		report.QueueOperations = limitQueueOperations(data.SessionAnalysis.QueueOperations, opts.Limit)
	}
	if data.TaskPlanAnalysis != nil {
		report.PlanLifecycle = data.TaskPlanAnalysis.PlanLifecycle
		report.ReminderSummary = data.TaskPlanAnalysis.ReminderSummary
		report.TopTaskSessions = limitReminderSessions(filterReminderSessions(data.TaskPlanAnalysis.ReminderSummary.TopTaskSessions, report.Filter), opts.Limit)
		report.TopTodoSessions = limitReminderSessions(filterReminderSessions(data.TaskPlanAnalysis.ReminderSummary.TopTodoSessions, report.Filter), opts.Limit)
	}
	if data.FailureAnalysis != nil {
		filter := cliInspectFailureFilter{Session: opts.Session, Project: opts.Project}
		var matched []ToolFailureSample
		for _, sample := range data.FailureAnalysis.Samples {
			if matchFailureSample(sample, filter) {
				matched = append(matched, sample)
			}
		}
		if len(matched) > opts.Samples {
			matched = matched[:opts.Samples]
		}
		report.FailureSamples = sanitizeFailureSamples(matched)
	}
	// session 过滤时显示该 session 的 hook 明细（session→hook）；否则显示全局 hook 统计（hook→session）。
	if opts.Session != "" && data.SessionAnalysis != nil {
		report.SessionHooks = sessionHooksForID(data.SessionAnalysis.Sessions, data.SessionAnalysis.TopFailures, data.SessionAnalysis.LongRunning, opts.Session, opts.Limit)
	} else if data.EventAnalysis != nil {
		report.Hooks = limitHookStats(data.EventAnalysis.Hooks, opts.Limit)
	}
	report.Insights = buildSessionInsights(report)
	return report
}

func buildSessionDiagnostics(data *DashboardData, longRunning, topFailures []SessionAnalysisItem, filter cliSessionReportFilter, limit int) []cliSessionDiagnostic {
	if data == nil {
		return nil
	}
	selected := make([]SessionAnalysisItem, 0, len(longRunning)+len(topFailures))
	seen := map[string]bool{}
	add := func(item SessionAnalysisItem) {
		if item.SessionID == "" || seen[item.SessionID] {
			return
		}
		if !matchEqual(filter.Session, item.SessionID) || !matchContains(filter.Project, item.Project) {
			return
		}
		seen[item.SessionID] = true
		selected = append(selected, item)
	}
	for _, item := range topFailures {
		add(item)
	}
	for _, item := range longRunning {
		add(item)
	}
	if len(selected) > limit {
		selected = selected[:limit]
	}

	failuresBySession := map[string][]ToolFailureSample{}
	if data.FailureAnalysis != nil {
		for _, sample := range data.FailureAnalysis.Samples {
			if sample.SessionID == "" || !seen[sample.SessionID] {
				continue
			}
			failuresBySession[sample.SessionID] = append(failuresBySession[sample.SessionID], sample)
		}
	}
	slowestBySession := map[string]ToolSlowCallItem{}
	if data.ToolPerformance != nil {
		for _, call := range data.ToolPerformance.SlowestCalls {
			if call.SessionID == "" || !seen[call.SessionID] {
				continue
			}
			prev, ok := slowestBySession[call.SessionID]
			if !ok || call.DurationMs > prev.DurationMs {
				slowestBySession[call.SessionID] = call
			}
		}
	}

	out := make([]cliSessionDiagnostic, 0, len(selected))
	for _, item := range selected {
		diag := cliSessionDiagnostic{
			SessionID:        item.SessionID,
			Project:          item.Project,
			DurationMs:       item.DurationMs,
			ToolCallCount:    item.ToolCallCount,
			ToolFailureCount: item.ToolFailureCount,
			TotalTokens:      item.TotalTokens,
			PrimaryModel:     item.PrimaryModel,
			Reasons:          topFailureReasonsForSession(failuresBySession[item.SessionID], 3),
			Tools:            topFailureToolsForSession(failuresBySession[item.SessionID], 3),
			Signals:          sessionDiagnosticSignals(item),
			NextSteps:        sessionDiagnosticNextSteps(item, data.TimeRange),
		}
		if item.ToolCallCount > 0 {
			diag.FailureRate = float64(item.ToolFailureCount) / float64(item.ToolCallCount) * 100
		}
		if slow, ok := slowestBySession[item.SessionID]; ok {
			diag.SlowestCall = &slow
		}
		out = append(out, diag)
	}
	return out
}

func topFailureReasonsForSession(samples []ToolFailureSample, limit int) []nameCount {
	counts := map[string]int{}
	for _, sample := range samples {
		key := nonEmpty(sample.Category, "unknown") + "/" + nonEmpty(sample.Reason, sample.Kind)
		counts[key]++
	}
	return topSessionNameCounts(counts, limit)
}

func topFailureToolsForSession(samples []ToolFailureSample, limit int) []nameCount {
	counts := map[string]int{}
	for _, sample := range samples {
		counts[nonEmpty(sample.Tool, "unknown")]++
	}
	return topSessionNameCounts(counts, limit)
}

func topSessionNameCounts(counts map[string]int, limit int) []nameCount {
	items := make([]nameCount, 0, len(counts))
	for name, count := range counts {
		items = append(items, nameCount{Name: name, Count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	if len(items) > limit {
		return append([]nameCount(nil), items[:limit]...)
	}
	return items
}

func sessionDiagnosticSignals(item SessionAnalysisItem) []string {
	signals := []string{}
	if item.DurationMs >= int64(60*60*1000) {
		signals = append(signals, "long_session")
	}
	if item.ToolCallCount > 0 {
		rate := float64(item.ToolFailureCount) / float64(item.ToolCallCount) * 100
		if item.ToolFailureCount >= 10 || rate >= 20 {
			signals = append(signals, "high_failure_rate")
		}
	}
	if item.PlanModeCount != item.PlanModeExitCount {
		signals = append(signals, "plan_lifecycle_unbalanced")
	}
	if item.HookErrorCount > 0 {
		signals = append(signals, "hook_errors")
	}
	if item.QueueOperationCount > 0 {
		signals = append(signals, "queue_operations")
	}
	if item.MissingResultCount > 0 {
		signals = append(signals, "missing_tool_results")
	}
	return signals
}

func sessionDiagnosticNextSteps(item SessionAnalysisItem, tr TimeRangeInfo) []diagnosticCommand {
	rangeArg := sessionDrilldownRangeArg(tr)
	steps := []diagnosticCommand{
		{Label: "Session 详情", Command: fmt.Sprintf("cc-insights ses %s --session %s -n 10", rangeArg, item.SessionID)},
		{Label: "失败样例", Command: fmt.Sprintf("cc-insights why %s --session %s -n 10", rangeArg, item.SessionID)},
	}
	if item.ToolFailureCount > 0 {
		steps = append(steps, diagnosticCommand{Label: "失败聚合", Command: fmt.Sprintf("cc-insights err %s -n 20", rangeArg)})
	}
	return steps
}

func sessionDrilldownRangeArg(tr TimeRangeInfo) string {
	if tr.Preset == "custom" && tr.Start != "" && tr.End != "" {
		return fmt.Sprintf("--start %s --end %s", tr.Start, tr.End)
	}
	if tr.Preset != "" {
		return "-p " + tr.Preset
	}
	return "-p 7d"
}

func filterSessionItems(items []SessionAnalysisItem, filter cliSessionReportFilter) []SessionAnalysisItem {
	if filter.Session == "" && filter.Project == "" {
		return append([]SessionAnalysisItem(nil), items...)
	}
	filtered := make([]SessionAnalysisItem, 0, len(items))
	for _, item := range items {
		if !matchEqual(filter.Session, item.SessionID) {
			continue
		}
		if !matchContains(filter.Project, item.Project) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterReminderSessions(items []ReminderSessionItem, filter cliSessionReportFilter) []ReminderSessionItem {
	if filter.Session == "" && filter.Project == "" {
		return append([]ReminderSessionItem(nil), items...)
	}
	filtered := make([]ReminderSessionItem, 0, len(items))
	for _, item := range items {
		if !matchEqual(filter.Session, item.SessionID) {
			continue
		}
		if !matchContains(filter.Project, item.Project) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func limitSessionItems(items []SessionAnalysisItem, limit int) []SessionAnalysisItem {
	if len(items) > limit {
		return append([]SessionAnalysisItem(nil), items[:limit]...)
	}
	return append([]SessionAnalysisItem(nil), items...)
}

func limitReminderSessions(items []ReminderSessionItem, limit int) []ReminderSessionItem {
	if len(items) > limit {
		return append([]ReminderSessionItem(nil), items[:limit]...)
	}
	return append([]ReminderSessionItem(nil), items...)
}

func limitQueueOperations(items []QueueOperationStat, limit int) []QueueOperationStat {
	if len(items) > limit {
		return append([]QueueOperationStat(nil), items[:limit]...)
	}
	return append([]QueueOperationStat(nil), items...)
}

// limitHookStats 截取 Top N hook 统计；finalize 已按（错误数, 总次数）排序。
func limitHookStats(items []HookStatItem, limit int) []HookStatItem {
	if len(items) > limit {
		return append([]HookStatItem(nil), items[:limit]...)
	}
	return append([]HookStatItem(nil), items...)
}

// sortHookBreakdown 把 session 的 HookBreakdown map 转成按（错误数, 总次数）排序的切片。
func sortHookBreakdown(breakdown map[string]*SessionHookStat) []SessionHookStat {
	out := make([]SessionHookStat, 0, len(breakdown))
	for _, h := range breakdown {
		out = append(out, *h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ErrorCount != out[j].ErrorCount {
			return out[i].ErrorCount > out[j].ErrorCount
		}
		return out[i].TotalCount > out[j].TotalCount
	})
	return out
}

func limitSessionHooks(items []SessionHookStat, limit int) []SessionHookStat {
	if len(items) > limit {
		return append([]SessionHookStat(nil), items[:limit]...)
	}
	return append([]SessionHookStat(nil), items...)
}

// sessionHooksForID 找到指定 session 的 hook 明细（session→hook），已排序并截断。
// 在 Sessions/TopFailures/LongRunning 三处合并查找，规避 limitSessionAnalysis 截断导致目标 session
// 不在单一列表（如某 session 在 TopFailures 却被 Sessions 的 100 上限截掉）的情况。
func sessionHooksForID(sessions, topFailures, longRunning []SessionAnalysisItem, sessionID string, limit int) []SessionHookStat {
	for _, pool := range [][]SessionAnalysisItem{sessions, topFailures, longRunning} {
		for _, s := range pool {
			if matchEqual(sessionID, s.SessionID) {
				return limitSessionHooks(sortHookBreakdown(s.HookBreakdown), limit)
			}
		}
	}
	return nil
}

func buildSessionInsights(report cliSessionReport) []string {
	insights := []string{}
	if report.TotalSessions == 0 {
		// hooks 是全局 event 维度，不受 session/project 过滤约束：有失败时仍单独报告。
		if msg := hookFailureInsight(report.Hooks); msg != "" {
			return []string{msg}
		}
		return []string{"当前过滤条件没有匹配到 session。"}
	}
	insights = append(insights, fmt.Sprintf("匹配到 %s 个 session。", formatInt(report.TotalSessions)))
	if len(report.LongRunning) > 0 {
		top := report.LongRunning[0]
		insights = append(insights, fmt.Sprintf("最长 session 是 %s，耗时 %s。", top.SessionID, formatDurationMs(top.DurationMs)))
	}
	if len(report.TopFailures) > 0 {
		top := report.TopFailures[0]
		insights = append(insights, fmt.Sprintf("失败最多 session 是 %s，工具失败 %s 次。", top.SessionID, formatInt(top.ToolFailureCount)))
	}
	if report.PlanLifecycle.EntryCount > 0 {
		insights = append(insights, fmt.Sprintf("Plan 进入 %s 次，退出 %s 次。", formatInt(report.PlanLifecycle.EntryCount), formatInt(report.PlanLifecycle.ExitCount)))
	}
	if msg := hookFailureInsight(report.Hooks); msg != "" {
		insights = append(insights, msg)
	}
	if msg := sessionHookFailureInsight(report.SessionHooks); msg != "" {
		insights = append(insights, msg)
	}
	return insights
}

// hookFailureInsight 返回失败数最多的 hook 的洞察文案；无失败时返回空串。
// hooks 已由 finalize 按（错误数, 总次数）排序，故取首项。
func hookFailureInsight(hooks []HookStatItem) string {
	if len(hooks) == 0 || hooks[0].ErrorCount == 0 {
		return ""
	}
	top := hooks[0]
	return fmt.Sprintf("最不稳定 hook 是 %s，失败 %s 次（失败率 %.1f%%）。", top.HookName, formatInt(top.ErrorCount), top.FailureRate)
}

// sessionHookFailureInsight 针对单 session 的 hook 明细给出洞察。
func sessionHookFailureInsight(hooks []SessionHookStat) string {
	if len(hooks) == 0 || hooks[0].ErrorCount == 0 {
		return ""
	}
	top := hooks[0]
	return fmt.Sprintf("该 session 中 %s hook 失败 %s 次（失败率 %.1f%%）。", top.HookName, formatInt(top.ErrorCount), top.FailureRate)
}
