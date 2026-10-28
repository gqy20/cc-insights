package main

import (
	"sort"
	"sync"
)

func newProjectAggregate() *ProjectAggregate {
	aggregate := &ProjectAggregate{
		ProjectStats:            make(map[string]*ProjectStatItem),
		DailyActivity:           make(map[string]int),
		DailySessions:           make(map[string]map[string]bool),
		DailyProjectCounts:      make(map[string]map[string]int),
		DailyModelCounts:        make(map[string]map[string]int),
		DailyModelTokens:        make(map[string]map[string]int),
		DailyHourlyCounts:       make(map[string][24]int),
		DailyRuntime:            make(map[string]*ProjectAggregate),
		DailyProjectRuntime:     make(map[string]map[string]*ProjectAggregate),
		DailySessionRuntime:     make(map[string]map[string]*ProjectAggregate),
		ModelUsage:              make(map[string]*ModelUsageItem),
		ToolStats:               make(map[string]*ToolStatItem),
		ToolModelStats:          make(map[string]*ToolModelStatItem),
		FailureReasons:          make(map[string]*FailureReasonStat),
		FailureToolReasons:      make(map[string]*FailureToolReasonStat),
		FailureModelReasons:     make(map[string]*FailureModelReasonStat),
		SessionStatsMap:         make(map[string]*SessionAnalysisItem),
		SessionQueueOps:         make(map[string]int),
		EventTypes:              make(map[string]int),
		HookStats:               make(map[string]*HookStatItem),
		SkillStats:              make(map[string]*SkillStatItem),
		InstalledSkills:         make(map[string]*InstalledSkillItem),
		SkillUsageStats:         make(map[string]*SkillUsageStat),
		SkillListingStats:       make(map[string]int),
		SkillProjectStats:       make(map[string]*SkillProjectStat),
		SkillModelStats:         make(map[string]*SkillModelStat),
		SkillAgentStats:         make(map[string]*SkillAgentStat),
		SkillSessionToolStats:   make(map[string]*SkillSessionToolStat),
		PermissionModes:         make(map[string]int),
		OpenedFiles:             make(map[string]*FileAccessStat),
		AgentStats:              make(map[string]*AgentStatItem),
		AgentModelStats:         make(map[string]*AgentModelStat),
		AgentSessions:           make(map[string]map[string]bool),
		BashCommandStats:        make(map[string]*BashCommandStat),
		BashCommandModelStats:   make(map[string]*BashCommandModelStat),
		FileOperationStats:      make(map[string]*FileOperationStat),
		FileOperationModelStats: make(map[string]*FileOperationModelStat),
		FileHotStats:            make(map[string]*FileHotStat),
		FileEditFailures:        make(map[string]*FileEditFailureAgg),
		FileSnapshotStats:       make(map[string]*FileSnapshotAgg),
		FileEditedStats:         make(map[string]*FileEditedAgg),
		FileDiffStats:           make(map[string]*FileDiffAgg),
		ToolPerfStats:           make(map[string]*ToolPerfAgg),
		HourlyCounts:            [24]int{},
		CostModelStats:          make(map[string]*CostModelStat),
		CostProjectStats:        make(map[string]*CostProjectStat),
		CostSessionStats:        make(map[string]*CostSessionStat),
		CostAgentStats:          make(map[string]*CostAgentStat),
		mu:                      sync.RWMutex{},
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
	for date, projects := range src.DailyProjectCounts {
		if dst.DailyProjectCounts[date] == nil {
			dst.DailyProjectCounts[date] = make(map[string]int)
		}
		for project, count := range projects {
			dst.DailyProjectCounts[date][project] += count
		}
	}
	for date, models := range src.DailyModelCounts {
		if dst.DailyModelCounts[date] == nil {
			dst.DailyModelCounts[date] = make(map[string]int)
		}
		for model, count := range models {
			dst.DailyModelCounts[date][model] += count
		}
	}
	for date, models := range src.DailyModelTokens {
		if dst.DailyModelTokens[date] == nil {
			dst.DailyModelTokens[date] = make(map[string]int)
		}
		for model, tokens := range models {
			dst.DailyModelTokens[date][model] += tokens
		}
	}
	for date, counts := range src.DailyHourlyCounts {
		dstCounts := dst.DailyHourlyCounts[date]
		for hour, count := range counts {
			dstCounts[hour] += count
		}
		dst.DailyHourlyCounts[date] = dstCounts
	}
	for date, runtimeAgg := range src.DailyRuntime {
		if runtimeAgg == nil {
			continue
		}
		dstRuntime := ensureDailyRuntimeAggregate(dst, date)
		mergeProjectAggregate(dstRuntime, runtimeAgg)
	}
	for date, projects := range src.DailyProjectRuntime {
		for project, runtimeAgg := range projects {
			if runtimeAgg == nil {
				continue
			}
			dstRuntime := ensureDailyProjectRuntimeAggregate(dst, date, project)
			mergeProjectAggregate(dstRuntime, runtimeAgg)
		}
	}
	for date, sessions := range src.DailySessionRuntime {
		for sessionID, runtimeAgg := range sessions {
			if runtimeAgg == nil {
				continue
			}
			dstRuntime := ensureDailySessionRuntimeAggregate(dst, date, sessionID)
			mergeProjectAggregate(dstRuntime, runtimeAgg)
		}
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
		dst.ModelUsage[model].InputTokens += stat.InputTokens
		dst.ModelUsage[model].CachedInputTokens += stat.CachedInputTokens
		dst.ModelUsage[model].OutputTokens += stat.OutputTokens
		dst.ModelUsage[model].ReasoningOutputTokens += stat.ReasoningOutputTokens
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
	mergeContextStats(dst, src.ContextStats)
	mergeTaskAgg(dst, src.TaskAgg)
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
	remainingFailures := maxFailureSamples - len(dst.FailureSamples)
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
			// 深拷贝 SessionIDs：值拷贝会共享底层 map，后续 dst record 时 mutate 会污染 src。
			if stat.SessionIDs != nil {
				statCopy.SessionIDs = make(map[string]bool, len(stat.SessionIDs))
				for sid := range stat.SessionIDs {
					statCopy.SessionIDs[sid] = true
				}
			}
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
		for sid := range stat.SessionIDs {
			if dstStat.SessionIDs == nil {
				dstStat.SessionIDs = make(map[string]bool)
			}
			dstStat.SessionIDs[sid] = true
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
	for name, stat := range src.InstalledSkills {
		if dst.InstalledSkills[name] == nil {
			statCopy := *stat
			dst.InstalledSkills[name] = &statCopy
		}
	}
	for name, stat := range src.SkillUsageStats {
		dstStat := ensureSkillUsageStat(dst, name)
		mergeSkillUsageStat(dstStat, stat)
	}
	for name, count := range src.SkillListingStats {
		dst.SkillListingStats[name] += count
	}
	for key, stat := range src.SkillProjectStats {
		if dst.SkillProjectStats[key] == nil {
			statCopy := *stat
			dst.SkillProjectStats[key] = &statCopy
			continue
		}
		mergeSkillProjectStat(dst.SkillProjectStats[key], stat)
	}
	for key, stat := range src.SkillModelStats {
		if dst.SkillModelStats[key] == nil {
			statCopy := *stat
			dst.SkillModelStats[key] = &statCopy
			continue
		}
		mergeSkillModelStat(dst.SkillModelStats[key], stat)
	}
	for key, stat := range src.SkillAgentStats {
		if dst.SkillAgentStats[key] == nil {
			statCopy := *stat
			dst.SkillAgentStats[key] = &statCopy
			continue
		}
		dst.SkillAgentStats[key].InvocationCount += stat.InvocationCount
	}
	for key, stat := range src.SkillSessionToolStats {
		if dst.SkillSessionToolStats[key] == nil {
			statCopy := *stat
			dst.SkillSessionToolStats[key] = &statCopy
			continue
		}
		dstStat := dst.SkillSessionToolStats[key]
		dstStat.CallCount += stat.CallCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResults += stat.MissingResults
	}
	dst.SkillListingEvents += src.SkillListingEvents
	dst.SkillInitialListings += src.SkillInitialListings
	dst.DynamicSkillEvents += src.DynamicSkillEvents
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
	for _, stat := range src.AgentModelStats {
		dstStat := ensureAgentModelStat(dst, stat.AgentID, stat.Model, stat.IsSidechain)
		if dstStat.AgentName == "" {
			dstStat.AgentName = stat.AgentName
		}
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
	for _, stat := range src.BashCommandModelStats {
		dstStat := ensureBashCommandModelStat(dst, stat.CommandName, stat.Model)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
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
	for _, stat := range src.FileOperationModelStats {
		dstStat := ensureFileOperationModelStat(dst, stat.Operation, stat.Path, stat.Model)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}

	// file_analysis: merge FileHotStats
	for path, stat := range src.FileHotStats {
		if dst.FileHotStats[path] == nil {
			dst.FileHotStats[path] = &FileHotStat{Path: path}
		}
		ds := dst.FileHotStats[path]
		ds.ReadCount += stat.ReadCount
		ds.EditCount += stat.EditCount
		ds.WriteCount += stat.WriteCount
		ds.SuccessCount += stat.SuccessCount
		ds.FailureCount += stat.FailureCount
		ds.MissingCount += stat.MissingCount
	}
	// file_analysis: merge FileEditFailures
	for path, ef := range src.FileEditFailures {
		if dst.FileEditFailures[path] == nil {
			dst.FileEditFailures[path] = &FileEditFailureAgg{Path: path, ReasonCounts: make(map[string]int)}
		}
		de := dst.FileEditFailures[path]
		de.TotalFailures += ef.TotalFailures
		for reason, count := range ef.ReasonCounts {
			de.ReasonCounts[reason] += count
		}
	}
	// file_analysis: merge FileSnapshotStats
	for path, ss := range src.FileSnapshotStats {
		if dst.FileSnapshotStats[path] == nil {
			dst.FileSnapshotStats[path] = &FileSnapshotAgg{Path: path, SessionSet: make(map[string]bool)}
		}
		ds := dst.FileSnapshotStats[path]
		ds.SnapshotCount += ss.SnapshotCount
		if ss.MaxVersion > ds.MaxVersion {
			ds.MaxVersion = ss.MaxVersion
		}
		if ss.SessionSet != nil && ds.SessionSet != nil {
			for sid := range ss.SessionSet {
				ds.SessionSet[sid] = true
			}
		}
		ds.IsUpdateCount += ss.IsUpdateCount
	}
	// file_analysis: merge FileEditedStats
	for path, ed := range src.FileEditedStats {
		if dst.FileEditedStats[path] == nil {
			dst.FileEditedStats[path] = &FileEditedAgg{Path: path}
		}
		de := dst.FileEditedStats[path]
		de.EditCount += ed.EditCount
		de.TotalLines += ed.TotalLines
		de.TotalChars += ed.TotalChars
	}
	for path, diff := range src.FileDiffStats {
		mergeFileDiffAgg(dst, path, diff)
	}

	// task_plan_analysis (M4): merge PlanModeAgg
	if src.PlanModeAgg != nil {
		if dst.PlanModeAgg == nil {
			dst.PlanModeAgg = &PlanModeAgg{
				ExitReasons:   make(map[string]int),
				PlanFilePaths: make(map[string]*PlanFileAgg),
				SessionSet:    make(map[string]bool),
			}
		}
		dst.PlanModeAgg.EntryCount += src.PlanModeAgg.EntryCount
		dst.PlanModeAgg.ExitCount += src.PlanModeAgg.ExitCount
		dst.PlanModeAgg.ReentryCount += src.PlanModeAgg.ReentryCount
		for reason, count := range src.PlanModeAgg.ExitReasons {
			dst.PlanModeAgg.ExitReasons[reason] += count
		}
		for fp, pf := range src.PlanModeAgg.PlanFilePaths {
			if existing := dst.PlanModeAgg.PlanFilePaths[fp]; existing != nil {
				existing.RefCount += pf.RefCount
				if existing.PlanContent == "" && pf.PlanContent != "" {
					existing.PlanContent = pf.PlanContent
					existing.Preview = pf.Preview
					existing.CharCount = pf.CharCount
					existing.LineCount = pf.LineCount
					existing.HasCode = pf.HasCode
				}
				for sid := range pf.RefSessions {
					existing.RefSessions[sid] = true
				}
			} else {
				pfCopy := *pf
				if pfCopy.RefSessions != nil {
					refSessions := make(map[string]bool)
					for sid := range pf.RefSessions {
						refSessions[sid] = true
					}
					pfCopy.RefSessions = refSessions
				} else {
					pfCopy.RefSessions = make(map[string]bool)
				}
				dst.PlanModeAgg.PlanFilePaths[fp] = &pfCopy
			}
		}
		for sid := range src.PlanModeAgg.SessionSet {
			dst.PlanModeAgg.SessionSet[sid] = true
		}
	}
	mergePlanStepAgg(dst, src.PlanStepAgg)

	// task_plan_analysis (M4): merge GoalStatusAgg
	if src.GoalStatusAgg != nil {
		if dst.GoalStatusAgg == nil {
			dst.GoalStatusAgg = &GoalStatusAgg{}
		}
		remaining := 50 - len(dst.GoalStatusAgg.Items)
		if remaining > 0 && len(src.GoalStatusAgg.Items) > 0 {
			items := src.GoalStatusAgg.Items
			if len(items) > remaining {
				items = items[:remaining]
			}
			dst.GoalStatusAgg.Items = append(dst.GoalStatusAgg.Items, items...)
		}
	}

	// task_plan_analysis (M4): merge ReminderAgg
	if src.ReminderAgg != nil {
		if dst.ReminderAgg == nil {
			dst.ReminderAgg = &ReminderAgg{
				TaskSessionCounts:   make(map[string]int),
				TodoSessionCounts:   make(map[string]int),
				TaskSessionProjects: make(map[string]string),
				TodoSessionProjects: make(map[string]string),
			}
		}
		dst.ReminderAgg.TaskReminderCount += src.ReminderAgg.TaskReminderCount
		dst.ReminderAgg.TodoReminderCount += src.ReminderAgg.TodoReminderCount
		for sid, cnt := range src.ReminderAgg.TaskSessionCounts {
			dst.ReminderAgg.TaskSessionCounts[sid] += cnt
		}
		for sid, cnt := range src.ReminderAgg.TodoSessionCounts {
			dst.ReminderAgg.TodoSessionCounts[sid] += cnt
		}
		for sid, proj := range src.ReminderAgg.TaskSessionProjects {
			if _, exists := dst.ReminderAgg.TaskSessionProjects[sid]; !exists {
				dst.ReminderAgg.TaskSessionProjects[sid] = proj
			}
		}
		for sid, proj := range src.ReminderAgg.TodoSessionProjects {
			if _, exists := dst.ReminderAgg.TodoSessionProjects[sid]; !exists {
				dst.ReminderAgg.TodoSessionProjects[sid] = proj
			}
		}
	}

	// M5: merge ToolPerfStats
	for key, stat := range src.ToolPerfStats {
		dstStat := ensureToolPerfAgg(dst, key)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.ErrorCount += stat.ErrorCount
		dstStat.MissingCount += stat.MissingCount
		dstStat.TotalDurationMs += stat.TotalDurationMs
		if stat.MinDurationMs < dstStat.MinDurationMs {
			dstStat.MinDurationMs = stat.MinDurationMs
		}
		if stat.MaxDurationMs > dstStat.MaxDurationMs {
			dstStat.MaxDurationMs = stat.MaxDurationMs
		}
		dstStat.TotalResultSize += stat.TotalResultSize
		dstStat.EmptyResults += stat.EmptyResults
		if dstStat.SampleInput == "" && stat.SampleInput != "" {
			dstStat.SampleInput = stat.SampleInput
		}
	}

	// Merge SlowestCalls (keep global top-N across merged aggregates)
	for _, sc := range src.SlowestCalls {
		if len(dst.SlowestCalls) < maxSlowCalls {
			dst.SlowestCalls = append(dst.SlowestCalls, sc)
			sortSlowCallsDesc(dst.SlowestCalls)
		} else if sc.DurationMs > dst.SlowestCalls[len(dst.SlowestCalls)-1].DurationMs {
			dst.SlowestCalls[len(dst.SlowestCalls)-1] = sc
			sortSlowCallsDesc(dst.SlowestCalls)
		}
	}
	// Merge turn duration stats (keep global top-N slow turns)
	mergeTurnStats(dst, src)
}

func aggregateToProjectFileAggregate(src *ProjectAggregate) ProjectFileAggregate {
	return aggregateToProjectFileAggregateWithDaily(src, true)
}

func aggregateToProjectFileAggregateWithDaily(src *ProjectAggregate, includeDaily bool) ProjectFileAggregate {
	out := ProjectFileAggregate{
		ProjectStats:            make(map[string]ProjectStatItem, len(src.ProjectStats)),
		WeekdayData:             src.WeekdayData,
		DailyActivity:           copyIntMap(src.DailyActivity),
		DailySessions:           boolSetMapToSlices(src.DailySessions),
		DailyProjectCounts:      copyNestedIntMap(src.DailyProjectCounts),
		DailyModelCounts:        copyNestedIntMap(src.DailyModelCounts),
		DailyModelTokens:        copyNestedIntMap(src.DailyModelTokens),
		DailyHourlyCounts:       copyDailyHourlyCounts(src.DailyHourlyCounts),
		DailyRuntime:            make(map[string]ProjectFileAggregate),
		DailyProjectRuntime:     make(map[string]map[string]ProjectFileAggregate),
		DailySessionRuntime:     make(map[string]map[string]ProjectFileAggregate),
		HourlyCounts:            src.HourlyCounts,
		ModelUsage:              make(map[string]ModelUsageItem, len(src.ModelUsage)),
		CostModelStats:          make(map[string]CostModelStat, len(src.CostModelStats)),
		CostProjectStats:        make(map[string]CostProjectStat, len(src.CostProjectStats)),
		CostSessionStats:        make(map[string]CostSessionStat, len(src.CostSessionStats)),
		CostAgentStats:          make(map[string]CostAgentStat, len(src.CostAgentStats)),
		ContextStats:            cloneContextAnalysis(src.ContextStats),
		BudgetTimeline:          append([]BudgetTimelineItem(nil), src.BudgetTimeline...),
		ToolStats:               make(map[string]ToolStatItem, len(src.ToolStats)),
		ToolModelStats:          make(map[string]ToolModelStatItem, len(src.ToolModelStats)),
		FailureReasons:          make(map[string]FailureReasonStat, len(src.FailureReasons)),
		FailureToolReasons:      make(map[string]FailureToolReasonStat, len(src.FailureToolReasons)),
		FailureModelReasons:     make(map[string]FailureModelReasonStat, len(src.FailureModelReasons)),
		FailureSamples:          append([]ToolFailureSample(nil), src.FailureSamples...),
		SessionStats:            make(map[string]SessionAnalysisItem, len(src.SessionStatsMap)),
		SessionQueueOps:         copyIntMap(src.SessionQueueOps),
		EventTypes:              copyIntMap(src.EventTypes),
		HookStats:               make(map[string]HookStatItem, len(src.HookStats)),
		SkillStats:              make(map[string]SkillStatItem, len(src.SkillStats)),
		InstalledSkills:         make(map[string]InstalledSkillItem, len(src.InstalledSkills)),
		SkillUsageStats:         make(map[string]SkillUsageStat, len(src.SkillUsageStats)),
		SkillListingStats:       copyIntMap(src.SkillListingStats),
		SkillProjectStats:       make(map[string]SkillProjectStat, len(src.SkillProjectStats)),
		SkillModelStats:         make(map[string]SkillModelStat, len(src.SkillModelStats)),
		SkillAgentStats:         make(map[string]SkillAgentStat, len(src.SkillAgentStats)),
		SkillSessionToolStats:   make(map[string]SkillSessionToolStat, len(src.SkillSessionToolStats)),
		SkillListingEvents:      src.SkillListingEvents,
		SkillInitialListings:    src.SkillInitialListings,
		DynamicSkillEvents:      src.DynamicSkillEvents,
		PermissionModes:         copyIntMap(src.PermissionModes),
		OpenedFiles:             make(map[string]FileAccessStat, len(src.OpenedFiles)),
		AgentStats:              make(map[string]AgentStatItem, len(src.AgentStats)),
		AgentModelStats:         make(map[string]AgentModelStat, len(src.AgentModelStats)),
		AgentSessions:           boolSetMapToSlices(src.AgentSessions),
		BashCommandStats:        make(map[string]BashCommandStat, len(src.BashCommandStats)),
		BashCommandModelStats:   make(map[string]BashCommandModelStat, len(src.BashCommandModelStats)),
		FileOperationStats:      make(map[string]FileOperationStat, len(src.FileOperationStats)),
		FileOperationModelStats: make(map[string]FileOperationModelStat, len(src.FileOperationModelStats)),
		FileHotStats:            make(map[string]FileHotStat, len(src.FileHotStats)),
		FileEditFailures:        make(map[string]FileEditFailureAgg, len(src.FileEditFailures)),
		FileSnapshotStats:       make(map[string]FileSnapshotAgg, len(src.FileSnapshotStats)),
		FileEditedStats:         make(map[string]FileEditedAgg, len(src.FileEditedStats)),
		FileDiffStats:           make(map[string]FileDiffAgg, len(src.FileDiffStats)),
		PlanModeAgg:             serializePlanModeAgg(src.PlanModeAgg),
		PlanStepAgg:             clonePlanStepAgg(src.PlanStepAgg),
		TaskAgg:                 cloneTaskAgg(src.TaskAgg),
		GoalStatusAgg:           serializeGoalStatusAgg(src.GoalStatusAgg),
		ReminderAgg:             serializeReminderAgg(src.ReminderAgg),
		ToolPerfStats:           make(map[string]ToolPerfAgg, len(src.ToolPerfStats)),
		SlowestCalls:            append([]ToolSlowCallItem(nil), src.SlowestCalls...),
		TurnCount:               src.TurnCount,
		TurnTotalDurationMs:     src.TurnTotalDurationMs,
		TurnMaxDurationMs:       src.TurnMaxDurationMs,
		SlowTurns:               append([]TurnSlowItem(nil), src.SlowTurns...),
	}
	if !includeDaily {
		out.DailyRuntime = nil
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
	for key, stat := range src.InstalledSkills {
		out.InstalledSkills[key] = *stat
	}
	for key, stat := range src.SkillUsageStats {
		out.SkillUsageStats[key] = *stat
	}
	for key, stat := range src.SkillProjectStats {
		out.SkillProjectStats[key] = *stat
	}
	for key, stat := range src.SkillModelStats {
		out.SkillModelStats[key] = *stat
	}
	for key, stat := range src.SkillAgentStats {
		out.SkillAgentStats[key] = *stat
	}
	for key, stat := range src.SkillSessionToolStats {
		out.SkillSessionToolStats[key] = *stat
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
	for key, stat := range src.AgentModelStats {
		out.AgentModelStats[key] = *stat
	}
	for key, stat := range src.BashCommandStats {
		out.BashCommandStats[key] = *stat
	}
	for key, stat := range src.BashCommandModelStats {
		out.BashCommandModelStats[key] = *stat
	}
	for key, stat := range src.FileOperationStats {
		out.FileOperationStats[key] = *stat
	}
	for key, stat := range src.FileOperationModelStats {
		out.FileOperationModelStats[key] = *stat
	}
	// file_analysis: serialize new maps
	for key, stat := range src.FileHotStats {
		out.FileHotStats[key] = *stat
	}
	for key, ef := range src.FileEditFailures {
		out.FileEditFailures[key] = *ef
	}
	for key, ss := range src.FileSnapshotStats {
		ssCopy := *ss
		ssCopy.SessionSet = nil // sessionSet is not serializable as map[bool], use slices format if needed
		// For now store session count in a separate field or just skip SessionSet
		out.FileSnapshotStats[key] = ssCopy
	}
	for key, stat := range src.ToolPerfStats {
		out.ToolPerfStats[key] = *stat
	}
	for key, ed := range src.FileEditedStats {
		out.FileEditedStats[key] = *ed
	}
	for key, diff := range src.FileDiffStats {
		out.FileDiffStats[key] = *diff
	}
	if includeDaily {
		for date, runtimeAgg := range src.DailyRuntime {
			if runtimeAgg != nil {
				out.DailyRuntime[date] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
			}
		}
		if len(out.DailyRuntime) == 0 {
			out.DailyRuntime = nil
		}
		for date, projects := range src.DailyProjectRuntime {
			for project, runtimeAgg := range projects {
				if runtimeAgg == nil {
					continue
				}
				if out.DailyProjectRuntime[date] == nil {
					out.DailyProjectRuntime[date] = make(map[string]ProjectFileAggregate)
				}
				out.DailyProjectRuntime[date][project] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
			}
		}
		if len(out.DailyProjectRuntime) == 0 {
			out.DailyProjectRuntime = nil
		}
		for date, sessions := range src.DailySessionRuntime {
			for sessionID, runtimeAgg := range sessions {
				if runtimeAgg == nil {
					continue
				}
				if out.DailySessionRuntime[date] == nil {
					out.DailySessionRuntime[date] = make(map[string]ProjectFileAggregate)
				}
				out.DailySessionRuntime[date][sessionID] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
			}
		}
		if len(out.DailySessionRuntime) == 0 {
			out.DailySessionRuntime = nil
		}
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
	// copyIntMap 对空 map 返回 nil：直接赋值会把 newProjectAggregate 初始化好的
	// map 置 nil，mergeProjectAggregate 对 nil map 赋值会 panic（跨 provider 合并
	// 日期快照时，一边有数据一边为空即触发）。因此仅在 src 非空时覆盖。
	if len(src.DailyActivity) > 0 {
		out.DailyActivity = copyIntMap(src.DailyActivity)
	}
	if len(src.DailySessions) > 0 {
		out.DailySessions = slicesMapToBoolSets(src.DailySessions)
	}
	if len(src.DailyProjectCounts) > 0 {
		out.DailyProjectCounts = copyNestedIntMap(src.DailyProjectCounts)
	}
	if len(src.DailyModelCounts) > 0 {
		out.DailyModelCounts = copyNestedIntMap(src.DailyModelCounts)
	}
	if len(src.DailyModelTokens) > 0 {
		out.DailyModelTokens = copyNestedIntMap(src.DailyModelTokens)
	}
	if len(src.DailyHourlyCounts) > 0 {
		out.DailyHourlyCounts = copyDailyHourlyCounts(src.DailyHourlyCounts)
	}
	for date, runtimeSnapshot := range src.DailyRuntime {
		out.DailyRuntime[date] = projectFileAggregateToAggregate(runtimeSnapshot)
	}
	for date, projects := range src.DailyProjectRuntime {
		for project, runtimeSnapshot := range projects {
			if out.DailyProjectRuntime[date] == nil {
				out.DailyProjectRuntime[date] = make(map[string]*ProjectAggregate)
			}
			out.DailyProjectRuntime[date][project] = projectFileAggregateToAggregate(runtimeSnapshot)
		}
	}
	for date, sessions := range src.DailySessionRuntime {
		for sessionID, runtimeSnapshot := range sessions {
			if out.DailySessionRuntime[date] == nil {
				out.DailySessionRuntime[date] = make(map[string]*ProjectAggregate)
			}
			out.DailySessionRuntime[date][sessionID] = projectFileAggregateToAggregate(runtimeSnapshot)
		}
	}
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
	out.ContextStats = cloneContextAnalysis(src.ContextStats)
	out.TaskAgg = cloneTaskAgg(src.TaskAgg)
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
	if len(src.SessionQueueOps) > 0 {
		out.SessionQueueOps = copyIntMap(src.SessionQueueOps)
	}
	if len(src.EventTypes) > 0 {
		out.EventTypes = copyIntMap(src.EventTypes)
	}
	for key, stat := range src.HookStats {
		statCopy := stat
		out.HookStats[key] = &statCopy
	}
	for key, stat := range src.SkillStats {
		statCopy := stat
		out.SkillStats[key] = &statCopy
	}
	for key, stat := range src.InstalledSkills {
		statCopy := stat
		out.InstalledSkills[key] = &statCopy
	}
	for key, stat := range src.SkillUsageStats {
		statCopy := stat
		out.SkillUsageStats[key] = &statCopy
	}
	for key, stat := range src.SkillProjectStats {
		statCopy := stat
		out.SkillProjectStats[key] = &statCopy
	}
	for key, stat := range src.SkillModelStats {
		statCopy := stat
		out.SkillModelStats[key] = &statCopy
	}
	for key, stat := range src.SkillAgentStats {
		statCopy := stat
		out.SkillAgentStats[key] = &statCopy
	}
	for key, stat := range src.SkillSessionToolStats {
		statCopy := stat
		out.SkillSessionToolStats[key] = &statCopy
	}
	out.SkillListingEvents = src.SkillListingEvents
	out.SkillInitialListings = src.SkillInitialListings
	out.DynamicSkillEvents = src.DynamicSkillEvents
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
	for key, stat := range src.AgentModelStats {
		statCopy := stat
		out.AgentModelStats[key] = &statCopy
	}
	out.AgentSessions = slicesMapToBoolSets(src.AgentSessions)
	for key, stat := range src.BashCommandStats {
		statCopy := stat
		out.BashCommandStats[key] = &statCopy
	}
	for key, stat := range src.BashCommandModelStats {
		statCopy := stat
		out.BashCommandModelStats[key] = &statCopy
	}
	for key, stat := range src.FileOperationStats {
		statCopy := stat
		out.FileOperationStats[key] = &statCopy
	}
	for key, stat := range src.FileOperationModelStats {
		statCopy := stat
		out.FileOperationModelStats[key] = &statCopy
	}
	// file_analysis: restore new maps
	for key, stat := range src.FileHotStats {
		statCopy := stat
		out.FileHotStats[key] = &statCopy
	}
	for key, ef := range src.FileEditFailures {
		efCopy := ef
		efCopy.ReasonCounts = make(map[string]int)
		for r, c := range ef.ReasonCounts {
			efCopy.ReasonCounts[r] = c
		}
		out.FileEditFailures[key] = &efCopy
	}
	for key, ss := range src.FileSnapshotStats {
		ssCopy := ss
		ssCopy.SessionSet = make(map[string]bool)
		out.FileSnapshotStats[key] = &ssCopy
	}
	for key, ed := range src.FileEditedStats {
		edCopy := ed
		out.FileEditedStats[key] = &edCopy
	}
	for key, diff := range src.FileDiffStats {
		diffCopy := diff
		out.FileDiffStats[key] = &diffCopy
	}
	// task_plan_analysis (M4): restore agg structs
	out.PlanModeAgg = deserializePlanModeAgg(src.PlanModeAgg)
	out.PlanStepAgg = clonePlanStepAgg(src.PlanStepAgg)
	out.GoalStatusAgg = deserializeGoalStatusAgg(src.GoalStatusAgg)
	out.ReminderAgg = deserializeReminderAgg(src.ReminderAgg)
	// M5: restore ToolPerfStats
	for key, stat := range src.ToolPerfStats {
		statCopy := stat
		out.ToolPerfStats[key] = &statCopy
	}
	out.SlowestCalls = append([]ToolSlowCallItem(nil), src.SlowestCalls...)
	// turn duration: restore scalars + slow turns Top-N
	out.TurnCount = src.TurnCount
	out.TurnTotalDurationMs = src.TurnTotalDurationMs
	out.TurnMaxDurationMs = src.TurnMaxDurationMs
	out.SlowTurns = append([]TurnSlowItem(nil), src.SlowTurns...)
	return out
}

func ensureDailyRuntimeAggregate(agg *ProjectAggregate, date string) *ProjectAggregate {
	if agg.DailyRuntime == nil {
		agg.DailyRuntime = make(map[string]*ProjectAggregate)
	}
	if agg.DailyRuntime[date] == nil {
		agg.DailyRuntime[date] = newProjectAggregate()
	}
	return agg.DailyRuntime[date]
}

func ensureDailyProjectRuntimeAggregate(agg *ProjectAggregate, date, project string) *ProjectAggregate {
	if agg.DailyProjectRuntime == nil {
		agg.DailyProjectRuntime = make(map[string]map[string]*ProjectAggregate)
	}
	if agg.DailyProjectRuntime[date] == nil {
		agg.DailyProjectRuntime[date] = make(map[string]*ProjectAggregate)
	}
	if agg.DailyProjectRuntime[date][project] == nil {
		agg.DailyProjectRuntime[date][project] = newProjectAggregate()
	}
	return agg.DailyProjectRuntime[date][project]
}

func ensureDailySessionRuntimeAggregate(agg *ProjectAggregate, date, sessionID string) *ProjectAggregate {
	if sessionID == "" {
		sessionID = "unknown"
	}
	if agg.DailySessionRuntime == nil {
		agg.DailySessionRuntime = make(map[string]map[string]*ProjectAggregate)
	}
	if agg.DailySessionRuntime[date] == nil {
		agg.DailySessionRuntime[date] = make(map[string]*ProjectAggregate)
	}
	if agg.DailySessionRuntime[date][sessionID] == nil {
		agg.DailySessionRuntime[date][sessionID] = newProjectAggregate()
	}
	return agg.DailySessionRuntime[date][sessionID]
}

func recordModelUsageLocked(agg *ProjectAggregate, model string, tokens int) {
	if model == "" {
		return
	}
	if agg.ModelUsage == nil {
		agg.ModelUsage = make(map[string]*ModelUsageItem)
	}
	if agg.ModelUsage[model] == nil {
		agg.ModelUsage[model] = &ModelUsageItem{Model: model}
	}
	agg.ModelUsage[model].Count++
	agg.ModelUsage[model].Tokens += tokens
}

func recordModelTokenBreakdownLocked(agg *ProjectAggregate, model string, inputTokens, cachedInputTokens, outputTokens, reasoningOutputTokens int) {
	if model == "" {
		return
	}
	if agg.ModelUsage == nil {
		agg.ModelUsage = make(map[string]*ModelUsageItem)
	}
	if agg.ModelUsage[model] == nil {
		agg.ModelUsage[model] = &ModelUsageItem{Model: model}
	}
	agg.ModelUsage[model].InputTokens += inputTokens
	agg.ModelUsage[model].CachedInputTokens += cachedInputTokens
	agg.ModelUsage[model].OutputTokens += outputTokens
	agg.ModelUsage[model].ReasoningOutputTokens += reasoningOutputTokens
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

func copyNestedIntMap(src map[string]map[string]int) map[string]map[string]int {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]map[string]int, len(src))
	for key, values := range src {
		out[key] = copyIntMap(values)
	}
	return out
}

func copyDailyHourlyCounts(src map[string][24]int) map[string][24]int {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string][24]int, len(src))
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

// === task_plan_analysis (M4): 序列化/反序列化辅助函数 ===

// serializePlanModeAgg 将 PlanModeAgg 转为可序列化格式（丢弃 RefSessions map）
func serializePlanModeAgg(agg *PlanModeAgg) *SerializedPlanModeAgg {
	if agg == nil {
		return nil
	}
	out := &SerializedPlanModeAgg{
		EntryCount:    agg.EntryCount,
		ExitCount:     agg.ExitCount,
		ReentryCount:  agg.ReentryCount,
		ExitReasons:   make(map[string]int),
		PlanFilePaths: make(map[string]*SerializedPlanFileAgg),
	}
	for sid := range agg.SessionSet {
		out.SessionIDs = append(out.SessionIDs, sid)
	}
	sort.Strings(out.SessionIDs)
	for r, c := range agg.ExitReasons {
		out.ExitReasons[r] = c
	}
	for fp, pf := range agg.PlanFilePaths {
		out.PlanFilePaths[fp] = &SerializedPlanFileAgg{
			FilePath:  pf.FilePath,
			FileName:  pf.FileName,
			Preview:   pf.Preview,
			CharCount: pf.CharCount,
			LineCount: pf.LineCount,
			HasCode:   pf.HasCode,
			RefCount:  pf.RefCount,
		}
	}
	return out
}

// deserializePlanModeAgg 从可序列化格式恢复 PlanModeAgg（重建空 RefSessions）
func deserializePlanModeAgg(s *SerializedPlanModeAgg) *PlanModeAgg {
	if s == nil {
		return nil
	}
	out := &PlanModeAgg{
		EntryCount:    s.EntryCount,
		ExitCount:     s.ExitCount,
		ReentryCount:  s.ReentryCount,
		ExitReasons:   make(map[string]int),
		PlanFilePaths: make(map[string]*PlanFileAgg),
		SessionSet:    make(map[string]bool),
	}
	for r, c := range s.ExitReasons {
		out.ExitReasons[r] = c
	}
	for _, sid := range s.SessionIDs {
		if sid != "" {
			out.SessionSet[sid] = true
		}
	}
	for fp, spf := range s.PlanFilePaths {
		out.PlanFilePaths[fp] = &PlanFileAgg{
			FilePath:    spf.FilePath,
			FileName:    spf.FileName,
			Preview:     spf.Preview,
			CharCount:   spf.CharCount,
			LineCount:   spf.LineCount,
			HasCode:     spf.HasCode,
			RefCount:    spf.RefCount,
			RefSessions: make(map[string]bool),
		}
	}
	return out
}

func mergeContextStats(dst *ProjectAggregate, src *ContextAnalysisData) {
	if src == nil {
		return
	}
	if dst.ContextStats == nil {
		dst.ContextStats = &ContextAnalysisData{}
	}
	dst.ContextStats.TokenEvents += src.TokenEvents
	dst.ContextStats.ContextCompactions += src.ContextCompactions
	dst.ContextStats.CompactDroppedTokens += src.CompactDroppedTokens
	dst.ContextStats.CompactDurationMs += src.CompactDurationMs
	dst.ContextStats.CompactMessagesSummary += src.CompactMessagesSummary
	if src.CompactMaxPreTokens > dst.ContextStats.CompactMaxPreTokens {
		dst.ContextStats.CompactMaxPreTokens = src.CompactMaxPreTokens
	}
	if src.MaxInputTokens > dst.ContextStats.MaxInputTokens {
		dst.ContextStats.MaxInputTokens = src.MaxInputTokens
	}
	if src.MaxTotalTokens > dst.ContextStats.MaxTotalTokens {
		dst.ContextStats.MaxTotalTokens = src.MaxTotalTokens
	}
	if src.PeakContextRatio > dst.ContextStats.PeakContextRatio {
		dst.ContextStats.PeakContextRatio = src.PeakContextRatio
	}
	dst.ContextStats.TotalCachedInputTokens += src.TotalCachedInputTokens
	dst.ContextStats.TotalReasoningTokens += src.TotalReasoningTokens
	// 上下文构成占比按类别取最大值：max 是单调量，跨文件/跨天合并都不会失真。
	for name, share := range src.CategoryShares {
		if dst.ContextStats.CategoryShares == nil {
			dst.ContextStats.CategoryShares = make(map[string]float64, len(src.CategoryShares))
		}
		if share > dst.ContextStats.CategoryShares[name] {
			dst.ContextStats.CategoryShares[name] = share
		}
	}
}

// mergeTaskAgg 合并任务中间聚合（只读 src，写 dst），跨文件/跨天合并用。
func mergeTaskAgg(dst *ProjectAggregate, src *TaskAgg) {
	if src == nil {
		return
	}
	if dst.TaskAgg == nil {
		dst.TaskAgg = &TaskAgg{StatusCounts: make(map[string]int), SessionTasks: make(map[string]*SessionTaskAgg)}
	}
	dst.TaskAgg.TotalTasks += src.TotalTasks
	for status, count := range src.StatusCounts {
		dst.TaskAgg.StatusCounts[status] += count
	}
	for sessionID, sa := range src.SessionTasks {
		dstSA := dst.TaskAgg.SessionTasks[sessionID]
		if dstSA == nil {
			dstSA = &SessionTaskAgg{SessionID: sessionID, StatusCountsLocal: make(map[string]int)}
			dst.TaskAgg.SessionTasks[sessionID] = dstSA
		}
		dstSA.TotalTasks += sa.TotalTasks
		dstSA.CompletedCount += sa.CompletedCount
		dstSA.PendingCount += sa.PendingCount
		dstSA.InProgressCount += sa.InProgressCount
		for status, count := range sa.StatusCountsLocal {
			dstSA.StatusCountsLocal[status] += count
		}
	}
}

// cloneTaskAgg 深拷贝任务中间聚合（含内部 map），避免序列化/还原共享底层 map。
func cloneTaskAgg(src *TaskAgg) *TaskAgg {
	if src == nil {
		return nil
	}
	out := &TaskAgg{
		TotalTasks:   src.TotalTasks,
		StatusCounts: copyIntMap(src.StatusCounts),
		SessionTasks: make(map[string]*SessionTaskAgg, len(src.SessionTasks)),
	}
	for sessionID, sa := range src.SessionTasks {
		saCopy := *sa
		saCopy.StatusCountsLocal = copyIntMap(sa.StatusCountsLocal)
		out.SessionTasks[sessionID] = &saCopy
	}
	return out
}

func cloneContextAnalysis(src *ContextAnalysisData) *ContextAnalysisData {
	if src == nil {
		return nil
	}
	copyValue := *src
	return &copyValue
}

func mergeFileDiffAgg(dst *ProjectAggregate, path string, src *FileDiffAgg) {
	if src == nil || path == "" {
		return
	}
	if dst.FileDiffStats == nil {
		dst.FileDiffStats = make(map[string]*FileDiffAgg)
	}
	if dst.FileDiffStats[path] == nil {
		dst.FileDiffStats[path] = &FileDiffAgg{Path: path, Extension: src.Extension}
	}
	stat := dst.FileDiffStats[path]
	if stat.Extension == "" {
		stat.Extension = src.Extension
	}
	stat.PatchEvents += src.PatchEvents
	stat.AddedLines += src.AddedLines
	stat.DeletedLines += src.DeletedLines
}

func mergePlanStepAgg(dst *ProjectAggregate, src *PlanStepAgg) {
	if src == nil {
		return
	}
	if dst.PlanStepAgg == nil {
		dst.PlanStepAgg = &PlanStepAgg{
			StatusCounts: make(map[string]int),
			StepCounts:   make(map[string]*PlanStepHotAgg),
		}
	}
	dst.PlanStepAgg.UpdateEvents += src.UpdateEvents
	dst.PlanStepAgg.TotalSteps += src.TotalSteps
	if src.MaxStepsPerUpdate > dst.PlanStepAgg.MaxStepsPerUpdate {
		dst.PlanStepAgg.MaxStepsPerUpdate = src.MaxStepsPerUpdate
	}
	for status, count := range src.StatusCounts {
		dst.PlanStepAgg.StatusCounts[status] += count
	}
	for step, item := range src.StepCounts {
		if item == nil {
			continue
		}
		if dst.PlanStepAgg.StepCounts[step] == nil {
			dst.PlanStepAgg.StepCounts[step] = &PlanStepHotAgg{Step: item.Step}
		}
		dstItem := dst.PlanStepAgg.StepCounts[step]
		dstItem.Count += item.Count
		if item.LastStatus != "" {
			dstItem.LastStatus = item.LastStatus
		}
	}
}

func clonePlanStepAgg(src *PlanStepAgg) *PlanStepAgg {
	if src == nil {
		return nil
	}
	out := &PlanStepAgg{
		UpdateEvents:      src.UpdateEvents,
		TotalSteps:        src.TotalSteps,
		MaxStepsPerUpdate: src.MaxStepsPerUpdate,
		StatusCounts:      make(map[string]int, len(src.StatusCounts)),
		StepCounts:        make(map[string]*PlanStepHotAgg, len(src.StepCounts)),
	}
	for status, count := range src.StatusCounts {
		out.StatusCounts[status] = count
	}
	for step, item := range src.StepCounts {
		if item == nil {
			continue
		}
		copyItem := *item
		out.StepCounts[step] = &copyItem
	}
	return out
}

// serializeGoalStatusAgg 深拷贝 GoalStatusAgg
func serializeGoalStatusAgg(agg *GoalStatusAgg) *GoalStatusAgg {
	if agg == nil {
		return nil
	}
	copyVal := *agg
	copyVal.Items = append([]GoalStatusItem(nil), agg.Items...)
	return &copyVal
}

// deserializeGoalStatusAgg 深拷贝 GoalStatusAgg
func deserializeGoalStatusAgg(agg *GoalStatusAgg) *GoalStatusAgg {
	return serializeGoalStatusAgg(agg)
}

// serializeReminderAgg 深拷贝 ReminderAgg（所有 map 都深拷贝）
func serializeReminderAgg(agg *ReminderAgg) *ReminderAgg {
	if agg == nil {
		return nil
	}
	copyVal := *agg
	if agg.TaskSessionCounts != nil {
		copyVal.TaskSessionCounts = make(map[string]int, len(agg.TaskSessionCounts))
		for k, v := range agg.TaskSessionCounts {
			copyVal.TaskSessionCounts[k] = v
		}
	}
	if agg.TodoSessionCounts != nil {
		copyVal.TodoSessionCounts = make(map[string]int, len(agg.TodoSessionCounts))
		for k, v := range agg.TodoSessionCounts {
			copyVal.TodoSessionCounts[k] = v
		}
	}
	if agg.TaskSessionProjects != nil {
		copyVal.TaskSessionProjects = make(map[string]string, len(agg.TaskSessionProjects))
		for k, v := range agg.TaskSessionProjects {
			copyVal.TaskSessionProjects[k] = v
		}
	}
	if agg.TodoSessionProjects != nil && &copyVal.TodoSessionProjects != &agg.TodoSessionProjects {
		// already copied above if non-nil
	}
	return &copyVal
}

// deserializeReminderAgg 深拷贝 ReminderAgg
func deserializeReminderAgg(agg *ReminderAgg) *ReminderAgg {
	return serializeReminderAgg(agg)
}

// EmptyMessages 快速判断聚合是否有消息数据（数据护栏用）。
func (agg *ProjectAggregate) EmptyMessages() bool {
	return len(agg.DailyActivity) == 0
}
