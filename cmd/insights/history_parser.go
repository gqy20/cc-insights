package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
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
