package main

import (
	"fmt"
)

func buildCLICommandReport(data *DashboardData, limit int) cliCommandReport {
	report := cliCommandReport{TimeRange: data.TimeRange}
	if data.CommandAnalysis == nil {
		return report
	}

	report.ByFamily = limitBashFamilies(data.CommandAnalysis.BashFamilies, limit)
	report.ByCommand = limitBashCommands(data.CommandAnalysis.BashCommands, limit)
	report.RiskyCommands = limitBashCommands(data.CommandAnalysis.RiskyCommands, limit)
	report.TotalCommands = len(data.CommandAnalysis.BashCommands)
	for _, cmd := range data.CommandAnalysis.BashCommands {
		report.TotalCalls += cmd.CallCount
	}
	report.Insights = buildCommandInsights(report)
	return report
}

func buildCommandInsights(report cliCommandReport) []string {
	if report.TotalCalls == 0 {
		return []string{"当前时间范围内没有 Bash 命令统计。"}
	}
	insights := []string{
		fmt.Sprintf("共统计到 %s 次 Bash 调用，覆盖 %s 个具体命令。", formatInt(report.TotalCalls), formatInt(report.TotalCommands)),
	}
	if len(report.ByFamily) > 0 {
		top := report.ByFamily[0]
		insights = append(insights, fmt.Sprintf("最活跃的命令族是 %s，共 %s 次，失败率 %.1f%%。", top.Family, formatInt(top.CallCount), top.FailureRate))
	}
	if len(report.RiskyCommands) > 0 {
		top := report.RiskyCommands[0]
		insights = append(insights, fmt.Sprintf("最高风险命令是 %s，风险等级 %s。", top.CommandName, top.RiskLevel))
	}
	return insights
}

func limitBashFamilies(items []BashCommandFamilyStat, limit int) []BashCommandFamilyStat {
	if len(items) <= limit {
		return append([]BashCommandFamilyStat(nil), items...)
	}
	return append([]BashCommandFamilyStat(nil), items[:limit]...)
}

func limitBashCommands(items []BashCommandStat, limit int) []BashCommandStat {
	if len(items) <= limit {
		return append([]BashCommandStat(nil), items...)
	}
	return append([]BashCommandStat(nil), items[:limit]...)
}
