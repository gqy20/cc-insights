package main

import (
	"fmt"
	"sort"
	"strings"
)

func buildCLIRecommendationReport(data *DashboardData, limit int) cliRecommendationReport {
	report := cliRecommendationReport{TimeRange: data.TimeRange}
	findings := make([]diagnosticFinding, 0, 12)
	findings = append(findings, buildCommandDiagnostics(data)...)
	findings = append(findings, buildFailureDiagnostics(data)...)
	findings = append(findings, buildPerformanceDiagnostics(data)...)
	findings = append(findings, buildContextDiagnostics(data)...)
	findings = append(findings, buildWorkflowDiagnostics(data)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if severityRank(findings[i].Severity) != severityRank(findings[j].Severity) {
			return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
		}
		if findings[i].Category != findings[j].Category {
			return findings[i].Category < findings[j].Category
		}
		return findings[i].ID < findings[j].ID
	})
	if len(findings) > limit {
		findings = findings[:limit]
	}

	report.Recommendations = findings
	report.TotalFindings = len(findings)
	report.ByCategory = countFindingsByCategory(findings)
	report.Insights = buildRecommendationInsights(report)
	return report
}

func buildCommandDiagnostics(data *DashboardData) []diagnosticFinding {
	if data.CommandAnalysis == nil {
		return nil
	}
	var findings []diagnosticFinding
	totalCalls := 0
	familyFailureFindings := make([]diagnosticFinding, 0)
	for _, family := range data.CommandAnalysis.BashFamilies {
		totalCalls += family.CallCount
		if family.Family != "other" && family.CallCount >= 30 && family.FailureRate >= 30 {
			familyFailureFindings = append(familyFailureFindings, diagnosticFinding{
				ID:       "command.family.high_failure." + sanitizeID(family.Family),
				Category: "command",
				Severity: severityFromRate(family.FailureRate, 45, 30),
				Title:    fmt.Sprintf("%s 命令族失败率偏高", family.Family),
				Summary:  fmt.Sprintf("%s 命令族共 %s 次调用，失败率 %s%%。", family.Family, formatInt(family.CallCount), formatFailureRate(family.FailureRate)),
				Evidence: []diagnosticEvidence{
					{Label: "命令族", Value: family.Family},
					{Label: "调用次数", Value: formatInt(family.CallCount)},
					{Label: "失败率", Value: formatFailureRate(family.FailureRate) + "%"},
					{Label: "Top 命令", Value: family.TopCommand},
				},
				Interpretation: "这通常说明该类命令的入口、环境前置条件或失败处理方式不够稳定，AI 容易重复尝试高成本动作。",
				NextSteps: []string{
					"检查 CLAUDE.md 是否写清该命令族的推荐入口和前置条件。",
					"用 `cc-insights why` 下钻失败样例，确认是否集中在缺依赖、路径、权限、服务未启动或超时。",
					"考虑把高频长命令封装为更短、更确定的脚本或任务命令。",
				},
				Confidence: "high",
			})
		}
	}
	sort.SliceStable(familyFailureFindings, func(i, j int) bool {
		if severityRank(familyFailureFindings[i].Severity) != severityRank(familyFailureFindings[j].Severity) {
			return severityRank(familyFailureFindings[i].Severity) > severityRank(familyFailureFindings[j].Severity)
		}
		return familyFailureFindings[i].ID < familyFailureFindings[j].ID
	})
	if len(familyFailureFindings) > 4 {
		familyFailureFindings = familyFailureFindings[:4]
	}
	findings = append(findings, familyFailureFindings...)
	if totalCalls > 0 {
		for _, family := range data.CommandAnalysis.BashFamilies {
			if family.Family != "other" {
				continue
			}
			ratio := float64(family.CallCount) / float64(totalCalls) * 100
			if family.CallCount >= 20 && ratio >= 10 {
				findings = append(findings, diagnosticFinding{
					ID:       "command.other.high_ratio",
					Category: "command",
					Severity: "medium",
					Title:    "Bash other 分类占比较高",
					Summary:  fmt.Sprintf("other 命令族占 Bash 调用 %s%%，说明仍有较多命令没有被解释清楚。", formatFailureRate(ratio)),
					Evidence: []diagnosticEvidence{
						{Label: "other 调用", Value: formatInt(family.CallCount)},
						{Label: "占比", Value: formatFailureRate(ratio) + "%"},
						{Label: "Top 命令", Value: family.TopCommand},
					},
					Interpretation: "命令分类不清会降低后续诊断质量，尤其会影响对测试、构建、依赖、网络和文件操作问题的归因。",
					NextSteps: []string{
						"查看 `cc-insights cmd -j` 中 other 的 Top 命令。",
						"补充 `~/.cc-insights/bash.yml` 或内置规则，把稳定模式归入明确命令族。",
					},
					Confidence: "medium",
				})
			}
			break
		}
	}
	if len(data.CommandAnalysis.RiskyCommands) > 0 {
		top := data.CommandAnalysis.RiskyCommands[0]
		findings = append(findings, diagnosticFinding{
			ID:       "command.risky.present",
			Category: "command",
			Severity: "medium",
			Title:    "存在高风险 Bash 命令",
			Summary:  fmt.Sprintf("检测到风险命令 %s，风险等级 %s。", top.CommandName, top.RiskLevel),
			Evidence: []diagnosticEvidence{
				{Label: "命令", Value: top.CommandName},
				{Label: "风险等级", Value: top.RiskLevel},
				{Label: "原因", Value: top.RiskReason},
				{Label: "调用次数", Value: formatInt(top.CallCount)},
			},
			Interpretation: "高风险命令通常需要更明确的确认边界，否则 AI 可能在不完整上下文中执行破坏性操作。",
			NextSteps: []string{
				"确认 CLAUDE.md 是否要求删除、sudo、批量修改前说明影响范围。",
				"检查是否需要 hook 对 rm/sudo/chmod 等命令做提示或拦截。",
			},
			Confidence: "high",
		})
	}
	return findings
}

func buildFailureDiagnostics(data *DashboardData) []diagnosticFinding {
	if data.FailureAnalysis == nil {
		return nil
	}
	var findings []diagnosticFinding
	if len(data.FailureAnalysis.ByReason) > 0 && data.FailureAnalysis.TotalFailures > 0 {
		top := data.FailureAnalysis.ByReason[0]
		ratio := float64(top.Count) / float64(data.FailureAnalysis.TotalFailures) * 100
		if top.Count >= 10 && ratio >= 25 {
			findings = append(findings, diagnosticFinding{
				ID:       "failure.reason.concentrated." + sanitizeID(top.Category+"."+top.Reason),
				Category: "failure",
				Severity: severityFromRate(ratio, 50, 25),
				Title:    "失败原因集中",
				Summary:  fmt.Sprintf("%s/%s 占全部失败 %s%%。", top.Category, top.Reason, formatFailureRate(ratio)),
				Evidence: []diagnosticEvidence{
					{Label: "原因", Value: top.Category + "/" + top.Reason},
					{Label: "次数", Value: formatInt(top.Count)},
					{Label: "占比", Value: formatFailureRate(ratio) + "%"},
				},
				Interpretation: "失败原因集中通常意味着可以通过一条明确的项目约定、环境检查或工具封装来降低反复失败。",
				NextSteps: []string{
					"运行 `cc-insights why -j --reason " + top.Reason + "` 查看样例。",
					"判断失败是否来自环境缺失、路径错误、权限问题、超时或工具参数错误。",
				},
				Confidence: "high",
			})
		}
	}
	if len(data.FailureAnalysis.ByToolReason) > 0 {
		top := data.FailureAnalysis.ByToolReason[0]
		if top.Count >= 10 {
			findings = append(findings, diagnosticFinding{
				ID:       "failure.tool_reason.top." + sanitizeID(top.Tool+"."+top.Reason),
				Category: "failure",
				Severity: "medium",
				Title:    "失败集中在特定工具",
				Summary:  fmt.Sprintf("%s 的 %s/%s 失败出现 %s 次。", top.Tool, top.Category, top.Reason, formatInt(top.Count)),
				Evidence: []diagnosticEvidence{
					{Label: "工具", Value: top.Tool},
					{Label: "原因", Value: top.Category + "/" + top.Reason},
					{Label: "次数", Value: formatInt(top.Count)},
				},
				Interpretation: "特定工具失败集中时，优先优化该工具的调用前置条件、参数约定或失败摘要，比泛泛调整提示词更有效。",
				NextSteps: []string{
					"按工具和原因下钻失败样例。",
					"检查是否需要在 CLAUDE.md 写明该工具的正确使用边界。",
				},
				Confidence: "medium",
			})
		}
	}
	return findings
}

func buildPerformanceDiagnostics(data *DashboardData) []diagnosticFinding {
	if data.ToolPerformance == nil {
		return nil
	}
	var findings []diagnosticFinding
	if len(data.ToolPerformance.ByCategory) > 0 {
		top := data.ToolPerformance.ByCategory[0]
		if top.TotalDurationMs >= 60_000 || top.AvgDurationMs >= 5_000 {
			findings = append(findings, diagnosticFinding{
				ID:       "performance.tool.slow." + sanitizeID(top.Category),
				Category: "performance",
				Severity: severityFromDuration(top.AvgDurationMs),
				Title:    "工具耗时集中",
				Summary:  fmt.Sprintf("%s 累计耗时 %s，平均耗时 %s。", top.Category, formatDurationMs(top.TotalDurationMs), formatDurationMs(int64(top.AvgDurationMs))),
				Evidence: []diagnosticEvidence{
					{Label: "类别", Value: top.Category},
					{Label: "调用次数", Value: formatInt(top.CallCount)},
					{Label: "平均耗时", Value: formatDurationMs(int64(top.AvgDurationMs))},
					{Label: "最大耗时", Value: formatDurationMs(top.MaxDurationMs)},
					{Label: "错误率", Value: formatFailureRate(top.ErrorRate) + "%"},
				},
				Interpretation: "耗时集中会放大 AI 的等待成本和上下文消耗。如果同时有错误率，说明它可能在反复执行高成本失败路径。",
				NextSteps: []string{
					"查看 slowest_calls 样例，确认慢调用集中在哪些项目、session 和模型。",
					"区分 quick path 与 full path，把高成本命令写成明确的短命令或脚本。",
				},
				Confidence: "high",
			})
		}
	}
	if len(data.ToolPerformance.SlowestCalls) > 0 {
		top := data.ToolPerformance.SlowestCalls[0]
		if top.DurationMs >= 10_000 {
			findings = append(findings, diagnosticFinding{
				ID:       "performance.slowest_call",
				Category: "performance",
				Severity: severityFromDuration(float64(top.DurationMs)),
				Title:    "存在明显慢调用样例",
				Summary:  fmt.Sprintf("%s 单次调用耗时 %s。", top.Category, formatDurationMs(top.DurationMs)),
				Evidence: []diagnosticEvidence{
					{Label: "工具", Value: top.Tool},
					{Label: "类别", Value: top.Category},
					{Label: "耗时", Value: formatDurationMs(top.DurationMs)},
					{Label: "项目", Value: nonEmpty(top.Project, "unknown")},
					{Label: "Session", Value: nonEmpty(top.SessionID, "unknown")},
				},
				Interpretation: "慢调用样例可以作为优化入口，尤其适合判断是否需要明确服务启动、测试范围或命令超时策略。",
				NextSteps: []string{
					"定位该 session 的上下文，确认慢调用前 AI 是否缺少前置判断。",
					"如果慢调用是测试/构建，补充 quick test 与 full test 的选择规则。",
				},
				Confidence: "high",
			})
		}
	}
	if globalCache != nil && globalCache.BuildStats != nil {
		stats := globalCache.BuildStats
		if stats.BuildDurationMs >= 10_000 || stats.ParsedFiles >= 500 {
			findings = append(findings, diagnosticFinding{
				ID:       "performance.cache.build_cost",
				Category: "performance",
				Severity: "medium",
				Title:    "缓存构建成本较高",
				Summary:  fmt.Sprintf("最近一次缓存构建耗时 %s，解析文件 %s 个。", formatDurationMs(stats.BuildDurationMs), formatInt(stats.ParsedFiles)),
				Evidence: []diagnosticEvidence{
					{Label: "构建耗时", Value: formatDurationMs(stats.BuildDurationMs)},
					{Label: "解析文件", Value: formatInt(stats.ParsedFiles)},
					{Label: "复用文件", Value: formatInt(stats.ReusedFiles)},
					{Label: "总文件", Value: formatInt(stats.TotalFiles)},
				},
				Interpretation: "缓存构建成本高会影响 CLI 和 Web 首次响应，规则或版本频繁变更时更明显。",
				NextSteps: []string{
					"避免频繁触发全量重建，优先利用文件级缓存复用。",
					"如果默认数据量很大，CLI 侧优先使用较短时间范围做诊断。",
				},
				Confidence: "medium",
			})
		}
	}
	return findings
}

func buildContextDiagnostics(data *DashboardData) []diagnosticFinding {
	if data.CostAnalysis == nil || data.CostAnalysis.Totals.TotalTokens == 0 {
		return nil
	}
	var findings []diagnosticFinding
	if len(data.CostAnalysis.ByProject) > 0 {
		top := data.CostAnalysis.ByProject[0]
		ratio := float64(top.TotalTokens) / float64(data.CostAnalysis.Totals.TotalTokens) * 100
		if top.TotalTokens >= 100_000 && ratio >= 40 {
			findings = append(findings, diagnosticFinding{
				ID:       "context.project.token_concentration",
				Category: "context",
				Severity: "medium",
				Title:    "Token 消耗集中在少数项目",
				Summary:  fmt.Sprintf("%s 消耗 %s token，占总量 %s%%。", top.Project, formatCompactInt(top.TotalTokens), formatFailureRate(ratio)),
				Evidence: []diagnosticEvidence{
					{Label: "项目", Value: top.Project},
					{Label: "Token", Value: formatCompactInt(top.TotalTokens)},
					{Label: "占比", Value: formatFailureRate(ratio) + "%"},
				},
				Interpretation: "Token 集中通常说明该项目上下文读取、长会话或重复探索成本较高，适合补充项目结构和常用入口说明。",
				NextSteps: []string{
					"检查该项目是否缺少 CLAUDE.md 或项目结构说明。",
					"查看高 token session，确认是否反复读取大文件或重复搜索。",
				},
				Confidence: "medium",
			})
		}
	}
	if len(data.CostAnalysis.BySession) > 0 {
		top := data.CostAnalysis.BySession[0]
		if top.TotalTokens >= 200_000 {
			findings = append(findings, diagnosticFinding{
				ID:       "context.session.large_token",
				Category: "context",
				Severity: "medium",
				Title:    "存在高 Token 会话",
				Summary:  fmt.Sprintf("Session %s 消耗 %s token。", top.SessionID, formatCompactInt(top.TotalTokens)),
				Evidence: []diagnosticEvidence{
					{Label: "Session", Value: top.SessionID},
					{Label: "项目", Value: top.Project},
					{Label: "Token", Value: formatCompactInt(top.TotalTokens)},
				},
				Interpretation: "单会话 token 过高通常来自任务范围过大、上下文反复读取或缺少清晰分阶段策略。",
				NextSteps: []string{
					"结合 session 生命周期查看该会话是否长时间运行或多次 plan/task 切换。",
					"考虑在项目说明中补充任务拆分和文件读取边界。",
				},
				Confidence: "medium",
			})
		}
	}
	return findings
}

func buildWorkflowDiagnostics(data *DashboardData) []diagnosticFinding {
	var findings []diagnosticFinding
	if data.SessionAnalysis != nil && len(data.SessionAnalysis.LongRunning) > 0 {
		top := data.SessionAnalysis.LongRunning[0]
		if top.DurationMs >= 30*60*1000 || top.ToolFailureCount >= 10 {
			findings = append(findings, diagnosticFinding{
				ID:       "workflow.session.long_running",
				Category: "workflow",
				Severity: "medium",
				Title:    "存在长会话或高失败会话",
				Summary:  fmt.Sprintf("Session %s 耗时 %s，工具失败 %s 次。", top.SessionID, formatDurationMs(top.DurationMs), formatInt(top.ToolFailureCount)),
				Evidence: []diagnosticEvidence{
					{Label: "Session", Value: top.SessionID},
					{Label: "项目", Value: top.Project},
					{Label: "耗时", Value: formatDurationMs(top.DurationMs)},
					{Label: "工具失败", Value: formatInt(top.ToolFailureCount)},
				},
				Interpretation: "长会话或高失败会话通常说明任务边界、计划拆分或失败恢复策略不够清晰。",
				NextSteps: []string{
					"查看该 session 的标题、plan/task 事件和失败样例。",
					"检查是否需要把复杂任务拆成更明确的阶段和验收点。",
				},
				Confidence: "medium",
			})
		}
	}
	if data.TaskPlanAnalysis != nil {
		plan := data.TaskPlanAnalysis.PlanLifecycle
		if plan.EntryCount >= 10 && plan.ExitCount == 0 {
			findings = append(findings, diagnosticFinding{
				ID:       "workflow.plan.no_exit",
				Category: "workflow",
				Severity: "low",
				Title:    "Plan 模式退出信号不足",
				Summary:  fmt.Sprintf("Plan 进入 %s 次，但退出记录为 0。", formatInt(plan.EntryCount)),
				Evidence: []diagnosticEvidence{
					{Label: "Plan 进入", Value: formatInt(plan.EntryCount)},
					{Label: "Plan 退出", Value: formatInt(plan.ExitCount)},
					{Label: "涉及 Session", Value: formatInt(plan.SessionsWithPlan)},
				},
				Interpretation: "Plan 生命周期不完整会让后续分析难判断任务是否真正从规划进入执行。",
				NextSteps: []string{
					"确认 Claude Code 事件里是否稳定记录 plan_mode_exit。",
					"如果缺失是数据格式问题，优先补解析；如果是行为问题，再考虑改工作流提示。",
				},
				Confidence: "low",
			})
		}
	}
	return findings
}

func countFindingsByCategory(findings []diagnosticFinding) []nameCount {
	counts := make(map[string]int)
	for _, finding := range findings {
		counts[finding.Category]++
	}
	items := make([]nameCount, 0, len(counts))
	for name, count := range counts {
		items = append(items, nameCount{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	return items
}

func buildRecommendationInsights(report cliRecommendationReport) []string {
	if report.TotalFindings == 0 {
		return []string{"当前时间范围内没有触发诊断规则。"}
	}
	insights := []string{fmt.Sprintf("触发 %s 条诊断，优先查看 high/medium 严重度项。", formatInt(report.TotalFindings))}
	if len(report.Recommendations) > 0 {
		top := report.Recommendations[0]
		insights = append(insights, fmt.Sprintf("最高优先级诊断是 %s（%s/%s）。", top.Title, top.Category, top.Severity))
	}
	return insights
}

func severityRank(value string) int {
	switch strings.ToLower(value) {
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

func severityFromRate(value, high, medium float64) string {
	switch {
	case value >= high:
		return "high"
	case value >= medium:
		return "medium"
	default:
		return "low"
	}
}

func severityFromDuration(avgMs float64) string {
	switch {
	case avgMs >= 30_000:
		return "high"
	case avgMs >= 5_000:
		return "medium"
	default:
		return "low"
	}
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_", ".", "_", "-", "_")
	return replacer.Replace(value)
}

func formatDurationMs(value int64) string {
	if value < 1000 {
		return fmt.Sprintf("%dms", value)
	}
	seconds := float64(value) / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	minutes := seconds / 60
	return fmt.Sprintf("%.1fmin", minutes)
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
