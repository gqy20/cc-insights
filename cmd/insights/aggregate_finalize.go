package main

import (
	"fmt"
	"sort"
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

	// 7. 转换运行事件、agent、命令/文件分析
	agg.finalizeEventAnalysis()
	agg.finalizeAgentAnalysis()
	agg.finalizeCommandAnalysis()
	agg.finalizeCostAnalysis()

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
	agg.AgentAnalysis = analysis
}

func (agg *ProjectAggregate) finalizeCommandAnalysis() {
	analysis := &CommandAnalysisData{
		BashCommands:   make([]BashCommandStat, 0, len(agg.BashCommandStats)),
		RiskyCommands:  make([]BashCommandStat, 0),
		FileOperations: make([]FileOperationStat, 0, len(agg.FileOperationStats)),
	}
	for _, stat := range agg.BashCommandStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.BashCommands = append(analysis.BashCommands, statCopy)
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
