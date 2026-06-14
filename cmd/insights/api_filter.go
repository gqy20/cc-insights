package main

import (
	"sort"
	"strings"
	"time"
)

func buildDashboardDataWithFilter(filter AnalysisFilter) (*DashboardData, string, error) {
	data, source, err := buildDashboardData(filter.TimeFilter, filter.Preset)
	if err != nil {
		return nil, source, err
	}
	applyDashboardFilter(data, filter)
	return data, source, nil
}

func applyDashboardFilter(data *DashboardData, filter AnalysisFilter) {
	if data == nil || !filter.hasDimensionFilter() {
		return
	}
	applyDimensionTimeSeries(data, filter)
	filterProjects(data, filter.Project)
	filterModels(data, filter.Model)
	filterTools(data, filter.Tool)
	filterFailures(data, filter)
	filterSessions(data, filter)
	filterCommands(data, filter.Family)
	filterCosts(data, filter)
	filterPerformance(data, filter)
	filterSkills(data, filter)
	filterAgents(data, filter.Session)
	applyPrecisionGuard(data, filter)
	recomputeDashboardTotals(data)
}

func (filter AnalysisFilter) hasDimensionFilter() bool {
	return strings.TrimSpace(filter.Project) != "" ||
		strings.TrimSpace(filter.Session) != "" ||
		strings.TrimSpace(filter.Tool) != "" ||
		strings.TrimSpace(filter.Model) != "" ||
		strings.TrimSpace(filter.Category) != "" ||
		strings.TrimSpace(filter.Reason) != "" ||
		strings.TrimSpace(filter.Family) != ""
}

func filterProjects(data *DashboardData, project string) {
	if project == "" || data.ProjectStats == nil {
		return
	}
	data.ProjectStats.Projects = filterSlice(data.ProjectStats.Projects, func(item ProjectStatItem) bool {
		return matchContains(project, item.Project)
	})
}

func applyDimensionTimeSeries(data *DashboardData, filter AnalysisFilter) {
	if applyRuntimeDailyTrend(data, filter) {
		return
	}
	if filter.Tool != "" || filter.Reason != "" || filter.Category != "" || filter.Session != "" || filter.Family != "" || (filter.Project != "" && filter.Model != "") {
		clearUnscopedTimeSeries(data)
		if filter.Reason != "" || filter.Category != "" || filter.Session != "" || filter.Family != "" {
			data.MCPTools = nil
		}
		return
	}
	if filter.Project == "" && filter.Model == "" {
		return
	}
	dates := append([]string(nil), data.DailyTrend.Dates...)
	counts := make([]int, 0, len(dates))
	weekdayStats := &WeekdayStats{WeekdayData: make([]WeekdayItem, 7)}
	for i := range weekdayStats.WeekdayData {
		weekdayStats.WeekdayData[i] = WeekdayItem{Weekday: i, WeekdayName: weekdayName(i)}
	}
	if globalCache == nil {
		clearUnscopedTimeSeries(data)
		return
	}
	for _, date := range dates {
		day := globalCache.DailyStats[date]
		count := 0
		if day != nil {
			if filter.Project != "" {
				count = sumMatchingIntMap(day.ProjectCounts, filter.Project)
			} else if filter.Model != "" {
				count = sumMatchingIntMap(day.ModelCounts, filter.Model)
			}
		}
		counts = append(counts, count)
		if parsed, err := parseDateOnly(date); err == nil {
			weekday := (int(parsed.Weekday()) + 6) % 7
			weekdayStats.WeekdayData[weekday].MessageCount += count
		}
	}
	data.DailyTrend = DailyTrendData{Dates: dates, Counts: counts}
	data.WeekdayStats = weekdayStats
	data.HourlyCounts = map[string]int{}
	data.WorkHoursStats = nil
	if data.Sessions != nil {
		data.Sessions.PeakDate = ""
		data.Sessions.PeakCount = 0
		data.Sessions.ValleyDate = ""
		data.Sessions.ValleyCount = 0
		data.Sessions.DailySessionMap = map[string]int{}
	}
}

func applyRuntimeDailyTrend(data *DashboardData, filter AnalysisFilter) bool {
	if globalCache == nil || len(globalCache.DailyRuntime) == 0 {
		return false
	}
	if filter.Project != "" || filter.Session != "" || filter.Family != "" || (filter.Tool == "" && filter.Reason == "" && filter.Category == "") {
		return false
	}
	dates := append([]string(nil), data.DailyTrend.Dates...)
	if len(dates) == 0 {
		for date := range globalCache.DailyStats {
			dates = append(dates, date)
		}
		sort.Strings(dates)
	}
	counts := make([]int, 0, len(dates))
	weekdayStats := &WeekdayStats{WeekdayData: make([]WeekdayItem, 7)}
	for i := range weekdayStats.WeekdayData {
		weekdayStats.WeekdayData[i] = WeekdayItem{Weekday: i, WeekdayName: weekdayName(i)}
	}
	for _, date := range dates {
		count := dailyRuntimeCount(globalCache.DailyRuntime[date], filter)
		counts = append(counts, count)
		if parsed, err := parseDateOnly(date); err == nil {
			weekday := (int(parsed.Weekday()) + 6) % 7
			weekdayStats.WeekdayData[weekday].MessageCount += count
		}
	}
	data.Commands = nil
	data.HourlyCounts = map[string]int{}
	data.DailyTrend = DailyTrendData{Dates: dates, Counts: counts}
	data.WeekdayStats = weekdayStats
	data.WorkHoursStats = nil
	if data.Sessions != nil {
		data.Sessions.PeakDate = ""
		data.Sessions.PeakCount = 0
		data.Sessions.ValleyDate = ""
		data.Sessions.ValleyCount = 0
		data.Sessions.DailySessionMap = map[string]int{}
	}
	return true
}

func dailyRuntimeCount(snapshot ProjectFileAggregate, filter AnalysisFilter) int {
	if filter.Tool != "" {
		total := 0
		for _, item := range snapshot.ToolStats {
			if matchContains(filter.Tool, item.Tool) {
				total += item.CallCount
			}
		}
		return total
	}
	if filter.Reason != "" || filter.Category != "" {
		total := 0
		for _, item := range snapshot.FailureReasons {
			if matchEqual(filter.Category, item.Category) && matchEqual(filter.Reason, item.Reason) {
				total += item.Count
			}
		}
		return total
	}
	return 0
}

func clearUnscopedTimeSeries(data *DashboardData) {
	data.Commands = nil
	data.HourlyCounts = map[string]int{}
	data.DailyTrend = DailyTrendData{}
	data.WeekdayStats = nil
	data.WorkHoursStats = nil
	if data.Sessions != nil {
		data.Sessions.PeakDate = ""
		data.Sessions.PeakCount = 0
		data.Sessions.ValleyDate = ""
		data.Sessions.ValleyCount = 0
		data.Sessions.DailySessionMap = map[string]int{}
	}
}

func sumMatchingIntMap(items map[string]int, filter string) int {
	total := 0
	for key, value := range items {
		if matchContains(filter, key) {
			total += value
		}
	}
	return total
}

func parseDateOnly(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

func filterModels(data *DashboardData, model string) {
	if model == "" {
		return
	}
	data.ModelUsage = filterSlice(data.ModelUsage, func(item ModelUsageItem) bool {
		return matchContains(model, item.Model)
	})
	if data.ToolAnalysis != nil {
		data.ToolAnalysis.ByModel = filterSlice(data.ToolAnalysis.ByModel, func(item ToolModelStatItem) bool {
			return matchContains(model, item.Model)
		})
	}
	if data.FailureAnalysis != nil {
		data.FailureAnalysis.ByModelReason = filterSlice(data.FailureAnalysis.ByModelReason, func(item FailureModelReasonStat) bool {
			return matchContains(model, item.Model)
		})
		data.FailureAnalysis.Samples = filterSlice(data.FailureAnalysis.Samples, func(item ToolFailureSample) bool {
			return matchContains(model, item.Model)
		})
	}
}

func filterTools(data *DashboardData, tool string) {
	if tool == "" {
		return
	}
	data.MCPTools = filterSlice(data.MCPTools, func(item MCPToolStats) bool {
		return matchContains(tool, item.Tool) || matchContains(tool, item.Server)
	})
	if data.ToolAnalysis != nil {
		data.ToolAnalysis.Tools = filterSlice(data.ToolAnalysis.Tools, func(item ToolStatItem) bool {
			return matchContains(tool, item.Tool)
		})
		data.ToolAnalysis.ByModel = filterSlice(data.ToolAnalysis.ByModel, func(item ToolModelStatItem) bool {
			return matchContains(tool, item.Tool)
		})
	}
	if data.CommandAnalysis != nil {
		data.CommandAnalysis.FileOperations = filterSlice(data.CommandAnalysis.FileOperations, func(item FileOperationStat) bool {
			return matchContains(tool, item.Operation)
		})
	}
	if data.FailureAnalysis != nil {
		data.FailureAnalysis.ByToolReason = filterSlice(data.FailureAnalysis.ByToolReason, func(item FailureToolReasonStat) bool {
			return matchContains(tool, item.Tool)
		})
		data.FailureAnalysis.Samples = filterSlice(data.FailureAnalysis.Samples, func(item ToolFailureSample) bool {
			return matchContains(tool, item.Tool)
		})
	}
}

func filterFailures(data *DashboardData, filter AnalysisFilter) {
	if data.FailureAnalysis == nil {
		return
	}
	data.FailureAnalysis.ByReason = filterSlice(data.FailureAnalysis.ByReason, func(item FailureReasonStat) bool {
		return matchEqual(filter.Category, item.Category) && matchEqual(filter.Reason, item.Reason)
	})
	data.FailureAnalysis.ByToolReason = filterSlice(data.FailureAnalysis.ByToolReason, func(item FailureToolReasonStat) bool {
		return matchEqual(filter.Category, item.Category) && matchEqual(filter.Reason, item.Reason) && matchContains(filter.Tool, item.Tool)
	})
	data.FailureAnalysis.ByModelReason = filterSlice(data.FailureAnalysis.ByModelReason, func(item FailureModelReasonStat) bool {
		return matchEqual(filter.Category, item.Category) && matchEqual(filter.Reason, item.Reason) && matchContains(filter.Model, item.Model)
	})
	data.FailureAnalysis.Samples = filterSlice(data.FailureAnalysis.Samples, func(item ToolFailureSample) bool {
		return matchFailureSample(item, cliInspectFailureFilter{
			Reason:   filter.Reason,
			Category: filter.Category,
			Tool:     filter.Tool,
			Model:    filter.Model,
			Project:  filter.Project,
			Session:  filter.Session,
		})
	})
	data.FailureAnalysis.TotalFailures = sumFailureReasons(data.FailureAnalysis.ByReason)
}

func filterSessions(data *DashboardData, filter AnalysisFilter) {
	if data.SessionAnalysis == nil {
		return
	}
	sf := cliSessionReportFilter{Session: filter.Session, Project: filter.Project}
	data.SessionAnalysis.Sessions = filterSessionItems(data.SessionAnalysis.Sessions, sf)
	data.SessionAnalysis.TopFailures = filterSessionItems(data.SessionAnalysis.TopFailures, sf)
	data.SessionAnalysis.LongRunning = filterSessionItems(data.SessionAnalysis.LongRunning, sf)
	if filter.Session != "" {
		data.SessionAnalysis.QueueOperations = nil
	}
	if data.Sessions != nil {
		data.Sessions.TotalSessions = len(data.SessionAnalysis.Sessions)
	}
	if data.TaskPlanAnalysis != nil {
		data.TaskPlanAnalysis.ReminderSummary.TopTaskSessions = filterReminderSessions(data.TaskPlanAnalysis.ReminderSummary.TopTaskSessions, sf)
		data.TaskPlanAnalysis.ReminderSummary.TopTodoSessions = filterReminderSessions(data.TaskPlanAnalysis.ReminderSummary.TopTodoSessions, sf)
	}
}

func filterCommands(data *DashboardData, family string) {
	if family == "" || data.CommandAnalysis == nil {
		return
	}
	data.CommandAnalysis.BashFamilies = filterCommandFamilies(data.CommandAnalysis.BashFamilies, family)
	data.CommandAnalysis.BashCommands = filterSlice(data.CommandAnalysis.BashCommands, func(item BashCommandStat) bool {
		return strings.Contains(strings.ToLower(item.CommandName), strings.ToLower(family)) ||
			strings.Contains(strings.ToLower(item.SampleCommand), strings.ToLower(family))
	})
	data.CommandAnalysis.RiskyCommands = filterSlice(data.CommandAnalysis.RiskyCommands, func(item BashCommandStat) bool {
		return strings.Contains(strings.ToLower(item.CommandName), strings.ToLower(family)) ||
			strings.Contains(strings.ToLower(item.SampleCommand), strings.ToLower(family))
	})
}

func filterCosts(data *DashboardData, filter AnalysisFilter) {
	if data.CostAnalysis == nil {
		return
	}
	data.CostAnalysis.ByProject = filterCostProjects(data.CostAnalysis.ByProject, filter.Project)
	data.CostAnalysis.ByModel = filterCostModels(data.CostAnalysis.ByModel, filter.Model)
	data.CostAnalysis.BySession = filterSlice(data.CostAnalysis.BySession, func(item CostSessionStat) bool {
		return matchContains(filter.Session, item.SessionID) &&
			matchContains(filter.Project, item.Project) &&
			matchContains(filter.Model, item.Model)
	})
	recomputeCostTotals(data.CostAnalysis)
}

func filterPerformance(data *DashboardData, filter AnalysisFilter) {
	if data.ToolPerformance == nil {
		return
	}
	data.ToolPerformance.ByCategory = filterSlice(data.ToolPerformance.ByCategory, func(item ToolPerfCategoryItem) bool {
		return matchContains(filter.Tool, item.BaseTool) &&
			matchContains(filter.Category, item.Category)
	})
	data.ToolPerformance.SlowestCalls = filterSlice(data.ToolPerformance.SlowestCalls, func(item ToolSlowCallItem) bool {
		return matchContains(filter.Tool, item.Tool) &&
			matchContains(filter.Category, item.Category) &&
			matchContains(filter.Project, item.Project) &&
			matchEqual(filter.Session, item.SessionID) &&
			matchContains(filter.Model, item.Model)
	})
	recomputeToolPerformanceTotals(data.ToolPerformance)
}

func filterSkills(data *DashboardData, filter AnalysisFilter) {
	if data.SkillAnalysis == nil {
		return
	}
	if filter.Project != "" {
		data.SkillAnalysis.ByProject = filterSlice(data.SkillAnalysis.ByProject, func(item SkillProjectStat) bool {
			return matchContains(filter.Project, item.Project)
		})
	}
	if filter.Model != "" {
		data.SkillAnalysis.ByModel = filterSlice(data.SkillAnalysis.ByModel, func(item SkillModelStat) bool {
			return matchContains(filter.Model, item.Model)
		})
	}
	if filter.Tool != "" {
		data.SkillAnalysis.ToolChains = filterSlice(data.SkillAnalysis.ToolChains, func(item SkillToolChainStat) bool {
			return matchContains(filter.Tool, item.Tool)
		})
	}
}

func filterAgents(data *DashboardData, session string) {
	if session == "" || data.AgentAnalysis == nil {
		return
	}
	data.AgentAnalysis.Agents = filterSlice(data.AgentAnalysis.Agents, func(item AgentStatItem) bool {
		return matchContains(session, item.AgentID)
	})
}

func applyPrecisionGuard(data *DashboardData, filter AnalysisFilter) {
	if filter.Project != "" || filter.Tool != "" || filter.Reason != "" || filter.Category != "" || filter.Session != "" || filter.Family != "" {
		data.Commands = nil
		data.HourlyCounts = map[string]int{}
		data.WorkHoursStats = nil
		data.MCPTools = filterMCPToolsForPrecision(data.MCPTools, filter)
	}

	if filter.Project != "" || filter.Tool != "" || filter.Reason != "" || filter.Category != "" || filter.Session != "" || filter.Family != "" {
		data.ModelUsage = nil
	}
	if filter.Model != "" {
		rebuildToolTotalsFromByModel(data.ToolAnalysis)
	}

	if filter.Project != "" || filter.Session != "" {
		data.ToolAnalysis = nil
		data.ToolPerformance = nil
	}
	if filter.Tool != "" || filter.Reason != "" || filter.Category != "" || filter.Family != "" {
		data.ProjectStats = nil
	}
	if filter.Tool != "" || filter.Reason != "" || filter.Category != "" || filter.Family != "" {
		data.CostAnalysis = nil
	}
	if filter.Project != "" && data.CostAnalysis != nil {
		data.CostAnalysis.ByModel = nil
	}
	if filter.Tool != "" || filter.Reason != "" || filter.Category != "" || filter.Model != "" || filter.Family != "" {
		data.SessionAnalysis = nil
		data.Sessions = nil
	}
	if filter.Project != "" || filter.Tool != "" || filter.Reason != "" || filter.Category != "" || filter.Session != "" || filter.Model != "" || filter.Family != "" {
		data.FileAnalysis = nil
		data.EventAnalysis = nil
		data.AgentAnalysis = nil
		data.SkillAnalysis = nil
		data.TaskPlanAnalysis = nil
		normalizeFailureAnalysis(data, filter)
	}
}

func filterMCPToolsForPrecision(items []MCPToolStats, filter AnalysisFilter) []MCPToolStats {
	if filter.Tool == "" {
		return nil
	}
	return filterSlice(items, func(item MCPToolStats) bool {
		return matchContains(filter.Tool, item.Tool) || matchContains(filter.Tool, item.Server)
	})
}

func rebuildToolTotalsFromByModel(analysis *ToolAnalysisData) {
	if analysis == nil || len(analysis.ByModel) == 0 {
		return
	}
	byTool := map[string]ToolStatItem{}
	for _, item := range analysis.ByModel {
		current := byTool[item.Tool]
		current.Tool = item.Tool
		current.CallCount += item.CallCount
		current.SuccessCount += item.SuccessCount
		current.FailureCount += item.FailureCount
		current.MissingResultCount += item.MissingResultCount
		byTool[item.Tool] = current
	}
	analysis.Tools = analysis.Tools[:0]
	for _, item := range byTool {
		if item.CallCount > 0 {
			item.FailureRate = float64(item.FailureCount) / float64(item.CallCount) * 100
		}
		analysis.Tools = append(analysis.Tools, item)
	}
	sortToolStats(analysis.Tools)
}

func rebuildFailureBreakdownsFromSamples(analysis *FailureAnalysisData) {
	if analysis == nil {
		return
	}
	reasons := map[string]FailureReasonStat{}
	toolReasons := map[string]FailureToolReasonStat{}
	modelReasons := map[string]FailureModelReasonStat{}
	for _, sample := range analysis.Samples {
		category := nonEmpty(sample.Category, "unknown")
		reason := nonEmpty(sample.Reason, sample.Kind)
		reasonKey := category + "\x00" + reason
		reasonItem := reasons[reasonKey]
		reasonItem.Category = category
		reasonItem.Reason = reason
		reasonItem.Count++
		reasons[reasonKey] = reasonItem

		toolKey := sample.Tool + "\x00" + reasonKey
		toolItem := toolReasons[toolKey]
		toolItem.Tool = nonEmpty(sample.Tool, "unknown")
		toolItem.Category = category
		toolItem.Reason = reason
		toolItem.Count++
		toolReasons[toolKey] = toolItem

		modelKey := sample.Model + "\x00" + reasonKey
		modelItem := modelReasons[modelKey]
		modelItem.Model = nonEmpty(sample.Model, "unknown")
		modelItem.Category = category
		modelItem.Reason = reason
		modelItem.Count++
		modelReasons[modelKey] = modelItem
	}
	analysis.ByReason = analysis.ByReason[:0]
	for _, item := range reasons {
		analysis.ByReason = append(analysis.ByReason, item)
	}
	analysis.ByToolReason = analysis.ByToolReason[:0]
	for _, item := range toolReasons {
		analysis.ByToolReason = append(analysis.ByToolReason, item)
	}
	analysis.ByModelReason = analysis.ByModelReason[:0]
	for _, item := range modelReasons {
		analysis.ByModelReason = append(analysis.ByModelReason, item)
	}
	sortFailureBreakdowns(analysis)
	analysis.TotalFailures = len(analysis.Samples)
}

func normalizeFailureAnalysis(data *DashboardData, filter AnalysisFilter) {
	analysis := data.FailureAnalysis
	if analysis == nil {
		return
	}
	if filter.Family != "" {
		data.FailureAnalysis = nil
		return
	}
	if filter.Project != "" || filter.Session != "" || (filter.Tool != "" && filter.Model != "") {
		rebuildFailureBreakdownsFromSamples(analysis)
		return
	}
	if filter.Tool != "" {
		rebuildFailureBreakdownsFromToolReasons(analysis)
		return
	}
	if filter.Model != "" {
		rebuildFailureBreakdownsFromModelReasons(analysis)
		return
	}
	sortFailureBreakdowns(analysis)
	analysis.TotalFailures = sumFailureReasons(analysis.ByReason)
}

func rebuildFailureBreakdownsFromToolReasons(analysis *FailureAnalysisData) {
	reasons := map[string]FailureReasonStat{}
	total := 0
	for _, item := range analysis.ByToolReason {
		key := item.Category + "\x00" + item.Reason
		reason := reasons[key]
		reason.Category = item.Category
		reason.Reason = item.Reason
		reason.Count += item.Count
		reasons[key] = reason
		total += item.Count
	}
	analysis.ByReason = analysis.ByReason[:0]
	for _, item := range reasons {
		analysis.ByReason = append(analysis.ByReason, item)
	}
	analysis.ByModelReason = nil
	sortFailureBreakdowns(analysis)
	analysis.TotalFailures = total
}

func rebuildFailureBreakdownsFromModelReasons(analysis *FailureAnalysisData) {
	reasons := map[string]FailureReasonStat{}
	total := 0
	for _, item := range analysis.ByModelReason {
		key := item.Category + "\x00" + item.Reason
		reason := reasons[key]
		reason.Category = item.Category
		reason.Reason = item.Reason
		reason.Count += item.Count
		reasons[key] = reason
		total += item.Count
	}
	analysis.ByReason = analysis.ByReason[:0]
	for _, item := range reasons {
		analysis.ByReason = append(analysis.ByReason, item)
	}
	analysis.ByToolReason = nil
	sortFailureBreakdowns(analysis)
	analysis.TotalFailures = total
}

func sortFailureBreakdowns(analysis *FailureAnalysisData) {
	sort.SliceStable(analysis.ByReason, func(i, j int) bool {
		return analysis.ByReason[i].Count > analysis.ByReason[j].Count
	})
	sort.SliceStable(analysis.ByToolReason, func(i, j int) bool {
		return analysis.ByToolReason[i].Count > analysis.ByToolReason[j].Count
	})
	sort.SliceStable(analysis.ByModelReason, func(i, j int) bool {
		return analysis.ByModelReason[i].Count > analysis.ByModelReason[j].Count
	})
}

func recomputeDashboardTotals(data *DashboardData) {
	if data.ProjectStats != nil {
		totalMessages := 0
		totalSessions := 0
		for _, item := range data.ProjectStats.Projects {
			totalMessages += item.MessageCount
			totalSessions += item.SessionCount
		}
		if len(data.ProjectStats.Projects) > 0 {
			data.ProjectStats.TotalMessages = totalMessages
			data.ProjectStats.TotalSessions = totalSessions
		}
	}
	if data.ToolAnalysis != nil {
		totalCalls := 0
		totalFailures := 0
		missing := 0
		for _, item := range data.ToolAnalysis.Tools {
			totalCalls += item.CallCount
			totalFailures += item.FailureCount
			missing += item.MissingResultCount
		}
		if len(data.ToolAnalysis.Tools) > 0 {
			data.ToolAnalysis.TotalCalls = totalCalls
			data.ToolAnalysis.TotalFailures = totalFailures
			data.ToolAnalysis.MissingResults = missing
		}
	}
	if data.FailureAnalysis != nil {
		data.FailureAnalysis.TotalFailures = sumFailureReasons(data.FailureAnalysis.ByReason)
	}
}

func recomputeCostTotals(cost *CostAnalysisData) {
	totals := TokenUsageBreakdown{}
	for _, item := range cost.BySession {
		totals.RequestCount += item.RequestCount
		totals.InputTokens += item.InputTokens
		totals.OutputTokens += item.OutputTokens
		totals.TotalTokens += item.TotalTokens
	}
	if totals.TotalTokens == 0 {
		for _, item := range cost.ByModel {
			totals.RequestCount += item.RequestCount
			totals.InputTokens += item.InputTokens
			totals.OutputTokens += item.OutputTokens
			totals.CacheReadInputTokens += item.CacheReadTokens
			totals.CacheCreationInputTokens += item.CacheCreationTokens
			totals.ServerToolUseRequests += item.ServerToolUseRequests
			totals.TotalTokens += item.TotalTokens
		}
	}
	if totals.TotalTokens > 0 {
		cost.Totals = totals
	}
}

func recomputeToolPerformanceTotals(tp *ToolPerformanceData) {
	totalCalls := 0
	totalErrors := 0
	totalDuration := int64(0)
	for _, item := range tp.ByCategory {
		totalCalls += item.CallCount
		totalErrors += item.ErrorCount
		totalDuration += item.TotalDurationMs
	}
	if totalCalls == 0 {
		return
	}
	tp.TotalPairedCalls = totalCalls
	tp.TotalErrors = totalErrors
	tp.OverallErrorRate = float64(totalErrors) / float64(totalCalls) * 100
	tp.OverallAvgDuration = float64(totalDuration) / float64(totalCalls)
}

func sumFailureReasons(items []FailureReasonStat) int {
	total := 0
	for _, item := range items {
		total += item.Count
	}
	return total
}
