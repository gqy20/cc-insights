package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func buildRecommendationDashboardData(tf TimeFilter, preset string) (*DashboardData, string, error) {
	if globalCache == nil {
		data, source, err := buildDashboardData(tf, preset)
		return data, source, err
	}
	if err := refreshGlobalCacheIfRulesChanged(); err != nil {
		Warn("Bash 规则刷新失败，继续尝试现有缓存", "error", err.Error())
	}

	var start, end time.Time
	if tf.Start != nil {
		start = *tf.Start
	}
	if tf.End != nil {
		end = *tf.End
	}
	queryStartedAt := time.Now()
	cached := globalCache.QueryByTimeRange(start, end)
	Debug("诊断缓存查询完成",
		"preset", preset,
		"query_duration", time.Since(queryStartedAt).Round(time.Millisecond),
		"messages", cached.TotalMessages,
	)

	rangeInfo := TimeRangeInfo{Preset: preset}
	if tf.Start != nil {
		rangeInfo.Start = tf.Start.Format("2006-01-02")
	}
	if tf.End != nil {
		rangeInfo.End = tf.End.Format("2006-01-02")
	}

	return &DashboardData{
		TimeRange:        rangeInfo,
		CommandAnalysis:  cloneCommandAnalysis(cached.CommandAnalysis),
		FailureAnalysis:  cloneFailureAnalysis(cached.FailureAnalysis),
		ToolPerformance:  cloneToolPerformance(cached.ToolPerformance),
		CostAnalysis:     cloneCostAnalysis(cached.CostAnalysis),
		SessionAnalysis:  cloneSessionAnalysis(cached.SessionAnalysis),
		TaskPlanAnalysis: cloneTaskPlanAnalysis(cached.TaskPlanAnalysis),
	}, "cache", nil
}

func buildCLIRecommendationReport(data *DashboardData, limit int) cliRecommendationReport {
	report := cliRecommendationReport{TimeRange: data.TimeRange}
	findings := make([]diagnosticFinding, 0, 12)
	findings = append(findings, buildCommandDiagnostics(data)...)
	findings = append(findings, buildFailureDiagnostics(data)...)
	findings = append(findings, buildPerformanceDiagnostics(data)...)
	findings = append(findings, buildContextDiagnostics(data)...)
	findings = append(findings, buildWorkflowDiagnostics(data)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if diagnosticRank(findings[i]) != diagnosticRank(findings[j]) {
			return diagnosticRank(findings[i]) > diagnosticRank(findings[j])
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
			interpretation, nextSteps := commandFamilyGuidance(family.Family)
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
				Interpretation: interpretation,
				NextSteps:      nextSteps,
				Confidence:     "high",
			})
		}
	}
	sort.SliceStable(familyFailureFindings, func(i, j int) bool {
		scoreI := commandFailureScore(familyFailureFindings[i])
		scoreJ := commandFailureScore(familyFailureFindings[j])
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return familyFailureFindings[i].ID < familyFailureFindings[j].ID
	})
	if len(familyFailureFindings) > 3 {
		familyFailureFindings = familyFailureFindings[:3]
	}
	findings = append(findings, familyFailureFindings...)
	findings = append(findings, buildCommandClassificationDiagnostics(data)...)
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
		top, ok := topNonInteractivePerfCategory(data.ToolPerformance.ByCategory)
		if ok && (top.TotalDurationMs >= 60_000 || top.AvgDurationMs >= 5_000) {
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
		top, ok := topNonInteractiveSlowCall(data.ToolPerformance.SlowestCalls)
		if ok && top.DurationMs >= 10_000 {
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
				Title:    "缓存刷新成本较高",
				Summary:  fmt.Sprintf("最近一次缓存刷新耗时 %s，解析文件 %s 个，复用文件 %s 个。", formatDurationMs(stats.BuildDurationMs), formatInt(stats.ParsedFiles), formatInt(stats.ReusedFiles)),
				Evidence: []diagnosticEvidence{
					{Label: "构建耗时", Value: formatDurationMs(stats.BuildDurationMs)},
					{Label: "解析文件", Value: formatInt(stats.ParsedFiles)},
					{Label: "复用文件", Value: formatInt(stats.ReusedFiles)},
					{Label: "总文件", Value: formatInt(stats.TotalFiles)},
				},
				Interpretation: "缓存刷新成本高会影响 CLI 和 Web 首次响应。即使文件级缓存大量复用，合并聚合和任务扫描仍可能产生可见耗时。",
				NextSteps: []string{
					"观察 parsed/reused 比例：parsed 高说明重建多，reused 高但仍慢说明聚合或附加扫描成本较高。",
					"如果默认数据量很大，CLI 侧优先使用较短时间范围做诊断，并考虑后续优化查询聚合路径。",
				},
				Confidence: "medium",
			})
		}
	}
	if top, ok := topInteractiveSlowCall(data.ToolPerformance.SlowestCalls); ok && top.DurationMs >= 10*60*1000 {
		findings = append(findings, diagnosticFinding{
			ID:       "workflow.interactive_tool.long_wait",
			Category: "workflow",
			Severity: severityFromDuration(float64(top.DurationMs)),
			Title:    "交互型工具等待时间过长",
			Summary:  fmt.Sprintf("%s 记录到 %s 的等待时间。", top.Tool, formatDurationMs(top.DurationMs)),
			Evidence: []diagnosticEvidence{
				{Label: "工具", Value: top.Tool},
				{Label: "耗时", Value: formatDurationMs(top.DurationMs)},
				{Label: "项目", Value: nonEmpty(top.Project, "unknown")},
				{Label: "Session", Value: nonEmpty(top.SessionID, "unknown")},
			},
			Interpretation: "ExitPlanMode、AskUserQuestion 等交互型工具的长耗时通常不代表工具执行慢，而是会话在等待确认或跨阶段停顿。",
			NextSteps: []string{
				"结合该 session 检查是否存在长时间人工等待、计划确认或任务暂停。",
				"不要把这类耗时直接归因到测试/构建性能问题；更适合作为 workflow latency 信号处理。",
			},
			Confidence: "medium",
		})
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

func diagnosticRank(item diagnosticFinding) int {
	rank := severityRank(item.Severity) * 100
	switch item.Category {
	case "performance":
		rank += 35
	case "failure":
		rank += 30
	case "workflow":
		rank += 25
	case "context":
		rank += 20
	case "command":
		rank += 10
	}
	if strings.Contains(item.ID, "classification") {
		rank += 15
	}
	return rank
}

func commandFailureScore(item diagnosticFinding) int {
	score := severityRank(item.Severity) * 1000
	for _, ev := range item.Evidence {
		switch ev.Label {
		case "调用次数":
			score += parseFormattedInt(ev.Value) / 10
		case "失败率":
			score += int(parsePercent(ev.Value) * 10)
		}
	}
	return score
}

func commandFamilyGuidance(family string) (string, []string) {
	switch family {
	case "test":
		return "测试失败率高通常说明测试入口、依赖服务、浏览器环境或测试范围不明确，AI 可能在没有前置检查时反复执行完整测试。",
			[]string{
				"在 CLAUDE.md 区分 quick test、targeted test、full test 的使用条件。",
				"下钻失败样例，确认是否集中在服务未启动、浏览器依赖、端口冲突或 fixture 数据缺失。",
				"把高频测试命令封装为稳定脚本，减少 AI 自行拼命令。",
			}
	case "build":
		return "构建失败率高通常来自环境版本、生成产物、依赖安装或构建入口不统一，容易导致 AI 在错误目录或错误包管理器下重试。",
			[]string{
				"明确项目标准构建命令和工作目录。",
				"补充构建前置条件，例如依赖安装、环境变量、代码生成步骤。",
				"区分类型检查、局部构建和完整构建，避免每次都跑最高成本路径。",
			}
	case "dependency":
		return "依赖命令失败率高通常意味着包管理器、锁文件、网络源或虚拟环境边界不清晰。",
			[]string{
				"写清项目使用 npm/pnpm/yarn/uv/pip/poetry 中的哪一个，以及禁止混用的规则。",
				"下钻失败样例，确认是否集中在网络、版本冲突、锁文件或权限问题。",
				"考虑增加环境检查命令，让 AI 先验证依赖状态再安装。",
			}
	case "network":
		return "网络命令失败率高通常不是简单命令问题，而是服务可达性、认证、代理、端口或外部站点稳定性问题。",
			[]string{
				"区分本地服务检查和外部网络访问，明确本地服务启动方式。",
				"下钻失败样例，确认是否集中在 4xx/5xx、连接拒绝、超时或认证失败。",
				"必要时给 curl/wget/playwright 访问增加短超时和失败摘要。",
			}
	case "javascript":
		return "JavaScript 命令失败率高通常说明包管理器、脚本名、工作目录或 Node 版本约定不清晰。",
			[]string{
				"在项目说明中写清推荐包管理器和常用 scripts。",
				"区分 lint、typecheck、test、build 的命令入口。",
				"下钻失败样例，确认是否是缺依赖、脚本不存在或 Node 版本不匹配。",
			}
	case "cleanup":
		return "清理命令失败率高通常需要先判断是否真的是清理动作；如果 Top 命令不是 rm 等清理命令，可能是分类规则误命中。",
			[]string{
				"检查 Top 命令是否符合 cleanup 语义。",
				"如果 Top 命令是 docker、git、find 等，应优先修正 Bash 规则。",
				"对 rm 等破坏性清理命令保留显式确认边界。",
			}
	default:
		return "这通常说明该类命令的入口、环境前置条件或失败处理方式不够稳定，AI 容易重复尝试高成本动作。",
			[]string{
				"检查 CLAUDE.md 是否写清该命令族的推荐入口和前置条件。",
				"用 `cc-insights why` 下钻失败样例，确认是否集中在缺依赖、路径、权限、服务未启动或超时。",
				"考虑把高频长命令封装为更短、更确定的脚本或任务命令。",
			}
	}
}

func buildCommandClassificationDiagnostics(data *DashboardData) []diagnosticFinding {
	var findings []diagnosticFinding
	for _, family := range data.CommandAnalysis.BashFamilies {
		if family.TopCommand == "" || family.CallCount < 20 {
			continue
		}
		if commandLooksLikeFamily(family.Family, family.TopCommand) {
			continue
		}
		findings = append(findings, diagnosticFinding{
			ID:       "command.classification.suspicious." + sanitizeID(family.Family),
			Category: "command",
			Severity: "medium",
			Title:    "疑似 Bash 分类异常",
			Summary:  fmt.Sprintf("%s 命令族的 Top 命令是 %s，语义可能不匹配。", family.Family, family.TopCommand),
			Evidence: []diagnosticEvidence{
				{Label: "命令族", Value: family.Family},
				{Label: "Top 命令", Value: family.TopCommand},
				{Label: "调用次数", Value: formatInt(family.CallCount)},
				{Label: "样例", Value: family.SampleCommand},
			},
			Interpretation: "分类异常会污染后续诊断，让失败率看起来像某类问题，实际可能只是规则误命中。",
			NextSteps: []string{
				"检查 `rules/bash.yml` 或 `~/.cc-insights/bash.yml` 中该命令族的 contains/priority/exclude 规则。",
				"优先给更明确的命令族提高 priority，或给误命中的 family 增加 exclude。",
			},
			Confidence: "medium",
		})
	}
	if len(findings) > 2 {
		return findings[:2]
	}
	return findings
}

func commandLooksLikeFamily(family, command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	switch family {
	case "cleanup":
		return command == "rm" || command == "rmdir" || command == "trash"
	case "test":
		return command == "go" || command == "npm" || command == "pnpm" || command == "bun" || command == "yarn" ||
			command == "npx" || command == "pytest" || command == "vitest" || command == "jest" || command == "cargo" ||
			command == "playwright" || command == "playwright-cli" || strings.Contains(command, "test")
	case "build":
		return command == "go" || command == "npm" || command == "pnpm" || command == "bun" || command == "yarn" ||
			command == "make" || command == "cargo" || strings.Contains(command, "build")
	case "dependency":
		return command == "npm" || command == "pnpm" || command == "yarn" || command == "bun" || command == "uv" ||
			command == "pip" || command == "pipx" || command == "poetry" || command == "go" || command == "cargo"
	case "network":
		return command == "curl" || command == "wget" || command == "playwright-cli"
	default:
		return true
	}
}

func topNonInteractivePerfCategory(items []ToolPerfCategoryItem) (ToolPerfCategoryItem, bool) {
	for _, item := range items {
		if !isInteractiveTool(item.BaseTool) && !isInteractiveTool(item.Category) {
			return item, true
		}
	}
	return ToolPerfCategoryItem{}, false
}

func topNonInteractiveSlowCall(items []ToolSlowCallItem) (ToolSlowCallItem, bool) {
	for _, item := range items {
		if !isInteractiveTool(item.Tool) && !isInteractiveTool(item.Category) {
			return item, true
		}
	}
	return ToolSlowCallItem{}, false
}

func topInteractiveSlowCall(items []ToolSlowCallItem) (ToolSlowCallItem, bool) {
	for _, item := range items {
		if isInteractiveTool(item.Tool) || isInteractiveTool(item.Category) {
			return item, true
		}
	}
	return ToolSlowCallItem{}, false
}

func isInteractiveTool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "exitplanmode", "askuserquestion", "todowrite":
		return true
	default:
		return false
	}
}

func parseFormattedInt(value string) int {
	value = strings.ReplaceAll(value, ",", "")
	value = strings.TrimSpace(value)
	total := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		total = total*10 + int(r-'0')
	}
	return total
}

func parsePercent(value string) float64 {
	value = strings.TrimSuffix(strings.TrimSpace(value), "%")
	var whole float64
	var fraction float64
	base := 10.0
	inFraction := false
	for _, r := range value {
		if r == '.' {
			inFraction = true
			continue
		}
		if r < '0' || r > '9' {
			break
		}
		if inFraction {
			fraction += float64(r-'0') / base
			base *= 10
			continue
		}
		whole = whole*10 + float64(r-'0')
	}
	return whole + fraction
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
