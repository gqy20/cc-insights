package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// APIResponse API 响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// globalCache 全局缓存实例
var globalCache *CacheFile

// DashboardData Dashboard 数据
type DashboardData struct {
	Timestamp        string                  `json:"timestamp"`
	TimeRange        TimeRangeInfo           `json:"time_range"`
	Commands         []CommandStats          `json:"commands"`
	HourlyCounts     map[string]int          `json:"hourly_counts"`
	DailyTrend       DailyTrendData          `json:"daily_trend"`
	RuntimeTools     []RuntimeToolSignal     `json:"runtime_tools"`
	Sessions         *SessionStats           `json:"sessions"`
	ProjectStats     *ProjectStatsData       `json:"project_stats,omitempty"`
	WeekdayStats     *WeekdayStats           `json:"weekday_stats,omitempty"`
	ModelUsage       []ModelUsageItem        `json:"model_usage,omitempty"`
	WorkHoursStats   *WorkHoursStats         `json:"work_hours_stats,omitempty"`
	ToolAnalysis     *ToolAnalysisData       `json:"tool_analysis,omitempty"`
	SkillAnalysis    *SkillAnalysisData      `json:"skill_analysis,omitempty"`
	EventAnalysis    *EventAnalysisData      `json:"event_analysis,omitempty"`
	AgentAnalysis    *AgentAnalysisData      `json:"agent_analysis,omitempty"`
	CommandAnalysis  *CommandAnalysisData    `json:"command_analysis,omitempty"`
	CostAnalysis     *CostAnalysisData       `json:"cost_analysis,omitempty"`
	FailureAnalysis  *FailureAnalysisData    `json:"failure_analysis,omitempty"`
	SessionAnalysis  *SessionAnalysisData    `json:"session_analysis,omitempty"`
	FileAnalysis     *FileAnalysisData       `json:"file_analysis,omitempty"`
	TaskPlanAnalysis *TaskPlanAnalysisData   `json:"task_plan_analysis,omitempty"`
	ToolPerformance  *ToolPerformanceData    `json:"tool_performance,omitempty"`
	Coverage         map[string]CoverageInfo `json:"coverage,omitempty"`
}

type CoverageInfo struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// TimeRangeInfo 时间范围信息
type TimeRangeInfo struct {
	Preset string `json:"preset"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
}

// DailyTrendData 每日趋势数据
type DailyTrendData struct {
	Dates  []string `json:"dates"`
	Counts []int    `json:"counts"`
}

// handleDataAPI 处理数据 API 请求
func handleDataAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// P1: 60秒超时保护，防止慢请求长时间占用连接
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendError(w, err.Error())
		return
	}

	// 使用 channel + select 实现超时控制
	type result struct {
		data *DashboardData
		err  error
	}
	resultCh := make(chan result, 1)

	go func() {
		data, source, err := buildDashboardDataWithFilter(filter)

		if err == nil {
			maybeValidateDashboardData(source, data)
		}

		resultCh <- result{data: data, err: err}
	}()

	// 等待结果或超时
	select {
	case <-ctx.Done():
		// 超时或客户端断开
		http.Error(w, `{"success":false,"error":"请求超时（数据处理超过60秒），请缩小时间范围后重试"}`, http.StatusRequestTimeout)
		return
	case res := <-resultCh:
		if res.err != nil {
			sendError(w, res.err.Error())
			return
		}
		sendJSON(w, APIResponse{
			Success: true,
			Data:    res.data,
		})
	}
}

func timeFilterFromAPIPreset(preset string) (TimeFilter, error) {
	preset = strings.TrimSpace(preset)
	if preset == "" || preset == string(RangeAll) {
		return TimeFilter{Start: nil, End: nil}, nil
	}
	switch RangePreset(preset) {
	case Range24Hours, Range7Days, Range30Days, Range90Days:
		return NewTimeFilterFromPreset(RangePreset(preset)), nil
	default:
		return TimeFilter{}, fmt.Errorf("不支持的 preset %q，支持 24h|7d|30d|90d|all", preset)
	}
}

// buildDataFromCache 从缓存数据构建 API 响应
func buildDataFromCache(tf TimeFilter, preset string) (*DashboardData, error) {
	startedAt := time.Now()

	type historyResult struct {
		commands []CommandStats
	}
	historyCh := make(chan historyResult, 1)
	go func() {
		cmdStats, _, _ := safeParseHistoryConcurrent(tf)
		historyCh <- historyResult{commands: cmdStats}
	}()

	// 确定查询时间范围
	var start, end time.Time
	if tf.Start != nil {
		start = *tf.Start
	}
	if tf.End != nil {
		end = *tf.End
	}

	// 从缓存查询时间范围数据
	queryStartedAt := time.Now()
	cached := globalCache.QueryByTimeRange(start, end)
	queryDuration := time.Since(queryStartedAt)

	// 构建 RuntimeTools（从缓存中的 RuntimeToolSignals 转换）
	runtimeTools := make([]RuntimeToolSignal, 0, len(cached.RuntimeToolSignals))
	for key, count := range cached.RuntimeToolSignals {
		// key 格式: "server::tool"
		// 需要拆分
		server := "unknown"
		tool := key
		if idx := strings.Index(key, "::"); idx > 0 {
			server = key[:idx]
			tool = key[idx+2:]
		}
		runtimeTools = append(runtimeTools, RuntimeToolSignal{
			Tool:   tool,
			Server: server,
			Count:  count,
		})
	}
	sortRuntimeToolSignals(runtimeTools)

	// 单次遍历 DailyStats：构建每日趋势和 session 峰谷。
	dates := make([]string, 0, len(cached.DailyStats))
	counts := make([]int, 0, len(cached.DailyStats))
	dailySessionMap := make(map[string]int, len(cached.DailyStats))
	peakDate, peakCount := "", 0
	valleyDate, valleyCount := "", 0
	for date, dayStats := range cached.DailyStats {
		dates = append(dates, date)
		counts = append(counts, dayStats.MessageCount)

		sessionCount := dayStats.SessionCount
		dailySessionMap[date] = sessionCount
		if sessionCount > peakCount {
			peakCount = sessionCount
			peakDate = date
		}
		if valleyCount == 0 || sessionCount < valleyCount {
			valleyCount = sessionCount
			valleyDate = date
		}
	}
	sortDatesAndCounts(dates, counts)

	sessionStats := &SessionStats{
		TotalSessions:   cached.TotalSessions,
		PeakDate:        peakDate,
		PeakCount:       peakCount,
		ValleyDate:      valleyDate,
		ValleyCount:     valleyCount,
		DailySessionMap: dailySessionMap,
	}

	// 单次遍历 HourlyStats：同时构建 hourly_counts 和工作时段统计。
	hourlyCountsMap := make(map[string]int, 24)
	hourlyData := make([]HourlyItem, 0, 24)
	workHoursCount := 0
	offHoursCount := 0
	peakHour := -1
	peakHourCount := 0

	for hour := 0; hour < 24; hour++ {
		count := 0
		hasHourlyAggregate := cached.HourlyStats[hour] != nil
		if hasHourlyAggregate {
			count = cached.HourlyStats[hour].MessageCount
		}

		isWorkHour := (hour >= 9 && hour <= 18)
		hourLabel := fmt.Sprintf("%02d:00", hour)

		hourlyData = append(hourlyData, HourlyItem{
			Hour:       hour,
			HourLabel:  hourLabel,
			Count:      count,
			IsWorkHour: isWorkHour,
		})

		if isWorkHour {
			workHoursCount += count
		} else {
			offHoursCount += count
		}

		if hasHourlyAggregate {
			hourlyCountsMap[fmt.Sprintf("%02d", hour)] = count
		}

		if count > peakHourCount {
			peakHourCount = count
			peakHour = hour
		}
	}

	totalCount := workHoursCount + offHoursCount
	workRatio := 0.0
	if totalCount > 0 {
		workRatio = float64(workHoursCount) / float64(totalCount) * 100
	}

	workHoursStats := &WorkHoursStats{
		HourlyData:     hourlyData,
		WorkHoursCount: workHoursCount,
		OffHoursCount:  offHoursCount,
		WorkHoursRatio: workRatio,
		PeakHour:       peakHour,
		PeakHourCount:  peakHourCount,
	}

	projects := make([]ProjectStatItem, 0, len(cached.ProjectStats))
	for _, stats := range cached.ProjectStats {
		projects = append(projects, *stats)
	}
	sortProjectStats(projects)

	weekdayStats := &WeekdayStats{
		WeekdayData: make([]WeekdayItem, 7),
	}
	for i, ws := range cached.WeekdayStats {
		if ws != nil {
			weekdayStats.WeekdayData[i] = *ws
		}
	}

	modelUsage := make([]ModelUsageItem, 0, len(cached.ModelUsage))
	for _, mu := range cached.ModelUsage {
		modelUsage = append(modelUsage, *mu)
	}
	sortModelUsage(modelUsage)

	toolAnalysis := buildToolAnalysisFromCache(cached)
	skillAnalysis := cloneSkillAnalysis(cached.SkillAnalysis)
	eventAnalysis := cloneEventAnalysis(cached.EventAnalysis)
	agentAnalysis := cloneAgentAnalysis(cached.AgentAnalysis)
	commandAnalysis := cloneCommandAnalysis(cached.CommandAnalysis)
	costAnalysis := cloneCostAnalysis(cached.CostAnalysis)
	failureAnalysis := cloneFailureAnalysis(cached.FailureAnalysis)
	sessionAnalysis := cloneSessionAnalysis(cached.SessionAnalysis)
	fileAnalysis := cloneFileAnalysis(cached.FileAnalysis)
	taskPlanAnalysis := cloneTaskPlanAnalysis(cached.TaskPlanAnalysis)
	toolPerformance := cloneToolPerformance(cached.ToolPerformance)

	// 构建时间范围信息
	rangeInfo := TimeRangeInfo{Preset: preset}
	if tf.Start != nil {
		rangeInfo.Start = tf.Start.Format("2006-01-02")
	}
	if tf.End != nil {
		rangeInfo.End = tf.End.Format("2006-01-02")
	}

	cmdStats := (<-historyCh).commands
	data := &DashboardData{
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		TimeRange:    rangeInfo,
		Commands:     cmdStats,
		HourlyCounts: hourlyCountsMap,
		DailyTrend:   DailyTrendData{Dates: dates, Counts: counts},
		RuntimeTools: runtimeTools,
		Sessions:     sessionStats,
		ProjectStats: &ProjectStatsData{
			Projects:      projects,
			TotalMessages: cached.TotalMessages,
			TotalSessions: cached.TotalSessions,
		},
		WeekdayStats:     weekdayStats,
		ModelUsage:       modelUsage,
		WorkHoursStats:   workHoursStats,
		ToolAnalysis:     toolAnalysis,
		SkillAnalysis:    skillAnalysis,
		EventAnalysis:    eventAnalysis,
		AgentAnalysis:    agentAnalysis,
		CommandAnalysis:  commandAnalysis,
		CostAnalysis:     costAnalysis,
		FailureAnalysis:  failureAnalysis,
		SessionAnalysis:  sessionAnalysis,
		FileAnalysis:     fileAnalysis,
		TaskPlanAnalysis: taskPlanAnalysis,
		ToolPerformance:  toolPerformance,
	}
	Debug("缓存数据组装完成",
		"preset", preset,
		"query_duration", queryDuration.Round(time.Millisecond),
		"total_duration", time.Since(startedAt).Round(time.Millisecond),
		"messages", cached.TotalMessages,
	)
	return data, nil
}

// buildDataFromParsing 通过实时解析构建 API 响应（优雅降级版）
// P0: 任何单个数据源失败不会导致整体失败，返回部分数据
func buildDataFromParsing(tf TimeFilter, preset string) (*DashboardData, error) {
	// P1 优化: 三大数据源并行解析（history / projects / debug 独立运行）
	var cmdStats []CommandStats
	var hourlyCountsMap map[string]int
	var aggregate *ProjectAggregate
	var toolStats []RuntimeToolSignal
	var taskAnalysis *TaskAnalysisData

	var wg sync.WaitGroup
	wg.Add(4)

	// 1. history.jsonl 解析（独立）
	go func() {
		defer wg.Done()
		var hc map[string]int
		cmdStats, hc, _ = safeParseHistoryConcurrent(tf)
		hourlyCountsMap = hc
	}()

	// 2. projects/*.jsonl 解析（独立，~22s 瓶颈）
	go func() {
		defer wg.Done()
		aggregate, _ = safeParseProjectsOnce(tf)
	}()

	// 3. debug/*.txt 解析（独立）
	go func() {
		defer wg.Done()
		toolStats, _ = safeParseDebugLogs(tf)
	}()

	// 4. tasks/ 目录扫描（M4: task_plan_analysis）
	go func() {
		defer wg.Done()
		taskAnalysis, _ = safeParseTasksOnce(tf)
	}()

	wg.Wait()

	// M4: 合并 task 分析结果到 aggregate
	if taskAnalysis != nil && aggregate != nil {
		if aggregate.TaskPlanAnalysis == nil {
			aggregate.TaskPlanAnalysis = &TaskPlanAnalysisData{}
		}
		aggregate.TaskPlanAnalysis.Tasks = *taskAnalysis
	}

	// SessionStats 从已解析的 aggregate 中提取（P0，依赖 projects结果）
	sessionStats, _ := extractSessionStatsFromAggregate(aggregate)

	// 从聚合数据中提取每日活动趋势（确保非 nil）
	dates := make([]string, 0)
	counts := make([]int, 0)
	for _, day := range aggregate.DailyActivityList {
		dates = append(dates, day.Date)
		counts = append(counts, day.MessageCount)
	}

	// 将小时数据转换为map格式
	hourlyCountsMap = make(map[string]int)
	for _, item := range aggregate.HourlyData {
		hourKey := fmt.Sprintf("%02d", item.Hour)
		hourlyCountsMap[hourKey] = item.Count
	}

	// 构建时间范围信息
	rangeInfo := TimeRangeInfo{Preset: preset}
	if tf.Start != nil {
		rangeInfo.Start = tf.Start.Format("2006-01-02")
	}
	if tf.End != nil {
		rangeInfo.End = tf.End.Format("2006-01-02")
	}

	// 构建响应数据
	projectStatsData := &ProjectStatsData{
		Projects:      aggregate.Projects,
		TotalMessages: 0,
		TotalSessions: 0,
	}
	for _, proj := range aggregate.Projects {
		projectStatsData.TotalMessages += proj.MessageCount
		projectStatsData.TotalSessions += proj.SessionCount
	}

	return &DashboardData{
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
		TimeRange:        rangeInfo,
		Commands:         cmdStats,
		HourlyCounts:     hourlyCountsMap,
		DailyTrend:       DailyTrendData{Dates: dates, Counts: counts},
		RuntimeTools:     toolStats,
		Sessions:         sessionStats,
		ProjectStats:     projectStatsData,
		WeekdayStats:     aggregate.WeekdayStats,
		ModelUsage:       aggregate.ModelUsageList,
		WorkHoursStats:   aggregate.WorkHoursStats,
		ToolAnalysis:     aggregate.ToolAnalysis,
		SkillAnalysis:    aggregate.SkillAnalysis,
		EventAnalysis:    aggregate.EventAnalysis,
		AgentAnalysis:    aggregate.AgentAnalysis,
		CommandAnalysis:  aggregate.CommandAnalysis,
		CostAnalysis:     aggregate.CostAnalysis,
		FailureAnalysis:  aggregate.FailureAnalysis,
		SessionAnalysis:  aggregate.SessionAnalysis,
		FileAnalysis:     aggregate.FileAnalysis,
		TaskPlanAnalysis: aggregate.TaskPlanAnalysis,
		ToolPerformance:  aggregate.ToolPerformance,
	}, nil
}

func buildToolAnalysisFromCache(cache *CacheFile) *ToolAnalysisData {
	if cache.ToolAnalysis == nil {
		return nil
	}
	analysisCopy := *cache.ToolAnalysis
	analysisCopy.Tools = append([]ToolStatItem(nil), cache.ToolAnalysis.Tools...)
	analysisCopy.ByModel = append([]ToolModelStatItem(nil), cache.ToolAnalysis.ByModel...)
	return &analysisCopy
}

// sortDatesAndCounts 按日期排序日期和计数数组
func sortDatesAndCounts(dates []string, counts []int) {
	items := make([]struct {
		date  string
		count int
	}, 0, len(dates))
	for i, date := range dates {
		count := 0
		if i < len(counts) {
			count = counts[i]
		}
		items = append(items, struct {
			date  string
			count int
		}{date: date, count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].date < items[j].date
	})
	for i, item := range items {
		dates[i] = item.date
		counts[i] = item.count
	}
}

// sortProjectStats 按消息数排序项目统计
func sortProjectStats(projects []ProjectStatItem) {
	sort.SliceStable(projects, func(i, j int) bool {
		if projects[i].MessageCount != projects[j].MessageCount {
			return projects[i].MessageCount > projects[j].MessageCount
		}
		return projects[i].Project < projects[j].Project
	})
}

func sortModelUsage(models []ModelUsageItem) {
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Count != models[j].Count {
			return models[i].Count > models[j].Count
		}
		return models[i].Model < models[j].Model
	})
}

func sortRuntimeToolSignals(tools []RuntimeToolSignal) {
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Count != tools[j].Count {
			return tools[i].Count > tools[j].Count
		}
		return tools[i].Server+"::"+tools[i].Tool < tools[j].Server+"::"+tools[j].Tool
	})
}

func sortToolStats(tools []ToolStatItem) {
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].CallCount != tools[j].CallCount {
			return tools[i].CallCount > tools[j].CallCount
		}
		return tools[i].Tool < tools[j].Tool
	})
}

func maybeValidateDashboardData(source string, data *DashboardData) {
	if !dashboardValidationEnabled() {
		return
	}
	report := buildDashboardConsistencyReport(data)
	if len(report.Issues) > 0 {
		Warn("Dashboard 数据一致性检查发现问题",
			"source", source,
			"issues", strings.Join(report.Issues, "; "),
		)
		return
	}
	Debug("Dashboard 数据一致性检查通过", "source", source)
}

func dashboardValidationEnabled() bool {
	value := strings.TrimSpace(os.Getenv("CC_INSIGHTS_VALIDATE"))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func buildDashboardData(tf TimeFilter, preset string) (*DashboardData, string, error) {
	if globalCache != nil {
		if err := refreshGlobalCacheIfRulesChanged(); err != nil {
			Warn("Bash 规则刷新失败，继续尝试现有缓存", "error", err.Error())
		}
		data, err := buildDataFromCache(tf, preset)
		if err == nil {
			return data, "cache", nil
		}
		Warn("缓存读取失败，降级到实时解析", "error", err.Error())
	}

	data, err := buildDataFromParsing(tf, preset)
	if err != nil {
		return nil, "parsing", err
	}
	return data, "parsing", nil
}

func refreshGlobalCacheIfRulesChanged() error {
	if globalCache == nil {
		return nil
	}
	rulesHash, err := currentBashRulesHash()
	if err != nil {
		return err
	}
	if globalCache.BashRulesHash == rulesHash {
		return nil
	}
	Info("Bash 命令规则已变更，刷新缓存")
	return refreshGlobalCache(false)
}

// sendJSON 发送 JSON 响应
func sendJSON(w http.ResponseWriter, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

// sendError 发送错误响应
func sendError(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	sendJSON(w, APIResponse{
		Success: false,
		Error:   message,
	})
}

// === P0: 优雅降级 - Safe Parse 容错包装函数 ===

// emptyProjectAggregate 返回安全的空 ProjectAggregate
func emptyProjectAggregate() *ProjectAggregate {
	agg := &ProjectAggregate{
		ProjectStats:          make(map[string]*ProjectStatItem),
		DailyActivity:         make(map[string]int),
		DailySessions:         make(map[string]map[string]bool),
		ModelUsage:            make(map[string]*ModelUsageItem),
		ToolStats:             make(map[string]*ToolStatItem),
		ToolModelStats:        make(map[string]*ToolModelStatItem),
		FailureReasons:        make(map[string]*FailureReasonStat),
		FailureToolReasons:    make(map[string]*FailureToolReasonStat),
		FailureModelReasons:   make(map[string]*FailureModelReasonStat),
		SessionStatsMap:       make(map[string]*SessionAnalysisItem),
		SessionQueueOps:       make(map[string]int),
		EventTypes:            make(map[string]int),
		HookStats:             make(map[string]*HookStatItem),
		SkillStats:            make(map[string]*SkillStatItem),
		InstalledSkills:       make(map[string]*InstalledSkillItem),
		SkillUsageStats:       make(map[string]*SkillUsageStat),
		SkillListingStats:     make(map[string]int),
		SkillProjectStats:     make(map[string]*SkillProjectStat),
		SkillModelStats:       make(map[string]*SkillModelStat),
		SkillAgentStats:       make(map[string]*SkillAgentStat),
		SkillSessionToolStats: make(map[string]*SkillSessionToolStat),
		PermissionModes:       make(map[string]int),
		OpenedFiles:           make(map[string]*FileAccessStat),
		AgentStats:            make(map[string]*AgentStatItem),
		AgentSessions:         make(map[string]map[string]bool),
		BashCommandStats:      make(map[string]*BashCommandStat),
		FileOperationStats:    make(map[string]*FileOperationStat),
		FileHotStats:          make(map[string]*FileHotStat),
		FileEditFailures:      make(map[string]*FileEditFailureAgg),
		FileSnapshotStats:     make(map[string]*FileSnapshotAgg),
		FileEditedStats:       make(map[string]*FileEditedAgg),
		ToolPerfStats:         make(map[string]*ToolPerfAgg),
		HourlyCounts:          [24]int{},
	}
	weekdayNames := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	for i := 0; i < 7; i++ {
		agg.WeekdayData[i] = WeekdayItem{Weekday: i, WeekdayName: weekdayNames[i]}
	}
	agg.finalize()
	return agg
}

// safeParseHistoryConcurrent 安全解析 history（容错包装）
func safeParseHistoryConcurrent(tf TimeFilter) ([]CommandStats, map[string]int, error) {
	cmdStats, hourlyCounts, err := ParseHistoryConcurrent(tf)
	if err != nil {
		Warn("ParseHistoryConcurrent 失败，使用空结果", "error", err.Error())
		return []CommandStats{}, make(map[string]int), nil
	}
	return cmdStats, hourlyCounts, nil
}

// safeParseProjectsOnce 安全解析项目数据（容错包装）
func safeParseProjectsOnce(tf TimeFilter) (*ProjectAggregate, error) {
	agg, err := ParseProjectsConcurrentOnce(tf)
	if err != nil {
		Warn("ParseProjectsConcurrentOnce 失败，使用空结果", "error", err.Error())
		return emptyProjectAggregate(), nil
	}
	return agg, nil
}

// safeParseDebugLogs 安全解析 debug 日志（容错包装）
func safeParseDebugLogs(tf TimeFilter) ([]RuntimeToolSignal, error) {
	tools, err := ParseDebugLogsConcurrent(tf)
	if err != nil {
		Warn("ParseDebugLogsConcurrent 失败，使用空结果", "error", err.Error())
		return []RuntimeToolSignal{}, nil
	}
	return tools, nil
}

// safeParseSessionStats 安全解析会话统计（容错包装）
func safeParseSessionStats(tf TimeFilter) (*SessionStats, error) {
	stats, err := ParseSessionStatsWithFilter(tf)
	if err != nil {
		Warn("ParseSessionStatsWithFilter 失败，使用空结果", "error", err.Error())
		return &SessionStats{
			TotalSessions:   0,
			DailySessionMap: make(map[string]int),
		}, nil
	}
	return stats, nil
}

// safeParseTasksOnce 安全解析 tasks/ 目录（容错包装，M4）
func safeParseTasksOnce(tf TimeFilter) (*TaskAnalysisData, error) {
	data, err := ParseTasksConcurrent(tf)
	if err != nil {
		Warn("ParseTasksConcurrent 失败，使用空结果", "error", err.Error())
		return &TaskAnalysisData{}, nil
	}
	return data, nil
}

// extractSessionStatsFromAggregate 从已解析的 ProjectAggregate 中提取会话统计
// P0 优化: 直接从内存中的 aggregate 提取，不重新读取文件
func extractSessionStatsFromAggregate(agg *ProjectAggregate) (*SessionStats, error) {
	if agg == nil || agg.DailySessions == nil {
		return &SessionStats{
			TotalSessions:   0,
			DailySessionMap: make(map[string]int),
		}, nil
	}

	dailyMap := make(map[string]int)
	totalSessions := 0
	peakDate, peakCount := "", 0
	valleyDate, valleyCount := "", 0

	for date, sessions := range agg.DailySessions {
		count := len(sessions)
		dailyMap[date] = count
		totalSessions += count
		if count > peakCount {
			peakCount = count
			peakDate = date
		}
		if valleyCount == 0 || count < valleyCount {
			valleyCount = count
			valleyDate = date
		}
	}

	return &SessionStats{
		TotalSessions:   totalSessions,
		PeakDate:        peakDate,
		PeakCount:       peakCount,
		ValleyDate:      valleyDate,
		ValleyCount:     valleyCount,
		DailySessionMap: dailyMap,
	}, nil
}
