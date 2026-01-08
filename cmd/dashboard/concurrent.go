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

// getWorkerCount 返回推荐的 worker 数量（CPU 核心数的一半，至少为 1）
func getWorkerCount() int {
	n := runtime.NumCPU() / 2
	if n < 1 {
		return 1
	}
	return n
}

// ParseHistoryConcurrent 并发解析 history.jsonl（优化版）
func ParseHistoryConcurrent(tf TimeFilter) ([]CommandStats, map[string]int, error) {
	path := GetDataPath("history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// 使用批量处理
	batchSize := 1000
	batches := make(chan []HistoryRecord, runtime.NumCPU())
	var wg sync.WaitGroup

	// producer: 读取文件并分批
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(batches)

		decoder := json.NewDecoder(f)
		batch := make([]HistoryRecord, 0, batchSize)

		for {
			var record HistoryRecord
			if err := decoder.Decode(&record); err != nil {
				if err == io.EOF {
					if len(batch) > 0 {
						batches <- batch
					}
					break
				}
				continue
			}

			// 时间过滤
			recordTime := time.Unix(record.Timestamp/1000, 0)
			if !tf.Contains(recordTime) {
				continue
			}

			batch = append(batch, record)
			if len(batch) >= batchSize {
				batches <- batch
				batch = make([]HistoryRecord, 0, batchSize)
			}
		}
	}()

	// consumers: 并发处理批次
	workers := getWorkerCount()
	cmdMu := sync.Mutex{}
	cmdCounts := make(map[string]int)
	hourlyCounts := make(map[string]int)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localCmds := make(map[string]int)
			localHourly := make(map[string]int)

			for batch := range batches {
				for _, record := range batch {
					// 统计 slash commands
					if strings.HasPrefix(record.Display, "/") {
						parts := strings.Fields(record.Display)
						if len(parts) > 0 {
							localCmds[parts[0]]++
						}
					}

					// 统计小时分布
					recordTime := time.Unix(record.Timestamp/1000, 0)
					hour := fmt.Sprintf("%02d", recordTime.Hour())
					localHourly[hour]++
				}
			}

			// 合并结果
			cmdMu.Lock()
			for cmd, count := range localCmds {
				cmdCounts[cmd] += count
			}
			for hour, count := range localHourly {
				hourlyCounts[hour] += count
			}
			cmdMu.Unlock()
		}()
	}

	wg.Wait()

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

// ParseDebugLogsConcurrent 并发解析 debug 日志（优化版）
func ParseDebugLogsConcurrent(tf TimeFilter) ([]MCPToolStats, error) {
	debugDir := GetDataPath("debug")

	entries, err := os.ReadDir(debugDir)
	if err != nil {
		return nil, err
	}

	// 获取文件信息并过滤
	var fileInfos []DebugFileInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			info, _ := entry.Info()
			fileInfo := DebugFileInfo{
				Path:    filepath.Join(debugDir, entry.Name()),
				ModTime: info.ModTime(),
			}
			// 时间过滤
			if tf.Contains(info.ModTime()) {
				fileInfos = append(fileInfos, fileInfo)
			}
		}
	}

	// 使用信号量控制并发数（CPU 核心数的一半）
	maxWorkers := getWorkerCount()
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	results := make(chan map[string]int, len(fileInfos))
	mcpPattern := regexp.MustCompile(`mcp__(\w+)__(\w+)`)

	for _, fileInfo := range fileInfos {
		wg.Add(1)
		go func(fp string) {
			defer wg.Done()
			defer func() { <-sem }()

			sem <- struct{}{}
			toolCounts := make(map[string]int)
			parseDebugFileOptimized(fp, toolCounts, mcpPattern)
			results <- toolCounts
		}(fileInfo.Path)
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

// parseDebugFileOptimized 优化的 debug 文件解析
func parseDebugFileOptimized(path string, counts map[string]int, pattern *regexp.Regexp) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	// 使用带缓冲的扫描器
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024) // 64KB 缓冲区
	scanner.Buffer(buf, 1024*1024)  // 最大1MB

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				key := match[1] + "::" + match[2]
				counts[key]++
			}
		}
	}
}
