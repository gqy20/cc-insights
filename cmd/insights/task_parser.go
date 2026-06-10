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
			CompletionRate:   cr,
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

