package main

func limitFailureReasons(items []FailureReasonStat, limit int) []FailureReasonStat {
	if len(items) <= limit {
		return append([]FailureReasonStat(nil), items...)
	}
	return append([]FailureReasonStat(nil), items[:limit]...)
}

func limitToolReasons(items []FailureToolReasonStat, limit int) []FailureToolReasonStat {
	if len(items) <= limit {
		return append([]FailureToolReasonStat(nil), items...)
	}
	return append([]FailureToolReasonStat(nil), items[:limit]...)
}

func limitToolModelStats(items []ToolModelStatItem, limit int) []ToolModelStatItem {
	if len(items) <= limit {
		return append([]ToolModelStatItem(nil), items...)
	}
	return append([]ToolModelStatItem(nil), items[:limit]...)
}

func limitCostModels(items []CostModelStat, limit int) []CostModelStat {
	if len(items) <= limit {
		return append([]CostModelStat(nil), items...)
	}
	return append([]CostModelStat(nil), items[:limit]...)
}

func limitCostProjects(items []CostProjectStat, limit int) []CostProjectStat {
	if len(items) <= limit {
		return append([]CostProjectStat(nil), items...)
	}
	return append([]CostProjectStat(nil), items[:limit]...)
}

func limitCostSessions(items []CostSessionStat, limit int) []CostSessionStat {
	if len(items) <= limit {
		return append([]CostSessionStat(nil), items...)
	}
	return append([]CostSessionStat(nil), items[:limit]...)
}
