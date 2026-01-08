package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

	var data *DashboardData

	// 优先使用缓存
	if globalCache != nil {
		data, err = buildDataFromCache(tf, preset)
		if err != nil {
			// 缓存读取失败，降级到实时解析
			fmt.Fprintf(os.Stderr, "缓存读取失败，降级到实时解析: %v\n", err)
			data, err = buildDataFromParsing(tf, preset)
			if err != nil {
				sendError(w, err.Error())
				return
			}
		}
	} else {
		// 无缓存，使用实时解析
		data, err = buildDataFromParsing(tf, preset)
		if err != nil {
			sendError(w, err.Error())
			return
		}
	}

	sendJSON(w, APIResponse{
		Success: true,
		Data:    data,
	})
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

	// 构建 CommandStats（缓存中没有，需要实时解析）
	cmdStats, _, err := ParseHistoryConcurrent(tf)
	if err != nil {
		return nil, fmt.Errorf("解析命令统计失败: %w", err)
	}

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

// buildDataFromParsing 通过实时解析构建 API 响应
func buildDataFromParsing(tf TimeFilter, preset string) (*DashboardData, error) {
	// 获取命令统计
	cmdStats, _, err := ParseHistoryConcurrent(tf)
	if err != nil {
		return nil, fmt.Errorf("解析历史数据失败: %w", err)
	}

	// 使用一次遍历获取所有项目相关统计
	aggregate, err := ParseProjectsConcurrentOnce(tf)
	if err != nil {
		return nil, fmt.Errorf("解析项目数据失败: %w", err)
	}

	// 获取debug日志统计
	toolStats, err := ParseDebugLogsConcurrent(tf)
	if err != nil {
		return nil, fmt.Errorf("解析 debug 日志失败: %w", err)
	}

	// 获取会话统计
	sessionStats, err := ParseSessionStatsWithFilter(tf)
	if err != nil {
		return nil, fmt.Errorf("解析会话统计失败: %w", err)
	}

	// 从聚合数据中提取每日活动趋势
	var dates []string
	var counts []int
	for _, day := range aggregate.DailyActivityList {
		dates = append(dates, day.Date)
		counts = append(counts, day.MessageCount)
	}

	// 将小时数据转换为map格式
	hourlyCountsMap := make(map[string]int)
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
