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
	ProjectStats      map[string]*ProjectStatItem       `json:"-"`          // 项目统计（map用于快速查找）
	Projects          []ProjectStatItem                `json:"projects"`   // 项目列表（排序后）
	WeekdayData       [7]WeekdayItem                   `json:"-"`          // 星期数据
	WeekdayStats      *WeekdayStats                    `json:"weekday"`    // 星期统计（输出格式）
	DailyActivity     map[string]int                    `json:"-"`          // 每日消息数（map）
	DailyActivityList []DailyActivity                  `json:"daily"`      // 每日活动（输出格式）
	DailySessions     map[string]map[string]bool         `json:"-"`          // 每日会话集 date→sessionID→true（用于提取SessionStats，避免重复解析）
	HourlyCounts      [24]int                          `json:"-"`          // 小时统计
	HourlyData        []HourlyItem                     `json:"-"`          // 小时数据
	ModelUsage        map[string]*ModelUsageItem         `json:"-"`          // 模型使用（map）
	ModelUsageList    []ModelUsageItem                 `json:"models"`     // 模型使用（输出格式）
	WorkHoursStats    *WorkHoursStats                  `json:"work_hours"` // 工作时段统计
	mu                sync.RWMutex                     `json:"-"`          // 保护并发写入
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
	Content []AssistantContent `json:"content"`
	Usage struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// AssistantContent 支持多种内容类型（text, thinking）
type AssistantContent struct {
	Type     string `json:"type"`              // "text" | "thinking"
	Text     string `json:"text"`              // text 类型内容
	Thinking string `json:"thinking,omitempty"` // thinking 类型内容
}

// SessionIndexEntry sessions-index.json 单个条目
type SessionIndexEntry struct {
	SessionID   string `json:"sessionId"`
	FullPath    string `json:"fullPath"`
	FileMtime   int64  `json:"fileMtime"`
	FirstPrompt string `json:"firstPrompt"`
	Summary     string `json:"summary"`
	MessageCount int   `json:"messageCount"`
	Created     string `json:"created"`
	Modified    string `json:"modified"`
	ProjectPath string `json:"projectPath"`
	IsSidechain bool   `json:"isSidechain"`
}

// SessionIndexResult sessions-index.json 解析结果
type SessionIndexResult struct {
	Version int                  `json:"version"`
	Entries []SessionIndexEntry  `json:"entries"`
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

// ParseSessionIndex 解析 sessions-index.json 文件
// 返回会话索引数据，可用于更准确的会话统计
func ParseSessionIndex(projectPath string) (*SessionIndexResult, error) {
	if projectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}

	indexPath := filepath.Join(projectPath, "sessions-index.json")
	f, err := os.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("open sessions-index.json: %w", err)
	}
	defer f.Close()

	var result SessionIndexResult
	if err := json.NewDecoder(f).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sessions-index.json: %w", err)
	}

	return &result, nil
}

// ParseSessionStatsWithFilter 带时间过滤解析会话统计
// 使用 ParseProjectsConcurrentOnce 一次遍历获取所有数据，然后从 aggregate 提取 session 统计
// 避免冗余的二次文件遍历
func ParseSessionStatsWithFilter(tf TimeFilter) (*SessionStats, error) {
	agg, err := ParseProjectsConcurrentOnce(tf)
	if err != nil {
		return nil, err
	}
	return extractSessionStatsFromAggregate(agg)
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
		DailySessions: make(map[string]map[string]bool),
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

		// 3.5 每日会话去重（同一 sessionID 同天只计一次）
		if record.SessionID != "" {
			if agg.DailySessions[dateKey] == nil {
				agg.DailySessions[dateKey] = make(map[string]bool)
			}
			if !agg.DailySessions[dateKey][record.SessionID] {
				agg.DailySessions[dateKey][record.SessionID] = true
			}
		}

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
