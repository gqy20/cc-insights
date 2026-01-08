package main

import (
	"encoding/json"
	"net/http"
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

	// 获取数据（使用优化的并发版本）
	cmdStats, _, err := ParseHistoryConcurrent(tf)
	if err != nil {
		sendError(w, "解析历史数据失败: "+err.Error())
		return
	}

	// 获取每日活动数据（从 projects/*.jsonl）
	dailyActivity, err := ParseDailyActivityFromProjects(tf)
	if err != nil {
		sendError(w, "解析每日活动失败: "+err.Error())
		return
	}

	// 获取小时统计（从 projects/*.jsonl）
	hourlyCountsFromProjects, err := ParseHourlyCountsFromProjects(tf)
	if err != nil {
		sendError(w, "解析小时统计失败: "+err.Error())
		return
	}

	toolStats, err := ParseDebugLogsConcurrent(tf)
	if err != nil {
		sendError(w, "解析 debug 日志失败: "+err.Error())
		return
	}

	// 获取会话统计
	sessionStats, err := ParseSessionStatsWithFilter(tf)
	if err != nil {
		sendError(w, "解析会话统计失败: "+err.Error())
		return
	}

	// 获取项目统计
	projectStats, err := ParseProjectStatsWithFilter(tf)
	if err != nil {
		sendError(w, "解析项目统计失败: "+err.Error())
		return
	}

	// 获取星期统计
	weekdayStats, err := ParseProjectStatsByWeekday(tf)
	if err != nil {
		sendError(w, "解析星期统计失败: "+err.Error())
		return
	}

	// 获取模型使用统计
	modelUsage, err := ParseModelUsageFromProjects(tf)
	if err != nil {
		sendError(w, "解析模型使用失败: "+err.Error())
		return
	}

	// 获取工作时段统计
	workHoursStats, err := ParseWorkHoursStats(tf)
	if err != nil {
		sendError(w, "解析工作时段统计失败: "+err.Error())
		return
	}

	// 构建每日趋势
	var dates []string
	var counts []int
	for _, day := range dailyActivity {
		dates = append(dates, day.Date)
		counts = append(counts, day.MessageCount)
	}

	// 构建时间范围信息
	rangeInfo := TimeRangeInfo{Preset: preset}
	if tf.Start != nil {
		rangeInfo.Start = tf.Start.Format("2006-01-02")
	}
	if tf.End != nil {
		rangeInfo.End = tf.End.Format("2006-01-02")
	}

	// 构建响应
	data := DashboardData{
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		TimeRange:    rangeInfo,
		Commands:     cmdStats,
		HourlyCounts: hourlyCountsFromProjects,
		DailyTrend: DailyTrendData{
			Dates:  dates,
			Counts: counts,
		},
		MCPTools:       toolStats,
		Sessions:       sessionStats,
		ProjectStats:   projectStats,
		WeekdayStats:   weekdayStats,
		ModelUsage:     modelUsage,
		WorkHoursStats: workHoursStats,
	}

	sendJSON(w, APIResponse{
		Success: true,
		Data:    data,
	})
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
