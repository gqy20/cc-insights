package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func writeTable(value any, w io.Writer) error {
	switch v := value.(type) {
	case cliSummary:
		fmt.Fprintf(w, "Claude Code Insights · %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "%-12s %s\n", "消息数", formatInt(v.Messages))
		fmt.Fprintf(w, "%-12s %s\n", "会话数", formatInt(v.Sessions))
		fmt.Fprintf(w, "%-12s %s\n", "命令调用", formatInt(v.Commands))
		fmt.Fprintf(w, "%-12s %s\n", "工具调用", formatInt(v.Tools))
		fmt.Fprintf(w, "%-12s %.1f%%\n", "工具失败率", v.ToolFailureRate)
		fmt.Fprintf(w, "%-12s %s\n", "Token", formatCompactInt(v.Tokens))
		fmt.Fprintf(w, "%-12s %s\n", "Top 项目", v.TopProject)
		fmt.Fprintf(w, "%-12s %s\n", "Top 模型", v.TopModel)
		writeInsights(w, v.Insights)
	case cliFailureReport:
		fmt.Fprintf(w, "Claude Code Failures · %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "工具调用: %s  失败: %s  Missing: %s  失败率: %.1f%%\n\n",
			formatInt(v.TotalCalls), formatInt(v.TotalFailures), formatInt(v.MissingResults), v.FailureRate)
		fmt.Fprintln(w, "Top 失败原因:")
		for _, item := range v.ByReason {
			fmt.Fprintf(w, "  %-16s %-24s %s\n", item.Category, item.Reason, formatInt(item.Count))
		}
		writeInsights(w, v.Insights)
	case cliCostReport:
		fmt.Fprintf(w, "Claude Code Cost · %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "请求: %s  Token: %s  输出: %s  缓存读取: %s\n\n",
			formatInt(v.Totals.RequestCount),
			formatCompactInt(v.Totals.TotalTokens),
			formatCompactInt(v.Totals.OutputTokens),
			formatCompactInt(v.Totals.CacheReadInputTokens))
		fmt.Fprintln(w, "Top 模型:")
		for _, item := range v.ByModel {
			fmt.Fprintf(w, "  %-28s %8s  %s requests\n", item.Model, formatCompactInt(item.TotalTokens), formatInt(item.RequestCount))
		}
		writeInsights(w, v.Insights)
	case cliCommandReport:
		fmt.Fprintf(w, "Claude Code Commands · %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "命令族: %s  具体命令: %s  调用: %s\n\n", formatInt(len(v.ByFamily)), formatInt(v.TotalCommands), formatInt(v.TotalCalls))
		fmt.Fprintln(w, "Top 命令族:")
		for _, item := range v.ByFamily {
			fmt.Fprintf(w, "  %-16s %8s  %s%%  top=%s\n", item.Family, formatInt(item.CallCount), formatFailureRate(item.FailureRate), item.TopCommand)
			if item.SampleCommand != "" {
				fmt.Fprintf(w, "    %s\n", item.SampleCommand)
			}
		}
		if len(v.RiskyCommands) > 0 {
			fmt.Fprintln(w, "\n高风险命令:")
			for _, item := range v.RiskyCommands {
				fmt.Fprintf(w, "  %-20s %-6s %s%%  %s\n", item.CommandName, item.RiskLevel, formatFailureRate(item.FailureRate), item.RiskReason)
			}
		}
		writeInsights(w, v.Insights)
	case cliRecommendationReport:
		fmt.Fprintf(w, "Claude Code Diagnostics · %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "诊断项: %s\n\n", formatInt(v.TotalFindings))
		if v.Runtime != nil {
			fmt.Fprintf(w, "耗时: prepare=%s  data=%s  rec=%s  total=%s  source=%s\n\n",
				formatDurationMs(v.Runtime.PrepareDurationMs),
				formatDurationMs(v.Runtime.DataDurationMs),
				formatDurationMs(v.Runtime.RecommendationDurationMs),
				formatDurationMs(v.Runtime.TotalDurationMs),
				v.Runtime.Source)
		}
		for _, item := range v.Recommendations {
			fmt.Fprintf(w, "[%s] %s · %s\n", strings.ToUpper(item.Severity), item.Category, item.Title)
			if item.Summary != "" {
				fmt.Fprintf(w, "  %s\n", item.Summary)
			}
			if len(item.Evidence) > 0 {
				fmt.Fprint(w, "  证据:")
				for _, ev := range item.Evidence {
					if ev.Value == "" {
						continue
					}
					fmt.Fprintf(w, " %s=%s;", ev.Label, ev.Value)
				}
				fmt.Fprintln(w)
			}
			if item.Interpretation != "" {
				fmt.Fprintf(w, "  解释: %s\n", item.Interpretation)
			}
			if len(item.NextSteps) > 0 {
				fmt.Fprintf(w, "  排查: %s\n", strings.Join(item.NextSteps, " / "))
			}
			writeDrilldownsTable(w, item.Drilldowns)
			fmt.Fprintln(w)
		}
		writeInsights(w, v.Insights)
	case cliSessionReport:
		fmt.Fprintf(w, "Claude Code Sessions · %s\n\n", formatRange(v.TimeRange))
		writeSessionFilter(w, v.Filter)
		fmt.Fprintf(w, "Session: %s  Plan: %s in / %s out  Task reminders: %s  Todo reminders: %s\n\n",
			formatInt(v.TotalSessions),
			formatInt(v.PlanLifecycle.EntryCount),
			formatInt(v.PlanLifecycle.ExitCount),
			formatInt(v.ReminderSummary.TaskReminderCount),
			formatInt(v.ReminderSummary.TodoReminderCount))
		writeSessionItemsTable(w, "长会话", v.LongRunning)
		writeSessionItemsTable(w, "高失败会话", v.TopFailures)
		if len(v.FailureSamples) > 0 {
			fmt.Fprintln(w, "失败样例:")
			for _, sample := range v.FailureSamples {
				fmt.Fprintf(w, "  %s %s %s/%s %s\n", sample.Timestamp, sample.Tool, sample.Category, sample.Reason, sample.Project)
				if sample.ContentPreview != "" {
					fmt.Fprintf(w, "    %s\n", sample.ContentPreview)
				}
			}
			fmt.Fprintln(w)
		}
		writeInsights(w, v.Insights)
	case cliInspectFailuresReport:
		fmt.Fprintf(w, "Claude Code Failure Inspection · %s\n\n", formatRange(v.TimeRange))
		writeFailureFilter(w, v.Filter)
		fmt.Fprintf(w, "样例: %s/%s matched\n\n", formatInt(v.Summary.MatchedSamples), formatInt(v.Summary.AvailableSamples))
		writeNameCounts(w, "Top 工具", v.Summary.TopTools)
		writeNameCounts(w, "Top 项目", v.Summary.TopProjects)
		fmt.Fprintln(w, "样例:")
		for _, sample := range v.Samples {
			fmt.Fprintf(w, "- %s %s %s/%s %s\n", sample.Timestamp, sample.Tool, sample.Category, sample.Reason, sample.Project)
			if sample.ContentPreview != "" {
				fmt.Fprintf(w, "  %s\n", sample.ContentPreview)
			}
		}
		writeInsights(w, v.Insights)
	default:
		return fmt.Errorf("不支持 table 输出类型 %T", value)
	}
	return nil
}

func writeMarkdown(value any, w io.Writer) error {
	switch v := value.(type) {
	case cliSummary:
		fmt.Fprintf(w, "# Claude Code Insights\n\n范围: %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "- 消息数: %s\n- 会话数: %s\n- Token: %s\n- 工具失败率: %.1f%%\n- Top 项目: %s\n- Top 模型: %s\n",
			formatInt(v.Messages), formatInt(v.Sessions), formatCompactInt(v.Tokens), v.ToolFailureRate, v.TopProject, v.TopModel)
		writeMarkdownInsights(w, v.Insights)
	case cliFailureReport:
		fmt.Fprintf(w, "# Claude Code Failures\n\n范围: %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "- 工具调用: %s\n- 失败: %s\n- Missing: %s\n- 失败率: %.1f%%\n\n", formatInt(v.TotalCalls), formatInt(v.TotalFailures), formatInt(v.MissingResults), v.FailureRate)
		fmt.Fprintln(w, "## Top 失败原因")
		fmt.Fprintln(w)
		for _, item := range v.ByReason {
			fmt.Fprintf(w, "- `%s/%s`: %s\n", item.Category, item.Reason, formatInt(item.Count))
		}
		writeMarkdownInsights(w, v.Insights)
	case cliCostReport:
		fmt.Fprintf(w, "# Claude Code Cost\n\n范围: %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "- 请求: %s\n- Token: %s\n- 输出: %s\n- 缓存读取: %s\n\n", formatInt(v.Totals.RequestCount), formatCompactInt(v.Totals.TotalTokens), formatCompactInt(v.Totals.OutputTokens), formatCompactInt(v.Totals.CacheReadInputTokens))
		fmt.Fprintln(w, "## Top 模型")
		fmt.Fprintln(w)
		for _, item := range v.ByModel {
			fmt.Fprintf(w, "- `%s`: %s (%s requests)\n", item.Model, formatCompactInt(item.TotalTokens), formatInt(item.RequestCount))
		}
		writeMarkdownInsights(w, v.Insights)
	case cliCommandReport:
		fmt.Fprintf(w, "# Claude Code Commands\n\n范围: %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "- 命令族: %s\n- 具体命令: %s\n- 调用: %s\n\n", formatInt(len(v.ByFamily)), formatInt(v.TotalCommands), formatInt(v.TotalCalls))
		fmt.Fprintln(w, "## Top 命令族")
		fmt.Fprintln(w)
		for _, item := range v.ByFamily {
			fmt.Fprintf(w, "- `%s`: %s, 失败率 %s%%, top `%s`\n", item.Family, formatInt(item.CallCount), formatFailureRate(item.FailureRate), item.TopCommand)
			if item.SampleCommand != "" {
				fmt.Fprintf(w, "  - %s\n", item.SampleCommand)
			}
		}
		if len(v.RiskyCommands) > 0 {
			fmt.Fprintln(w, "\n## 高风险命令")
			fmt.Fprintln(w)
			for _, item := range v.RiskyCommands {
				fmt.Fprintf(w, "- `%s`: %s, 失败率 %s%%, %s\n", item.CommandName, item.RiskLevel, formatFailureRate(item.FailureRate), item.RiskReason)
			}
		}
		writeMarkdownInsights(w, v.Insights)
	case cliRecommendationReport:
		fmt.Fprintf(w, "# Claude Code Diagnostics\n\n范围: %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "- 诊断项: %s\n\n", formatInt(v.TotalFindings))
		if v.Runtime != nil {
			fmt.Fprintf(w, "- 准备耗时: %s\n- 数据耗时: %s\n- 诊断耗时: %s\n- 总耗时: %s\n- 数据源: `%s`\n\n",
				formatDurationMs(v.Runtime.PrepareDurationMs),
				formatDurationMs(v.Runtime.DataDurationMs),
				formatDurationMs(v.Runtime.RecommendationDurationMs),
				formatDurationMs(v.Runtime.TotalDurationMs),
				v.Runtime.Source)
		}
		for _, item := range v.Recommendations {
			fmt.Fprintf(w, "## %s\n\n", item.Title)
			fmt.Fprintf(w, "- 分类: `%s`\n- 严重度: `%s`\n- 置信度: `%s`\n", item.Category, item.Severity, item.Confidence)
			if item.Summary != "" {
				fmt.Fprintf(w, "- 摘要: %s\n", item.Summary)
			}
			if len(item.Evidence) > 0 {
				fmt.Fprintln(w, "\n证据:")
				for _, ev := range item.Evidence {
					if ev.Value == "" {
						continue
					}
					fmt.Fprintf(w, "- %s: %s\n", ev.Label, ev.Value)
				}
			}
			if item.Interpretation != "" {
				fmt.Fprintf(w, "\n解释:\n%s\n", item.Interpretation)
			}
			if len(item.NextSteps) > 0 {
				fmt.Fprintln(w, "\n下一步:")
				for _, step := range item.NextSteps {
					fmt.Fprintf(w, "- %s\n", step)
				}
			}
			writeDrilldownsMarkdown(w, item.Drilldowns)
			fmt.Fprintln(w)
		}
		writeMarkdownInsights(w, v.Insights)
	case cliSessionReport:
		fmt.Fprintf(w, "# Claude Code Sessions\n\n范围: %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "- Session: %s\n- Plan: %s in / %s out\n- Task reminders: %s\n- Todo reminders: %s\n\n",
			formatInt(v.TotalSessions),
			formatInt(v.PlanLifecycle.EntryCount),
			formatInt(v.PlanLifecycle.ExitCount),
			formatInt(v.ReminderSummary.TaskReminderCount),
			formatInt(v.ReminderSummary.TodoReminderCount))
		writeSessionItemsMarkdown(w, "长会话", v.LongRunning)
		writeSessionItemsMarkdown(w, "高失败会话", v.TopFailures)
		if len(v.FailureSamples) > 0 {
			fmt.Fprintln(w, "## 失败样例")
			fmt.Fprintln(w)
			for _, sample := range v.FailureSamples {
				fmt.Fprintf(w, "- `%s` `%s` `%s/%s` `%s`\n", sample.Timestamp, sample.Tool, sample.Category, sample.Reason, sample.Project)
				if sample.ContentPreview != "" {
					fmt.Fprintf(w, "  - %s\n", sample.ContentPreview)
				}
			}
			fmt.Fprintln(w)
		}
		writeMarkdownInsights(w, v.Insights)
	case cliInspectFailuresReport:
		fmt.Fprintf(w, "# Claude Code Failure Inspection\n\n范围: %s\n\n", formatRange(v.TimeRange))
		fmt.Fprintf(w, "匹配样例: %s/%s\n\n", formatInt(v.Summary.MatchedSamples), formatInt(v.Summary.AvailableSamples))
		writeMarkdownNameCounts(w, "Top 工具", v.Summary.TopTools)
		writeMarkdownNameCounts(w, "Top 项目", v.Summary.TopProjects)
		fmt.Fprintln(w, "## 样例")
		fmt.Fprintln(w)
		for _, sample := range v.Samples {
			fmt.Fprintf(w, "- `%s` `%s` `%s/%s` `%s`\n", sample.Timestamp, sample.Tool, sample.Category, sample.Reason, sample.Project)
			if sample.ContentPreview != "" {
				fmt.Fprintf(w, "  - %s\n", sample.ContentPreview)
			}
		}
		writeMarkdownInsights(w, v.Insights)
	default:
		return fmt.Errorf("不支持 markdown 输出类型 %T", value)
	}
	return nil
}

func writeFailureFilter(w io.Writer, filter cliInspectFailureFilter) {
	parts := []string{}
	if filter.Reason != "" {
		parts = append(parts, "reason="+filter.Reason)
	}
	if filter.Category != "" {
		parts = append(parts, "category="+filter.Category)
	}
	if filter.Tool != "" {
		parts = append(parts, "tool="+filter.Tool)
	}
	if filter.Model != "" {
		parts = append(parts, "model="+filter.Model)
	}
	if filter.Project != "" {
		parts = append(parts, "project="+filter.Project)
	}
	if filter.Session != "" {
		parts = append(parts, "session="+filter.Session)
	}
	if len(parts) == 0 {
		fmt.Fprintln(w, "过滤: none")
		return
	}
	fmt.Fprintf(w, "过滤: %s\n", strings.Join(parts, ", "))
}

func writeSessionFilter(w io.Writer, filter cliSessionReportFilter) {
	parts := []string{}
	if filter.Session != "" {
		parts = append(parts, "session="+filter.Session)
	}
	if filter.Project != "" {
		parts = append(parts, "project="+filter.Project)
	}
	if len(parts) == 0 {
		fmt.Fprintln(w, "过滤: none")
		return
	}
	fmt.Fprintf(w, "过滤: %s\n", strings.Join(parts, ", "))
}

func writeSessionItemsTable(w io.Writer, title string, items []SessionAnalysisItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(w, title+":")
	for _, item := range items {
		fmt.Fprintf(w, "  %-24s %8s  fail=%s  tok=%s  %s\n",
			item.SessionID,
			formatDurationMs(item.DurationMs),
			formatInt(item.ToolFailureCount),
			formatCompactInt(item.TotalTokens),
			item.Project)
		if item.Title != "" {
			fmt.Fprintf(w, "    %s\n", item.Title)
		}
	}
	fmt.Fprintln(w)
}

func writeSessionItemsMarkdown(w io.Writer, title string, items []SessionAnalysisItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "## %s\n\n", title)
	for _, item := range items {
		fmt.Fprintf(w, "- `%s`: %s, fail=%s, token=%s, `%s`\n",
			item.SessionID,
			formatDurationMs(item.DurationMs),
			formatInt(item.ToolFailureCount),
			formatCompactInt(item.TotalTokens),
			item.Project)
		if item.Title != "" {
			fmt.Fprintf(w, "  - %s\n", item.Title)
		}
	}
	fmt.Fprintln(w)
}

func writeDrilldownsTable(w io.Writer, commands []diagnosticCommand) {
	if len(commands) == 0 {
		return
	}
	parts := make([]string, 0, len(commands))
	for _, command := range commands {
		if command.Command == "" {
			continue
		}
		if command.Label != "" {
			parts = append(parts, command.Label+": `"+command.Command+"`")
			continue
		}
		parts = append(parts, "`"+command.Command+"`")
	}
	if len(parts) > 0 {
		fmt.Fprintf(w, "  下钻: %s\n", strings.Join(parts, " / "))
	}
}

func writeDrilldownsMarkdown(w io.Writer, commands []diagnosticCommand) {
	if len(commands) == 0 {
		return
	}
	fmt.Fprintln(w, "\n下钻命令:")
	for _, command := range commands {
		if command.Command == "" {
			continue
		}
		if command.Label != "" {
			fmt.Fprintf(w, "- %s: `%s`\n", command.Label, command.Command)
			continue
		}
		fmt.Fprintf(w, "- `%s`\n", command.Command)
	}
}

func writeNameCounts(w io.Writer, title string, items []nameCount) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(w, title+":")
	for _, item := range items {
		fmt.Fprintf(w, "  %-40s %s\n", item.Name, formatInt(item.Count))
	}
	fmt.Fprintln(w)
}

func writeMarkdownNameCounts(w io.Writer, title string, items []nameCount) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "## %s\n\n", title)
	for _, item := range items {
		fmt.Fprintf(w, "- `%s`: %s\n", item.Name, formatInt(item.Count))
	}
	fmt.Fprintln(w)
}

func writeInsights(w io.Writer, insights []string) {
	if len(insights) == 0 {
		return
	}
	fmt.Fprintln(w, "\n主要发现:")
	for _, insight := range insights {
		fmt.Fprintf(w, "- %s\n", insight)
	}
}

func writeMarkdownInsights(w io.Writer, insights []string) {
	if len(insights) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## 主要发现")
	fmt.Fprintln(w)
	for _, insight := range insights {
		fmt.Fprintf(w, "- %s\n", insight)
	}
}

func formatRange(value TimeRangeInfo) string {
	if value.Preset == "custom" && value.Start != "" && value.End != "" {
		return value.Start + " 至 " + value.End
	}
	if value.Preset != "" {
		return value.Preset
	}
	return "all"
}

func formatInt(value int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	raw := strconv.Itoa(value)
	if len(raw) <= 3 {
		return sign + raw
	}
	out := make([]byte, 0, len(raw)+len(raw)/3)
	first := len(raw) % 3
	if first == 0 {
		first = 3
	}
	out = append(out, raw[:first]...)
	for i := first; i < len(raw); i += 3 {
		out = append(out, ',')
		out = append(out, raw[i:i+3]...)
	}
	return sign + string(out)
}

func formatCompactInt(value int) string {
	number := float64(value)
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", number/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.1fM", number/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fk", number/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func formatFailureRate(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}
