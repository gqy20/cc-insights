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

// CacheBuilder 缓存构建器
type CacheBuilder struct {
	CachePath string // 缓存文件路径
	DataDir   string // 数据目录路径
}

var cacheRefreshMu sync.Mutex

type projectFileInfo struct {
	RelPath string
	AbsPath string
	Size    int64
	ModTime int64
}

type projectFileResult struct {
	Info      projectFileInfo
	Cache     *ProjectFileCache
	Aggregate *ProjectAggregate
}

// BuildFullCache 构建完整缓存
func (cb *CacheBuilder) BuildFullCache() error {
	buildStartedAt := time.Now()
	Info("开始构建完整缓存")

	dataDir := cb.DataDir
	if dataDir == "" {
		dataDir = cfg.DataDir
	}
	rulesHash, err := currentBashRulesHash()
	if err != nil {
		return fmt.Errorf("加载 Bash 规则失败: %w", err)
	}

	previous, _ := LoadCacheFile(cb.CachePath)
	if previous != nil && (previous.Version != CacheVersion || previous.BashRulesHash != rulesHash) {
		previous = nil
	}

	aggregate, projectFiles, reused, parsed, err := cb.buildProjectAggregateIncremental(dataDir, previous)
	if err != nil {
		return fmt.Errorf("解析项目数据失败: %w", err)
	}
	if reused > 0 || parsed > 0 {
		Info("项目文件处理完成", "reused", reused, "parsed", parsed)
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
		Version:       CacheVersion,
		LastUpdate:    time.Now(),
		TimeRange:     TimeRange{},
		BashRulesHash: rulesHash,
		BuildStats: &CacheBuildStats{
			BuiltAt:       buildStartedAt.Format(time.RFC3339),
			TotalFiles:    reused + parsed,
			ReusedFiles:   reused,
			ParsedFiles:   parsed,
			BashRulesHash: rulesHash,
		},
		DailyStats:          make(map[string]*DayAggregate),
		TotalMessages:       totalMessages,
		TotalSessions:       sessionStats.TotalSessions,
		ProjectStats:        make(map[string]*ProjectStatItem),
		ModelUsage:          make(map[string]*ModelUsageItem),
		RuntimeToolSignals:  make(map[string]int),
		ToolStats:           make(map[string]*ToolStatItem),
		ToolAnalysis:        aggregate.ToolAnalysis,
		SkillAnalysis:       aggregate.SkillAnalysis,
		FailureAnalysis:     aggregate.FailureAnalysis,
		SessionAnalysis:     aggregate.SessionAnalysis,
		EventAnalysis:       aggregate.EventAnalysis,
		AgentAnalysis:       aggregate.AgentAnalysis,
		CommandAnalysis:     aggregate.CommandAnalysis,
		CostAnalysis:        aggregate.CostAnalysis,
		FileAnalysis:        aggregate.FileAnalysis,
		TaskPlanAnalysis:    aggregate.TaskPlanAnalysis,
		ToolPerformance:     aggregate.ToolPerformance,
		ProjectFiles:        projectFiles,
		DailyRuntime:        make(map[string]ProjectFileAggregate, len(aggregate.DailyRuntime)),
		DailyProjectRuntime: make(map[string]map[string]ProjectFileAggregate, len(aggregate.DailyProjectRuntime)),
		DailySessionRuntime: make(map[string]map[string]ProjectFileAggregate, len(aggregate.DailySessionRuntime)),
	}
	for date, runtimeAgg := range aggregate.DailyRuntime {
		if runtimeAgg != nil {
			cache.DailyRuntime[date] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
		}
	}
	for date, projects := range aggregate.DailyProjectRuntime {
		for project, runtimeAgg := range projects {
			if runtimeAgg == nil {
				continue
			}
			if cache.DailyProjectRuntime[date] == nil {
				cache.DailyProjectRuntime[date] = make(map[string]ProjectFileAggregate)
			}
			cache.DailyProjectRuntime[date][project] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
		}
	}
	for date, sessions := range aggregate.DailySessionRuntime {
		for sessionID, runtimeAgg := range sessions {
			if runtimeAgg == nil {
				continue
			}
			if cache.DailySessionRuntime[date] == nil {
				cache.DailySessionRuntime[date] = make(map[string]ProjectFileAggregate)
			}
			cache.DailySessionRuntime[date][sessionID] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
		}
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
			HourlyCounts:  aggregate.DailyHourlyCounts[day.Date],
			ProjectCounts: copyIntMap(aggregate.DailyProjectCounts[day.Date]),
			ModelCounts:   copyIntMap(aggregate.DailyModelCounts[day.Date]),
			ModelTokens:   copyIntMap(aggregate.DailyModelTokens[day.Date]),
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

	// 填充工具调用分析
	for _, tool := range aggregate.ToolAnalysis.Tools {
		toolCopy := tool
		cache.ToolStats[tool.Tool] = &toolCopy
		if server, name, ok := splitRuntimeMCPToolName(tool.Tool); ok {
			cache.RuntimeToolSignals[server+"::"+name] = tool.CallCount
		}
	}
	if cache.BuildStats != nil {
		cache.BuildStats.BuildDurationMs = time.Since(buildStartedAt).Milliseconds()
	}
	// 4. 保存缓存
	if err := cache.Save(cb.CachePath); err != nil {
		return fmt.Errorf("保存缓存失败: %w", err)
	}
	if err := cache.SaveDiagnostics(diagnosticsCachePath(cb.CachePath)); err != nil {
		Warn("保存诊断缓存失败", "error", err.Error())
	}

	Info("缓存构建完成", "messages", cache.TotalMessages, "sessions", cache.TotalSessions)
	return nil
}

func diagnosticsCachePath(cachePath string) string {
	return filepath.Join(filepath.Dir(cachePath), "diagnostics.db")
}

func refreshGlobalCache(force bool) error {
	cacheRefreshMu.Lock()
	defer cacheRefreshMu.Unlock()

	if err := os.MkdirAll(cfg.CacheDir, 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}

	cachePath := filepath.Join(cfg.CacheDir, "cache.db")
	builder := &CacheBuilder{
		CachePath: cachePath,
		DataDir:   cfg.DataDir,
	}

	if force || builder.NeedsRebuild() {
		Info("正在构建缓存...", "cache_path", cachePath, "force", force)
		start := time.Now()
		if err := builder.BuildFullCache(); err != nil {
			Error("缓存构建失败", "error", err.Error())
			return fmt.Errorf("构建缓存失败: %w", err)
		}
		elapsed := time.Since(start)
		Info("缓存构建完成", "duration_sec", elapsed.Seconds())
	} else {
		Info("使用现有缓存", "cache_path", cachePath)
	}

	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		Error("加载缓存失败", "error", err.Error())
		return fmt.Errorf("加载缓存失败: %w", err)
	}

	globalCache = cache
	Info("缓存已加载",
		"messages", globalCache.TotalMessages,
		"sessions", globalCache.TotalSessions,
	)
	return nil
}

func splitRuntimeMCPToolName(tool string) (string, string, bool) {
	if !strings.HasPrefix(tool, "mcp__") {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(tool, "mcp__"), "__", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (cb *CacheBuilder) buildProjectAggregateIncremental(dataDir string, previous *CacheFile) (*ProjectAggregate, map[string]*ProjectFileCache, int, int, error) {
	files, err := listProjectJSONLFileInfos(dataDir)
	if err != nil {
		return nil, nil, 0, 0, err
	}

	aggregate := newProjectAggregate()
	projectFiles := make(map[string]*ProjectFileCache, len(files))
	var toParse []projectFileInfo
	reused := 0

	for _, info := range files {
		if previous != nil && previous.ProjectFiles != nil {
			if cached := previous.ProjectFiles[info.RelPath]; cached != nil && cached.Size == info.Size && cached.ModTimeUnix == info.ModTime {
				fileAggregate := projectFileAggregateToAggregate(cached.Aggregate)
				mergeProjectAggregate(aggregate, fileAggregate)
				projectFiles[info.RelPath] = cached
				reused++
				continue
			}
		}
		toParse = append(toParse, info)
	}

	parsedCaches, err := parseProjectFilesForCache(toParse)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	for _, item := range parsedCaches {
		mergeProjectAggregate(aggregate, item.Aggregate)
		projectFiles[item.Info.RelPath] = item.Cache
	}
	recordInstalledSkillsLocked(aggregate, dataDir)

	aggregate.finalize()

	// task_plan_analysis (M4): 扫描 tasks/ 目录
	taskAnalysis, taskErr := ParseTasksConcurrentFromDir(TimeFilter{}, dataDir)
	if taskErr != nil {
		Warn("tasks/ 目录扫描失败", "error", taskErr.Error())
	}
	if taskAnalysis != nil && aggregate.TaskPlanAnalysis == nil {
		aggregate.TaskPlanAnalysis = &TaskPlanAnalysisData{}
	}
	if taskAnalysis != nil {
		aggregate.TaskPlanAnalysis.Tasks = *taskAnalysis
	}

	return aggregate, projectFiles, reused, len(parsedCaches), nil
}

func listProjectJSONLFileInfos(dataDir string) ([]projectFileInfo, error) {
	projectsDir := filepath.Join(dataDir, "projects")
	var files []projectFileInfo
	err := filepath.WalkDir(projectsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			rel = path
		}
		files = append(files, projectFileInfo{
			RelPath: filepath.ToSlash(rel),
			AbsPath: path,
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})
	return files, nil
}

func parseProjectFilesForCache(files []projectFileInfo) ([]projectFileResult, error) {
	if len(files) == 0 {
		return nil, nil
	}
	maxWorkers := runtime.NumCPU()
	if len(files) < maxWorkers {
		maxWorkers = len(files)
	}

	jobs := make(chan projectFileInfo, maxWorkers*2)
	results := make(chan projectFileResult, len(files))
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for info := range jobs {
				fileAggregate := newProjectAggregate()
				parseProjectFileAggregate(info.AbsPath, TimeFilter{}, fileAggregate)
				results <- projectFileResult{
					Info:      info,
					Aggregate: fileAggregate,
					Cache: &ProjectFileCache{
						Path:        info.RelPath,
						Size:        info.Size,
						ModTimeUnix: info.ModTime,
						Aggregate:   aggregateToProjectFileAggregate(fileAggregate),
					},
				}
			}
		}()
	}
	for _, info := range files {
		jobs <- info
	}
	close(jobs)
	wg.Wait()
	close(results)

	items := make([]projectFileResult, 0, len(files))
	for item := range results {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Info.RelPath < items[j].Info.RelPath
	})
	return items, nil
}

// RebuildIfChanged 在数据变化时重建缓存。
func (cb *CacheBuilder) RebuildIfChanged() error {
	cache, err := LoadCacheFile(cb.CachePath)
	if err != nil {
		Info("缓存不存在，开始构建完整缓存")
		return cb.BuildFullCache()
	}

	Info("检查缓存是否需要重建", "cache_time", cache.LastUpdate.Format("2006-01-02 15:04:05"))

	lastDataMod, err := cb.GetLastDataModified()
	if err != nil {
		return fmt.Errorf("获取数据修改时间失败: %w", err)
	}

	if !lastDataMod.After(cache.LastUpdate) {
		Info("缓存已是最新，无需更新")
		return nil
	}

	Info("数据已更新，重新构建缓存")
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
	if cache.Version != CacheVersion {
		return true
	}
	rulesHash, err := currentBashRulesHash()
	if err != nil {
		return true
	}
	if cache.BashRulesHash != rulesHash {
		return true
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

	projectsDir := filepath.Join(cb.DataDir, "projects")
	if err := cb.scanDirectory(projectsDir, &lastMod); err != nil {
		// 目录不存在不是错误
		if !os.IsNotExist(err) {
			return time.Time{}, err
		}
	}

	return lastMod, nil
}

// scanDirectory 递归扫描目录获取最后修改时间
func (cb *CacheBuilder) scanDirectory(dirPath string, lastMod *time.Time) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			// 递归扫描子目录
			if err := cb.scanDirectory(fullPath, lastMod); err != nil {
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

	if cache.RuntimeToolSignals == nil {
		cache.RuntimeToolSignals = make(map[string]int)
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

	// 使用正则匹配 Runtime 工具信号
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
				cache.RuntimeToolSignals[key]++
			}
		}
	}

	return nil
}

// newScanner 创建带缓冲的 scanner（如果 bufio.Scanner 不可用）
func newScanner(r io.Reader, buf []byte, maxBufSize int) *bufio.Scanner {
	return bufio.NewScanner(r)
}
