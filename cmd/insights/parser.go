package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// HistoryRecord history.jsonl 记录
type HistoryRecord struct {
	Display        string            `json:"display"`
	PastedContents map[string]string `json:"pastedContents"`
	Timestamp      int64             `json:"timestamp"`
	Project        string            `json:"project"`
}

// DailyActivity 每日活动统计
type DailyActivity struct {
	Date          string `json:"date"`
	MessageCount  int    `json:"messageCount"`
	SessionCount  int    `json:"sessionCount"`
	ToolCallCount int    `json:"toolCallCount"`
}

// StatsCache stats-cache.json 结构
type StatsCache struct {
	DailyActivity    []DailyActivity          `json:"dailyActivity"`
	DailyModelTokens []map[string]interface{} `json:"dailyModelTokens"`
	ModelUsage       map[string]struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	} `json:"modelUsage"`
	HourCounts map[string]int `json:"hourCounts"`
}

// CommandStats 命令统计
type CommandStats struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
}

// ProjectStats 项目统计
type ProjectStats struct {
	Project string
	Count   int
}

// ProjectStatsData 项目统计数据（扩展版）
type ProjectStatsData struct {
	Projects      []ProjectStatItem `json:"projects"`
	TotalMessages int               `json:"total_messages"`
	TotalSessions int               `json:"total_sessions"`
}

// ProjectStatItem 单个项目统计
type ProjectStatItem struct {
	Project      string `json:"project"`
	SessionCount int    `json:"session_count"`
	MessageCount int    `json:"message_count"`
}

// WeekdayStats 星期统计
type WeekdayStats struct {
	WeekdayData []WeekdayItem `json:"weekday_data"`
}

// WeekdayItem 单个星期数据
type WeekdayItem struct {
	Weekday      int    `json:"weekday"`      // 0=周一, 6=周日
	WeekdayName  string `json:"weekday_name"` // "周一"..."周日"
	MessageCount int    `json:"message_count"`
}

// ModelUsageItem 单个模型使用统计
type ModelUsageItem struct {
	Model  string `json:"model"`
	Count  int    `json:"count"`
	Tokens int    `json:"tokens"`
}

// WorkHoursStats 工作时段统计
type WorkHoursStats struct {
	HourlyData     []HourlyItem `json:"hourly_data"` // 每小时数据
	WorkHoursCount int          `json:"work_hours"`  // 工作时段(9-18点)总次数
	OffHoursCount  int          `json:"off_hours"`   // 非工作时段总次数
	WorkHoursRatio float64      `json:"work_ratio"`  // 工作时段占比
	PeakHour       int          `json:"peak_hour"`   // 峰值小时
	PeakHourCount  int          `json:"peak_count"`  // 峰值小时次数
}

// HourlyItem 单小时数据
type HourlyItem struct {
	Hour       int    `json:"hour"`         // 小时(0-23)
	HourLabel  string `json:"hour_label"`   // 标签 "09:00"
	Count      int    `json:"count"`        // 次数
	IsWorkHour bool   `json:"is_work_hour"` // 是否工作时段
}

// ProjectAggregate 一次遍历获取的所有统计数据
type ProjectAggregate struct {
	ProjectStats      map[string]*ProjectStatItem `json:"-"`          // 项目统计（map用于快速查找）
	Projects          []ProjectStatItem           `json:"projects"`   // 项目列表（排序后）
	WeekdayData       [7]WeekdayItem              `json:"-"`          // 星期数据
	WeekdayStats      *WeekdayStats               `json:"weekday"`    // 星期统计（输出格式）
	DailyActivity     map[string]int              `json:"-"`          // 每日活动（map）
	DailyActivityList []DailyActivity             `json:"daily"`      // 每日活动（输出格式）
	HourlyCounts      [24]int                     `json:"-"`          // 小时统计
	HourlyData        []HourlyItem                `json:"-"`          // 小时数据
	ModelUsage        map[string]*ModelUsageItem  `json:"-"`          // 模型使用（map）
	ModelUsageList    []ModelUsageItem            `json:"models"`     // 模型使用（输出格式）
	WorkHoursStats    *WorkHoursStats             `json:"work_hours"` // 工作时段统计
	mu                sync.RWMutex                `json:"-"`          // 保护并发写入
}

// ProjectRecord projects/*.jsonl 记录
type ProjectRecord struct {
	ParentUUID  string          `json:"parentUuid"`
	IsSidechain bool            `json:"isSidechain"`
	UserType    string          `json:"userType"`
	Cwd         string          `json:"cwd"`
	SessionID   string          `json:"sessionId"`
	Version     string          `json:"version"`
	GitBranch   string          `json:"gitBranch"`
	AgentID     string          `json:"agentId"`
	Type        string          `json:"type"`    // "user" | "assistant"
	Message     json.RawMessage `json:"message"` // 可以是 user 或 assistant 消息
	Timestamp   string          `json:"timestamp"`
}

// AssistantMessage assistant 消息详情
type AssistantMessage struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// MCPToolStats MCP工具统计
type MCPToolStats struct {
	Tool   string `json:"tool"`
	Server string `json:"server"`
	Count  int    `json:"count"`
}

// DebugFileInfo debug 文件信息
type DebugFileInfo struct {
	Path    string
	ModTime time.Time
}

var (
	mcpPattern = regexp.MustCompile(`mcp__(\w+)__(\w+)`)
)

// ParseHistoryWithFilter 带时间过滤解析 history.jsonl
func ParseHistoryWithFilter(tf TimeFilter) ([]CommandStats, map[string]int, error) {
	path := GetDataPath("history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 history.jsonl 失败: %w", err)
	}
	defer f.Close()

	cmdCounts := make(map[string]int)
	hourlyCounts := make(map[string]int)

	decoder := json.NewDecoder(f)
	for {
		var record HistoryRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 时间过滤
		recordTime := time.Unix(record.Timestamp/1000, 0)
		if !tf.Contains(recordTime) {
			continue
		}

		// 统计 slash commands
		if strings.HasPrefix(record.Display, "/") {
			parts := strings.Fields(record.Display)
			if len(parts) > 0 {
				cmdCounts[parts[0]]++
			}
		}

		// 统计小时分布
		hour := fmt.Sprintf("%02d", recordTime.Hour())
		hourlyCounts[hour]++
	}

	// 转换为切片并排序
	var cmdStats []CommandStats
	for cmd, count := range cmdCounts {
		cmdStats = append(cmdStats, CommandStats{Command: cmd, Count: count})
	}
	sort.Slice(cmdStats, func(i, j int) bool {
		return cmdStats[i].Count > cmdStats[j].Count
	})

	return cmdStats, hourlyCounts, nil
}

// ParseHistory 解析 history.jsonl（全部数据）
func ParseHistory() ([]CommandStats, map[string]int, error) {
	return ParseHistoryWithFilter(TimeFilter{Start: nil, End: nil})
}

// ParseStatsCacheWithFilter 带时间过滤解析 stats-cache.json
func ParseStatsCacheWithFilter(tf TimeFilter) (*StatsCache, error) {
	cache, err := ParseStatsCache()
	if err != nil {
		return nil, err
	}

	// 过滤每日活动
	cache.DailyActivity = FilterDailyActivity(cache.DailyActivity, tf)

	return cache, nil
}

// ParseStatsCache 解析 stats-cache.json
func ParseStatsCache() (*StatsCache, error) {
	path := GetDataPath("stats-cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 stats-cache.json 失败: %w", err)
	}

	var cache StatsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("解析 stats-cache.json 失败: %w", err)
	}

	return &cache, nil
}

// ParseDebugLogs 解析 debug 日志目录
func ParseDebugLogs() ([]MCPToolStats, error) {
	debugDir := GetDataPath("debug")

	entries, err := os.ReadDir(debugDir)
	if err != nil {
		return nil, fmt.Errorf("读取 debug 目录失败: %w", err)
	}

	// 并发解析
	var wg sync.WaitGroup
	results := make(chan map[string]int, len(entries))
	workers := 8

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			files = append(files, filepath.Join(debugDir, entry.Name()))
		}
	}

	// 分批处理
	batchSize := (len(files) + workers - 1) / workers
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		wg.Add(1)
		go func(files []string) {
			defer wg.Done()
			toolCounts := make(map[string]int)

			for _, file := range files {
				parseDebugFile(file, toolCounts)
			}
			results <- toolCounts
		}(files[i:end])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 汇总结果
	aggregateCounts := make(map[string]int)
	for counts := range results {
		for tool, count := range counts {
			aggregateCounts[tool] += count
		}
	}

	// 转换为切片
	var toolStats []MCPToolStats
	for fullTool, count := range aggregateCounts {
		parts := strings.Split(fullTool, "::")
		if len(parts) == 2 {
			toolStats = append(toolStats, MCPToolStats{
				Tool:   parts[1],
				Server: parts[0],
				Count:  count,
			})
		}
	}
	sort.Slice(toolStats, func(i, j int) bool {
		return toolStats[i].Count > toolStats[j].Count
	})

	return toolStats, nil
}

// ParseDebugLogsWithFilter 带时间过滤解析 debug 日志目录
func ParseDebugLogsWithFilter(tf TimeFilter) ([]MCPToolStats, error) {
	debugDir := GetDataPath("debug")

	entries, err := os.ReadDir(debugDir)
	if err != nil {
		return nil, fmt.Errorf("读取 debug 目录失败: %w", err)
	}

	// 获取文件信息用于时间过滤
	var fileInfos []DebugFileInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			info, _ := entry.Info()
			fileInfos = append(fileInfos, DebugFileInfo{
				Path:    filepath.Join(debugDir, entry.Name()),
				ModTime: info.ModTime(),
			})
		}
	}

	// 时间过滤
	filteredFiles := FilterDebugFiles(fileInfos, tf)

	// 并发解析
	var wg sync.WaitGroup
	results := make(chan map[string]int, len(filteredFiles))
	workers := 8

	files := make([]string, 0, len(filteredFiles))
	for _, info := range filteredFiles {
		files = append(files, info.Path)
	}

	// 分批处理
	batchSize := (len(files) + workers - 1) / workers
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		wg.Add(1)
		go func(files []string) {
			defer wg.Done()
			toolCounts := make(map[string]int)

			for _, file := range files {
				parseDebugFile(file, toolCounts)
			}
			results <- toolCounts
		}(files[i:end])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 汇总结果
	aggregateCounts := make(map[string]int)
	for counts := range results {
		for tool, count := range counts {
			aggregateCounts[tool] += count
		}
	}

	// 转换为切片
	var toolStats []MCPToolStats
	for fullTool, count := range aggregateCounts {
		parts := strings.Split(fullTool, "::")
		if len(parts) == 2 {
			toolStats = append(toolStats, MCPToolStats{
				Tool:   parts[1],
				Server: parts[0],
				Count:  count,
			})
		}
	}
	sort.Slice(toolStats, func(i, j int) bool {
		return toolStats[i].Count > toolStats[j].Count
	})

	return toolStats, nil
}

func parseDebugFile(path string, counts map[string]int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		matches := mcpPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				key := match[1] + "::" + match[2]
				counts[key]++
			}
		}
	}
}

// GetDailyTrend 获取每日趋势（最近7天）
func GetDailyTrend() ([]string, []int, error) {
	cache, err := ParseStatsCache()
	if err != nil {
		return nil, nil, err
	}

	// 取最近7天
	n := len(cache.DailyActivity)
	start := 0
	if n > 7 {
		start = n - 7
	}

	var dates []string
	var counts []int
	for i := start; i < n; i++ {
		dates = append(dates, cache.DailyActivity[i].Date)
		counts = append(counts, cache.DailyActivity[i].MessageCount)
	}

	return dates, counts, nil
}

// SessionStats 会话统计数据
type SessionStats struct {
	TotalSessions   int            `json:"total_sessions"`
	PeakDate        string         `json:"peak_date"`
	PeakCount       int            `json:"peak_count"`
	ValleyDate      string         `json:"valley_date"`
	ValleyCount     int            `json:"valley_count"`
	DailySessionMap map[string]int `json:"daily_session_map"`
}

// buildSessionStatsFromActivity 从 DailyActivity 构建 SessionStats（辅助函数）
func buildSessionStatsFromActivity(activity []DailyActivity) *SessionStats {
	if len(activity) == 0 {
		return &SessionStats{
			DailySessionMap: make(map[string]int),
		}
	}

	// 计算总会话数和每日映射
	totalSessions := 0
	dailySessionMap := make(map[string]int, len(activity))

	for _, day := range activity {
		totalSessions += day.SessionCount
		dailySessionMap[day.Date] = day.SessionCount
	}

	// 找峰值和谷值
	peakDay, valleyDay := activity[0], activity[0]
	for _, day := range activity {
		if day.SessionCount > peakDay.SessionCount {
			peakDay = day
		}
		if day.SessionCount < valleyDay.SessionCount {
			valleyDay = day
		}
	}

	return &SessionStats{
		TotalSessions:   totalSessions,
		PeakDate:        peakDay.Date,
		PeakCount:       peakDay.SessionCount,
		ValleyDate:      valleyDay.Date,
		ValleyCount:     valleyDay.SessionCount,
		DailySessionMap: dailySessionMap,
	}
}

// ParseSessionStats 解析会话统计（全部数据）
func ParseSessionStats() (*SessionStats, error) {
	tf := TimeFilter{Start: nil, End: nil}
	activity, err := ParseDailyActivityFromProjects(tf)
	if err != nil {
		return nil, err
	}
	return buildSessionStatsFromActivity(activity), nil
}

// ParseSessionStatsWithFilter 带时间过滤解析会话统计
func ParseSessionStatsWithFilter(tf TimeFilter) (*SessionStats, error) {
	activity, err := ParseDailyActivityFromProjects(tf)
	if err != nil {
		return nil, err
	}
	return buildSessionStatsFromActivity(activity), nil
}

// GetDailySessionTrend 获取每日会话趋势
func GetDailySessionTrend() ([]string, []int, error) {
	activity, err := ParseDailyActivityFromProjects(TimeFilter{Start: nil, End: nil})
	if err != nil {
		return nil, nil, err
	}

	if len(activity) == 0 {
		return []string{}, []int{}, nil
	}

	dates := make([]string, len(activity))
	counts := make([]int, len(activity))

	for i, day := range activity {
		dates[i] = day.Date
		counts[i] = day.SessionCount
	}

	return dates, counts, nil
}

// ParseProjectStatsWithFilter 带时间过滤解析项目统计
func ParseProjectStatsWithFilter(tf TimeFilter) (*ProjectStatsData, error) {
	projectsDir := GetDataPath("projects")

	// 读取所有项目目录
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	// 统计数据
	projectMap := make(map[string]*ProjectStatItem)
	var totalMessages int
	sessions := make(map[string]bool)

	// 遍历每个项目目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		// 解析该项目的所有 jsonl 文件
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			filePath := filepath.Join(projectDir, file.Name())
			err := parseProjectJSONL(filePath, tf, projectMap, &totalMessages, sessions)
			if err != nil {
				// 记录错误但继续处理其他文件
				continue
			}
		}
	}

	// 转换为切片并排序
	var projects []ProjectStatItem
	for _, proj := range projectMap {
		projects = append(projects, *proj)
	}

	// 按消息数排序
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].MessageCount > projects[j].MessageCount
	})

	return &ProjectStatsData{
		Projects:      projects,
		TotalMessages: totalMessages,
		TotalSessions: len(sessions),
	}, nil
}

// parseProjectJSONL 解析单个项目的 jsonl 文件
func parseProjectJSONL(filePath string, tf TimeFilter, projectMap map[string]*ProjectStatItem, totalMessages *int, sessions map[string]bool) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 解析时间戳
		timestamp, err := time.Parse(time.RFC3339Nano, record.Timestamp)
		if err != nil {
			continue
		}

		// 时间过滤
		if !tf.Contains(timestamp) {
			continue
		}

		// 使用 cwd 作为项目名
		projectName := record.Cwd
		if projectName == "" {
			projectName = "Unknown"
		}

		// 初始化项目统计
		if projectMap[projectName] == nil {
			projectMap[projectName] = &ProjectStatItem{
				Project: projectName,
			}
		}

		// 统计会话
		if record.SessionID != "" {
			sessions[record.SessionID] = true
			if projectMap[projectName].SessionCount == 0 {
				projectMap[projectName].SessionCount = 1
			}
		}

		// 统计消息（只统计 assistant 消息）
		if record.Type == "assistant" {
			projectMap[projectName].MessageCount++
			*totalMessages++
		}
	}

	return nil
}

// ParseProjectStatsByWeekday 解析按星期统计的项目数据
func ParseProjectStatsByWeekday(tf TimeFilter) (*WeekdayStats, error) {
	projectsDir := GetDataPath("projects")

	// 初始化7天的统计数据
	weekdayData := make([]WeekdayItem, 7)
	weekdayNames := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	for i := 0; i < 7; i++ {
		weekdayData[i] = WeekdayItem{
			Weekday:      i,
			WeekdayName:  weekdayNames[i],
			MessageCount: 0,
		}
	}

	// 读取所有项目目录
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	// 遍历每个项目目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		// 解析该项目的所有 jsonl 文件
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			filePath := filepath.Join(projectDir, file.Name())
			err := parseProjectJSONLForWeekday(filePath, tf, weekdayData)
			if err != nil {
				continue
			}
		}
	}

	return &WeekdayStats{
		WeekdayData: weekdayData,
	}, nil
}

// parseProjectJSONLForWeekday 解析文件用于星期统计
func parseProjectJSONLForWeekday(filePath string, tf TimeFilter, weekdayData []WeekdayItem) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 解析时间戳
		timestamp, err := time.Parse(time.RFC3339Nano, record.Timestamp)
		if err != nil {
			continue
		}

		// 时间过滤
		if !tf.Contains(timestamp) {
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		// 获取星期几（0=周一, 6=周日）
		weekday := int(timestamp.Weekday())
		// time.Weekday() 返回 0=周日, 1=周一, ..., 6=周六
		// 转换为 0=周一, ..., 6=周日
		if weekday == 0 {
			weekday = 6 // 周日
		} else {
			weekday-- // 周一到周六
		}

		weekdayData[weekday].MessageCount++
	}

	return nil
}

// ParseDailyActivityFromProjects 从 projects/*.jsonl 生成每日活动数据
func ParseDailyActivityFromProjects(tf TimeFilter) ([]DailyActivity, error) {
	projectsDir := GetDataPath("projects")

	// 使用 map 聚合每日数据
	dailyMap := make(map[string]*DailyActivity)
	sessionsPerDay := make(map[string]map[string]bool)

	// 读取所有项目目录
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	// 遍历每个项目目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		// 解析该项目的所有 jsonl 文件
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			filePath := filepath.Join(projectDir, file.Name())
			err := parseProjectJSONLForDailyActivity(filePath, tf, dailyMap, sessionsPerDay)
			if err != nil {
				continue
			}
		}
	}

	// 转换为切片并排序
	var activity []DailyActivity
	for _, day := range dailyMap {
		activity = append(activity, *day)
	}

	sort.Slice(activity, func(i, j int) bool {
		return activity[i].Date < activity[j].Date
	})

	return activity, nil
}

// parseProjectJSONLForDailyActivity 解析文件用于每日活动统计
func parseProjectJSONLForDailyActivity(filePath string, tf TimeFilter, dailyMap map[string]*DailyActivity, sessionsPerDay map[string]map[string]bool) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 解析时间戳
		timestamp, err := time.Parse(time.RFC3339Nano, record.Timestamp)
		if err != nil {
			continue
		}

		// 时间过滤
		if !tf.Contains(timestamp) {
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		// 提取日期 (YYYY-MM-DD)
		date := timestamp.Format("2006-01-02")

		// 初始化日期统计
		if dailyMap[date] == nil {
			dailyMap[date] = &DailyActivity{
				Date:          date,
				MessageCount:  0,
				SessionCount:  0,
				ToolCallCount: 0,
			}
			sessionsPerDay[date] = make(map[string]bool)
		}

		// 统计消息
		dailyMap[date].MessageCount++

		// 统计会话（首次遇到该 SessionID 时计数）
		if record.SessionID != "" {
			if !sessionsPerDay[date][record.SessionID] {
				dailyMap[date].SessionCount++
				sessionsPerDay[date][record.SessionID] = true
			}
		}
	}

	return nil
}

// ParseHourlyCountsFromProjects 从 projects/*.jsonl 生成小时统计数据
func ParseHourlyCountsFromProjects(tf TimeFilter) (map[string]int, error) {
	projectsDir := GetDataPath("projects")

	// 初始化24小时的统计数据
	hourlyCounts := make(map[string]int)
	for i := 0; i < 24; i++ {
		hourKey := fmt.Sprintf("%02d", i)
		hourlyCounts[hourKey] = 0
	}

	// 读取所有项目目录
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	// 遍历每个项目目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		// 解析该项目的所有 jsonl 文件
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			filePath := filepath.Join(projectDir, file.Name())
			err := parseProjectJSONLForHourly(filePath, tf, hourlyCounts)
			if err != nil {
				continue
			}
		}
	}

	return hourlyCounts, nil
}

// parseProjectJSONLForHourly 解析文件用于小时统计
func parseProjectJSONLForHourly(filePath string, tf TimeFilter, hourlyCounts map[string]int) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 解析时间戳
		timestamp, err := time.Parse(time.RFC3339Nano, record.Timestamp)
		if err != nil {
			continue
		}

		// 时间过滤
		if !tf.Contains(timestamp) {
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		// 获取小时 (00-23)
		hour := timestamp.Hour()
		hourKey := fmt.Sprintf("%02d", hour)
		hourlyCounts[hourKey]++
	}

	return nil
}

// ParseModelUsageFromProjects 从 projects/*.jsonl 解析模型使用统计
func ParseModelUsageFromProjects(tf TimeFilter) ([]ModelUsageItem, error) {
	projectsDir := GetDataPath("projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取projects目录失败: %w", err)
	}

	// 使用map聚合统计数据
	modelStats := make(map[string]*ModelUsageItem)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		subEntries, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, file := range subEntries {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			filePath := filepath.Join(projectDir, file.Name())
			err := parseProjectJSONLForModelUsage(filePath, tf, modelStats)
			if err != nil {
				continue
			}
		}
	}

	// 转换为slice并排序
	result := make([]ModelUsageItem, 0, len(modelStats))
	for _, item := range modelStats {
		result = append(result, *item)
	}

	// 按请求次数降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result, nil
}

// parseProjectJSONLForModelUsage 解析文件用于模型使用统计
func parseProjectJSONLForModelUsage(filePath string, tf TimeFilter, modelStats map[string]*ModelUsageItem) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 解析时间戳
		timestamp, err := time.Parse(time.RFC3339Nano, record.Timestamp)
		if err != nil {
			continue
		}

		// 时间过滤
		if !tf.Contains(timestamp) {
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		// 解析 assistant 消息获取模型和token信息
		var msg AssistantMessage
		if err := json.Unmarshal(record.Message, &msg); err != nil {
			continue
		}

		if msg.Model == "" {
			continue
		}

		// 初始化或更新统计
		if _, exists := modelStats[msg.Model]; !exists {
			modelStats[msg.Model] = &ModelUsageItem{
				Model:  msg.Model,
				Count:  0,
				Tokens: 0,
			}
		}

		modelStats[msg.Model].Count++
		modelStats[msg.Model].Tokens += msg.Usage.InputTokens + msg.Usage.OutputTokens
	}

	return nil
}

// ParseWorkHoursStats 解析工作时段统计
func ParseWorkHoursStats(tf TimeFilter) (*WorkHoursStats, error) {
	// 复用现有的小时统计
	hourlyCounts, err := ParseHourlyCountsFromProjects(tf)
	if err != nil {
		return nil, err
	}

	// 构建小时数据
	hourlyData := make([]HourlyItem, 24)
	for i := 0; i < 24; i++ {
		hourKey := fmt.Sprintf("%02d", i)
		count := hourlyCounts[hourKey]
		hourlyData[i] = HourlyItem{
			Hour:       i,
			HourLabel:  fmt.Sprintf("%02d:00", i),
			Count:      count,
			IsWorkHour: i >= 9 && i <= 18,
		}
	}

	// 计算工作时段统计
	var workHoursCount, offHoursCount int
	var peakHour, peakCount int

	for _, item := range hourlyData {
		if item.IsWorkHour {
			workHoursCount += item.Count
		} else {
			offHoursCount += item.Count
		}

		if item.Count > peakCount {
			peakCount = item.Count
			peakHour = item.Hour
		}
	}

	total := workHoursCount + offHoursCount
	var workRatio float64
	if total > 0 {
		workRatio = float64(workHoursCount) / float64(total) * 100
	}

	return &WorkHoursStats{
		HourlyData:     hourlyData,
		WorkHoursCount: workHoursCount,
		OffHoursCount:  offHoursCount,
		WorkHoursRatio: workRatio,
		PeakHour:       peakHour,
		PeakHourCount:  peakCount,
	}, nil
}

// ParseProjectsConcurrentOnce 一次遍历并发解析所有项目统计
// 这个函数将所有统计合并到一次遍历中，大幅提升性能
func ParseProjectsConcurrentOnce(tf TimeFilter) (*ProjectAggregate, error) {
	projectsDir := GetDataPath("projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	// 初始化聚合数据
	aggregate := &ProjectAggregate{
		ProjectStats:  make(map[string]*ProjectStatItem),
		DailyActivity: make(map[string]int),
		ModelUsage:    make(map[string]*ModelUsageItem),
		HourlyCounts:  [24]int{},
		mu:            sync.RWMutex{},
	}

	// 初始化星期数据
	weekdayNames := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	for i := 0; i < 7; i++ {
		aggregate.WeekdayData[i] = WeekdayItem{
			Weekday:      i,
			WeekdayName:  weekdayNames[i],
			MessageCount: 0,
		}
	}

	// 使用信号量控制并发数（使用所有CPU核心，因为是I/O密集）
	maxWorkers := runtime.NumCPU()
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	// 遍历项目目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		subEntries, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		// 为每个文件启动一个goroutine
		for _, file := range subEntries {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			filePath := filepath.Join(projectDir, file.Name())
			wg.Add(1)
			go func(fp string) {
				defer wg.Done()
				defer func() { <-sem }()

				sem <- struct{}{}
				parseProjectFileAggregate(fp, tf, aggregate)
			}(filePath)
		}
	}

	wg.Wait()

	// 后处理：生成输出格式数据
	aggregate.finalize()

	return aggregate, nil
}

// parseProjectFileAggregate 解析单个项目文件并更新聚合数据
func parseProjectFileAggregate(filePath string, tf TimeFilter, agg *ProjectAggregate) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 解析时间戳
		timestamp, err := time.Parse(time.RFC3339Nano, record.Timestamp)
		if err != nil {
			continue
		}

		// 时间过滤
		if !tf.Contains(timestamp) {
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		// 获取锁保护并发写入
		agg.mu.Lock()

		// 1. 项目统计
		projectName := record.Cwd
		if projectName == "" {
			projectName = "Unknown"
		}
		if agg.ProjectStats[projectName] == nil {
			agg.ProjectStats[projectName] = &ProjectStatItem{
				Project: projectName,
			}
		}
		agg.ProjectStats[projectName].MessageCount++

		// 2. 星期统计
		weekday := int(timestamp.Weekday())  // 0=周日, 1=周一...
		adjustedWeekday := (weekday + 6) % 7 // 转换为0=周一
		agg.WeekdayData[adjustedWeekday].MessageCount++

		// 3. 每日活动
		dateKey := timestamp.Format("2006-01-02")
		agg.DailyActivity[dateKey]++

		// 4. 小时统计
		hour := timestamp.Hour()
		agg.HourlyCounts[hour]++

		// 5. 模型使用统计
		var msg AssistantMessage
		if err := json.Unmarshal(record.Message, &msg); err == nil {
			if msg.Model != "" {
				if agg.ModelUsage[msg.Model] == nil {
					agg.ModelUsage[msg.Model] = &ModelUsageItem{
						Model: msg.Model,
					}
				}
				agg.ModelUsage[msg.Model].Count++
				agg.ModelUsage[msg.Model].Tokens += msg.Usage.InputTokens + msg.Usage.OutputTokens
			}
		}

		agg.mu.Unlock()
	}
}

// finalize 生成输出格式的数据
func (agg *ProjectAggregate) finalize() {
	// 1. 转换项目列表并排序
	agg.Projects = make([]ProjectStatItem, 0, len(agg.ProjectStats))
	for _, proj := range agg.ProjectStats {
		agg.Projects = append(agg.Projects, *proj)
	}
	sort.Slice(agg.Projects, func(i, j int) bool {
		return agg.Projects[i].MessageCount > agg.Projects[j].MessageCount
	})

	// 2. 转换星期统计
	weekdayData := make([]WeekdayItem, 7)
	copy(weekdayData, agg.WeekdayData[:])
	agg.WeekdayStats = &WeekdayStats{WeekdayData: weekdayData}

	// 3. 转换每日活动为列表
	dates := make([]string, 0, len(agg.DailyActivity))
	for date := range agg.DailyActivity {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	agg.DailyActivityList = make([]DailyActivity, len(dates))
	for i, date := range dates {
		agg.DailyActivityList[i] = DailyActivity{
			Date:         date,
			MessageCount: agg.DailyActivity[date],
		}
	}

	// 4. 转换小时数据
	agg.HourlyData = make([]HourlyItem, 24)
	for i := 0; i < 24; i++ {
		agg.HourlyData[i] = HourlyItem{
			Hour:       i,
			HourLabel:  fmt.Sprintf("%02d:00", i),
			Count:      agg.HourlyCounts[i],
			IsWorkHour: i >= 9 && i <= 18,
		}
	}

	// 5. 转换模型使用列表
	agg.ModelUsageList = make([]ModelUsageItem, 0, len(agg.ModelUsage))
	for _, model := range agg.ModelUsage {
		agg.ModelUsageList = append(agg.ModelUsageList, *model)
	}
	sort.Slice(agg.ModelUsageList, func(i, j int) bool {
		return agg.ModelUsageList[i].Count > agg.ModelUsageList[j].Count
	})

	// 6. 生成工作时段统计
	var workHoursCount, offHoursCount int
	var peakHour, peakCount int

	for _, item := range agg.HourlyData {
		if item.IsWorkHour {
			workHoursCount += item.Count
		} else {
			offHoursCount += item.Count
		}

		if item.Count > peakCount {
			peakCount = item.Count
			peakHour = item.Hour
		}
	}

	total := workHoursCount + offHoursCount
	var workRatio float64
	if total > 0 {
		workRatio = float64(workHoursCount) / float64(total) * 100
	}

	agg.WorkHoursStats = &WorkHoursStats{
		HourlyData:     agg.HourlyData,
		WorkHoursCount: workHoursCount,
		OffHoursCount:  offHoursCount,
		WorkHoursRatio: workRatio,
		PeakHour:       peakHour,
		PeakHourCount:  peakCount,
	}
}
