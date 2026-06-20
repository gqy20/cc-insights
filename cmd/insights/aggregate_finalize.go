package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// finalize 生成输出格式的数据
func (agg *ProjectAggregate) finalize() {
	// 1. 转换项目列表并排序
	agg.Projects = make([]ProjectStatItem, 0, len(agg.ProjectStats))
	for _, proj := range agg.ProjectStats {
		agg.Projects = append(agg.Projects, *proj)
	}
	sort.Slice(agg.Projects, func(i, j int) bool {
		return agg.Projects[i].MessageCount > agg.Projects[j].MessageCount
	})

	// 2. 转换星期统计
	weekdayData := make([]WeekdayItem, 7)
	copy(weekdayData, agg.WeekdayData[:])
	agg.WeekdayStats = &WeekdayStats{WeekdayData: weekdayData}

	// 3. 转换每日活动为列表
	dates := make([]string, 0, len(agg.DailyActivity))
	for date := range agg.DailyActivity {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	agg.DailyActivityList = make([]DailyActivity, len(dates))
	for i, date := range dates {
		agg.DailyActivityList[i] = DailyActivity{
			Date:         date,
			MessageCount: agg.DailyActivity[date],
		}
	}

	// 4. 转换小时数据
	agg.HourlyData = make([]HourlyItem, 24)
	for i := 0; i < 24; i++ {
		agg.HourlyData[i] = HourlyItem{
			Hour:       i,
			HourLabel:  fmt.Sprintf("%02d:00", i),
			Count:      agg.HourlyCounts[i],
			IsWorkHour: i >= 9 && i <= 18,
		}
	}

	// 5. 转换模型使用列表
	agg.ModelUsageList = make([]ModelUsageItem, 0, len(agg.ModelUsage))
	for _, model := range agg.ModelUsage {
		agg.ModelUsageList = append(agg.ModelUsageList, *model)
	}
	sort.Slice(agg.ModelUsageList, func(i, j int) bool {
		return agg.ModelUsageList[i].Count > agg.ModelUsageList[j].Count
	})

	// 6. 转换工具分析
	agg.finalizeToolAnalysis()
	agg.finalizeFailureAnalysis()
	finalizeSkillAnalysisLocked(agg)

	// 7. 转换运行事件、agent、命令/文件分析
	agg.finalizeEventAnalysis()
	agg.finalizeAgentAnalysis()
	agg.finalizeCommandAnalysis()
	agg.finalizeCostAnalysis()
	agg.finalizeSessionAnalysis()
	agg.finalizeFileAnalysis()
	agg.finalizeTaskPlanAnalysis()
	agg.finalizeToolPerformance()

	// 8. 生成工作时段统计
	var workHoursCount, offHoursCount int
	var peakHour, peakCount int

	for _, item := range agg.HourlyData {
		if item.IsWorkHour {
			workHoursCount += item.Count
		} else {
			offHoursCount += item.Count
		}

		if item.Count > peakCount {
			peakCount = item.Count
			peakHour = item.Hour
		}
	}

	total := workHoursCount + offHoursCount
	var workRatio float64
	if total > 0 {
		workRatio = float64(workHoursCount) / float64(total) * 100
	}

	agg.WorkHoursStats = &WorkHoursStats{
		HourlyData:     agg.HourlyData,
		WorkHoursCount: workHoursCount,
		OffHoursCount:  offHoursCount,
		WorkHoursRatio: workRatio,
		PeakHour:       peakHour,
		PeakHourCount:  peakCount,
	}
}

func (agg *ProjectAggregate) finalizeToolAnalysis() {
	analysis := &ToolAnalysisData{
		Tools:   make([]ToolStatItem, 0, len(agg.ToolStats)),
		ByModel: make([]ToolModelStatItem, 0, len(agg.ToolModelStats)),
	}

	for _, stat := range agg.ToolStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.TotalCalls += statCopy.CallCount
		analysis.TotalFailures += statCopy.FailureCount
		analysis.MissingResults += statCopy.MissingResultCount
		analysis.Tools = append(analysis.Tools, statCopy)
	}
	sort.Slice(analysis.Tools, func(i, j int) bool {
		if analysis.Tools[i].CallCount == analysis.Tools[j].CallCount {
			return analysis.Tools[i].Tool < analysis.Tools[j].Tool
		}
		return analysis.Tools[i].CallCount > analysis.Tools[j].CallCount
	})

	for _, stat := range agg.ToolModelStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.ByModel = append(analysis.ByModel, statCopy)
	}
	sort.Slice(analysis.ByModel, func(i, j int) bool {
		if analysis.ByModel[i].FailureCount == analysis.ByModel[j].FailureCount {
			if analysis.ByModel[i].CallCount == analysis.ByModel[j].CallCount {
				if analysis.ByModel[i].Model == analysis.ByModel[j].Model {
					return analysis.ByModel[i].Tool < analysis.ByModel[j].Tool
				}
				return analysis.ByModel[i].Model < analysis.ByModel[j].Model
			}
			return analysis.ByModel[i].CallCount > analysis.ByModel[j].CallCount
		}
		return analysis.ByModel[i].FailureCount > analysis.ByModel[j].FailureCount
	})

	agg.ToolAnalysis = analysis
}

func (agg *ProjectAggregate) finalizeFailureAnalysis() {
	analysis := &FailureAnalysisData{
		ByReason:      make([]FailureReasonStat, 0, len(agg.FailureReasons)),
		ByToolReason:  make([]FailureToolReasonStat, 0, len(agg.FailureToolReasons)),
		ByModelReason: make([]FailureModelReasonStat, 0, len(agg.FailureModelReasons)),
	}
	analysis.Samples = append([]ToolFailureSample(nil), agg.FailureSamples...)

	for _, stat := range agg.FailureReasons {
		statCopy := *stat
		analysis.TotalFailures += statCopy.Count
		analysis.ByReason = append(analysis.ByReason, statCopy)
	}
	sort.Slice(analysis.ByReason, func(i, j int) bool {
		if analysis.ByReason[i].Count == analysis.ByReason[j].Count {
			if analysis.ByReason[i].Category == analysis.ByReason[j].Category {
				return analysis.ByReason[i].Reason < analysis.ByReason[j].Reason
			}
			return analysis.ByReason[i].Category < analysis.ByReason[j].Category
		}
		return analysis.ByReason[i].Count > analysis.ByReason[j].Count
	})

	for _, stat := range agg.FailureToolReasons {
		statCopy := *stat
		if toolStat := agg.ToolStats[statCopy.Tool]; toolStat != nil && toolStat.CallCount > 0 {
			statCopy.Rate = float64(statCopy.Count) / float64(toolStat.CallCount) * 100
		}
		analysis.ByToolReason = append(analysis.ByToolReason, statCopy)
	}
	sort.Slice(analysis.ByToolReason, func(i, j int) bool {
		if analysis.ByToolReason[i].Count == analysis.ByToolReason[j].Count {
			if analysis.ByToolReason[i].Tool == analysis.ByToolReason[j].Tool {
				if analysis.ByToolReason[i].Category == analysis.ByToolReason[j].Category {
					return analysis.ByToolReason[i].Reason < analysis.ByToolReason[j].Reason
				}
				return analysis.ByToolReason[i].Category < analysis.ByToolReason[j].Category
			}
			return analysis.ByToolReason[i].Tool < analysis.ByToolReason[j].Tool
		}
		return analysis.ByToolReason[i].Count > analysis.ByToolReason[j].Count
	})

	for _, stat := range agg.FailureModelReasons {
		statCopy := *stat
		var modelCalls int
		for _, toolStat := range agg.ToolModelStats {
			if toolStat.Model == statCopy.Model {
				modelCalls += toolStat.CallCount
			}
		}
		if modelCalls > 0 {
			statCopy.Rate = float64(statCopy.Count) / float64(modelCalls) * 100
		}
		analysis.ByModelReason = append(analysis.ByModelReason, statCopy)
	}
	sort.Slice(analysis.ByModelReason, func(i, j int) bool {
		if analysis.ByModelReason[i].Count == analysis.ByModelReason[j].Count {
			if analysis.ByModelReason[i].Model == analysis.ByModelReason[j].Model {
				if analysis.ByModelReason[i].Category == analysis.ByModelReason[j].Category {
					return analysis.ByModelReason[i].Reason < analysis.ByModelReason[j].Reason
				}
				return analysis.ByModelReason[i].Category < analysis.ByModelReason[j].Category
			}
			return analysis.ByModelReason[i].Model < analysis.ByModelReason[j].Model
		}
		return analysis.ByModelReason[i].Count > analysis.ByModelReason[j].Count
	})

	agg.FailureAnalysis = analysis
}

// sampleSessionIDs 从 session 去重集合取前 limit 个 session id（确定性排序），用于 hook→session 下钻入口。
func sampleSessionIDs(ids map[string]bool, limit int) []string {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, 0, len(ids))
	for sid := range ids {
		keys = append(keys, sid)
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
}

func (agg *ProjectAggregate) finalizeEventAnalysis() {
	analysis := &EventAnalysisData{
		ByType:          make([]EventTypeStat, 0, len(agg.EventTypes)),
		Hooks:           make([]HookStatItem, 0, len(agg.HookStats)),
		Skills:          make([]SkillStatItem, 0, len(agg.SkillStats)),
		PermissionModes: make([]PermissionModeStat, 0, len(agg.PermissionModes)),
		OpenedFiles:     make([]FileAccessStat, 0, len(agg.OpenedFiles)),
		Samples:         append([]EventSample(nil), agg.EventSamples...),
	}
	for eventType, count := range agg.EventTypes {
		analysis.TotalEvents += count
		analysis.ByType = append(analysis.ByType, EventTypeStat{Type: eventType, Count: count})
	}
	sort.Slice(analysis.ByType, func(i, j int) bool {
		if analysis.ByType[i].Count == analysis.ByType[j].Count {
			return analysis.ByType[i].Type < analysis.ByType[j].Type
		}
		return analysis.ByType[i].Count > analysis.ByType[j].Count
	})

	for _, stat := range agg.HookStats {
		statCopy := *stat
		if statCopy.TotalCount > 0 {
			statCopy.FailureRate = float64(statCopy.CancelledCount+statCopy.ErrorCount) / float64(statCopy.TotalCount) * 100
		}
		// hook→session：把去重集合转成计数 + 代表 session，完整集合不随 EventAnalysis 输出。
		statCopy.SessionCount = len(statCopy.SessionIDs)
		statCopy.SampleSession = sampleSessionIDs(statCopy.SessionIDs, 8)
		statCopy.SessionIDs = nil
		analysis.Hooks = append(analysis.Hooks, statCopy)
	}
	sort.Slice(analysis.Hooks, func(i, j int) bool {
		if analysis.Hooks[i].ErrorCount == analysis.Hooks[j].ErrorCount {
			return analysis.Hooks[i].TotalCount > analysis.Hooks[j].TotalCount
		}
		return analysis.Hooks[i].ErrorCount > analysis.Hooks[j].ErrorCount
	})

	for _, stat := range agg.SkillStats {
		analysis.Skills = append(analysis.Skills, *stat)
	}
	sort.Slice(analysis.Skills, func(i, j int) bool {
		if analysis.Skills[i].Count == analysis.Skills[j].Count {
			return analysis.Skills[i].Name < analysis.Skills[j].Name
		}
		return analysis.Skills[i].Count > analysis.Skills[j].Count
	})

	for mode, count := range agg.PermissionModes {
		analysis.PermissionModes = append(analysis.PermissionModes, PermissionModeStat{Mode: mode, Count: count})
	}
	sort.Slice(analysis.PermissionModes, func(i, j int) bool {
		if analysis.PermissionModes[i].Count == analysis.PermissionModes[j].Count {
			return analysis.PermissionModes[i].Mode < analysis.PermissionModes[j].Mode
		}
		return analysis.PermissionModes[i].Count > analysis.PermissionModes[j].Count
	})

	for _, stat := range agg.OpenedFiles {
		analysis.OpenedFiles = append(analysis.OpenedFiles, *stat)
	}
	sort.Slice(analysis.OpenedFiles, func(i, j int) bool {
		if analysis.OpenedFiles[i].Count == analysis.OpenedFiles[j].Count {
			return analysis.OpenedFiles[i].Path < analysis.OpenedFiles[j].Path
		}
		return analysis.OpenedFiles[i].Count > analysis.OpenedFiles[j].Count
	})
	if len(analysis.OpenedFiles) > 50 {
		analysis.OpenedFiles = analysis.OpenedFiles[:50]
	}

	analysis.QueuedCommands = agg.EventTypes["attachment:queued_command"]
	analysis.PlanModeCount = agg.EventTypes["attachment:plan_mode"]
	analysis.PlanModeExitCount = agg.EventTypes["attachment:plan_mode_exit"]
	if agg.BudgetSummary != nil {
		budgetCopy := *agg.BudgetSummary
		analysis.Budget = &budgetCopy
	}
	agg.EventAnalysis = analysis
}

func (agg *ProjectAggregate) finalizeAgentAnalysis() {
	analysis := &AgentAnalysisData{
		Agents: make([]AgentStatItem, 0, len(agg.AgentStats)),
	}
	for _, stat := range agg.AgentStats {
		statCopy := *stat
		if statCopy.ToolCallCount > 0 {
			statCopy.FailureRate = float64(statCopy.ToolFailureCount) / float64(statCopy.ToolCallCount) * 100
		}
		if statCopy.IsSidechain {
			analysis.SidechainToolCalls += statCopy.ToolCallCount
		} else {
			analysis.MainToolCalls += statCopy.ToolCallCount
		}
		analysis.Agents = append(analysis.Agents, statCopy)
	}
	sort.Slice(analysis.Agents, func(i, j int) bool {
		if analysis.Agents[i].ToolFailureCount == analysis.Agents[j].ToolFailureCount {
			if analysis.Agents[i].ToolCallCount == analysis.Agents[j].ToolCallCount {
				return analysis.Agents[i].AgentID < analysis.Agents[j].AgentID
			}
			return analysis.Agents[i].ToolCallCount > analysis.Agents[j].ToolCallCount
		}
		return analysis.Agents[i].ToolFailureCount > analysis.Agents[j].ToolFailureCount
	})

	analysis.ByModel = make([]AgentModelStat, 0, len(agg.AgentModelStats))
	for _, stat := range agg.AgentModelStats {
		statCopy := *stat
		if statCopy.ToolCallCount > 0 {
			statCopy.FailureRate = float64(statCopy.ToolFailureCount) / float64(statCopy.ToolCallCount) * 100
		}
		analysis.ByModel = append(analysis.ByModel, statCopy)
	}
	sort.Slice(analysis.ByModel, func(i, j int) bool {
		if analysis.ByModel[i].ToolFailureCount == analysis.ByModel[j].ToolFailureCount {
			if analysis.ByModel[i].ToolCallCount == analysis.ByModel[j].ToolCallCount {
				if analysis.ByModel[i].Model == analysis.ByModel[j].Model {
					return analysis.ByModel[i].AgentID < analysis.ByModel[j].AgentID
				}
				return analysis.ByModel[i].Model < analysis.ByModel[j].Model
			}
			return analysis.ByModel[i].ToolCallCount > analysis.ByModel[j].ToolCallCount
		}
		return analysis.ByModel[i].ToolFailureCount > analysis.ByModel[j].ToolFailureCount
	})
	agg.AgentAnalysis = analysis
}

func (agg *ProjectAggregate) finalizeCommandAnalysis() {
	analysis := &CommandAnalysisData{
		BashCommands:   make([]BashCommandStat, 0, len(agg.BashCommandStats)),
		BashFamilies:   make([]BashCommandFamilyStat, 0),
		RiskyCommands:  make([]BashCommandStat, 0),
		FileOperations: make([]FileOperationStat, 0, len(agg.FileOperationStats)),
	}
	familyStats := make(map[string]*BashCommandFamilyStat)
	for _, stat := range agg.BashCommandStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.BashCommands = append(analysis.BashCommands, statCopy)
		familyName := classifyBashCommandFamily(statCopy)
		family := familyStats[familyName]
		if family == nil {
			family = &BashCommandFamilyStat{Family: familyName}
			familyStats[familyName] = family
		}
		family.CallCount += statCopy.CallCount
		family.SuccessCount += statCopy.SuccessCount
		family.FailureCount += statCopy.FailureCount
		family.MissingResultCount += statCopy.MissingResultCount
		if statCopy.CallCount > family.TopCommandCalls || (statCopy.CallCount == family.TopCommandCalls && statCopy.CommandName < family.TopCommand) {
			family.TopCommand = statCopy.CommandName
			family.TopCommandCalls = statCopy.CallCount
			family.SampleCommand = statCopy.SampleCommand
		}
		if statCopy.RiskLevel != "" {
			analysis.RiskyCommands = append(analysis.RiskyCommands, statCopy)
		}
	}
	sort.Slice(analysis.BashCommands, func(i, j int) bool {
		if analysis.BashCommands[i].CallCount == analysis.BashCommands[j].CallCount {
			return analysis.BashCommands[i].CommandName < analysis.BashCommands[j].CommandName
		}
		return analysis.BashCommands[i].CallCount > analysis.BashCommands[j].CallCount
	})
	sort.Slice(analysis.RiskyCommands, func(i, j int) bool {
		if analysis.RiskyCommands[i].RiskLevel == analysis.RiskyCommands[j].RiskLevel {
			return analysis.RiskyCommands[i].CallCount > analysis.RiskyCommands[j].CallCount
		}
		return riskRank(analysis.RiskyCommands[i].RiskLevel) > riskRank(analysis.RiskyCommands[j].RiskLevel)
	})
	for _, stat := range familyStats {
		familyCopy := *stat
		if familyCopy.CallCount > 0 {
			familyCopy.FailureRate = float64(familyCopy.FailureCount) / float64(familyCopy.CallCount) * 100
		}
		analysis.BashFamilies = append(analysis.BashFamilies, familyCopy)
	}
	sort.Slice(analysis.BashFamilies, func(i, j int) bool {
		if analysis.BashFamilies[i].CallCount == analysis.BashFamilies[j].CallCount {
			if analysis.BashFamilies[i].FailureCount == analysis.BashFamilies[j].FailureCount {
				return analysis.BashFamilies[i].Family < analysis.BashFamilies[j].Family
			}
			return analysis.BashFamilies[i].FailureCount > analysis.BashFamilies[j].FailureCount
		}
		return analysis.BashFamilies[i].CallCount > analysis.BashFamilies[j].CallCount
	})

	for _, stat := range agg.FileOperationStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.FileOperations = append(analysis.FileOperations, statCopy)
	}
	sort.Slice(analysis.FileOperations, func(i, j int) bool {
		if analysis.FileOperations[i].FailureCount == analysis.FileOperations[j].FailureCount {
			if analysis.FileOperations[i].CallCount == analysis.FileOperations[j].CallCount {
				return analysis.FileOperations[i].Path < analysis.FileOperations[j].Path
			}
			return analysis.FileOperations[i].CallCount > analysis.FileOperations[j].CallCount
		}
		return analysis.FileOperations[i].FailureCount > analysis.FileOperations[j].FailureCount
	})
	if len(analysis.BashCommands) > 50 {
		analysis.BashCommands = analysis.BashCommands[:50]
	}
	if len(analysis.RiskyCommands) > 50 {
		analysis.RiskyCommands = analysis.RiskyCommands[:50]
	}
	if len(analysis.FileOperations) > 100 {
		analysis.FileOperations = analysis.FileOperations[:100]
	}

	analysis.ByModel = make([]BashCommandModelStat, 0, len(agg.BashCommandModelStats))
	for _, stat := range agg.BashCommandModelStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.ByModel = append(analysis.ByModel, statCopy)
	}
	sort.Slice(analysis.ByModel, func(i, j int) bool {
		if analysis.ByModel[i].FailureCount == analysis.ByModel[j].FailureCount {
			if analysis.ByModel[i].CallCount == analysis.ByModel[j].CallCount {
				if analysis.ByModel[i].Model == analysis.ByModel[j].Model {
					return analysis.ByModel[i].CommandName < analysis.ByModel[j].CommandName
				}
				return analysis.ByModel[i].Model < analysis.ByModel[j].Model
			}
			return analysis.ByModel[i].CallCount > analysis.ByModel[j].CallCount
		}
		return analysis.ByModel[i].FailureCount > analysis.ByModel[j].FailureCount
	})
	if len(analysis.ByModel) > 200 {
		analysis.ByModel = analysis.ByModel[:200]
	}
	agg.CommandAnalysis = analysis
}

func riskRank(level string) int {
	switch level {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// finalizeFileAnalysis 生成文件与编辑质量分析结果
func (agg *ProjectAggregate) finalizeFileAnalysis() {
	analysis := &FileAnalysisData{
		HotFiles:     make([]FileHotItem, 0, len(agg.FileHotStats)),
		EditFailures: make([]FileEditFailureItem, 0, len(agg.FileEditFailures)),
		Snapshots:    make([]FileSnapshotItem, 0, len(agg.FileSnapshotStats)),
		EditedFiles:  make([]FileEditedItem, 0, len(agg.FileEditedStats)),
	}

	totalReads, totalEdits, totalWrites := 0, 0, 0
	totalEditFailures, totalWriteFailures := 0, 0

	// --- P0: Hot Files（按路径聚合的跨操作类型统计）---
	for path, stat := range agg.FileHotStats {
		totalReads += stat.ReadCount
		totalEdits += stat.EditCount
		totalWrites += stat.WriteCount
		totalOps := stat.ReadCount + stat.EditCount + stat.WriteCount
		failureRate := 0.0
		if totalOps > 0 {
			failureRate = float64(stat.FailureCount) / float64(totalOps) * 100
		}
		editFailureRate := 0.0
		if stat.EditCount > 0 {
			if ef, ok := agg.FileEditFailures[path]; ok {
				editFailureRate = float64(ef.TotalFailures) / float64(stat.EditCount) * 100
			}
		}
		analysis.HotFiles = append(analysis.HotFiles, FileHotItem{
			Path:            path,
			ReadCount:       stat.ReadCount,
			EditCount:       stat.EditCount,
			WriteCount:      stat.WriteCount,
			TotalOps:        totalOps,
			SuccessCount:    stat.SuccessCount,
			FailureCount:    stat.FailureCount,
			MissingCount:    stat.MissingCount,
			FailureRate:     failureRate,
			EditFailureRate: editFailureRate,
		})
	}
	sort.Slice(analysis.HotFiles, func(i, j int) bool {
		return analysis.HotFiles[i].TotalOps > analysis.HotFiles[j].TotalOps
	})
	if len(analysis.HotFiles) > 50 {
		analysis.HotFiles = analysis.HotFiles[:50]
	}

	// --- P0: Edit Failures per File（按文件的 Edit 失败原因分布）---
	for path, ef := range agg.FileEditFailures {
		item := FileEditFailureItem{
			Path:           path,
			TotalFailures:  ef.TotalFailures,
			FailureReasons: make([]FileFailureReasonDetail, 0, len(ef.ReasonCounts)),
		}
		totalEditFailures += ef.TotalFailures
		for reason, count := range ef.ReasonCounts {
			rate := 0.0
			if ef.TotalFailures > 0 {
				rate = float64(count) / float64(ef.TotalFailures) * 100
			}
			item.FailureReasons = append(item.FailureReasons, FileFailureReasonDetail{
				Reason: reason,
				Count:  count,
				Rate:   rate,
			})
		}
		sort.Slice(item.FailureReasons, func(i, j int) bool {
			return item.FailureReasons[i].Count > item.FailureReasons[j].Count
		})
		analysis.EditFailures = append(analysis.EditFailures, item)
	}
	sort.Slice(analysis.EditFailures, func(i, j int) bool {
		return analysis.EditFailures[i].TotalFailures > analysis.EditFailures[j].TotalFailures
	})
	if len(analysis.EditFailures) > 30 {
		analysis.EditFailures = analysis.EditFailures[:30]
	}

	// --- P1: Snapshots（file-history-snapshot 统计）---
	for path, ss := range agg.FileSnapshotStats {
		analysis.Totals.SnapshotEventCount += ss.SnapshotCount
		analysis.Snapshots = append(analysis.Snapshots, FileSnapshotItem{
			Path:          path,
			SnapshotCount: ss.SnapshotCount,
			MaxVersion:    ss.MaxVersion,
			SessionCount:  len(ss.SessionSet),
			IsUpdateCount: ss.IsUpdateCount,
		})
	}
	sort.Slice(analysis.Snapshots, func(i, j int) bool {
		return analysis.Snapshots[i].SessionCount > analysis.Snapshots[j].SessionCount
	})
	if len(analysis.Snapshots) > 30 {
		analysis.Snapshots = analysis.Snapshots[:30]
	}

	// --- P2: Edited Files（edited_text_file 统计）---
	for path, ed := range agg.FileEditedStats {
		analysis.Totals.EditedFileCount++
		avgLines := 0
		if ed.EditCount > 0 {
			avgLines = ed.TotalLines / ed.EditCount
		}
		avgChars := 0
		if ed.EditCount > 0 {
			avgChars = int(ed.TotalChars / int64(ed.EditCount))
		}
		analysis.EditedFiles = append(analysis.EditedFiles, FileEditedItem{
			Path:       path,
			EditCount:  ed.EditCount,
			AvgLines:   avgLines,
			AvgChars:   avgChars,
			TotalChars: ed.TotalChars,
		})
	}
	sort.Slice(analysis.EditedFiles, func(i, j int) bool {
		return analysis.EditedFiles[i].TotalChars > analysis.EditedFiles[j].TotalChars
	})
	if len(analysis.EditedFiles) > 20 {
		analysis.EditedFiles = analysis.EditedFiles[:20]
	}

	// --- Totals ---
	uniqueFiles := len(agg.FileHotStats)
	overallEditFailureRate := 0.0
	if totalEdits > 0 {
		overallEditFailureRate = float64(totalEditFailures) / float64(totalEdits) * 100
	}

	// Write failures from hot stats (write failure = total failure - edit failure for write ops is tracked separately)
	// We approximate: Write failures are part of FailureCount in FileHotStat for Write operations
	// For now use the total failure count minus known edit failures as approximation
	for _, hf := range analysis.HotFiles {
		if hf.WriteCount > 0 && hf.FailureCount > 0 {
			// Approximate write failures proportionally
			writeRatio := float64(hf.WriteCount) / float64(max(hf.TotalOps, 1))
			totalWriteFailures += int(float64(hf.FailureCount) * writeRatio * 0.5) // rough estimate
		}
	}

	analysis.Totals = FileAnalysisTotals{
		UniqueFiles:            uniqueFiles,
		TotalReads:             totalReads,
		TotalEdits:             totalEdits,
		TotalWrites:            totalWrites,
		TotalEditFailures:      totalEditFailures,
		TotalWriteFailures:     totalWriteFailures,
		OverallEditFailureRate: overallEditFailureRate,
		SnapshotEventCount:     analysis.Totals.SnapshotEventCount,
		EditedFileCount:        analysis.Totals.EditedFileCount,
	}

	analysis.ByModel = make([]FileOperationModelStat, 0, len(agg.FileOperationModelStats))
	for _, stat := range agg.FileOperationModelStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.ByModel = append(analysis.ByModel, statCopy)
	}
	sort.Slice(analysis.ByModel, func(i, j int) bool {
		if analysis.ByModel[i].FailureCount == analysis.ByModel[j].FailureCount {
			if analysis.ByModel[i].CallCount == analysis.ByModel[j].CallCount {
				if analysis.ByModel[i].Model == analysis.ByModel[j].Model {
					return analysis.ByModel[i].Path < analysis.ByModel[j].Path
				}
				return analysis.ByModel[i].Model < analysis.ByModel[j].Model
			}
			return analysis.ByModel[i].CallCount > analysis.ByModel[j].CallCount
		}
		return analysis.ByModel[i].FailureCount > analysis.ByModel[j].FailureCount
	})
	if len(analysis.ByModel) > 200 {
		analysis.ByModel = analysis.ByModel[:200]
	}
	agg.FileAnalysis = analysis
}

// finalizeTaskPlanAnalysis 生成 Task / Plan 结构分析结果
func (agg *ProjectAggregate) finalizeTaskPlanAnalysis() {
	out := &TaskPlanAnalysisData{}

	// 1. Plan Lifecycle
	if agg.PlanModeAgg != nil {
		pma := agg.PlanModeAgg
		lifecycle := PlanLifecycleData{
			EntryCount:       pma.EntryCount,
			ExitCount:        pma.ExitCount,
			ReentryCount:     pma.ReentryCount,
			UniquePlans:      len(pma.PlanFilePaths),
			SessionsWithPlan: len(pma.SessionSet),
		}
		for reason, count := range pma.ExitReasons {
			lifecycle.ExitReasons = append(lifecycle.ExitReasons, PlanExitReasonItem{
				Reason: reason,
				Count:  count,
			})
		}
		sort.Slice(lifecycle.ExitReasons, func(i, j int) bool {
			return lifecycle.ExitReasons[i].Count > lifecycle.ExitReasons[j].Count
		})
		out.PlanLifecycle = lifecycle

		// 2. Plan Files (top 20 by ref count)
		for _, pf := range pma.PlanFilePaths {
			out.PlanFiles = append(out.PlanFiles, PlanFileItem{
				FilePath:  pf.FilePath,
				FileName:  pf.FileName,
				CharCount: pf.CharCount,
				LineCount: pf.LineCount,
				HasCode:   pf.HasCode,
				Preview:   pf.Preview,
				RefCount:  pf.RefCount,
			})
		}
		sort.Slice(out.PlanFiles, func(i, j int) bool {
			return out.PlanFiles[i].RefCount > out.PlanFiles[j].RefCount
		})
		if len(out.PlanFiles) > 20 {
			out.PlanFiles = out.PlanFiles[:20]
		}
	}

	// 3. Goal Status
	if agg.GoalStatusAgg != nil && len(agg.GoalStatusAgg.Items) > 0 {
		out.GoalStatus = append([]GoalStatusItem(nil), agg.GoalStatusAgg.Items...)
	}

	// 4. Reminder Summary
	if agg.ReminderAgg != nil {
		ra := agg.ReminderAgg
		reminder := ReminderSummaryData{
			TaskReminderCount: ra.TaskReminderCount,
			TodoReminderCount: ra.TodoReminderCount,
			SessionsWithTask:  len(ra.TaskSessionCounts),
			SessionsWithTodo:  len(ra.TodoSessionCounts),
		}
		// Top task sessions
		type sessionCount struct {
			sid   string
			count int
		}
		var taskList []sessionCount
		for sid, cnt := range ra.TaskSessionCounts {
			taskList = append(taskList, sessionCount{sid, cnt})
		}
		sort.Slice(taskList, func(i, j int) bool { return taskList[i].count > taskList[j].count })
		if len(taskList) > 10 {
			taskList = taskList[:10]
		}
		for _, sc := range taskList {
			proj := ra.TaskSessionProjects[sc.sid]
			reminder.TopTaskSessions = append(reminder.TopTaskSessions, ReminderSessionItem{
				SessionID: sc.sid, Project: proj, Count: sc.count,
			})
		}
		// Top todo sessions
		var todoList []sessionCount
		for sid, cnt := range ra.TodoSessionCounts {
			todoList = append(todoList, sessionCount{sid, cnt})
		}
		sort.Slice(todoList, func(i, j int) bool { return todoList[i].count > todoList[j].count })
		if len(todoList) > 10 {
			todoList = todoList[:10]
		}
		for _, sc := range todoList {
			proj := ra.TodoSessionProjects[sc.sid]
			reminder.TopTodoSessions = append(reminder.TopTodoSessions, ReminderSessionItem{
				SessionID: sc.sid, Project: proj, Count: sc.count,
			})
		}
		out.ReminderSummary = reminder
	}

	// 5. Tasks (from independent scan)
	if agg.TaskAgg != nil {
		out.Tasks = *finalizeTaskAnalysis(agg.TaskAgg)
	}

	agg.TaskPlanAnalysis = out
}

// finalizeToolPerformance 生成工具性能与质量分析结果 (M5)
// 按 BaseTool 分层截断，避免文件路径淹没命令分类
func (agg *ProjectAggregate) finalizeToolPerformance() {
	if len(agg.ToolPerfStats) == 0 {
		return
	}

	// Per-BaseTool 截断配额：每种工具类型独立 Top-N（每类至少 50）
	var perToolCap = map[string]int{
		"Bash":      80, // 命令最多样化
		"Read":      60, // 文件路径
		"Edit":      60, // 文件路径
		"Write":     50,
		"MultiEdit": 50,
		"default":   50, // Agent/TaskOutput/MCP 等内置工具
	}

	out := &ToolPerformanceData{
		ByCategory:          make([]ToolPerfCategoryItem, 0),
		CategoryGroups:      make(map[string][]ToolPerfCategoryItem),
		SlowestCalls:        agg.SlowestCalls,
		QualityDistribution: buildQualityDistribution(agg.ToolPerfStats),
	}

	totalPaired := 0
	totalErrors := 0
	grandDuration := int64(0)

	// Phase 1: 构建 + 按 BaseTool 分组
	groups := make(map[string][]ToolPerfCategoryItem)

	for key, stat := range agg.ToolPerfStats {
		item := ToolPerfCategoryItem{
			Category:        formatCategoryDisplayFromKey(key),
			CallCount:       stat.CallCount,
			SuccessCount:    stat.SuccessCount,
			ErrorCount:      stat.ErrorCount,
			MissingCount:    stat.MissingCount,
			TotalDurationMs: stat.TotalDurationMs,
			MinDurationMs:   stat.MinDurationMs,
			MaxDurationMs:   stat.MaxDurationMs,
			TotalResultSize: stat.TotalResultSize,
			EmptyResults:    stat.EmptyResults,
			SampleInput:     stat.SampleInput,
		}

		// Split key into baseTool and subKey
		if idx := strings.IndexByte(key, '\x00'); idx >= 0 {
			item.BaseTool = key[:idx]
			item.SubKey = key[idx+1:]
		} else {
			item.BaseTool = key
		}

		// Fix MinDurationMs if no valid durations were recorded
		if item.MinDurationMs == math.MaxInt64 {
			item.MinDurationMs = 0
		}

		// Derived metrics
		if item.CallCount > 0 {
			item.ErrorRate = float64(item.ErrorCount) / float64(item.CallCount) * 100
			item.AvgDurationMs = float64(item.TotalDurationMs) / float64(item.CallCount)
			item.AvgResultSize = float64(item.TotalResultSize) / float64(item.CallCount)
		}

		groups[item.BaseTool] = append(groups[item.BaseTool], item)

		paired := stat.CallCount - stat.MissingCount
		if paired > 0 {
			totalPaired += paired
		}
		totalErrors += stat.ErrorCount
		grandDuration += stat.TotalDurationMs
	}

	// Phase 2: 每组内按 TotalDurationMs 降序排列，应用配额截断
	for baseTool, items := range groups {
		sort.Slice(items, func(i, j int) bool {
			return items[i].TotalDurationMs > items[j].TotalDurationMs
		})

		cap := perToolCap["default"]
		if c, ok := perToolCap[baseTool]; ok {
			cap = c
		}
		// Runtime 工具通常数量少且每个都有意义，不截断
		if strings.HasPrefix(baseTool, "mcp__") {
			cap = len(items)
		}

		if len(items) > cap {
			items = items[:cap]
		}

		out.CategoryGroups[baseTool] = items
		out.ByCategory = append(out.ByCategory, items...)
	}

	// Phase 3: 最终按 TotalDurationMs 全局排序（保持分层结果的顺序可读）
	sort.Slice(out.ByCategory, func(i, j int) bool {
		return out.ByCategory[i].TotalDurationMs > out.ByCategory[j].TotalDurationMs
	})

	out.TotalPairedCalls = totalPaired
	out.TotalErrors = totalErrors
	if totalPaired > 0 {
		out.OverallErrorRate = float64(totalErrors) / float64(totalPaired) * 100
	}
	if totalPaired > 0 {
		out.OverallAvgDuration = float64(grandDuration) / float64(totalPaired)
	}

	agg.ToolPerformance = out
}

func formatCategoryDisplayFromKey(key string) string {
	if idx := strings.IndexByte(key, '\x00'); idx >= 0 {
		base := key[:idx]
		sub := key[idx+1:]
		if sub == "" {
			return base
		}
		return base + ":" + sub
	}
	return key
}

func buildQualityDistribution(stats map[string]*ToolPerfAgg) []QualityBucketItem {
	buckets := []struct {
		Name  string
		Limit int // upper bound (exclusive); 0 means exactly 0
	}{
		{"空", 0},
		{"极小(≤50B)", 50},
		{"小(≤200B)", 200},
		{"中(≤1KB)", 1024},
		{"大(≤5KB)", 5120},
		{"巨大(>5KB)", 999999999},
	}

	counts := make([]int, len(buckets))
	totalCalls := int64(0)

	for _, stat := range stats {
		avgSize := 0
		if stat.CallCount > 0 {
			avgSize = int(stat.TotalResultSize / int64(stat.CallCount))
		}
		totalCalls += int64(stat.CallCount)

		bucketIdx := len(buckets) - 1
		for i, b := range buckets {
			if i == 0 { // empty
				if stat.EmptyResults > stat.CallCount/2 {
					bucketIdx = 0
					break
				}
				continue
			}
			if avgSize < b.Limit && avgSize > 0 {
				bucketIdx = i
				break
			} else if avgSize == 0 && i < len(buckets)-1 {
				bucketIdx = i
				break
			}
		}
		counts[bucketIdx] += stat.CallCount
	}

	result := make([]QualityBucketItem, 0, len(buckets))
	for i, b := range buckets {
		rate := 0.0
		if totalCalls > 0 {
			rate = float64(counts[i]) / float64(totalCalls) * 100
		}
		result = append(result, QualityBucketItem{
			Bucket: b.Name,
			Count:  counts[i],
			Rate:   rate,
		})
	}
	return result
}
