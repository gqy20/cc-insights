package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

// DashboardData Dashboard 数据
type DashboardData struct {
	Timestamp      string            `json:"timestamp"`
	TimeRange      TimeRangeInfo     `json:"time_range"`
	Commands       []CommandStats    `json:"commands"`
	HourlyCounts   map[string]int    `json:"hourly_counts"`
	DailyTrend     DailyTrendData    `json:"daily_trend"`
	MCPTools       []MCPToolStats    `json:"mcp_tools"`
	Sessions       *SessionStats     `json:"sessions"`
	ProjectStats   *ProjectStatsData `json:"project_stats,omitempty"`
	WeekdayStats   *WeekdayStats     `json:"weekday_stats,omitempty"`
	ModelUsage     []ModelUsageItem  `json:"model_usage,omitempty"`
	WorkHoursStats *WorkHoursStats   `json:"work_hours_stats,omitempty"`
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

	// 解析参数
	preset := r.URL.Query().Get("preset")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	// 创建时间过滤器
	var tf TimeFilter
	var err error

	if startStr != "" && endStr != "" {
		// 自定义时间范围
		tf, err = NewTimeFilterCustom(startStr, endStr)
		if err != nil {
			sendError(w, "无效的时间格式: "+err.Error())
			return
		}
	} else if preset != "" {
		// 预设时间范围
		tf = NewTimeFilterFromPreset(RangePreset(preset))
	} else {
		// 默认全部数据
		tf = TimeFilter{Start: nil, End: nil}
	}

	// 使用 channel + select 实现超时控制
	type result struct {
		data *DashboardData
		err  error
	}
	resultCh := make(chan result, 1)

	go func() {
		var data *DashboardData

		// 优先使用缓存
		if globalCache != nil {
			data, err = buildDataFromCache(tf, preset)
			if err != nil {
				fmt.Fprintf(os.Stderr, "缓存读取失败，降级到实时解析: %v\n", err)
				data, err = buildDataFromParsing(tf, preset)
			}
		} else {
			data, err = buildDataFromParsing(tf, preset)
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

// buildDataFromCache 从缓存数据构建 API 响应
func buildDataFromCache(tf TimeFilter, preset string) (*DashboardData, error) {
	// 确定查询时间范围
	var start, end time.Time
	if tf.Start != nil {
		start = *tf.Start
	}
	if tf.End != nil {
		end = *tf.End
	}

	// 从缓存查询时间范围数据
	cached := globalCache.QueryByTimeRange(start, end)

	// 构建 CommandStats（缓存中没有，需要实时解析，使用安全包装）
	cmdStats, _, _ := safeParseHistoryConcurrent(tf)

	// 构建 MCPTools（从缓存中的 MCPToolStats 转换）
	mcpTools := make([]MCPToolStats, 0, len(cached.MCPToolStats))
	for key, count := range cached.MCPToolStats {
		// key 格式: "server::tool"
		// 需要拆分
		server := "unknown"
		tool := key
		if idx := strings.Index(key, "::"); idx > 0 {
			server = key[:idx]
			tool = key[idx+2:]
		}
		mcpTools = append(mcpTools, MCPToolStats{
			Tool:   tool,
			Server: server,
			Count:  count,
		})
	}

	// 构建每日趋势
	var dates []string
	var counts []int
	for date := range cached.DailyStats {
		dates = append(dates, date)
		counts = append(counts, cached.DailyStats[date].MessageCount)
	}
	// 按日期排序
	sortDatesAndCounts(dates, counts, cached.DailyStats)

	// 构建小时统计
	hourlyCountsMap := make(map[string]int)
	for hour, aggregate := range cached.HourlyStats {
		if aggregate != nil {
			hourKey := fmt.Sprintf("%02d", hour)
			hourlyCountsMap[hourKey] = aggregate.MessageCount
		}
	}

	// 构建项目统计
	projects := make([]ProjectStatItem, 0, len(cached.ProjectStats))
	for _, stats := range cached.ProjectStats {
		projects = append(projects, *stats)
	}
	// 按消息数排序
	sortProjectStats(projects)

	// 构建星期统计
	weekdayStats := &WeekdayStats{
		WeekdayData: make([]WeekdayItem, 7),
	}
	for i, ws := range cached.WeekdayStats {
		if ws != nil {
			weekdayStats.WeekdayData[i] = *ws
		}
	}

	// 构建模型使用统计
	modelUsage := make([]ModelUsageItem, 0, len(cached.ModelUsage))
	for _, mu := range cached.ModelUsage {
		modelUsage = append(modelUsage, *mu)
	}

	// 构建会话统计（从 DailyStats 构建）
	dailySessionMap := make(map[string]int)
	for date, dayStats := range cached.DailyStats {
		dailySessionMap[date] = dayStats.SessionCount
	}

	// 找出峰值和谷值
	peakDate, peakCount := "", 0
	valleyDate, valleyCount := "", 0
	for date, count := range dailySessionMap {
		if count > peakCount {
			peakCount = count
			peakDate = date
		}
		if valleyCount == 0 || count < valleyCount {
			valleyCount = count
			valleyDate = date
		}
	}

	sessionStats := &SessionStats{
		TotalSessions:   cached.TotalSessions,
		PeakDate:        peakDate,
		PeakCount:       peakCount,
		ValleyDate:      valleyDate,
		ValleyCount:     valleyCount,
		DailySessionMap: dailySessionMap,
	}

	// 构建工作时段统计（从 HourlyStats 计算）
	hourlyData := make([]HourlyItem, 0, 24)
	workHoursCount := 0
	offHoursCount := 0
	peakHour := -1
	peakHourCount := 0

	for hour := 0; hour < 24; hour++ {
		count := 0
		if cached.HourlyStats[hour] != nil {
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

	// 构建时间范围信息
	rangeInfo := TimeRangeInfo{Preset: preset}
	if tf.Start != nil {
		rangeInfo.Start = tf.Start.Format("2006-01-02")
	}
	if tf.End != nil {
		rangeInfo.End = tf.End.Format("2006-01-02")
	}

	return &DashboardData{
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		TimeRange:    rangeInfo,
		Commands:     cmdStats,
		HourlyCounts: hourlyCountsMap,
		DailyTrend:   DailyTrendData{Dates: dates, Counts: counts},
		MCPTools:     mcpTools,
		Sessions:     sessionStats,
		ProjectStats: &ProjectStatsData{
			Projects:      projects,
			TotalMessages: cached.TotalMessages,
			TotalSessions: cached.TotalSessions,
		},
		WeekdayStats:   weekdayStats,
		ModelUsage:     modelUsage,
		WorkHoursStats: workHoursStats,
	}, nil
}

// buildDataFromParsing 通过实时解析构建 API 响应（优雅降级版）
// P0: 任何单个数据源失败不会导致整体失败，返回部分数据
func buildDataFromParsing(tf TimeFilter, preset string) (*DashboardData, error) {
	// P1 优化: 三大数据源并行解析（history / projects / debug 独立运行）
	var cmdStats []CommandStats
	var hourlyCountsMap map[string]int
	var aggregate *ProjectAggregate
	var toolStats []MCPToolStats

	var wg sync.WaitGroup
	wg.Add(3)

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

	wg.Wait()

	// SessionStats 从已解析的 aggregate 中提取（P0，依赖 projects 结果）
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
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
		TimeRange:      rangeInfo,
		Commands:       cmdStats,
		HourlyCounts:   hourlyCountsMap,
		DailyTrend:     DailyTrendData{Dates: dates, Counts: counts},
		MCPTools:       toolStats,
		Sessions:       sessionStats,
		ProjectStats:   projectStatsData,
		WeekdayStats:   aggregate.WeekdayStats,
		ModelUsage:     aggregate.ModelUsageList,
		WorkHoursStats: aggregate.WorkHoursStats,
	}, nil
}

// sortDatesAndCounts 按日期排序日期和计数数组
func sortDatesAndCounts(dates []string, counts []int, dailyStats map[string]*DayAggregate) {
	// 简单冒泡排序
	n := len(dates)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if dates[j] > dates[j+1] {
				dates[j], dates[j+1] = dates[j+1], dates[j]
				counts[j], counts[j+1] = counts[j+1], counts[j]
			}
		}
	}
}

// sortProjectStats 按消息数排序项目统计
func sortProjectStats(projects []ProjectStatItem) {
	n := len(projects)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if projects[j].MessageCount < projects[j+1].MessageCount {
				projects[j], projects[j+1] = projects[j+1], projects[j]
			}
		}
	}
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
		ProjectStats:  make(map[string]*ProjectStatItem),
		DailyActivity: make(map[string]int),
		ModelUsage:    make(map[string]*ModelUsageItem),
		HourlyCounts:  [24]int{},
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
		fmt.Fprintf(os.Stderr, "[降级] ParseHistoryConcurrent 失败(使用空结果): %v\n", err)
		return []CommandStats{}, make(map[string]int), nil
	}
	return cmdStats, hourlyCounts, nil
}

// safeParseProjectsOnce 安全解析项目数据（容错包装）
func safeParseProjectsOnce(tf TimeFilter) (*ProjectAggregate, error) {
	agg, err := ParseProjectsConcurrentOnce(tf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[降级] ParseProjectsConcurrentOnce 失败(使用空结果): %v\n", err)
		return emptyProjectAggregate(), nil
	}
	return agg, nil
}

// safeParseDebugLogs 安全解析 debug 日志（容错包装）
func safeParseDebugLogs(tf TimeFilter) ([]MCPToolStats, error) {
	tools, err := ParseDebugLogsConcurrent(tf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[降级] ParseDebugLogsConcurrent 失败(使用空结果): %v\n", err)
		return []MCPToolStats{}, nil
	}
	return tools, nil
}

// safeParseSessionStats 安全解析会话统计（容错包装）
func safeParseSessionStats(tf TimeFilter) (*SessionStats, error) {
	stats, err := ParseSessionStatsWithFilter(tf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[降级] ParseSessionStatsWithFilter 失败(使用空结果): %v\n", err)
		return &SessionStats{
			TotalSessions:   0,
			DailySessionMap: make(map[string]int),
		}, nil
	}
	return stats, nil
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
