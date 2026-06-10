package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ParseProjectsConcurrentOnce 一次遍历并发解析所有项目统计
// 这个函数将所有统计合并到一次遍历中，大幅提升性能
func ParseProjectsConcurrentOnce(tf TimeFilter) (*ProjectAggregate, error) {
	return ParseProjectsConcurrentOnceFromDir(tf, cfg.DataDir)
}

// ParseProjectsConcurrentOnceFromDir 一次遍历并发解析指定数据目录下的项目统计
func ParseProjectsConcurrentOnceFromDir(tf TimeFilter, dataDir string) (*ProjectAggregate, error) {
	projectsDir := filepath.Join(dataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	aggregate := newProjectAggregate()

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		projectFiles, err := projectJSONLFiles(projectDir)
		if err != nil {
			continue
		}
		files = append(files, projectFiles...)
	}
	sort.Strings(files)

	maxWorkers := runtime.NumCPU()
	if len(files) < maxWorkers {
		maxWorkers = len(files)
	}
	if maxWorkers == 0 {
		aggregate.finalize()
		return aggregate, nil
	}

	jobs := make(chan string, maxWorkers*2)
	results := make(chan *ProjectAggregate, maxWorkers)
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerAggregate := newProjectAggregate()
			for filePath := range jobs {
				parseProjectFileAggregate(filePath, tf, workerAggregate)
			}
			results <- workerAggregate
		}()
	}

	for _, filePath := range files {
		jobs <- filePath
	}
	close(jobs)
	go func() {
		wg.Wait()
		close(results)
	}()

	for fileAggregate := range results {
		mergeProjectAggregate(aggregate, fileAggregate)
	}

	// 后处理：生成输出格式数据
	aggregate.finalize()

	return aggregate, nil
}

func projectJSONLFiles(projectDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(projectDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// parseProjectFileAggregate 解析单个项目文件并更新聚合数据
func parseProjectFileAggregate(filePath string, tf TimeFilter, agg *ProjectAggregate) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	pendingTools := make(map[string]pendingToolCall)
	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		timestamp, hasTimestamp := parseProjectRecordTimestamp(record.Timestamp)
		if hasTimestamp && !tf.Contains(timestamp) {
			continue
		}
		if !hasTimestamp && hasTimeFilter(tf) {
			continue
		}

		projectName := record.Cwd
		if projectName == "" {
			projectName = "Unknown"
		}

		recordRuntimeEventLocked(agg, record, timestamp, projectName)

		if record.Type == "user" {
			parseToolResults(record, timestamp, projectName, pendingTools, agg)
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(record.Message, &msg); err != nil {
			continue
		}

		// 1. 项目统计
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
		if msg.Model != "" {
			if agg.ModelUsage[msg.Model] == nil {
				agg.ModelUsage[msg.Model] = &ModelUsageItem{
					Model: msg.Model,
				}
			}
			agg.ModelUsage[msg.Model].Count++
			agg.ModelUsage[msg.Model].Tokens += msg.Usage.InputTokens + msg.Usage.OutputTokens
			recordCostUsageLocked(agg, msg, record, projectName)
		}

		// 6. 工具调用统计
		for _, content := range msg.Content {
			if content.Type != "tool_use" || content.ID == "" || content.Name == "" {
				continue
			}
			pendingTools[content.ID] = pendingToolCall{
				ID:          content.ID,
				Tool:        content.Name,
				Model:       msg.Model,
				Project:     projectName,
				SessionID:   record.SessionID,
				AgentID:     record.AgentID,
				IsSidechain: record.IsSidechain,
				Timestamp:   timestamp,
				Input:       content.Input,
			}
			call := pendingTools[content.ID]
			addToolCallLocked(agg, content.Name, msg.Model)
			addAgentToolCallLocked(agg, call)
			recordStructuredToolInputLocked(agg, &call)
			pendingTools[content.ID] = call
		}
	}

	if len(pendingTools) > 0 {
		for _, call := range pendingTools {
			addMissingToolResultLocked(agg, call)
		}
	}
}

func parseProjectRecordTimestamp(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return timestamp, true
}

func hasTimeFilter(tf TimeFilter) bool {
	return tf.Start != nil || tf.End != nil
}
