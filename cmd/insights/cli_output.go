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
