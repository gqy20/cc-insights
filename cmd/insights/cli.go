package main

import (
	"encoding/json"
	"errors"
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
	ID       string
	Detail   bool
	Prompts  bool

	jsonOut     bool // -j：输出 JSON（仅分析命令注册）
	markdownOut bool // -m：输出 Markdown（仅分析命令注册）
}

type resolvedCommand struct {
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

type cliRecommendationReport struct {
	TimeRange       TimeRangeInfo       `json:"time_range"`
	TotalFindings   int                 `json:"total_findings"`
	ByCategory      []nameCount         `json:"by_category"`
	Recommendations []diagnosticFinding `json:"recommendations"`
	Runtime         *cliRuntimeInfo     `json:"runtime,omitempty"`
	Detail          bool                `json:"detail"`
	FilterID        string              `json:"filter_id,omitempty"`
	Insights        []string            `json:"insights"`
}

type cliSessionReport struct {
	TimeRange       TimeRangeInfo          `json:"time_range"`
	TotalSessions   int                    `json:"total_sessions"`
	LongRunning     []SessionAnalysisItem  `json:"long_running"`
	TopFailures     []SessionAnalysisItem  `json:"top_failures"`
	Outcomes        []SessionOutcomeStat   `json:"outcomes"`
	QueueOperations []QueueOperationStat   `json:"queue_operations"`
	PlanLifecycle   PlanLifecycleData      `json:"plan_lifecycle"`
	ReminderSummary ReminderSummaryData    `json:"reminder_summary"`
	TopTaskSessions []ReminderSessionItem  `json:"top_task_sessions"`
	TopTodoSessions []ReminderSessionItem  `json:"top_todo_sessions"`
	FailureSamples  []ToolFailureSample    `json:"failure_samples"`
	Hooks           []HookStatItem         `json:"hooks"`
	SessionHooks    []SessionHookStat      `json:"session_hooks,omitempty"`
	Filter          cliSessionReportFilter `json:"filter"`
	Insights        []string               `json:"insights"`
}

type cliSessionReportFilter struct {
	Session string `json:"session,omitempty"`
	Project string `json:"project,omitempty"`
}

type cliRuntimeInfo struct {
	Source                   string `json:"source"`
	PrepareDurationMs        int64  `json:"prepare_duration_ms"`
	DataDurationMs           int64  `json:"data_duration_ms"`
	RecommendationDurationMs int64  `json:"recommendation_duration_ms"`
	TotalDurationMs          int64  `json:"total_duration_ms"`
}

type diagnosticFinding struct {
	ID             string                `json:"id"`
	Category       string                `json:"category"`
	Severity       string                `json:"severity"`
	Title          string                `json:"title"`
	Summary        string                `json:"summary"`
	Evidence       []diagnosticEvidence  `json:"evidence"`
	Trigger        *diagnosticTrigger    `json:"trigger,omitempty"`
	RootCauses     []diagnosticRootCause `json:"root_causes,omitempty"`
	Targets        []string              `json:"targets,omitempty"`
	Examples       []diagnosticExample   `json:"examples,omitempty"`
	Actions        []diagnosticAction    `json:"actions,omitempty"`
	Interpretation string                `json:"interpretation"`
	NextSteps      []string              `json:"next_steps"`
	Drilldowns     []diagnosticCommand   `json:"drilldown_commands,omitempty"`
	Confidence     string                `json:"confidence"`
}

type diagnosticTrigger struct {
	Metric    string `json:"metric"`
	Value     string `json:"value"`
	Threshold string `json:"threshold"`
	Source    string `json:"source"`
	Rationale string `json:"rationale,omitempty"`
}

type diagnosticRootCause struct {
	Type                 string   `json:"type"`
	Confidence           string   `json:"confidence"`
	Summary              string   `json:"summary"`
	Evidence             []string `json:"evidence,omitempty"`
	RecommendationTarget string   `json:"recommendation_target"`
}

type diagnosticExample struct {
	Tool           string `json:"tool"`
	Category       string `json:"category,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Project        string `json:"project,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	Timestamp      string `json:"timestamp,omitempty"`
	ContentPreview string `json:"content_preview,omitempty"`
}

type diagnosticAction struct {
	Target string `json:"target"`
	Action string `json:"action"`
	Why    string `json:"why"`
}

type diagnosticEvidence struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type diagnosticCommand struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

func runCLI(args []string) error {
	if len(args) > 0 && isHelpRequest(args[0]) {
		printGlobalHelp(os.Stdout)
		return nil
	}

	resolved := resolveCLICommand(args)
	cmd := lookupCommand(resolved.Name)
	if cmd == nil {
		return fmt.Errorf("未知命令 %q，运行 cc-insights help 查看用法", resolved.Name)
	}

	opts, err := parseCLIOptions(cmd, resolved.Args)
	if err != nil {
		if errors.Is(err, errHelp) {
			return nil
		}
		return err
	}
	cfg = opts.Config
	return cmd.Run(opts)
}

func isHelpRequest(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func resolveCLICommand(args []string) resolvedCommand {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return resolvedCommand{Name: "sum", Args: args}
	}
	return resolvedCommand{Name: args[0], Args: args[1:]}
}

func parseCLIOptions(cmd *Command, args []string) (cliOptions, error) {
	opts := cliOptions{
		Config:  defaultConfig(),
		Preset:  "30d",
		Format:  "table",
		Limit:   10,
		Samples: -1,
	}
	fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	fs.Usage = func() { printCommandHelp(cmd, fs, os.Stdout) }
	fs.SetOutput(os.Stdout)
	registerConfigFlags(fs, &opts.Config)

	if cmd.Flags != nil {
		cmd.Flags(fs, &opts) // 分析命令注册通用分析 flag（含 -j/-m）；web 注册 --addr/--base
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, errHelp
		}
		return opts, err
	}

	if opts.jsonOut {
		opts.Format = "json"
	}
	if opts.markdownOut {
		opts.Format = "markdown"
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
	return prepareCLIDataWithCacheRefresh(true)
}

func prepareCLIDataForRecommendations() error {
	return prepareCLIDataWithCacheRefresh(false)
}

func prepareCLIDataWithCacheRefresh(refreshStale bool) error {
	if _, err := os.Stat(cfg.DataDir); os.IsNotExist(err) {
		return fmt.Errorf("数据目录不存在: %s", cfg.DataDir)
	}
	logDir := filepath.Join(filepath.Dir(cfg.CacheDir), "logs")
	if err := InitLogger(logDir); err != nil {
		return fmt.Errorf("日志初始化失败: %w", err)
	}
	if !refreshStale {
		if loaded, err := loadReusableCacheSnapshot(); err == nil {
			globalCache = loaded
			Info("使用现有缓存快照", "messages", globalCache.TotalMessages, "sessions", globalCache.TotalSessions)
			return nil
		} else {
			Warn("缓存快照不可复用，将刷新缓存", "error", err.Error())
		}
	}
	if err := initializeCache(); err != nil {
		Warn("缓存初始化失败，将使用实时解析模式", "error", err.Error())
	}
	return nil
}

func loadReusableCacheSnapshot() (*CacheFile, error) {
	cachePath := filepath.Join(cfg.CacheDir, "cache.db")
	cache, err := LoadCacheFile(diagnosticsCachePath(cachePath))
	if err != nil {
		cache, err = LoadCacheFile(cachePath)
	}
	if err != nil {
		return nil, err
	}
	if cache.Version != CacheVersion {
		return nil, fmt.Errorf("缓存版本 %s != %s", cache.Version, CacheVersion)
	}
	rulesHash, err := currentBashRulesHash()
	if err != nil {
		return nil, err
	}
	if cache.BashRulesHash != rulesHash {
		return nil, fmt.Errorf("Bash 规则已变更")
	}
	return cache, nil
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
