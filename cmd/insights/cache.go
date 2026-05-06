package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheFile 缓存文件结构
type CacheFile struct {
	Version    string    // 缓存格式版本
	LastUpdate time.Time // 最后缓存时间戳
	TimeRange  TimeRange // 缓存覆盖的时间范围

	// 预聚合数据
	DailyStats  map[string]*DayAggregate // "2026-01-08" -> 当天所有统计
	HourlyStats [24]*HourAggregate       // 每小时统计

	// 全局统计
	TotalMessages int // 总消息数
	TotalSessions int // 总会话数
	ProjectStats  map[string]*ProjectStatItem
	ModelUsage    map[string]*ModelUsageItem
	WeekdayStats  [7]*WeekdayItem
	MCPToolStats  map[string]int
}

// DayAggregate 每日聚合数据
type DayAggregate struct {
	Date          string         // "2026-01-08"
	MessageCount  int            // 当天消息数
	SessionCount  int            // 当天会话数
	ToolCallCount int            // 当天工具调用数
	HourlyCounts  [24]int        // 每小时消息数
	ProjectCounts map[string]int // 项目 -> 消息数
	ModelCounts   map[string]int // 模型 -> 请求次数
}

// HourAggregate 每小时聚合数据
type HourAggregate struct {
	Hour         int // 0-23
	MessageCount int // 该小时消息数
	SessionCount int // 该小时会话数
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Contains 检查时间是否在范围内
func (tr TimeRange) Contains(t time.Time) bool {
	// 无限制情况
	if tr.Start.IsZero() && tr.End.IsZero() {
		return true
	}

	// 检查开始时间
	if !tr.Start.IsZero() && t.Before(tr.Start) {
		return false
	}

	// 检查结束时间
	if !tr.End.IsZero() && t.After(tr.End) {
		return false
	}

	return true
}

// Save 保存缓存到文件
func (cf *CacheFile) Save(path string) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化缓存失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入缓存文件失败: %w", err)
	}

	return nil
}

// LoadCacheFile 从文件加载缓存
func LoadCacheFile(path string) (*CacheFile, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("缓存文件不存在: %s", path)
	}

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取缓存文件失败: %w", err)
	}

	// 反序列化
	var cache CacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("解析缓存文件失败: %w", err)
	}

	return &cache, nil
}

// IsExpired 检查缓存是否过期
func (cf *CacheFile) IsExpired(dataLastModified time.Time) bool {
	// 如果数据文件的修改时间晚于缓存更新时间，则缓存过期
	return dataLastModified.After(cf.LastUpdate)
}

// QueryByTimeRange 按时间范围查询缓存数据
func (cf *CacheFile) QueryByTimeRange(start, end time.Time) *CacheFile {
	result := &CacheFile{
		Version:    cf.Version,
		LastUpdate: cf.LastUpdate,
		TimeRange: TimeRange{
			Start: start,
			End:   end,
		},
		DailyStats:   make(map[string]*DayAggregate),
		HourlyStats:  [24]*HourAggregate{},
		ProjectStats: make(map[string]*ProjectStatItem),
		ModelUsage:   make(map[string]*ModelUsageItem),
		MCPToolStats: make(map[string]int),
	}

	queryRange := TimeRange{Start: start, End: end}

	// 遍历每日统计，过滤时间范围
	for date, dayStats := range cf.DailyStats {
		dateParsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}

		if queryRange.Contains(dateParsed) {
			dayCopy := *dayStats
			result.DailyStats[date] = &dayCopy

			result.TotalMessages += dayStats.MessageCount
			result.TotalSessions += dayStats.SessionCount
		}
	}

	// 从过滤后的 DailyStats 中收集有效项目名和模型名
	activeProjects := make(map[string]bool)
	activeModels := make(map[string]bool)
	for _, dayStats := range result.DailyStats {
		for proj := range dayStats.ProjectCounts {
			activeProjects[proj] = true
		}
		for model := range dayStats.ModelCounts {
			activeModels[model] = true
		}
	}

	// 复制 HourlyStats（全局分布模式，不过滤）
	for i, hs := range cf.HourlyStats {
		if hs != nil {
			hsCopy := *hs
			result.HourlyStats[i] = &hsCopy
		}
	}

	// 复制 WeekdayStats（全局分布模式，不过滤）
	for i, ws := range cf.WeekdayStats {
		if ws != nil {
			result.WeekdayStats[i] = ws
		}
	}

	// 过滤 ProjectStats：只保留时间范围内有活动的项目
	for proj, stats := range cf.ProjectStats {
		if activeProjects[proj] {
			projCopy := *stats
			result.ProjectStats[proj] = &projCopy
		}
	}

	// 过滤 ModelUsage：只保留时间范围内使用的模型
	for model, mu := range cf.ModelUsage {
		if activeModels[model] {
			muCopy := *mu
			result.ModelUsage[model] = &muCopy
		}
	}

	// MCPToolStats（全局统计，不过滤——debug日志无每日分解）
	for tool, count := range cf.MCPToolStats {
		result.MCPToolStats[tool] = count
	}

	return result
}

// AddMessage 添加一条消息记录到每日聚合
func (da *DayAggregate) AddMessage(project string, hour int) {
	da.MessageCount++

	// 更新小时统计
	if hour >= 0 && hour < 24 {
		da.HourlyCounts[hour]++
	}

	// 更新项目统计
	if da.ProjectCounts == nil {
		da.ProjectCounts = make(map[string]int)
	}
	da.ProjectCounts[project]++
}
