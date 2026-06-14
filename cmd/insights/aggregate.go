package main

import (
	"sort"
	"sync"
)

func newProjectAggregate() *ProjectAggregate {
	aggregate := &ProjectAggregate{
		ProjectStats:          make(map[string]*ProjectStatItem),
		DailyActivity:         make(map[string]int),
		DailySessions:         make(map[string]map[string]bool),
		DailyProjectCounts:    make(map[string]map[string]int),
		DailyModelCounts:      make(map[string]map[string]int),
		DailyModelTokens:      make(map[string]map[string]int),
		DailyHourlyCounts:     make(map[string][24]int),
		DailyRuntime:          make(map[string]*ProjectAggregate),
		DailyProjectRuntime:   make(map[string]map[string]*ProjectAggregate),
		DailySessionRuntime:   make(map[string]map[string]*ProjectAggregate),
		ModelUsage:            make(map[string]*ModelUsageItem),
		ToolStats:             make(map[string]*ToolStatItem),
		ToolModelStats:        make(map[string]*ToolModelStatItem),
		FailureReasons:        make(map[string]*FailureReasonStat),
		FailureToolReasons:    make(map[string]*FailureToolReasonStat),
		FailureModelReasons:   make(map[string]*FailureModelReasonStat),
		SessionStatsMap:       make(map[string]*SessionAnalysisItem),
		SessionQueueOps:       make(map[string]int),
		EventTypes:            make(map[string]int),
		HookStats:             make(map[string]*HookStatItem),
		SkillStats:            make(map[string]*SkillStatItem),
		InstalledSkills:       make(map[string]*InstalledSkillItem),
		SkillUsageStats:       make(map[string]*SkillUsageStat),
		SkillListingStats:     make(map[string]int),
		SkillProjectStats:     make(map[string]*SkillProjectStat),
		SkillModelStats:       make(map[string]*SkillModelStat),
		SkillAgentStats:       make(map[string]*SkillAgentStat),
		SkillSessionToolStats: make(map[string]*SkillSessionToolStat),
		PermissionModes:       make(map[string]int),
		OpenedFiles:           make(map[string]*FileAccessStat),
		AgentStats:            make(map[string]*AgentStatItem),
		AgentSessions:         make(map[string]map[string]bool),
		BashCommandStats:      make(map[string]*BashCommandStat),
		FileOperationStats:    make(map[string]*FileOperationStat),
		FileHotStats:          make(map[string]*FileHotStat),
		FileEditFailures:      make(map[string]*FileEditFailureAgg),
		FileSnapshotStats:     make(map[string]*FileSnapshotAgg),
		FileEditedStats:       make(map[string]*FileEditedAgg),
		ToolPerfStats:         make(map[string]*ToolPerfAgg),
		HourlyCounts:          [24]int{},
		CostModelStats:        make(map[string]*CostModelStat),
		CostProjectStats:      make(map[string]*CostProjectStat),
		CostSessionStats:      make(map[string]*CostSessionStat),
		CostAgentStats:        make(map[string]*CostAgentStat),
		mu:                    sync.RWMutex{},
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
}

func aggregateToProjectFileAggregate(src *ProjectAggregate) ProjectFileAggregate {
	return aggregateToProjectFileAggregateWithDaily(src, true)
}

func aggregateToProjectFileAggregateWithDaily(src *ProjectAggregate, includeDaily bool) ProjectFileAggregate {
	out := ProjectFileAggregate{
		ProjectStats:          make(map[string]ProjectStatItem, len(src.ProjectStats)),
		WeekdayData:           src.WeekdayData,
		DailyActivity:         copyIntMap(src.DailyActivity),
		DailySessions:         boolSetMapToSlices(src.DailySessions),
		DailyProjectCounts:    copyNestedIntMap(src.DailyProjectCounts),
		DailyModelCounts:      copyNestedIntMap(src.DailyModelCounts),
		DailyModelTokens:      copyNestedIntMap(src.DailyModelTokens),
		DailyHourlyCounts:     copyDailyHourlyCounts(src.DailyHourlyCounts),
		DailyRuntime:          make(map[string]ProjectFileAggregate),
		DailyProjectRuntime:   make(map[string]map[string]ProjectFileAggregate),
		DailySessionRuntime:   make(map[string]map[string]ProjectFileAggregate),
		HourlyCounts:          src.HourlyCounts,
		ModelUsage:            make(map[string]ModelUsageItem, len(src.ModelUsage)),
		CostModelStats:        make(map[string]CostModelStat, len(src.CostModelStats)),
		CostProjectStats:      make(map[string]CostProjectStat, len(src.CostProjectStats)),
		CostSessionStats:      make(map[string]CostSessionStat, len(src.CostSessionStats)),
		CostAgentStats:        make(map[string]CostAgentStat, len(src.CostAgentStats)),
		BudgetTimeline:        append([]BudgetTimelineItem(nil), src.BudgetTimeline...),
		ToolStats:             make(map[string]ToolStatItem, len(src.ToolStats)),
		ToolModelStats:        make(map[string]ToolModelStatItem, len(src.ToolModelStats)),
		FailureReasons:        make(map[string]FailureReasonStat, len(src.FailureReasons)),
		FailureToolReasons:    make(map[string]FailureToolReasonStat, len(src.FailureToolReasons)),
		FailureModelReasons:   make(map[string]FailureModelReasonStat, len(src.FailureModelReasons)),
		FailureSamples:        append([]ToolFailureSample(nil), src.FailureSamples...),
		SessionStats:          make(map[string]SessionAnalysisItem, len(src.SessionStatsMap)),
		SessionQueueOps:       copyIntMap(src.SessionQueueOps),
		EventTypes:            copyIntMap(src.EventTypes),
		HookStats:             make(map[string]HookStatItem, len(src.HookStats)),
		SkillStats:            make(map[string]SkillStatItem, len(src.SkillStats)),
		InstalledSkills:       make(map[string]InstalledSkillItem, len(src.InstalledSkills)),
		SkillUsageStats:       make(map[string]SkillUsageStat, len(src.SkillUsageStats)),
		SkillListingStats:     copyIntMap(src.SkillListingStats),
		SkillProjectStats:     make(map[string]SkillProjectStat, len(src.SkillProjectStats)),
		SkillModelStats:       make(map[string]SkillModelStat, len(src.SkillModelStats)),
		SkillAgentStats:       make(map[string]SkillAgentStat, len(src.SkillAgentStats)),
		SkillSessionToolStats: make(map[string]SkillSessionToolStat, len(src.SkillSessionToolStats)),
		SkillListingEvents:    src.SkillListingEvents,
		SkillInitialListings:  src.SkillInitialListings,
		DynamicSkillEvents:    src.DynamicSkillEvents,
		PermissionModes:       copyIntMap(src.PermissionModes),
		OpenedFiles:           make(map[string]FileAccessStat, len(src.OpenedFiles)),
		AgentStats:            make(map[string]AgentStatItem, len(src.AgentStats)),
		AgentSessions:         boolSetMapToSlices(src.AgentSessions),
		BashCommandStats:      make(map[string]BashCommandStat, len(src.BashCommandStats)),
		FileOperationStats:    make(map[string]FileOperationStat, len(src.FileOperationStats)),
		FileHotStats:          make(map[string]FileHotStat, len(src.FileHotStats)),
		FileEditFailures:      make(map[string]FileEditFailureAgg, len(src.FileEditFailures)),
		FileSnapshotStats:     make(map[string]FileSnapshotAgg, len(src.FileSnapshotStats)),
		FileEditedStats:       make(map[string]FileEditedAgg, len(src.FileEditedStats)),
		PlanModeAgg:           serializePlanModeAgg(src.PlanModeAgg),
		GoalStatusAgg:         serializeGoalStatusAgg(src.GoalStatusAgg),
		ReminderAgg:           serializeReminderAgg(src.ReminderAgg),
		ToolPerfStats:         make(map[string]ToolPerfAgg, len(src.ToolPerfStats)),
		SlowestCalls:          append([]ToolSlowCallItem(nil), src.SlowestCalls...),
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
	for key, stat := range src.BashCommandStats {
		out.BashCommandStats[key] = *stat
	}
	for key, stat := range src.FileOperationStats {
		out.FileOperationStats[key] = *stat
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
	out.DailyActivity = copyIntMap(src.DailyActivity)
	out.DailySessions = slicesMapToBoolSets(src.DailySessions)
	out.DailyProjectCounts = copyNestedIntMap(src.DailyProjectCounts)
	out.DailyModelCounts = copyNestedIntMap(src.DailyModelCounts)
	out.DailyModelTokens = copyNestedIntMap(src.DailyModelTokens)
	out.DailyHourlyCounts = copyDailyHourlyCounts(src.DailyHourlyCounts)
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
	out.AgentSessions = slicesMapToBoolSets(src.AgentSessions)
	for key, stat := range src.BashCommandStats {
		statCopy := stat
		out.BashCommandStats[key] = &statCopy
	}
	for key, stat := range src.FileOperationStats {
		statCopy := stat
		out.FileOperationStats[key] = &statCopy
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
	// task_plan_analysis (M4): restore agg structs
	out.PlanModeAgg = deserializePlanModeAgg(src.PlanModeAgg)
	out.GoalStatusAgg = deserializeGoalStatusAgg(src.GoalStatusAgg)
	out.ReminderAgg = deserializeReminderAgg(src.ReminderAgg)
	// M5: restore ToolPerfStats
	for key, stat := range src.ToolPerfStats {
		statCopy := stat
		out.ToolPerfStats[key] = &statCopy
	}
	out.SlowestCalls = append([]ToolSlowCallItem(nil), src.SlowestCalls...)
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
