package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type cliOptions struct {
	Config
	Preset   string
	Start    string
	End      string
	Format   string
	Limit    int
	Samples  int
	Reason   string
	Category string
	Tool     string
	Model    string
	Project  string
	Session  string
}

type normalizedCommand struct {
	Name string
	Args []string
}

type cliSummary struct {
	TimeRange       TimeRangeInfo `json:"time_range"`
	Messages        int           `json:"messages"`
	Sessions        int           `json:"sessions"`
	Commands        int           `json:"commands"`
	Tools           int           `json:"tools"`
	ToolFailureRate float64       `json:"tool_failure_rate"`
	Tokens          int           `json:"tokens"`
	TopProject      string        `json:"top_project"`
	TopModel        string        `json:"top_model"`
	Insights        []string      `json:"insights"`
}

type cliFailureReport struct {
	TimeRange      TimeRangeInfo           `json:"time_range"`
	TotalCalls     int                     `json:"total_calls"`
	TotalFailures  int                     `json:"total_failures"`
	MissingResults int                     `json:"missing_results"`
	FailureRate    float64                 `json:"failure_rate"`
	ByReason       []FailureReasonStat     `json:"by_reason"`
	ByToolReason   []FailureToolReasonStat `json:"by_tool_reason"`
	ByModelTool    []ToolModelStatItem     `json:"by_model_tool"`
	Insights       []string                `json:"insights"`
}

type cliCostReport struct {
	TimeRange TimeRangeInfo       `json:"time_range"`
	Totals    TokenUsageBreakdown `json:"totals"`
	ByModel   []CostModelStat     `json:"by_model"`
	ByProject []CostProjectStat   `json:"by_project"`
	BySession []CostSessionStat   `json:"by_session"`
	Insights  []string            `json:"insights"`
}

type cliCommandReport struct {
	TimeRange     TimeRangeInfo           `json:"time_range"`
	TotalCommands int                     `json:"total_commands"`
	TotalCalls    int                     `json:"total_calls"`
	ByFamily      []BashCommandFamilyStat `json:"by_family"`
	ByCommand     []BashCommandStat       `json:"by_command"`
	RiskyCommands []BashCommandStat       `json:"risky_commands"`
	Insights      []string                `json:"insights"`
}

func runCLI(args []string) error {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printCLIHelp(os.Stdout)
		return nil
	}

	normalized := normalizeCLICommand(args)
	args = normalized.Args
	if normalized.Name == "web" {
		opts, err := parseCLIOptions(args[0], args[1:], false)
		if err != nil {
			return err
		}
		cfg = opts.Config
		return runWebServer()
	}

	command := normalized.Name
	commandArgs := normalized.Args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		commandArgs = args[1:]
	}
	if command != "sum" && command != "err" && command != "why" && command != "tok" && command != "cmd" {
		return fmt.Errorf("未知命令 %q，运行 cc-insights help 查看用法", command)
	}

	opts, err := parseCLIOptions(command, commandArgs, true)
	if err != nil {
		return err
	}
	cfg = opts.Config

	tf, preset, err := timeFilterFromCLIOptions(opts)
	if err != nil {
		return err
	}

	if err := prepareCLIData(); err != nil {
		return err
	}
	defer CloseLogger()

	data, _, err := buildDashboardData(tf, preset)
	if err != nil {
		return err
	}

	switch command {
	case "sum":
		return outputCLI(buildCLISummary(data), opts.Format, os.Stdout)
	case "err":
		return outputCLI(buildCLIFailureReport(data, opts.Limit), opts.Format, os.Stdout)
	case "tok":
		return outputCLI(buildCLICostReport(data, opts.Limit), opts.Format, os.Stdout)
	case "why":
		return outputCLI(buildCLIInspectFailuresReport(data, opts), opts.Format, os.Stdout)
	case "cmd":
		return outputCLI(buildCLICommandReport(data, opts.Limit), opts.Format, os.Stdout)
	}
	return nil
}

func normalizeCLICommand(args []string) normalizedCommand {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return normalizedCommand{Name: "sum", Args: args}
	}

	switch args[0] {
	case "sum", "err", "why", "tok", "cmd", "web":
		return normalizedCommand{Name: args[0], Args: args}
	default:
		return normalizedCommand{Name: args[0], Args: args}
	}
}

func parseCLIOptions(command string, args []string, analysisCommand bool) (cliOptions, error) {
	opts := cliOptions{
		Config:  defaultConfig(),
		Preset:  "30d",
		Format:  "table",
		Limit:   10,
		Samples: -1,
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerConfigFlags(fs, &opts.Config)
	var jsonOutput *bool
	var markdownOutput *bool
	if analysisCommand {
		fs.StringVar(&opts.Preset, "preset", opts.Preset, "时间范围: 24h|7d|30d|90d|all")
		fs.StringVar(&opts.Preset, "p", opts.Preset, "短参数: --preset")
		fs.StringVar(&opts.Start, "start", "", "自定义开始日期 YYYY-MM-DD")
		fs.StringVar(&opts.End, "end", "", "自定义结束日期 YYYY-MM-DD")
		fs.StringVar(&opts.Format, "format", opts.Format, "输出格式: table|json|markdown")
		fs.StringVar(&opts.Format, "f", opts.Format, "短参数: --format")
		jsonOutput = fs.Bool("j", false, "输出 JSON")
		markdownOutput = fs.Bool("m", false, "输出 Markdown")
		fs.IntVar(&opts.Limit, "limit", opts.Limit, "Top N 结果数量")
		fs.IntVar(&opts.Limit, "n", opts.Limit, "Top N / 样例数量")
		fs.IntVar(&opts.Samples, "samples", opts.Samples, "失败样例数量")
		fs.StringVar(&opts.Reason, "reason", "", "按失败 reason 过滤")
		fs.StringVar(&opts.Category, "category", "", "按失败 category 过滤")
		fs.StringVar(&opts.Tool, "tool", "", "按工具名过滤")
		fs.StringVar(&opts.Model, "model", "", "按模型名过滤")
		fs.StringVar(&opts.Project, "project", "", "按项目路径片段过滤")
		fs.StringVar(&opts.Session, "session", "", "按 session_id 过滤")
	}
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if analysisCommand {
		if *jsonOutput {
			opts.Format = "json"
		}
		if *markdownOutput {
			opts.Format = "markdown"
		}
		if command == "why" && opts.Samples <= 0 {
			opts.Samples = opts.Limit
		}
	}
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	if opts.Format == "" {
		opts.Format = "table"
	}
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	if opts.Samples <= 0 {
		opts.Samples = opts.Limit
	}
	return opts, nil
}

func prepareCLIData() error {
	if _, err := os.Stat(cfg.DataDir); os.IsNotExist(err) {
		return fmt.Errorf("数据目录不存在: %s", cfg.DataDir)
	}
	logDir := filepath.Join(filepath.Dir(cfg.CacheDir), "logs")
	if err := InitLogger(logDir); err != nil {
		return fmt.Errorf("日志初始化失败: %w", err)
	}
	if err := initializeCache(); err != nil {
		Warn("缓存初始化失败，将使用实时解析模式", "error", err.Error())
	}
	return nil
}

func timeFilterFromCLIOptions(opts cliOptions) (TimeFilter, string, error) {
	if opts.Start != "" || opts.End != "" {
		if opts.Start == "" || opts.End == "" {
			return TimeFilter{}, "", fmt.Errorf("--start 和 --end 必须同时提供")
		}
		tf, err := NewTimeFilterCustom(opts.Start, opts.End)
		return tf, "custom", err
	}
	preset := strings.TrimSpace(opts.Preset)
	if preset == "" {
		preset = "30d"
	}
	if preset == "all" {
		return TimeFilter{Start: nil, End: nil}, preset, nil
	}
	switch RangePreset(preset) {
	case Range24Hours, Range7Days, Range30Days, Range90Days:
	default:
		return TimeFilter{}, "", fmt.Errorf("不支持的 preset %q，支持 24h|7d|30d|90d|all", preset)
	}
	return NewTimeFilterFromPreset(RangePreset(preset)), preset, nil
}

func buildCLISummary(data *DashboardData) cliSummary {
	totalCommands := 0
	for _, command := range data.Commands {
		totalCommands += command.Count
	}
	totalTools := 0
	if data.ToolAnalysis != nil {
		totalTools = data.ToolAnalysis.TotalCalls
	} else {
		for _, tool := range data.MCPTools {
			totalTools += tool.Count
		}
	}
	totalMessages := 0
	totalSessions := 0
	if data.ProjectStats != nil {
		totalMessages = data.ProjectStats.TotalMessages
		totalSessions = data.ProjectStats.TotalSessions
	}
	if totalSessions == 0 && data.Sessions != nil {
		totalSessions = data.Sessions.TotalSessions
	}
	tokens := 0
	if data.CostAnalysis != nil {
		tokens = data.CostAnalysis.Totals.TotalTokens
	}
	topProject := "-"
	if data.ProjectStats != nil && len(data.ProjectStats.Projects) > 0 {
		topProject = data.ProjectStats.Projects[0].Project
	}
	topModel := "-"
	if len(data.ModelUsage) > 0 {
		topModel = data.ModelUsage[0].Model
	}
	failureRate := 0.0
	if data.ToolAnalysis != nil && data.ToolAnalysis.TotalCalls > 0 {
		failureRate = float64(data.ToolAnalysis.TotalFailures) / float64(data.ToolAnalysis.TotalCalls) * 100
	}
	summary := cliSummary{
		TimeRange:       data.TimeRange,
		Messages:        totalMessages,
		Sessions:        totalSessions,
		Commands:        totalCommands,
		Tools:           totalTools,
		ToolFailureRate: failureRate,
		Tokens:          tokens,
		TopProject:      topProject,
		TopModel:        topModel,
	}
	summary.Insights = buildSummaryInsights(data, summary)
	return summary
}

func buildSummaryInsights(data *DashboardData, summary cliSummary) []string {
	insights := []string{}
	if data.WeekdayStats != nil && len(data.WeekdayStats.WeekdayData) > 0 {
		top := data.WeekdayStats.WeekdayData[0]
		for _, item := range data.WeekdayStats.WeekdayData {
			if item.MessageCount > top.MessageCount {
				top = item
			}
		}
		if top.MessageCount > 0 {
			insights = append(insights, fmt.Sprintf("最活跃星期是 %s，消息数 %s。", top.WeekdayName, formatInt(top.MessageCount)))
		}
	}
	if data.FailureAnalysis != nil && len(data.FailureAnalysis.ByReason) > 0 {
		top := data.FailureAnalysis.ByReason[0]
		insights = append(insights, fmt.Sprintf("失败最多的原因是 %s/%s，共 %s 次。", top.Category, top.Reason, formatInt(top.Count)))
	}
	if summary.Tokens > 0 {
		insights = append(insights, fmt.Sprintf("Token 总量 %s，Top 模型 %s。", formatCompactInt(summary.Tokens), summary.TopModel))
	}
	return insights
}

func buildCLIFailureReport(data *DashboardData, limit int) cliFailureReport {
	report := cliFailureReport{TimeRange: data.TimeRange}
	if data.ToolAnalysis != nil {
		report.TotalCalls = data.ToolAnalysis.TotalCalls
		report.TotalFailures = data.ToolAnalysis.TotalFailures
		report.MissingResults = data.ToolAnalysis.MissingResults
		if report.TotalCalls > 0 {
			report.FailureRate = float64(report.TotalFailures) / float64(report.TotalCalls) * 100
		}
		report.ByModelTool = limitToolModelStats(data.ToolAnalysis.ByModel, limit)
	}
	if data.FailureAnalysis != nil {
		report.TotalFailures = data.FailureAnalysis.TotalFailures
		report.ByReason = limitFailureReasons(data.FailureAnalysis.ByReason, limit)
		report.ByToolReason = limitToolReasons(data.FailureAnalysis.ByToolReason, limit)
	}
	if len(report.ByReason) > 0 {
		top := report.ByReason[0]
		report.Insights = append(report.Insights, fmt.Sprintf("Top 失败原因是 %s/%s，共 %s 次。", top.Category, top.Reason, formatInt(top.Count)))
	}
	if len(report.ByModelTool) > 0 {
		top := report.ByModelTool[0]
		report.Insights = append(report.Insights, fmt.Sprintf("Top 模型工具组合是 %s/%s，失败率 %.1f%%。", top.Model, top.Tool, top.FailureRate))
	}
	return report
}

func buildCLICostReport(data *DashboardData, limit int) cliCostReport {
	report := cliCostReport{TimeRange: data.TimeRange}
	if data.CostAnalysis == nil {
		return report
	}
	report.Totals = data.CostAnalysis.Totals
	report.ByModel = limitCostModels(data.CostAnalysis.ByModel, limit)
	report.ByProject = limitCostProjects(data.CostAnalysis.ByProject, limit)
	report.BySession = limitCostSessions(data.CostAnalysis.BySession, limit)
	if len(report.ByModel) > 0 {
		report.Insights = append(report.Insights, fmt.Sprintf("Token 最高模型是 %s，共 %s。", report.ByModel[0].Model, formatCompactInt(report.ByModel[0].TotalTokens)))
	}
	if len(report.ByProject) > 0 {
		report.Insights = append(report.Insights, fmt.Sprintf("Token 最高项目是 %s，共 %s。", report.ByProject[0].Project, formatCompactInt(report.ByProject[0].TotalTokens)))
	}
	return report
}

func outputCLI(value any, format string, w io.Writer) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	case "markdown", "md":
		return writeMarkdown(value, w)
	case "table":
		return writeTable(value, w)
	default:
		return fmt.Errorf("不支持的输出格式 %q，支持 table|json|markdown", format)
	}
}

func printCLIHelp(w io.Writer) {
	fmt.Fprintln(w, `cc-insights analyzes Claude Code usage data.

Usage:
  cc-insights
  cc-insights err -p 7d -j
  cc-insights tok -p 30d -j
  cc-insights why -p 7d --reason error_text -n 20 -j
  cc-insights cmd -p 30d -j
  cc-insights web [--addr :8932]

Commands:
  sum   usage summary
  err   failure breakdown
  why   failure samples with filters
  tok   token and cost breakdown
  cmd   bash command families
  web   dashboard server

Global flags:
  --data PATH    Claude Code data directory, default ~/.claude
  --cache PATH   cache directory, default ~/.cc-insights/cache

Time flags:
  -p, --preset 24h|7d|30d|90d|all
  --start YYYY-MM-DD --end YYYY-MM-DD
  -j              output JSON
  -m              output Markdown
  -f, --format    table|json|markdown
  -n              Top N / sample count

Inspect failure filters:
  --reason VALUE --category VALUE --tool VALUE --model VALUE
  --project VALUE --session VALUE --samples N`)
}
