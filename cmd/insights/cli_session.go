package main

import "fmt"

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
	report.Insights = buildSessionInsights(report)
	return report
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

func buildSessionInsights(report cliSessionReport) []string {
	insights := []string{}
	if report.TotalSessions == 0 {
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
	return insights
}
