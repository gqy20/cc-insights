package main

import "sort"

func recordCostUsageLocked(agg *ProjectAggregate, msg AssistantMessage, record ProjectRecord, projectName string) {
	if msg.Model == "" {
		return
	}

	serverToolUseRequests := msg.Usage.ServerToolUse.WebSearchRequests + msg.Usage.ServerToolUse.WebFetchRequests
	totalTokens := msg.Usage.InputTokens +
		msg.Usage.OutputTokens +
		msg.Usage.CacheReadInputTokens +
		msg.Usage.CacheCreationInputTokens

	modelStat := ensureCostModelStat(agg, msg.Model)
	modelStat.RequestCount++
	modelStat.InputTokens += msg.Usage.InputTokens
	modelStat.OutputTokens += msg.Usage.OutputTokens
	modelStat.CacheReadTokens += msg.Usage.CacheReadInputTokens
	modelStat.CacheCreationTokens += msg.Usage.CacheCreationInputTokens
	modelStat.ServerToolUseRequests += serverToolUseRequests
	modelStat.TotalTokens += totalTokens

	projectStat := ensureCostProjectStat(agg, projectName)
	projectStat.RequestCount++
	projectStat.InputTokens += msg.Usage.InputTokens
	projectStat.OutputTokens += msg.Usage.OutputTokens
	projectStat.TotalTokens += totalTokens

	if record.SessionID != "" {
		sessionStat := ensureCostSessionStat(agg, record.SessionID, projectName)
		sessionStat.RequestCount++
		sessionStat.InputTokens += msg.Usage.InputTokens
		sessionStat.OutputTokens += msg.Usage.OutputTokens
		sessionStat.TotalTokens += totalTokens
		if sessionStat.Model == "" {
			sessionStat.Model = msg.Model
		} else if sessionStat.Model != msg.Model {
			sessionStat.Model = "mixed"
		}
	}

	agentStat := ensureCostAgentStat(agg, record.AgentID, record.IsSidechain)
	agentStat.RequestCount++
	agentStat.InputTokens += msg.Usage.InputTokens
	agentStat.OutputTokens += msg.Usage.OutputTokens
	agentStat.TotalTokens += totalTokens
}

func ensureCostModelStat(agg *ProjectAggregate, model string) *CostModelStat {
	if agg.CostModelStats == nil {
		agg.CostModelStats = make(map[string]*CostModelStat)
	}
	if model == "" {
		model = "unknown"
	}
	if agg.CostModelStats[model] == nil {
		agg.CostModelStats[model] = &CostModelStat{Model: model}
	}
	return agg.CostModelStats[model]
}

func ensureCostProjectStat(agg *ProjectAggregate, project string) *CostProjectStat {
	if agg.CostProjectStats == nil {
		agg.CostProjectStats = make(map[string]*CostProjectStat)
	}
	if project == "" {
		project = "Unknown"
	}
	if agg.CostProjectStats[project] == nil {
		agg.CostProjectStats[project] = &CostProjectStat{Project: project}
	}
	return agg.CostProjectStats[project]
}

func ensureCostSessionStat(agg *ProjectAggregate, sessionID, project string) *CostSessionStat {
	if agg.CostSessionStats == nil {
		agg.CostSessionStats = make(map[string]*CostSessionStat)
	}
	if sessionID == "" {
		sessionID = "unknown"
	}
	if agg.CostSessionStats[sessionID] == nil {
		agg.CostSessionStats[sessionID] = &CostSessionStat{SessionID: sessionID, Project: project}
	}
	if agg.CostSessionStats[sessionID].Project == "" {
		agg.CostSessionStats[sessionID].Project = project
	}
	return agg.CostSessionStats[sessionID]
}

func ensureCostAgentStat(agg *ProjectAggregate, agentID string, isSidechain bool) *CostAgentStat {
	if agg.CostAgentStats == nil {
		agg.CostAgentStats = make(map[string]*CostAgentStat)
	}
	if agentID == "" {
		if isSidechain {
			agentID = "sidechain:unknown"
		} else {
			agentID = "main"
		}
	}
	if agg.CostAgentStats[agentID] == nil {
		agg.CostAgentStats[agentID] = &CostAgentStat{AgentID: agentID, IsSidechain: isSidechain}
	}
	if isSidechain {
		agg.CostAgentStats[agentID].IsSidechain = true
	}
	return agg.CostAgentStats[agentID]
}

func mergeCostModelStat(dst *CostModelStat, src *CostModelStat) {
	dst.RequestCount += src.RequestCount
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CacheReadTokens += src.CacheReadTokens
	dst.CacheCreationTokens += src.CacheCreationTokens
	dst.ServerToolUseRequests += src.ServerToolUseRequests
	dst.TotalTokens += src.TotalTokens
}

func mergeCostProjectStat(dst *CostProjectStat, src *CostProjectStat) {
	dst.RequestCount += src.RequestCount
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.TotalTokens += src.TotalTokens
}

func mergeCostSessionStat(dst *CostSessionStat, src *CostSessionStat) {
	dst.RequestCount += src.RequestCount
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.TotalTokens += src.TotalTokens
}

func mergeCostAgentStat(dst *CostAgentStat, src *CostAgentStat) {
	dst.RequestCount += src.RequestCount
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.TotalTokens += src.TotalTokens
}

func (agg *ProjectAggregate) finalizeCostAnalysis() {
	analysis := &CostAnalysisData{
		ByModel:        make([]CostModelStat, 0, len(agg.CostModelStats)),
		ByProject:      make([]CostProjectStat, 0, len(agg.CostProjectStats)),
		BySession:      make([]CostSessionStat, 0, len(agg.CostSessionStats)),
		ByAgent:        make([]CostAgentStat, 0, len(agg.CostAgentStats)),
		BudgetTimeline: append([]BudgetTimelineItem(nil), agg.BudgetTimeline...),
	}

	for _, stat := range agg.CostModelStats {
		statCopy := *stat
		if statCopy.RequestCount > 0 {
			statCopy.AvgOutputTokens = float64(statCopy.OutputTokens) / float64(statCopy.RequestCount)
		}
		inputTotal := statCopy.InputTokens + statCopy.CacheReadTokens + statCopy.CacheCreationTokens
		if inputTotal > 0 {
			statCopy.CacheReadRatio = float64(statCopy.CacheReadTokens) / float64(inputTotal) * 100
		}
		analysis.Totals.InputTokens += statCopy.InputTokens
		analysis.Totals.OutputTokens += statCopy.OutputTokens
		analysis.Totals.CacheReadInputTokens += statCopy.CacheReadTokens
		analysis.Totals.CacheCreationInputTokens += statCopy.CacheCreationTokens
		analysis.Totals.ServerToolUseRequests += statCopy.ServerToolUseRequests
		analysis.Totals.TotalTokens += statCopy.TotalTokens
		analysis.Totals.RequestCount += statCopy.RequestCount
		analysis.ByModel = append(analysis.ByModel, statCopy)
	}
	analysis.Totals.BillableInputTokens = analysis.Totals.InputTokens + analysis.Totals.CacheCreationInputTokens

	sort.Slice(analysis.ByModel, func(i, j int) bool {
		if analysis.ByModel[i].TotalTokens == analysis.ByModel[j].TotalTokens {
			return analysis.ByModel[i].Model < analysis.ByModel[j].Model
		}
		return analysis.ByModel[i].TotalTokens > analysis.ByModel[j].TotalTokens
	})

	for _, stat := range agg.CostProjectStats {
		analysis.ByProject = append(analysis.ByProject, *stat)
	}
	sort.Slice(analysis.ByProject, func(i, j int) bool {
		if analysis.ByProject[i].TotalTokens == analysis.ByProject[j].TotalTokens {
			return analysis.ByProject[i].Project < analysis.ByProject[j].Project
		}
		return analysis.ByProject[i].TotalTokens > analysis.ByProject[j].TotalTokens
	})

	for _, stat := range agg.CostSessionStats {
		analysis.BySession = append(analysis.BySession, *stat)
	}
	sort.Slice(analysis.BySession, func(i, j int) bool {
		if analysis.BySession[i].TotalTokens == analysis.BySession[j].TotalTokens {
			return analysis.BySession[i].SessionID < analysis.BySession[j].SessionID
		}
		return analysis.BySession[i].TotalTokens > analysis.BySession[j].TotalTokens
	})

	for _, stat := range agg.CostAgentStats {
		analysis.ByAgent = append(analysis.ByAgent, *stat)
	}
	sort.Slice(analysis.ByAgent, func(i, j int) bool {
		if analysis.ByAgent[i].TotalTokens == analysis.ByAgent[j].TotalTokens {
			return analysis.ByAgent[i].AgentID < analysis.ByAgent[j].AgentID
		}
		return analysis.ByAgent[i].TotalTokens > analysis.ByAgent[j].TotalTokens
	})

	sort.Slice(analysis.BudgetTimeline, func(i, j int) bool {
		return analysis.BudgetTimeline[i].Timestamp < analysis.BudgetTimeline[j].Timestamp
	})
	if len(analysis.ByProject) > 50 {
		analysis.ByProject = analysis.ByProject[:50]
	}
	if len(analysis.BySession) > 50 {
		analysis.BySession = analysis.BySession[:50]
	}
	if len(analysis.ByAgent) > 50 {
		analysis.ByAgent = analysis.ByAgent[:50]
	}
	if len(analysis.BudgetTimeline) > 200 {
		analysis.BudgetTimeline = analysis.BudgetTimeline[:200]
	}

	agg.CostAnalysis = analysis
}
