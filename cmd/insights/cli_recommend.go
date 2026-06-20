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
		ToolAnalysis:     cloneToolAnalysis(cached.ToolAnalysis),
		CommandAnalysis:  cloneCommandAnalysis(cached.CommandAnalysis),
		FailureAnalysis:  cloneFailureAnalysis(cached.FailureAnalysis),
		ToolPerformance:  cloneToolPerformance(cached.ToolPerformance),
		CostAnalysis:     cloneCostAnalysis(cached.CostAnalysis),
		SessionAnalysis:  cloneSessionAnalysis(cached.SessionAnalysis),
		EventAnalysis:    cloneEventAnalysis(cached.EventAnalysis),
		TaskPlanAnalysis: cloneTaskPlanAnalysis(cached.TaskPlanAnalysis),
	}, "cache", nil
}

func buildCLIRecommendationReport(data *DashboardData, opts cliOptions) cliRecommendationReport {
	report := cliRecommendationReport{TimeRange: data.TimeRange, Detail: opts.Detail, FilterID: strings.TrimSpace(opts.ID)}
	findings := make([]diagnosticFinding, 0, 12)
	findings = append(findings, buildCommandDiagnostics(data)...)
	findings = append(findings, buildFailureDiagnostics(data)...)
	findings = append(findings, buildPerformanceDiagnostics(data)...)
	findings = append(findings, buildContextDiagnostics(data)...)
	findings = append(findings, buildWorkflowDiagnostics(data)...)
	enrichDiagnosticFindings(data, findings)

	sort.SliceStable(findings, func(i, j int) bool {
		if diagnosticRank(findings[i]) != diagnosticRank(findings[j]) {
			return diagnosticRank(findings[i]) > diagnosticRank(findings[j])
		}
		if findings[i].Category != findings[j].Category {
			return findings[i].Category < findings[j].Category
		}
		return findings[i].ID < findings[j].ID
	})
	if report.FilterID != "" {
		findings = filterDiagnosticFindingsByID(findings, report.FilterID)
	} else if len(findings) > opts.Limit {
		findings = findings[:opts.Limit]
	}
	addDiagnosticDrilldowns(data.TimeRange, findings)

	report.Recommendations = findings
	report.TotalFindings = len(findings)
	report.ByCategory = countFindingsByCategory(findings)
	report.Insights = buildRecommendationInsights(report)
	return report
}

func filterDiagnosticFindingsByID(findings []diagnosticFinding, id string) []diagnosticFinding {
	filtered := make([]diagnosticFinding, 0, 1)
	for _, finding := range findings {
		if finding.ID == id {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func enrichDiagnosticFindings(data *DashboardData, findings []diagnosticFinding) {
	for i := range findings {
		findings[i] = enrichDiagnosticFinding(data, findings[i])
	}
}

func enrichDiagnosticFinding(data *DashboardData, finding diagnosticFinding) diagnosticFinding {
	finding.Trigger = buildDiagnosticTrigger(finding)
	finding.Examples = buildDiagnosticExamples(data, finding, 3)
	finding.RootCauses = buildDiagnosticRootCauses(finding, finding.Examples)
	finding.Targets = diagnosticTargets(finding)
	finding.Actions = diagnosticActions(finding)
	return finding
}

func buildDiagnosticTrigger(finding diagnosticFinding) *diagnosticTrigger {
	value := diagnosticTriggerValue(finding)
	if rule, ok := diagnosticRuleForID(finding.ID); ok {
		return &diagnosticTrigger{
			Metric:    rule.Metric,
			Value:     value,
			Threshold: rule.Threshold,
			Source:    rule.Source,
			Rationale: rule.Rationale,
		}
	}
	return &diagnosticTrigger{
		Metric:    "diagnostic_rule",
		Value:     nonEmpty(value, finding.Severity),
		Threshold: "内置诊断规则触发",
		Source:    "dashboard_data",
		Rationale: "该诊断由聚合数据触发。",
	}
}

func diagnosticTriggerValue(finding diagnosticFinding) string {
	switch {
	case strings.HasPrefix(finding.ID, "command.family.high_failure."):
		return evidenceValue(finding, "失败率")
	case finding.ID == "command.other.high_ratio":
		return evidenceValue(finding, "占比")
	case finding.ID == "command.risky.present":
		return evidenceValue(finding, "风险等级")
	case strings.HasPrefix(finding.ID, "command.classification.suspicious."):
		return evidenceValue(finding, "Top 命令")
	case strings.HasPrefix(finding.ID, "failure.reason.concentrated."):
		return evidenceValue(finding, "占比")
	case strings.HasPrefix(finding.ID, "failure.tool_reason.top."):
		return evidenceValue(finding, "次数")
	case strings.HasPrefix(finding.ID, "performance.tool.slow."):
		return evidenceValue(finding, "平均耗时")
	case finding.ID == "performance.slowest_call":
		return evidenceValue(finding, "耗时")
	case finding.ID == "performance.cache.build_cost":
		return evidenceValue(finding, "构建耗时")
	case finding.ID == "performance.turn.slow":
		return evidenceValue(finding, "平均回合耗时")
	case strings.HasPrefix(finding.ID, "performance.throughput.low."):
		return evidenceValue(finding, "吞吐")
	case finding.ID == "context.project.token_concentration":
		return evidenceValue(finding, "占比")
	case finding.ID == "context.session.large_token":
		return evidenceValue(finding, "Token")
	case finding.ID == "workflow.session.long_running":
		return evidenceValue(finding, "耗时")
	case finding.ID == "workflow.interactive_tool.long_wait":
		return evidenceValue(finding, "耗时")
	case finding.ID == "workflow.plan.no_exit":
		return evidenceValue(finding, "Plan 退出")
	default:
		return finding.Severity
	}
}

func buildDiagnosticExamples(data *DashboardData, finding diagnosticFinding, limit int) []diagnosticExample {
	if data == nil || data.FailureAnalysis == nil || limit <= 0 {
		return nil
	}
	examples := make([]diagnosticExample, 0, limit)
	for _, sample := range data.FailureAnalysis.Samples {
		if !matchesDiagnosticSample(finding, sample) {
			continue
		}
		examples = append(examples, diagnosticExample{
			Tool:           sample.Tool,
			Category:       sample.Category,
			Reason:         sample.Reason,
			Project:        sample.Project,
			SessionID:      sample.SessionID,
			Timestamp:      sample.Timestamp,
			ContentPreview: sample.ContentPreview,
		})
		if len(examples) >= limit {
			break
		}
	}
	return examples
}

func matchesDiagnosticSample(finding diagnosticFinding, sample ToolFailureSample) bool {
	reasonCategory, reason := splitCategoryReason(evidenceValue(finding, "原因"))
	tool := evidenceValue(finding, "工具")
	session := evidenceValue(finding, "Session")
	project := evidenceValue(finding, "项目")
	switch finding.Category {
	case "failure":
		if tool != "" && !strings.EqualFold(tool, sample.Tool) {
			return false
		}
		if reasonCategory != "" && !strings.EqualFold(reasonCategory, sample.Category) {
			return false
		}
		if reason != "" && !strings.EqualFold(reason, sample.Reason) {
			return false
		}
		return true
	case "command":
		return strings.EqualFold(sample.Tool, "Bash")
	case "performance", "workflow", "context":
		if session != "" && session != "unknown" {
			return strings.EqualFold(session, sample.SessionID)
		}
		if project != "" && project != "unknown" {
			return strings.Contains(strings.ToLower(sample.Project), strings.ToLower(project))
		}
	}
	return false
}

func buildDiagnosticRootCauses(finding diagnosticFinding, examples []diagnosticExample) []diagnosticRootCause {
	switch finding.Category {
	case "failure":
		return failureRootCauses(finding, examples)
	case "command":
		return commandRootCauses(finding, examples)
	case "performance":
		return performanceRootCauses(finding, examples)
	case "context":
		return contextRootCauses(finding, examples)
	case "workflow":
		return workflowRootCauses(finding, examples)
	default:
		return nil
	}
}

func failureRootCauses(finding diagnosticFinding, examples []diagnosticExample) []diagnosticRootCause {
	reason := strings.ToLower(evidenceValue(finding, "原因"))
	tool := evidenceValue(finding, "工具")
	if cause, ok := detailedFailureRootCause(finding, examples); ok {
		return []diagnosticRootCause{cause}
	}
	if strings.Contains(reason, "not_found") || strings.Contains(reason, "missing") || strings.Contains(reason, "file") {
		return []diagnosticRootCause{{
			Type:                 "missing_path_or_file_assumption",
			Confidence:           finding.Confidence,
			Summary:              "失败更像路径、文件或目录假设错误。",
			Evidence:             compactStrings(evidenceStatement(finding, "原因"), evidenceStatement(finding, "次数"), evidenceStatement(finding, "工具")),
			RecommendationTarget: "CLAUDE.md",
		}}
	}
	if strings.Contains(reason, "timeout") {
		return []diagnosticRootCause{{
			Type:                 "timeout_or_task_scope_too_large",
			Confidence:           finding.Confidence,
			Summary:              "失败更像超时、任务范围过大或缺少短路径验证。",
			Evidence:             compactStrings(evidenceStatement(finding, "原因"), evidenceStatement(finding, "次数"), evidenceStatement(finding, "工具")),
			RecommendationTarget: "workflow",
		}}
	}
	if strings.Contains(reason, "permission") || strings.Contains(reason, "auth") {
		return []diagnosticRootCause{{
			Type:                 "permission_or_auth_boundary",
			Confidence:           finding.Confidence,
			Summary:              "失败更像权限、认证或访问边界不清。",
			Evidence:             compactStrings(evidenceStatement(finding, "原因"), evidenceStatement(finding, "工具")),
			RecommendationTarget: "MCP",
		}}
	}
	return []diagnosticRootCause{{
		Type:                 "tool_precondition_or_parameter_contract",
		Confidence:           finding.Confidence,
		Summary:              "失败集中在特定原因或工具，优先检查调用前置条件和参数约定。",
		Evidence:             compactStrings(evidenceStatement(finding, "原因"), "工具="+nonEmpty(tool, "unknown")),
		RecommendationTarget: "CLAUDE.md",
	}}
}

func commandRootCauses(finding diagnosticFinding, examples []diagnosticExample) []diagnosticRootCause {
	family := strings.ToLower(evidenceValue(finding, "命令族"))
	if cause, ok := detailedFailureRootCause(finding, examples); ok {
		cause.Confidence = minConfidence(cause.Confidence, finding.Confidence)
		return []diagnosticRootCause{cause}
	}
	if strings.Contains(finding.ID, "classification") {
		return []diagnosticRootCause{{
			Type:                 "bash_rule_misclassification",
			Confidence:           "medium",
			Summary:              "命令族和 Top 命令语义不匹配，可能是 Bash 分类规则误命中。",
			Evidence:             compactStrings(evidenceStatement(finding, "命令族"), evidenceStatement(finding, "Top 命令"), evidenceStatement(finding, "样例")),
			RecommendationTarget: "tool",
		}}
	}
	if strings.Contains(finding.ID, "risky") {
		return []diagnosticRootCause{{
			Type:                 "unsafe_command_requires_guardrail",
			Confidence:           finding.Confidence,
			Summary:              "高风险命令需要显式确认、提示或 hook 保护。",
			Evidence:             compactStrings(evidenceStatement(finding, "命令"), evidenceStatement(finding, "风险等级"), evidenceStatement(finding, "原因")),
			RecommendationTarget: "hook",
		}}
	}
	target := "CLAUDE.md"
	causeType := "unstable_command_entrypoint"
	if family == "network" {
		target = "MCP"
		causeType = "service_or_network_precondition"
	}
	if family == "test" || family == "build" || family == "dependency" || family == "javascript" {
		target = "tool"
	}
	return []diagnosticRootCause{{
		Type:                 causeType,
		Confidence:           finding.Confidence,
		Summary:              "命令失败率偏高，通常来自入口、工作目录、依赖或前置条件不稳定。",
		Evidence:             compactStrings(evidenceStatement(finding, "命令族"), evidenceStatement(finding, "失败率"), evidenceStatement(finding, "Top 命令")),
		RecommendationTarget: target,
	}}
}

func performanceRootCauses(finding diagnosticFinding, examples []diagnosticExample) []diagnosticRootCause {
	if cause, ok := detailedFailureRootCause(finding, examples); ok {
		cause.Confidence = "medium"
		return []diagnosticRootCause{cause}
	}
	if finding.ID == "performance.cache.build_cost" {
		return []diagnosticRootCause{{
			Type:                 "cache_refresh_or_aggregation_cost",
			Confidence:           finding.Confidence,
			Summary:              "性能问题更像缓存刷新、聚合或附加扫描成本。",
			Evidence:             compactStrings(evidenceStatement(finding, "构建耗时"), evidenceStatement(finding, "解析文件"), evidenceStatement(finding, "复用文件")),
			RecommendationTarget: "tool",
		}}
	}
	return []diagnosticRootCause{{
		Type:                 "slow_tool_or_expensive_path",
		Confidence:           finding.Confidence,
		Summary:              "耗时更像慢工具调用、高成本命令或缺少 quick path。",
		Evidence:             compactStrings(evidenceStatement(finding, "工具"), evidenceStatement(finding, "类别"), evidenceStatement(finding, "耗时"), evidenceStatement(finding, "平均耗时")),
		RecommendationTarget: "tool",
	}}
}

func contextRootCauses(finding diagnosticFinding, examples []diagnosticExample) []diagnosticRootCause {
	if cause, ok := detailedFailureRootCause(finding, examples); ok {
		cause.Confidence = "medium"
		return []diagnosticRootCause{cause}
	}
	return []diagnosticRootCause{{
		Type:                 "large_context_or_repeated_exploration",
		Confidence:           finding.Confidence,
		Summary:              "Token 消耗偏高通常来自上下文反复读取、任务范围过大或缺少项目结构说明。",
		Evidence:             compactStrings(evidenceStatement(finding, "项目"), evidenceStatement(finding, "Session"), evidenceStatement(finding, "Token"), evidenceStatement(finding, "占比")),
		RecommendationTarget: "CLAUDE.md",
	}}
}

func workflowRootCauses(finding diagnosticFinding, examples []diagnosticExample) []diagnosticRootCause {
	if cause, ok := detailedFailureRootCause(finding, examples); ok {
		cause.Confidence = "medium"
		return []diagnosticRootCause{cause}
	}
	if finding.ID == "workflow.interactive_tool.long_wait" {
		return []diagnosticRootCause{{
			Type:                 "human_or_plan_wait",
			Confidence:           finding.Confidence,
			Summary:              "长耗时更像人工确认、计划确认或会话暂停。",
			Evidence:             compactStrings(evidenceStatement(finding, "工具"), evidenceStatement(finding, "耗时"), evidenceStatement(finding, "Session")),
			RecommendationTarget: "workflow",
		}}
	}
	if finding.ID == "workflow.plan.no_exit" {
		return []diagnosticRootCause{{
			Type:                 "missing_plan_lifecycle_signal",
			Confidence:           finding.Confidence,
			Summary:              "Plan 事件缺少退出信号，可能是解析缺口或工作流没有稳定结束标记。",
			Evidence:             compactStrings(evidenceStatement(finding, "Plan 进入"), evidenceStatement(finding, "Plan 退出")),
			RecommendationTarget: "workflow",
		}}
	}
	return []diagnosticRootCause{{
		Type:                 "task_boundary_or_recovery_strategy",
		Confidence:           finding.Confidence,
		Summary:              "长会话或高失败会话通常来自任务边界过大、计划拆分不足或失败恢复策略不清。",
		Evidence:             compactStrings(evidenceStatement(finding, "Session"), evidenceStatement(finding, "耗时"), evidenceStatement(finding, "工具失败")),
		RecommendationTarget: "workflow",
	}}
}

func detailedFailureRootCause(finding diagnosticFinding, examples []diagnosticExample) (diagnosticRootCause, bool) {
	text := strings.ToLower(evidenceValue(finding, "原因") + " " + evidenceValue(finding, "工具") + " " + examplesText(examples))
	evidence := detailedExampleEvidence(examples)
	confidence := "medium"
	if len(examples) > 0 {
		confidence = "high"
	}
	switch {
	case containsAny(text, "executable doesn't exist", "playwright", "chromium", "browser"):
		return diagnosticRootCause{
			Type:                 "browser_missing",
			Confidence:           confidence,
			Summary:              "失败更像浏览器运行时或 Playwright 依赖缺失。",
			Evidence:             evidence,
			RecommendationTarget: "MCP",
		}, true
	case containsAny(text, "connection refused", "econnrefused", "localhost", "127.0.0.1"):
		return diagnosticRootCause{
			Type:                 "service_not_running",
			Confidence:           confidence,
			Summary:              "失败更像本地服务未启动、端口不可达或服务地址不一致。",
			Evidence:             evidence,
			RecommendationTarget: "CLAUDE.md",
		}, true
	case containsAny(text, "address already in use", "eaddrinuse", "port is already allocated"):
		return diagnosticRootCause{
			Type:                 "port_conflict",
			Confidence:           confidence,
			Summary:              "失败更像端口冲突或已有服务占用。",
			Evidence:             evidence,
			RecommendationTarget: "hook",
		}, true
	case containsAny(text, "command not found", "module not found", "cannot find module", "no module named", "package not found"):
		return diagnosticRootCause{
			Type:                 "dependency_missing",
			Confidence:           confidence,
			Summary:              "失败更像依赖、命令或运行环境缺失。",
			Evidence:             evidence,
			RecommendationTarget: "tool",
		}, true
	case containsAny(text, "no such file", "file not found", "not found in file", "不存在", "cannot find"):
		return diagnosticRootCause{
			Type:                 "missing_path",
			Confidence:           confidence,
			Summary:              "失败更像文件、目录或替换目标不存在。",
			Evidence:             evidence,
			RecommendationTarget: "CLAUDE.md",
		}, true
	case containsAny(text, "permission denied", "operation not permitted", "unauthorized", "forbidden", "invalid api key"):
		return diagnosticRootCause{
			Type:                 "permission_or_auth",
			Confidence:           confidence,
			Summary:              "失败更像权限、认证或访问边界不清。",
			Evidence:             evidence,
			RecommendationTarget: "MCP",
		}, true
	case containsAny(text, "http_4", "http 4", "http_5", "http 5", "status 4", "status 5", " 412", " 429", " 500", " 502", " 503"):
		return diagnosticRootCause{
			Type:                 "network_4xx_5xx",
			Confidence:           confidence,
			Summary:              "失败更像外部服务 HTTP 错误、限流或认证失败。",
			Evidence:             evidence,
			RecommendationTarget: "MCP",
		}, true
	case containsAny(text, "timed out", "timeout", "deadline exceeded", "超时"):
		return diagnosticRootCause{
			Type:                 "task_scope_too_large",
			Confidence:           confidence,
			Summary:              "失败更像任务范围过大、慢调用缺少短超时或缺少 quick path。",
			Evidence:             evidence,
			RecommendationTarget: "workflow",
		}, true
	case containsAny(text, "not a git repository", "no such directory", "working directory", "cwd"):
		return diagnosticRootCause{
			Type:                 "wrong_workdir",
			Confidence:           confidence,
			Summary:              "失败更像工作目录或项目入口判断错误。",
			Evidence:             evidence,
			RecommendationTarget: "CLAUDE.md",
		}, true
	default:
		return diagnosticRootCause{}, false
	}
}

func examplesText(examples []diagnosticExample) string {
	parts := make([]string, 0, len(examples)*4)
	for _, example := range examples {
		parts = append(parts, example.Tool, example.Category, example.Reason, example.ContentPreview)
	}
	return strings.Join(parts, " ")
}

func detailedExampleEvidence(examples []diagnosticExample) []string {
	if len(examples) == 0 {
		return nil
	}
	items := make([]string, 0, len(examples))
	for _, example := range examples {
		reason := strings.Trim(example.Category+"/"+example.Reason, "/")
		items = append(items, fmt.Sprintf("%s %s: %s", example.Tool, reason, previewString(example.ContentPreview, 120)))
	}
	return items
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func minConfidence(a, b string) string {
	if severityRank(a) < severityRank(b) {
		return a
	}
	return b
}

func diagnosticTargets(finding diagnosticFinding) []string {
	targets := make([]string, 0, 4)
	for _, cause := range finding.RootCauses {
		targets = append(targets, cause.RecommendationTarget)
	}
	switch finding.Category {
	case "failure":
		targets = append(targets, "CLAUDE.md", "hook")
	case "command":
		targets = append(targets, "CLAUDE.md", "tool")
	case "performance":
		targets = append(targets, "tool", "workflow")
	case "context":
		targets = append(targets, "CLAUDE.md", "workflow")
	case "workflow":
		targets = append(targets, "workflow", "CLAUDE.md")
	}
	return uniqueNonEmptyStrings(targets)
}

func diagnosticActions(finding diagnosticFinding) []diagnosticAction {
	actions := make([]diagnosticAction, 0, len(finding.RootCauses)+2)
	for _, cause := range finding.RootCauses {
		actions = append(actions, actionForRootCause(cause))
	}
	if len(actions) == 0 {
		for _, target := range finding.Targets {
			actions = append(actions, diagnosticAction{
				Target: target,
				Action: genericActionForTarget(target),
				Why:    "该诊断指向 " + finding.Category + " 类问题。",
			})
		}
	}
	return dedupeDiagnosticActions(actions)
}

func actionForRootCause(cause diagnosticRootCause) diagnosticAction {
	action := genericActionForTarget(cause.RecommendationTarget)
	switch cause.Type {
	case "missing_path", "missing_path_or_file_assumption":
		action = "补充项目目录结构、常用文件入口和执行前检查路径存在的约定。"
	case "wrong_workdir":
		action = "写清标准工作目录、子项目入口和命令执行目录。"
	case "service_not_running":
		action = "补充本地服务启动、健康检查和端口约定。"
	case "port_conflict":
		action = "增加端口占用检查或 hook 提示，避免重复启动服务。"
	case "dependency_missing":
		action = "封装环境检查命令，并写清包管理器、安装步骤和禁止混用规则。"
	case "browser_missing":
		action = "补充浏览器/Playwright 安装检查，或在 MCP/工具初始化中验证运行时依赖。"
	case "network_4xx_5xx":
		action = "补充认证、代理、限流和外部服务失败的处理约定。"
	case "permission_or_auth", "permission_or_auth_boundary":
		action = "明确权限、认证和密钥配置边界，必要时增加前置校验。"
	case "task_scope_too_large", "timeout_or_task_scope_too_large":
		action = "拆分任务阶段，区分 quick path 和 full path，并为慢命令设置短超时。"
	case "unsafe_command_requires_guardrail":
		action = "为删除、sudo、权限修改等命令增加显式确认或 hook 拦截。"
	case "bash_rule_misclassification":
		action = "调整 Bash 分类规则，修正 priority、contains 或 exclude。"
	case "slow_tool_or_expensive_path":
		action = "把高频慢路径封装成稳定脚本，提供快速验证入口。"
	case "large_context_or_repeated_exploration":
		action = "补充项目结构、任务拆分和文件读取边界，减少重复探索。"
	case "human_or_plan_wait", "task_boundary_or_recovery_strategy":
		action = "明确计划确认、任务拆分、暂停恢复和验收点。"
	}
	return diagnosticAction{
		Target: cause.RecommendationTarget,
		Action: action,
		Why:    cause.Summary,
	}
}

func genericActionForTarget(target string) string {
	switch target {
	case "CLAUDE.md":
		return "补充项目结构、常用命令、前置条件和任务边界。"
	case "hook":
		return "增加前置检查、危险操作确认或失败提示。"
	case "MCP":
		return "检查工具依赖、认证、网络、浏览器运行时和参数契约。"
	case "tool":
		return "把高频或高成本动作封装为稳定脚本或短命令。"
	case "workflow":
		return "拆分任务阶段，明确计划、执行、验证和恢复策略。"
	default:
		return "根据诊断证据补充可执行约定。"
	}
}

func dedupeDiagnosticActions(actions []diagnosticAction) []diagnosticAction {
	seen := make(map[string]bool)
	deduped := make([]diagnosticAction, 0, len(actions))
	for _, action := range actions {
		key := action.Target + "\x00" + action.Action
		if action.Target == "" || action.Action == "" || seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, action)
	}
	return deduped
}

func addDiagnosticDrilldowns(timeRange TimeRangeInfo, findings []diagnosticFinding) {
	for i := range findings {
		findings[i].Drilldowns = buildDiagnosticDrilldowns(timeRange, findings[i])
	}
}

func buildDiagnosticDrilldowns(timeRange TimeRangeInfo, finding diagnosticFinding) []diagnosticCommand {
	rangeArgs := cliRangeArgs(timeRange)
	base := "cc-insights"
	commands := []diagnosticCommand{}
	switch finding.Category {
	case "failure":
		reasonCategory, reason := splitCategoryReason(evidenceValue(finding, "原因"))
		tool := evidenceValue(finding, "工具")
		args := []string{"why"}
		args = append(args, rangeArgs...)
		if reasonCategory != "" {
			args = append(args, "--category", reasonCategory)
		}
		if reason != "" {
			args = append(args, "--reason", reason)
		}
		if tool != "" {
			args = append(args, "--tool", tool)
		}
		args = append(args, "-n", "10")
		commands = append(commands, diagnosticCommand{Label: "失败样例", Command: joinCLICommand(base, args)})
		commands = append(commands, diagnosticCommand{Label: "失败汇总", Command: joinCLICommand(base, append([]string{"err"}, rangeArgs...))})
	case "command":
		commands = append(commands, diagnosticCommand{Label: "命令分布", Command: joinCLICommand(base, append([]string{"cmd"}, append(rangeArgs, "-n", "20")...))})
		if strings.Contains(finding.ID, "high_failure") || strings.Contains(finding.ID, "risky") {
			commands = append(commands, diagnosticCommand{Label: "Bash 失败样例", Command: joinCLICommand(base, append(append([]string{"why"}, rangeArgs...), "--tool", "Bash", "-n", "10"))})
		}
	case "performance":
		session := evidenceValue(finding, "Session")
		if session != "" && session != "unknown" {
			commands = append(commands, diagnosticCommand{Label: "Session", Command: joinCLICommand(base, append(append([]string{"ses"}, rangeArgs...), "--session", session, "-n", "10"))})
			commands = append(commands, diagnosticCommand{Label: "Session 失败", Command: joinCLICommand(base, append(append([]string{"why"}, rangeArgs...), "--session", session, "-n", "10"))})
		}
		commands = append(commands, diagnosticCommand{Label: "命令分布", Command: joinCLICommand(base, append([]string{"cmd"}, rangeArgs...))})
	case "context":
		session := evidenceValue(finding, "Session")
		project := evidenceValue(finding, "项目")
		if session != "" {
			commands = append(commands, diagnosticCommand{Label: "Session", Command: joinCLICommand(base, append(append([]string{"ses"}, rangeArgs...), "--session", session, "-n", "10"))})
		} else if project != "" {
			commands = append(commands, diagnosticCommand{Label: "项目 Session", Command: joinCLICommand(base, append(append([]string{"ses"}, rangeArgs...), "--project", project, "-n", "10"))})
		}
		commands = append(commands, diagnosticCommand{Label: "Token 明细", Command: joinCLICommand(base, append(append([]string{"tok"}, rangeArgs...), "-n", "20"))})
	case "workflow":
		session := evidenceValue(finding, "Session")
		if session != "" && session != "unknown" {
			commands = append(commands, diagnosticCommand{Label: "Session", Command: joinCLICommand(base, append(append([]string{"ses"}, rangeArgs...), "--session", session, "-n", "10"))})
			commands = append(commands, diagnosticCommand{Label: "Session 失败", Command: joinCLICommand(base, append(append([]string{"why"}, rangeArgs...), "--session", session, "-n", "10"))})
		} else {
			commands = append(commands, diagnosticCommand{Label: "Session 概览", Command: joinCLICommand(base, append(append([]string{"ses"}, rangeArgs...), "-n", "20"))})
		}
	}
	return dedupeDiagnosticCommands(commands)
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
	// 单轮请求(turn)耗时：来自 system/turn_duration，含工具执行。与吞吐互补区分瓶颈位置。
	if data.ToolPerformance != nil && data.ToolPerformance.TurnCount > 0 {
		tp := data.ToolPerformance
		if tp.AvgTurnDurationMs >= 60_000 || tp.MaxTurnDurationMs >= 10*60*1000 {
			evidence := []diagnosticEvidence{
				{Label: "回合数", Value: formatInt(int(tp.TurnCount))},
				{Label: "平均回合耗时", Value: formatDurationMs(int64(tp.AvgTurnDurationMs))},
				{Label: "最长回合", Value: formatDurationMs(tp.MaxTurnDurationMs)},
			}
			if len(tp.SlowTurns) > 0 {
				evidence = append(evidence, diagnosticEvidence{Label: "最慢回合", Value: formatDurationMs(tp.SlowTurns[0].DurationMs)})
			}
			findings = append(findings, diagnosticFinding{
				ID:             "performance.turn.slow",
				Category:       "performance",
				Severity:       severityFromDuration(tp.AvgTurnDurationMs),
				Title:          "单轮请求耗时偏长",
				Summary:        fmt.Sprintf("平均一轮耗时 %s，最长 %s，共 %s 轮。", formatDurationMs(int64(tp.AvgTurnDurationMs)), formatDurationMs(tp.MaxTurnDurationMs), formatInt(int(tp.TurnCount))),
				Evidence:       evidence,
				Interpretation: "回合耗时含工具执行。若吞吐正常则慢点在工具/hooks/任务拆分；若吞吐也低则慢点在模型本身。",
				NextSteps: []string{
					"查看 slow_turns 样例，确认慢回合集中在哪些项目、session。",
					"对照按模型吞吐，区分慢在工具执行还是模型生成。",
				},
				Confidence: "medium",
			})
		}
	}
	// 模型吞吐(output_tokens / 单请求耗时)：找吞吐最低且样本充足的模型
	if data.CostAnalysis != nil && len(data.CostAnalysis.ByModel) > 0 {
		var slowest *CostModelStat
		for i := range data.CostAnalysis.ByModel {
			m := &data.CostAnalysis.ByModel[i]
			if m.OutputTokensPerSec <= 0 || m.RequestCount < 10 {
				continue
			}
			if slowest == nil || m.OutputTokensPerSec < slowest.OutputTokensPerSec {
				slowest = m
			}
		}
		if slowest != nil && slowest.OutputTokensPerSec < 10 {
			findings = append(findings, diagnosticFinding{
				ID:       "performance.throughput.low." + sanitizeID(slowest.Model),
				Category: "performance",
				Severity: severityFromThroughput(slowest.OutputTokensPerSec),
				Title:    "模型响应吞吐偏低",
				Summary:  fmt.Sprintf("%s 平均吞吐 %.1f tok/s，平均单请求 %s。", slowest.Model, slowest.OutputTokensPerSec, formatDurationMs(int64(slowest.AvgRoundTripMs))),
				Evidence: []diagnosticEvidence{
					{Label: "模型", Value: slowest.Model},
					{Label: "吞吐", Value: fmt.Sprintf("%.1f tok/s", slowest.OutputTokensPerSec)},
					{Label: "平均单请求", Value: formatDurationMs(int64(slowest.AvgRoundTripMs))},
					{Label: "请求数", Value: formatInt(slowest.RequestCount)},
				},
				Interpretation: "吞吐 = output_tokens / 单请求耗时。偏低通常与大上下文 cache miss、限流或模型档位相关。",
				NextSteps: []string{
					"检查该模型的 cache_read 占比：cache miss 高会拉低吞吐。",
					"对比其他模型吞吐，确认是否特定模型/档位问题。",
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

func cliRangeArgs(timeRange TimeRangeInfo) []string {
	if timeRange.Preset != "" && timeRange.Preset != "custom" {
		return []string{"-p", timeRange.Preset}
	}
	if timeRange.Start != "" && timeRange.End != "" {
		return []string{"--start", timeRange.Start, "--end", timeRange.End}
	}
	return nil
}

func evidenceValue(finding diagnosticFinding, label string) string {
	for _, ev := range finding.Evidence {
		if ev.Label == label {
			return strings.TrimSpace(ev.Value)
		}
	}
	return ""
}

func evidenceStatement(finding diagnosticFinding, label string) string {
	value := evidenceValue(finding, label)
	if value == "" {
		return ""
	}
	return label + "=" + value
}

func compactStrings(values ...string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]bool)
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		items = append(items, value)
	}
	return items
}

func splitCategoryReason(value string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(value), "/", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(value)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func joinCLICommand(base string, args []string) string {
	parts := []string{base}
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			continue
		}
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' ||
			r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' ||
			r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == '=' || r == '%')
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func dedupeDiagnosticCommands(commands []diagnosticCommand) []diagnosticCommand {
	seen := make(map[string]bool)
	deduped := make([]diagnosticCommand, 0, len(commands))
	for _, command := range commands {
		if command.Command == "" || seen[command.Command] {
			continue
		}
		seen[command.Command] = true
		deduped = append(deduped, command)
	}
	return deduped
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

// severityFromThroughput 按吞吐(tok/s)定级：越低越严重。
func severityFromThroughput(tokPerSec float64) string {
	switch {
	case tokPerSec < 5:
		return "high"
	case tokPerSec < 15:
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
