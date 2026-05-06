package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// CacheBuilder 缓存构建器
type CacheBuilder struct {
	CachePath string // 缓存文件路径
	DataDir   string // 数据目录路径
}

// BuildFullCache 构建完整缓存
func (cb *CacheBuilder) BuildFullCache() error {
	fmt.Println("🔨 开始构建完整缓存...")

	// 创建时间过滤器（全部数据）
	tf := TimeFilter{Start: nil, End: nil}

	// 使用一次遍历获取所有统计数据
	aggregate, err := ParseProjectsConcurrentOnce(tf)
	if err != nil {
		return fmt.Errorf("解析项目数据失败: %w", err)
	}

	// 解析 debug 日志获取 MCP 工具统计
	mcpStats, err := ParseDebugLogsConcurrent(tf)
	if err != nil {
		return fmt.Errorf("解析 debug 日志失败: %w", err)
	}

	// 从已解析的 aggregate 中提取会话统计（无需重新遍历 projects 文件）
	sessionStats, _ := extractSessionStatsFromAggregate(aggregate)

	// 计算总消息数（从每日活动汇总）
	totalMessages := 0
	for _, count := range aggregate.DailyActivity {
		totalMessages += count
	}

	// 创建缓存结构
	cache := &CacheFile{
		Version:       "1.0",
		LastUpdate:    time.Now(),
		TimeRange:     TimeRange{},
		DailyStats:    make(map[string]*DayAggregate),
		TotalMessages: totalMessages,
		TotalSessions: sessionStats.TotalSessions,
		ProjectStats:  make(map[string]*ProjectStatItem),
		ModelUsage:    make(map[string]*ModelUsageItem),
		MCPToolStats:  make(map[string]int),
	}
	// 填充 HourlyStats
	for i := 0; i < 24; i++ {
		count := aggregate.HourlyCounts[i]
		if count > 0 {
			cache.HourlyStats[i] = &HourAggregate{
				Hour:         i,
				MessageCount: count,
				SessionCount: 0,
			}
		}
	}

	// 填充 WeekdayStats
	for i := 0; i < 7; i++ {
		cache.WeekdayStats[i] = &aggregate.WeekdayStats.WeekdayData[i]
	}

	// 填充 DailyStats
	for _, day := range aggregate.DailyActivityList {
		sessionCount := 0
		if sessionStats != nil && sessionStats.DailySessionMap != nil {
			sessionCount = sessionStats.DailySessionMap[day.Date]
		}
		cache.DailyStats[day.Date] = &DayAggregate{
			Date:          day.Date,
			MessageCount:  day.MessageCount,
			SessionCount:  sessionCount,
			ToolCallCount: 0,
			HourlyCounts:  [24]int{},
		}
	}

	// 填充 ProjectStats（直接使用 map，已去重）
	for _, proj := range aggregate.ProjectStats {
		cache.ProjectStats[proj.Project] = proj
	}

	// 填充 ModelUsage（直接使用 map，已去重）
	for _, mu := range aggregate.ModelUsage {
		cache.ModelUsage[mu.Model] = mu
	}

	// 填充 MCPToolStats
	for _, tool := range mcpStats {
		key := tool.Server + "::" + tool.Tool
		cache.MCPToolStats[key] = tool.Count
	}

	// 4. 保存缓存
	if err := cache.Save(cb.CachePath); err != nil {
		return fmt.Errorf("保存缓存失败: %w", err)
	}

	fmt.Printf("✅ 缓存构建完成！共 %d 条消息，%d 个会话\n", cache.TotalMessages, cache.TotalSessions)
	return nil
}

// IncrementalUpdate 增量更新缓存
func (cb *CacheBuilder) IncrementalUpdate() error {
	// 1. 加载现有缓存
	cache, err := LoadCacheFile(cb.CachePath)
	if err != nil {
		// 缓存不存在，构建完整缓存
		fmt.Println("📝 缓存不存在，开始构建完整缓存...")
		return cb.BuildFullCache()
	}

	fmt.Printf("🔄 检查增量更新（缓存时间: %s）...\n", cache.LastUpdate.Format("2006-01-02 15:04:05"))

	// 2. 检查是否有新数据
	lastDataMod, err := cb.GetLastDataModified()
	if err != nil {
		return fmt.Errorf("获取数据修改时间失败: %w", err)
	}

	if !lastDataMod.After(cache.LastUpdate) {
		fmt.Println("✅ 缓存已是最新，无需更新")
		return nil
	}

	// 3. 增量解析新数据
	// 重新解析（简化实现：完整重建）
	// TODO: 实现真正的增量解析
	fmt.Println("🔄 数据已更新，重新构建缓存...")
	return cb.BuildFullCache()
}

// NeedsRebuild 检查是否需要重建缓存
func (cb *CacheBuilder) NeedsRebuild() bool {
	// 检查缓存文件是否存在
	if _, err := os.Stat(cb.CachePath); os.IsNotExist(err) {
		return true // 缓存不存在，需要重建
	}

	// 加载缓存
	cache, err := LoadCacheFile(cb.CachePath)
	if err != nil {
		return true // 缓存损坏，需要重建
	}

	// 获取数据最后修改时间
	lastDataMod, err := cb.GetLastDataModified()
	if err != nil {
		return true // 无法获取修改时间，保守重建
	}

	// 如果数据文件比缓存新，需要重建
	return lastDataMod.After(cache.LastUpdate)
}

// GetLastDataModified 获取数据目录中所有文件的最后修改时间
func (cb *CacheBuilder) GetLastDataModified() (time.Time, error) {
	var lastMod time.Time
	var visitedDirs []string

	// 需要检查的文件列表
	files := []string{
		"history.jsonl",
		"stats-cache.json",
	}

	// 遍历文件
	for _, file := range files {
		path := filepath.Join(cb.DataDir, file)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // 文件不存在，跳过
			}
			return time.Time{}, err
		}

		if info.ModTime().After(lastMod) {
			lastMod = info.ModTime()
		}
	}

	// 递归检查所有子目录
	dirs := []string{"debug", "projects"}
	for _, dirName := range dirs {
		dirPath := filepath.Join(cb.DataDir, dirName)
		visitedDirs = append(visitedDirs, dirPath)
		if err := cb.scanDirectory(dirPath, &lastMod, &visitedDirs); err != nil {
			// 目录不存在不是错误
			if !os.IsNotExist(err) {
				return time.Time{}, err
			}
		}
	}

	return lastMod, nil
}

// scanDirectory 递归扫描目录获取最后修改时间
func (cb *CacheBuilder) scanDirectory(dirPath string, lastMod *time.Time, visitedDirs *[]string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			// 递归扫描子目录
			*visitedDirs = append(*visitedDirs, fullPath)
			if err := cb.scanDirectory(fullPath, lastMod, visitedDirs); err != nil {
				return err
			}
		} else {
			// 检查文件修改时间
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().After(*lastMod) {
				*lastMod = info.ModTime()
			}
		}
	}

	return nil
}

// buildFromHistory 从 history.jsonl 构建缓存
func (cb *CacheBuilder) buildFromHistory(cache *CacheFile) error {
	path := filepath.Join(cb.DataDir, "history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在不是错误
		}
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record HistoryRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 解析时间戳
		timestamp := time.Unix(record.Timestamp/1000, 0)
		dateKey := timestamp.Format("2006-01-02")
		hour := timestamp.Hour()

		// 获取或创建每日统计
		if cache.DailyStats[dateKey] == nil {
			cache.DailyStats[dateKey] = &DayAggregate{
				Date:          dateKey,
				ProjectCounts: make(map[string]int),
				ModelCounts:   make(map[string]int),
			}
		}

		// 添加消息
		cache.DailyStats[dateKey].AddMessage(record.Project, hour)
		cache.TotalMessages++
	}

	return nil
}

// buildFromProjects 从 projects/*.jsonl 构建缓存
func (cb *CacheBuilder) buildFromProjects(cache *CacheFile) error {
	projectsDir := filepath.Join(cb.DataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在不是错误
		}
		return err
	}

	// 统计会话数
	sessions := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if !file.IsDir() && filepath.Ext(file.Name()) == ".jsonl" {
				filePath := filepath.Join(projectDir, file.Name())
				if err := cb.parseProjectFile(filePath, cache, sessions); err != nil {
					// 记录错误但继续处理其他文件
					continue
				}
			}
		}
	}

	cache.TotalSessions = len(sessions)
	return nil
}

// buildFromDebugLogs 从 debug 日志构建缓存
func (cb *CacheBuilder) buildFromDebugLogs(cache *CacheFile) error {
	debugDir := filepath.Join(cb.DataDir, "debug")
	entries, err := os.ReadDir(debugDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在不是错误
		}
		return err
	}

	if cache.MCPToolStats == nil {
		cache.MCPToolStats = make(map[string]int)
	}

	// 遍历 debug 日志文件
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matched, _ := filepath.Match("*.txt", entry.Name())
		if !matched {
			continue
		}

		filePath := filepath.Join(debugDir, entry.Name())
		if err := cb.parseDebugFile(filePath, cache); err != nil {
			// 继续处理其他文件
			continue
		}
	}

	return nil
}

// parseProjectFile 解析单个项目文件
func (cb *CacheBuilder) parseProjectFile(filePath string, cache *CacheFile, sessions map[string]bool) error {
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

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		// 统计会话
		if record.SessionID != "" {
			sessions[record.SessionID] = true
		}

		// 统计模型使用
		var msg AssistantMessage
		if err := json.Unmarshal(record.Message, &msg); err == nil {
			if msg.Model != "" {
				dateKey := timestamp.Format("2006-01-02")
				if cache.DailyStats[dateKey] == nil {
					cache.DailyStats[dateKey] = &DayAggregate{
						Date:          dateKey,
						ProjectCounts: make(map[string]int),
						ModelCounts:   make(map[string]int),
					}
				}
				cache.DailyStats[dateKey].ModelCounts[msg.Model]++
			}
		}
	}

	return nil
}

// parseDebugFile 解析单个 debug 日志文件
func (cb *CacheBuilder) parseDebugFile(filePath string, cache *CacheFile) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// 使用正则匹配 MCP 工具调用
	pattern := mcpPattern
	if pattern == nil {
		pattern = regexp.MustCompile(`mcp__(\w+)__(\w+)`)
	}

	buf := make([]byte, 0, 64*1024)
	scanner := newScanner(f, buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				key := match[1] + "::" + match[2]
				cache.MCPToolStats[key]++
			}
		}
	}

	return nil
}

// newScanner 创建带缓冲的 scanner（如果 bufio.Scanner 不可用）
func newScanner(r io.Reader, buf []byte, maxBufSize int) *bufio.Scanner {
	return bufio.NewScanner(r)
}
