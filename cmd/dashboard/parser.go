package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	cache, err := ParseStatsCache()
	if err != nil {
		return nil, err
	}
	return buildSessionStatsFromActivity(cache.DailyActivity), nil
}

// ParseSessionStatsWithFilter 带时间过滤解析会话统计
func ParseSessionStatsWithFilter(tf TimeFilter) (*SessionStats, error) {
	cache, err := ParseStatsCacheWithFilter(tf)
	if err != nil {
		return nil, err
	}
	return buildSessionStatsFromActivity(cache.DailyActivity), nil
}

// GetDailySessionTrend 获取每日会话趋势
func GetDailySessionTrend() ([]string, []int, error) {
	cache, err := ParseStatsCache()
	if err != nil {
		return nil, nil, err
	}

	if len(cache.DailyActivity) == 0 {
		return []string{}, []int{}, nil
	}

	dates := make([]string, len(cache.DailyActivity))
	counts := make([]int, len(cache.DailyActivity))

	for i, day := range cache.DailyActivity {
		dates[i] = day.Date
		counts[i] = day.SessionCount
	}

	return dates, counts, nil
}
