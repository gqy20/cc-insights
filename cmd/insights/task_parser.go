package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ParseTasksConcurrent scans ~/.claude/tasks/ directory concurrently
// Returns aggregated task statistics filtered by time range
func ParseTasksConcurrent(tf TimeFilter) (*TaskAnalysisData, error) {
	return ParseTasksConcurrentFromDir(tf, cfg.DataDir)
}

// recordTaskAggLocked 把单条任务计入中间聚合（status 计数 + 按 session 归并）。
// claude tasks/ 与 zcode todo 共用；目标可为顶层或按天 runtime 子聚合。
func recordTaskAggLocked(agg *ProjectAggregate, sessionID, status string) {
	if agg.TaskAgg == nil {
		agg.TaskAgg = &TaskAgg{StatusCounts: make(map[string]int), SessionTasks: make(map[string]*SessionTaskAgg)}
	}
	if agg.TaskAgg.StatusCounts == nil {
		agg.TaskAgg.StatusCounts = make(map[string]int)
	}
	if agg.TaskAgg.SessionTasks == nil {
		agg.TaskAgg.SessionTasks = make(map[string]*SessionTaskAgg)
	}
	agg.TaskAgg.TotalTasks++
	agg.TaskAgg.StatusCounts[status]++
	sa := agg.TaskAgg.SessionTasks[sessionID]
	if sa == nil {
		sa = &SessionTaskAgg{SessionID: sessionID, StatusCountsLocal: make(map[string]int)}
		agg.TaskAgg.SessionTasks[sessionID] = sa
	}
	sa.TotalTasks++
	switch status {
	case "completed":
		sa.CompletedCount++
	case "in_progress":
		sa.InProgressCount++
	default:
		// claude 语义：unknown 一律归 pending 桶。
		sa.PendingCount++
	}
	if sa.StatusCountsLocal == nil {
		sa.StatusCountsLocal = make(map[string]int)
	}
	sa.StatusCountsLocal[status]++
}

// recordClaudeTasksIntoAgg 扫描 tasks/ 目录并把任务统计写入 agg.TaskAgg，
// 同时按任务文件 mtime 归因到按天 runtime 快照（时间过滤查询的重建原料）。
// 替代旧的"合并后直接赋值 TaskPlanAnalysis.Tasks"路径——那条路径的数据
// 进不了按天快照，任何时间过滤查询都会丢失。
func recordClaudeTasksIntoAgg(dataDir string, agg *ProjectAggregate) error {
	tasksDir := filepath.Join(dataDir, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading tasks directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionDir := filepath.Join(tasksDir, entry.Name())
		sessionID := entry.Name()
		files, err := os.ReadDir(sessionDir)
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			info, err := file.Info()
			if err != nil {
				continue
			}
			task, err := parseTaskJSONFile(filepath.Join(sessionDir, file.Name()))
			if err != nil {
				continue
			}
			status := task.Status
			if status == "" {
				status = "pending"
			}
			recordTaskAggLocked(agg, sessionID, status)
			dateKey := info.ModTime().Format("2006-01-02")
			recordTaskAggLocked(ensureDailyRuntimeAggregate(agg, dateKey), sessionID, status)
		}
	}
	return nil
}

// ParseTasksConcurrentFromDir scans tasks directory under specified dataDir
func ParseTasksConcurrentFromDir(tf TimeFilter, dataDir string) (*TaskAnalysisData, error) {
	tasksDir := filepath.Join(dataDir, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &TaskAnalysisData{}, nil
		}
		return nil, fmt.Errorf("reading tasks directory: %w", err)
	}

	var sessionDirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionDirs = append(sessionDirs, filepath.Join(tasksDir, entry.Name()))
	}

	if len(sessionDirs) == 0 {
		return &TaskAnalysisData{}, nil
	}

	maxWorkers := getWorkerCount()
	if maxWorkers > 64 {
		maxWorkers = 64
	}
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	globalAgg := &TaskAgg{
		StatusCounts: make(map[string]int),
		SessionTasks: make(map[string]*SessionTaskAgg),
	}

	for _, sd := range sessionDirs {
		wg.Add(1)
		go func(sessionDirPath string) {
			defer wg.Done()
			defer func() { <-sem }()
			sem <- struct{}{}

			sessionAgg := parseTaskSessionDir(sessionDirPath, tf)
			if sessionAgg == nil || sessionAgg.TotalTasks == 0 {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			globalAgg.SessionTasks[sessionAgg.SessionID] = sessionAgg
			globalAgg.TotalTasks += sessionAgg.TotalTasks
			for s, c := range sessionAgg.StatusCountsLocal {
				globalAgg.StatusCounts[s] += c
			}
		}(sd)
	}
	wg.Wait()

	return finalizeTaskAnalysis(globalAgg), nil
}

// parseTaskSessionDir parses a single session's task directory
// Returns a SessionTaskAgg or nil if dir is empty / outside time filter
func parseTaskSessionDir(sessionDirPath string, tf TimeFilter) *SessionTaskAgg {
	dirName := filepath.Base(sessionDirPath)

	dirInfo, err := os.Stat(sessionDirPath)
	if err != nil {
		return nil
	}
	if hasTimeFilter(tf) && !tf.Contains(dirInfo.ModTime()) {
		return nil
	}

	entries, err := os.ReadDir(sessionDirPath)
	if err != nil {
		return nil
	}

	agg := &SessionTaskAgg{
		SessionID:         dirName,
		StatusCountsLocal: make(map[string]int),
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if hasTimeFilter(tf) {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if !tf.Contains(info.ModTime()) {
				continue
			}
		}

		filePath := filepath.Join(sessionDirPath, entry.Name())
		task, err := parseTaskJSONFile(filePath)
		if err != nil {
			continue
		}
		agg.TotalTasks++
		switch task.Status {
		case "completed":
			agg.CompletedCount++
		case "pending":
			agg.PendingCount++
		case "in_progress":
			agg.InProgressCount++
		default:
			agg.PendingCount++ // treat unknown as pending
		}
		agg.StatusCountsLocal[task.Status]++
	}

	if agg.TotalTasks == 0 {
		return nil
	}
	return agg
}

// parseTaskJSONFile reads and parses a single task JSON file
func parseTaskJSONFile(filePath string) (*TaskRaw, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var task TaskRaw
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// finalizeTaskAnalysis converts intermediate TaskAgg -> output TaskAnalysisData
func finalizeTaskAnalysis(agg *TaskAgg) *TaskAnalysisData {
	if agg == nil || agg.TotalTasks == 0 {
		return &TaskAnalysisData{}
	}

	out := &TaskAnalysisData{
		TotalTasks:    agg.TotalTasks,
		TotalSessions: len(agg.SessionTasks),
	}

	// Status distribution (already aggregated in globalAgg.StatusCounts)
	for status, count := range agg.StatusCounts {
		rate := 0.0
		if out.TotalTasks > 0 {
			rate = float64(count) / float64(out.TotalTasks) * 100
		}
		out.StatusDistribution = append(out.StatusDistribution, TaskStatusItem{
			Status: status,
			Count:  count,
			Rate:   rate,
		})
	}
	sort.Slice(out.StatusDistribution, func(i, j int) bool {
		return out.StatusDistribution[i].Count > out.StatusDistribution[j].Count
	})

	// Per-session counts (top N by total tasks)
	for _, sa := range agg.SessionTasks {
		cr := 0.0
		if sa.TotalTasks > 0 {
			cr = float64(sa.CompletedCount) / float64(sa.TotalTasks) * 100
		}
		out.SessionTaskCounts = append(out.SessionTaskCounts, SessionTaskItem{
			SessionID:       sa.SessionID,
			TotalTasks:      sa.TotalTasks,
			CompletedCount:  sa.CompletedCount,
			PendingCount:    sa.PendingCount,
			InProgressCount: sa.InProgressCount,
			CompletionRate:  cr,
		})
	}
	sort.Slice(out.SessionTaskCounts, func(i, j int) bool {
		return out.SessionTaskCounts[i].TotalTasks > out.SessionTaskCounts[j].TotalTasks
	})
	if len(out.SessionTaskCounts) > 20 {
		out.SessionTaskCounts = out.SessionTaskCounts[:20]
	}

	// Overall completion rate
	completed := agg.StatusCounts["completed"]
	if out.TotalTasks > 0 {
		out.CompletionRate = float64(completed) / float64(out.TotalTasks) * 100
	}
	if len(agg.SessionTasks) > 0 {
		out.AvgTasksPerSession = float64(out.TotalTasks) / float64(len(agg.SessionTasks))
	}

	return out
}
