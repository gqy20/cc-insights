package main

import (
	"sort"
	"sync"
)

func newProjectAggregate() *ProjectAggregate {
	aggregate := &ProjectAggregate{
		ProjectStats:        make(map[string]*ProjectStatItem),
		DailyActivity:       make(map[string]int),
		DailySessions:       make(map[string]map[string]bool),
		ModelUsage:          make(map[string]*ModelUsageItem),
		ToolStats:           make(map[string]*ToolStatItem),
		ToolModelStats:      make(map[string]*ToolModelStatItem),
		FailureReasons:      make(map[string]*FailureReasonStat),
		FailureToolReasons:  make(map[string]*FailureToolReasonStat),
		FailureModelReasons: make(map[string]*FailureModelReasonStat),
		SessionStatsMap:     make(map[string]*SessionAnalysisItem),
		SessionQueueOps:     make(map[string]int),
		EventTypes:          make(map[string]int),
		HookStats:           make(map[string]*HookStatItem),
		SkillStats:          make(map[string]*SkillStatItem),
		PermissionModes:     make(map[string]int),
		OpenedFiles:         make(map[string]*FileAccessStat),
		AgentStats:          make(map[string]*AgentStatItem),
		AgentSessions:       make(map[string]map[string]bool),
		BashCommandStats:    make(map[string]*BashCommandStat),
		FileOperationStats:  make(map[string]*FileOperationStat),
		HourlyCounts:        [24]int{},
		CostModelStats:      make(map[string]*CostModelStat),
		CostProjectStats:    make(map[string]*CostProjectStat),
		CostSessionStats:    make(map[string]*CostSessionStat),
		CostAgentStats:      make(map[string]*CostAgentStat),
		mu:                  sync.RWMutex{},
	}

	// 初始化星期数据
	weekdayNames := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	for i := 0; i < 7; i++ {
		aggregate.WeekdayData[i] = WeekdayItem{
			Weekday:      i,
			WeekdayName:  weekdayNames[i],
			MessageCount: 0,
		}
	}
	return aggregate
}

func mergeProjectAggregate(dst, src *ProjectAggregate) {
	for project, stat := range src.ProjectStats {
		if dst.ProjectStats[project] == nil {
			dst.ProjectStats[project] = &ProjectStatItem{Project: project}
		}
		dst.ProjectStats[project].MessageCount += stat.MessageCount
		dst.ProjectStats[project].SessionCount += stat.SessionCount
	}
	for i := range src.WeekdayData {
		dst.WeekdayData[i].MessageCount += src.WeekdayData[i].MessageCount
	}
	for date, count := range src.DailyActivity {
		dst.DailyActivity[date] += count
	}
	for date, sessions := range src.DailySessions {
		if dst.DailySessions[date] == nil {
			dst.DailySessions[date] = make(map[string]bool)
		}
		for sessionID := range sessions {
			dst.DailySessions[date][sessionID] = true
		}
	}
	for hour, count := range src.HourlyCounts {
		dst.HourlyCounts[hour] += count
	}
	for model, stat := range src.ModelUsage {
		if dst.ModelUsage[model] == nil {
			dst.ModelUsage[model] = &ModelUsageItem{Model: model}
		}
		dst.ModelUsage[model].Count += stat.Count
		dst.ModelUsage[model].Tokens += stat.Tokens
	}
	for model, stat := range src.CostModelStats {
		dstStat := ensureCostModelStat(dst, model)
		mergeCostModelStat(dstStat, stat)
	}
	for project, stat := range src.CostProjectStats {
		dstStat := ensureCostProjectStat(dst, project)
		mergeCostProjectStat(dstStat, stat)
	}
	for sessionID, stat := range src.CostSessionStats {
		dstStat := ensureCostSessionStat(dst, sessionID, stat.Project)
		if dstStat.Model == "" {
			dstStat.Model = stat.Model
		}
		mergeCostSessionStat(dstStat, stat)
	}
	for agentID, stat := range src.CostAgentStats {
		dstStat := ensureCostAgentStat(dst, agentID, stat.IsSidechain)
		mergeCostAgentStat(dstStat, stat)
	}
	for tool, stat := range src.ToolStats {
		dstStat := ensureToolStat(dst, tool)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
	for _, stat := range src.ToolModelStats {
		dstStat := ensureToolModelStat(dst, stat.Tool, stat.Model)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
	for key, stat := range src.FailureReasons {
		dstStat := ensureFailureReasonStat(dst, stat.Category, stat.Reason)
		dstStat.Count += stat.Count
		if key == "" {
			continue
		}
	}
	for _, stat := range src.FailureToolReasons {
		dstStat := ensureFailureToolReasonStat(dst, stat.Tool, stat.Category, stat.Reason)
		dstStat.Count += stat.Count
	}
	for _, stat := range src.FailureModelReasons {
		dstStat := ensureFailureModelReasonStat(dst, stat.Model, stat.Category, stat.Reason)
		dstStat.Count += stat.Count
	}
	remainingFailures := 30 - len(dst.FailureSamples)
	if remainingFailures > 0 {
		samples := src.FailureSamples
		if len(samples) > remainingFailures {
			samples = samples[:remainingFailures]
		}
		dst.FailureSamples = append(dst.FailureSamples, samples...)
	}
	for sessionID, stat := range src.SessionStatsMap {
		dstStat := ensureSessionStat(dst, sessionID, stat.Project)
		mergeSessionStat(dstStat, stat)
	}
	for operation, count := range src.SessionQueueOps {
		dst.SessionQueueOps[operation] += count
	}
	for eventType, count := range src.EventTypes {
		dst.EventTypes[eventType] += count
	}
	for key, stat := range src.HookStats {
		if dst.HookStats[key] == nil {
			statCopy := *stat
			dst.HookStats[key] = &statCopy
			continue
		}
		dstStat := dst.HookStats[key]
		oldTotal := dstStat.TotalCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.CancelledCount += stat.CancelledCount
		dstStat.ErrorCount += stat.ErrorCount
		dstStat.TotalCount += stat.TotalCount
		if dstStat.TotalCount > 0 {
			dstStat.AvgDurationMs = (dstStat.AvgDurationMs*float64(oldTotal) + stat.AvgDurationMs*float64(stat.TotalCount)) / float64(dstStat.TotalCount)
		}
		if stat.LastError != "" {
			dstStat.LastError = stat.LastError
		}
		if stat.LastCommand != "" {
			dstStat.LastCommand = stat.LastCommand
		}
	}
	for name, stat := range src.SkillStats {
		if dst.SkillStats[name] == nil {
			dst.SkillStats[name] = &SkillStatItem{Name: stat.Name, Path: stat.Path}
		}
		dst.SkillStats[name].Count += stat.Count
		if dst.SkillStats[name].Path == "" {
			dst.SkillStats[name].Path = stat.Path
		}
	}
	for mode, count := range src.PermissionModes {
		dst.PermissionModes[mode] += count
	}
	for path, stat := range src.OpenedFiles {
		if dst.OpenedFiles[path] == nil {
			dst.OpenedFiles[path] = &FileAccessStat{Path: path}
		}
		dst.OpenedFiles[path].Count += stat.Count
	}
	if src.BudgetSummary != nil {
		if dst.BudgetSummary == nil {
			budgetCopy := *src.BudgetSummary
			dst.BudgetSummary = &budgetCopy
		} else {
			dst.BudgetSummary.LatestUsed = src.BudgetSummary.LatestUsed
			dst.BudgetSummary.LatestTotal = src.BudgetSummary.LatestTotal
			dst.BudgetSummary.LatestRemaining = src.BudgetSummary.LatestRemaining
			dst.BudgetSummary.EventCount += src.BudgetSummary.EventCount
			if src.BudgetSummary.MaxUsed > dst.BudgetSummary.MaxUsed {
				dst.BudgetSummary.MaxUsed = src.BudgetSummary.MaxUsed
			}
		}
	}
	remainingBudgetEvents := 200 - len(dst.BudgetTimeline)
	if remainingBudgetEvents > 0 {
		timeline := src.BudgetTimeline
		if len(timeline) > remainingBudgetEvents {
			timeline = timeline[:remainingBudgetEvents]
		}
		dst.BudgetTimeline = append(dst.BudgetTimeline, timeline...)
	}
	remainingEvents := 40 - len(dst.EventSamples)
	if remainingEvents > 0 {
		samples := src.EventSamples
		if len(samples) > remainingEvents {
			samples = samples[:remainingEvents]
		}
		dst.EventSamples = append(dst.EventSamples, samples...)
	}
	for agentID, stat := range src.AgentStats {
		dstStat := ensureAgentStat(dst, agentID, stat.IsSidechain)
		if dstStat.AgentName == "" {
			dstStat.AgentName = stat.AgentName
		}
		dstStat.MessageCount += stat.MessageCount
		dstStat.ToolCallCount += stat.ToolCallCount
		dstStat.ToolFailureCount += stat.ToolFailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
	for agentID, sessions := range src.AgentSessions {
		if dst.AgentSessions[agentID] == nil {
			dst.AgentSessions[agentID] = make(map[string]bool)
		}
		dstStat := ensureAgentStat(dst, agentID, src.AgentStats[agentID].IsSidechain)
		for sessionID := range sessions {
			if !dst.AgentSessions[agentID][sessionID] {
				dstStat.SessionCount++
			}
			dst.AgentSessions[agentID][sessionID] = true
		}
	}
	for commandName, stat := range src.BashCommandStats {
		dstStat := ensureBashCommandStat(dst, commandName)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
		if dstStat.RiskLevel == "" || riskRank(stat.RiskLevel) > riskRank(dstStat.RiskLevel) {
			dstStat.RiskLevel = stat.RiskLevel
			dstStat.RiskReason = stat.RiskReason
		}
		if dstStat.SampleCommand == "" {
			dstStat.SampleCommand = stat.SampleCommand
		}
	}
	for key, stat := range src.FileOperationStats {
		if dst.FileOperationStats[key] == nil {
			dst.FileOperationStats[key] = &FileOperationStat{Operation: stat.Operation, Path: stat.Path}
		}
		dstStat := dst.FileOperationStats[key]
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
}

func aggregateToProjectFileAggregate(src *ProjectAggregate) ProjectFileAggregate {
	out := ProjectFileAggregate{
		ProjectStats:        make(map[string]ProjectStatItem, len(src.ProjectStats)),
		WeekdayData:         src.WeekdayData,
		DailyActivity:       copyIntMap(src.DailyActivity),
		DailySessions:       boolSetMapToSlices(src.DailySessions),
		HourlyCounts:        src.HourlyCounts,
		ModelUsage:          make(map[string]ModelUsageItem, len(src.ModelUsage)),
		CostModelStats:      make(map[string]CostModelStat, len(src.CostModelStats)),
		CostProjectStats:    make(map[string]CostProjectStat, len(src.CostProjectStats)),
		CostSessionStats:    make(map[string]CostSessionStat, len(src.CostSessionStats)),
		CostAgentStats:      make(map[string]CostAgentStat, len(src.CostAgentStats)),
		BudgetTimeline:      append([]BudgetTimelineItem(nil), src.BudgetTimeline...),
		ToolStats:           make(map[string]ToolStatItem, len(src.ToolStats)),
		ToolModelStats:      make(map[string]ToolModelStatItem, len(src.ToolModelStats)),
		FailureReasons:      make(map[string]FailureReasonStat, len(src.FailureReasons)),
		FailureToolReasons:  make(map[string]FailureToolReasonStat, len(src.FailureToolReasons)),
		FailureModelReasons: make(map[string]FailureModelReasonStat, len(src.FailureModelReasons)),
		FailureSamples:      append([]ToolFailureSample(nil), src.FailureSamples...),
		SessionStats:        make(map[string]SessionAnalysisItem, len(src.SessionStatsMap)),
		SessionQueueOps:     copyIntMap(src.SessionQueueOps),
		EventTypes:          copyIntMap(src.EventTypes),
		HookStats:           make(map[string]HookStatItem, len(src.HookStats)),
		SkillStats:          make(map[string]SkillStatItem, len(src.SkillStats)),
		PermissionModes:     copyIntMap(src.PermissionModes),
		OpenedFiles:         make(map[string]FileAccessStat, len(src.OpenedFiles)),
		AgentStats:          make(map[string]AgentStatItem, len(src.AgentStats)),
		AgentSessions:       boolSetMapToSlices(src.AgentSessions),
		BashCommandStats:    make(map[string]BashCommandStat, len(src.BashCommandStats)),
		FileOperationStats:  make(map[string]FileOperationStat, len(src.FileOperationStats)),
	}
	for key, stat := range src.ProjectStats {
		out.ProjectStats[key] = *stat
	}
	for key, stat := range src.ModelUsage {
		out.ModelUsage[key] = *stat
	}
	for key, stat := range src.CostModelStats {
		out.CostModelStats[key] = *stat
	}
	for key, stat := range src.CostProjectStats {
		out.CostProjectStats[key] = *stat
	}
	for key, stat := range src.CostSessionStats {
		out.CostSessionStats[key] = *stat
	}
	for key, stat := range src.CostAgentStats {
		out.CostAgentStats[key] = *stat
	}
	for key, stat := range src.ToolStats {
		out.ToolStats[key] = *stat
	}
	for key, stat := range src.ToolModelStats {
		out.ToolModelStats[key] = *stat
	}
	for key, stat := range src.FailureReasons {
		out.FailureReasons[key] = *stat
	}
	for key, stat := range src.FailureToolReasons {
		out.FailureToolReasons[key] = *stat
	}
	for key, stat := range src.FailureModelReasons {
		out.FailureModelReasons[key] = *stat
	}
	for key, stat := range src.SessionStatsMap {
		out.SessionStats[key] = *stat
	}
	for key, stat := range src.HookStats {
		out.HookStats[key] = *stat
	}
	for key, stat := range src.SkillStats {
		out.SkillStats[key] = *stat
	}
	for key, stat := range src.OpenedFiles {
		out.OpenedFiles[key] = *stat
	}
	if src.BudgetSummary != nil {
		budgetCopy := *src.BudgetSummary
		out.BudgetSummary = &budgetCopy
	}
	out.EventSamples = append([]EventSample(nil), src.EventSamples...)
	for key, stat := range src.AgentStats {
		out.AgentStats[key] = *stat
	}
	for key, stat := range src.BashCommandStats {
		out.BashCommandStats[key] = *stat
	}
	for key, stat := range src.FileOperationStats {
		out.FileOperationStats[key] = *stat
	}
	return out
}

func projectFileAggregateToAggregate(src ProjectFileAggregate) *ProjectAggregate {
	out := newProjectAggregate()
	for key, stat := range src.ProjectStats {
		statCopy := stat
		out.ProjectStats[key] = &statCopy
	}
	out.WeekdayData = src.WeekdayData
	out.DailyActivity = copyIntMap(src.DailyActivity)
	out.DailySessions = slicesMapToBoolSets(src.DailySessions)
	out.HourlyCounts = src.HourlyCounts
	for key, stat := range src.ModelUsage {
		statCopy := stat
		out.ModelUsage[key] = &statCopy
	}
	for key, stat := range src.CostModelStats {
		statCopy := stat
		out.CostModelStats[key] = &statCopy
	}
	for key, stat := range src.CostProjectStats {
		statCopy := stat
		out.CostProjectStats[key] = &statCopy
	}
	for key, stat := range src.CostSessionStats {
		statCopy := stat
		out.CostSessionStats[key] = &statCopy
	}
	for key, stat := range src.CostAgentStats {
		statCopy := stat
		out.CostAgentStats[key] = &statCopy
	}
	out.BudgetTimeline = append([]BudgetTimelineItem(nil), src.BudgetTimeline...)
	for key, stat := range src.ToolStats {
		statCopy := stat
		out.ToolStats[key] = &statCopy
	}
	for key, stat := range src.ToolModelStats {
		statCopy := stat
		out.ToolModelStats[key] = &statCopy
	}
	for key, stat := range src.FailureReasons {
		statCopy := stat
		out.FailureReasons[key] = &statCopy
	}
	for key, stat := range src.FailureToolReasons {
		statCopy := stat
		out.FailureToolReasons[key] = &statCopy
	}
	for key, stat := range src.FailureModelReasons {
		statCopy := stat
		out.FailureModelReasons[key] = &statCopy
	}
	out.FailureSamples = append([]ToolFailureSample(nil), src.FailureSamples...)
	for key, stat := range src.SessionStats {
		statCopy := stat
		out.SessionStatsMap[key] = &statCopy
	}
	out.SessionQueueOps = copyIntMap(src.SessionQueueOps)
	out.EventTypes = copyIntMap(src.EventTypes)
	for key, stat := range src.HookStats {
		statCopy := stat
		out.HookStats[key] = &statCopy
	}
	for key, stat := range src.SkillStats {
		statCopy := stat
		out.SkillStats[key] = &statCopy
	}
	out.PermissionModes = copyIntMap(src.PermissionModes)
	for key, stat := range src.OpenedFiles {
		statCopy := stat
		out.OpenedFiles[key] = &statCopy
	}
	if src.BudgetSummary != nil {
		budgetCopy := *src.BudgetSummary
		out.BudgetSummary = &budgetCopy
	}
	out.EventSamples = append([]EventSample(nil), src.EventSamples...)
	for key, stat := range src.AgentStats {
		statCopy := stat
		out.AgentStats[key] = &statCopy
	}
	out.AgentSessions = slicesMapToBoolSets(src.AgentSessions)
	for key, stat := range src.BashCommandStats {
		statCopy := stat
		out.BashCommandStats[key] = &statCopy
	}
	for key, stat := range src.FileOperationStats {
		statCopy := stat
		out.FileOperationStats[key] = &statCopy
	}
	return out
}

func copyIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]int, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func boolSetMapToSlices(src map[string]map[string]bool) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string][]string, len(src))
	for key, values := range src {
		items := make([]string, 0, len(values))
		for value := range values {
			items = append(items, value)
		}
		sort.Strings(items)
		out[key] = items
	}
	return out
}

func slicesMapToBoolSets(src map[string][]string) map[string]map[string]bool {
	if len(src) == 0 {
		return make(map[string]map[string]bool)
	}
	out := make(map[string]map[string]bool, len(src))
	for key, values := range src {
		out[key] = make(map[string]bool, len(values))
		for _, value := range values {
			out[key][value] = true
		}
	}
	return out
}
