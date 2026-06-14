package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCacheBuilderBuildFullCache 测试构建完整缓存
func TestCacheBuilderBuildFullCache(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, tmpDir)

	cachePath := filepath.Join(tmpDir, "cache.db")
	builder := &CacheBuilder{
		CachePath: cachePath,
		DataDir:   dataDir,
	}

	// Act
	err := builder.BuildFullCache()

	// Assert
	if err != nil {
		t.Fatalf("BuildFullCache() failed: %v", err)
	}

	// 验证缓存文件存在
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}

	// 验证缓存可以加载
	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("LoadCacheFile() failed: %v", err)
	}

	if cache == nil {
		t.Fatal("Loaded cache is nil")
	}

	// 验证基本字段
	if cache.Version != CacheVersion {
		t.Errorf("Cache version = %s, want %s", cache.Version, CacheVersion)
	}

	if cache.TotalMessages == 0 {
		t.Error("TotalMessages should be > 0")
	}
	if cache.BuildStats == nil {
		t.Fatal("BuildStats should not be nil")
	}
	if cache.BuildStats.ParsedFiles == 0 {
		t.Error("BuildStats.ParsedFiles should be > 0")
	}
	if cache.BuildStats.TotalFiles != cache.BuildStats.ParsedFiles+cache.BuildStats.ReusedFiles {
		t.Errorf("BuildStats.TotalFiles = %d, want parsed+reused %d", cache.BuildStats.TotalFiles, cache.BuildStats.ParsedFiles+cache.BuildStats.ReusedFiles)
	}
	if cache.BuildStats.BashRulesHash != cache.BashRulesHash {
		t.Errorf("BuildStats.BashRulesHash = %q, want %q", cache.BuildStats.BashRulesHash, cache.BashRulesHash)
	}

	diagnosticsCache, err := LoadCacheFile(diagnosticsCachePath(cachePath))
	if err != nil {
		t.Fatalf("Load diagnostics cache failed: %v", err)
	}
	if diagnosticsCache.ProjectFiles != nil {
		t.Fatal("diagnostics cache should not include ProjectFiles")
	}
	if diagnosticsCache.CommandAnalysis == nil {
		t.Fatal("diagnostics cache should retain CommandAnalysis")
	}
}

func TestCacheBuilderBuildFullCacheDailyProjectAndModelCounts(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, tmpDir)
	cachePath := filepath.Join(tmpDir, "cache.db")
	builder := &CacheBuilder{CachePath: cachePath, DataDir: dataDir}

	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("BuildFullCache() failed: %v", err)
	}

	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("LoadCacheFile() failed: %v", err)
	}

	var queryDate string
	for date, day := range cache.DailyStats {
		if day.ProjectCounts["/tmp/test-project"] > 0 && day.ModelCounts["claude-sonnet-4.5"] > 0 {
			queryDate = date
			break
		}
	}
	if queryDate == "" {
		t.Fatalf("DailyStats did not retain project/model counts: %+v", cache.DailyStats)
	}

	start, err := time.Parse("2006-01-02", queryDate)
	if err != nil {
		t.Fatalf("Parse query date failed: %v", err)
	}
	result := cache.QueryByTimeRange(start, start.AddDate(0, 0, 1))
	if len(result.ProjectStats) == 0 {
		t.Fatal("QueryByTimeRange returned no ProjectStats")
	}
	if result.ProjectStats["/tmp/test-project"] == nil {
		t.Fatalf("QueryByTimeRange missing /tmp/test-project: %+v", result.ProjectStats)
	}
	if result.ProjectStats["/tmp/test-project"].MessageCount != 2 {
		t.Fatalf("Project message count=%d, want 2", result.ProjectStats["/tmp/test-project"].MessageCount)
	}
	if len(result.ModelUsage) == 0 {
		t.Fatal("QueryByTimeRange returned no ModelUsage")
	}
	if result.ModelUsage["claude-sonnet-4.5"] == nil {
		t.Fatalf("QueryByTimeRange missing claude-sonnet-4.5: %+v", result.ModelUsage)
	}
	if result.ModelUsage["claude-sonnet-4.5"].Count != 2 {
		t.Fatalf("Model count=%d, want 2", result.ModelUsage["claude-sonnet-4.5"].Count)
	}
	if result.ModelUsage["claude-sonnet-4.5"].Tokens != 30 {
		t.Fatalf("Model tokens=%d, want 30", result.ModelUsage["claude-sonnet-4.5"].Tokens)
	}
	hourlyTotal := 0
	for _, hourly := range result.HourlyStats {
		if hourly != nil {
			hourlyTotal += hourly.MessageCount
		}
	}
	if hourlyTotal != result.TotalMessages {
		t.Fatalf("Hourly total=%d, want %d", hourlyTotal, result.TotalMessages)
	}
	weekdayTotal := 0
	for _, weekday := range result.WeekdayStats {
		if weekday != nil {
			weekdayTotal += weekday.MessageCount
		}
	}
	if weekdayTotal != result.TotalMessages {
		t.Fatalf("Weekday total=%d, want %d", weekdayTotal, result.TotalMessages)
	}
}

func TestCacheBuilderBuildFullCacheUsesBuilderDataDirForTasks(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, filepath.Join(tmpDir, "builder"))
	globalDataDir := createTestDataDir(t, filepath.Join(tmpDir, "global"))

	writeTaskFixture(t, dataDir, "builder-session", "completed")
	writeTaskFixture(t, globalDataDir, "global-session", "pending")
	writeTaskFixture(t, globalDataDir, "global-session", "in_progress")

	origDataDir := cfg.DataDir
	cfg.DataDir = globalDataDir
	defer func() { cfg.DataDir = origDataDir }()

	cachePath := filepath.Join(tmpDir, "cache.db")
	builder := &CacheBuilder{CachePath: cachePath, DataDir: dataDir}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("BuildFullCache() failed: %v", err)
	}

	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("LoadCacheFile() failed: %v", err)
	}
	if cache.TaskPlanAnalysis == nil {
		t.Fatal("TaskPlanAnalysis is nil")
	}
	tasks := cache.TaskPlanAnalysis.Tasks
	if tasks.TotalTasks != 1 {
		t.Fatalf("TotalTasks = %d, want 1; status=%+v", tasks.TotalTasks, tasks.StatusDistribution)
	}
	if tasks.TotalSessions != 1 {
		t.Fatalf("TotalSessions = %d, want 1", tasks.TotalSessions)
	}
	if len(tasks.StatusDistribution) != 1 || tasks.StatusDistribution[0].Status != "completed" {
		t.Fatalf("StatusDistribution = %+v, want only completed from builder data dir", tasks.StatusDistribution)
	}
}

func writeTaskFixture(t *testing.T, dataDir, sessionID, status string) {
	t.Helper()

	taskDir := filepath.Join(dataDir, "tasks", sessionID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("Create task dir failed: %v", err)
	}
	taskPath := filepath.Join(taskDir, status+".json")
	content := `{"id":"` + status + `","subject":"test","status":"` + status + `"}`
	if err := os.WriteFile(taskPath, []byte(content), 0644); err != nil {
		t.Fatalf("Write task fixture failed: %v", err)
	}
}

func TestCacheQueryByTimeRangeScopesRuntimeAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "runtime-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Create projects dir failed: %v", err)
	}

	oldTime := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 2, 11, 0, 0, 0, time.UTC)
	content := toolUseRecordWithInput("/tmp/runtime-project", "old-session", oldTime, "old-call", "Bash", "old-model", false, "", `{"command":"go test ./..."}`) + "\n" +
		toolResultRecord("/tmp/runtime-project", "old-session", oldTime.Add(time.Second), "old-call", "ok") + "\n" +
		toolUseRecordWithInput("/tmp/runtime-project", "new-session", newTime, "new-call", "mcp__crawl__extract_url", "new-model", false, "", `{"url":"https://example.com"}`) + "\n" +
		toolResultRecord("/tmp/runtime-project", "new-session", newTime.Add(2*time.Second), "new-call", "fetched") + "\n"
	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("Write project jsonl failed: %v", err)
	}

	cachePath := filepath.Join(tmpDir, "cache.db")
	builder := &CacheBuilder{CachePath: cachePath, DataDir: dataDir}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("BuildFullCache() failed: %v", err)
	}

	cache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("LoadCacheFile() failed: %v", err)
	}
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	result := cache.QueryByTimeRange(start, end)

	if result.ToolAnalysis == nil || result.ToolAnalysis.TotalCalls != 1 {
		t.Fatalf("ToolAnalysis.TotalCalls=%v, want 1", result.ToolAnalysis)
	}
	if len(result.ToolAnalysis.Tools) != 1 || result.ToolAnalysis.Tools[0].Tool != "mcp__crawl__extract_url" {
		t.Fatalf("ToolAnalysis.Tools=%+v, want only mcp__crawl__extract_url", result.ToolAnalysis.Tools)
	}
	if result.RuntimeToolSignals["crawl::extract_url"] != 1 {
		t.Fatalf("RuntimeToolSignals=%+v, want crawl::extract_url=1", result.RuntimeToolSignals)
	}
	if result.CostAnalysis == nil || result.CostAnalysis.Totals.RequestCount != 1 {
		t.Fatalf("CostAnalysis totals=%+v, want request_count=1", result.CostAnalysis)
	}
	if result.CostAnalysis.ByModel[0].Model != "new-model" {
		t.Fatalf("CostAnalysis.ByModel=%+v, want new-model", result.CostAnalysis.ByModel)
	}
	if result.ToolPerformance == nil || result.ToolPerformance.TotalPairedCalls != 1 {
		t.Fatalf("ToolPerformance=%+v, want one paired call", result.ToolPerformance)
	}
}

// TestCacheBuilderRebuildIfChanged 测试数据变化时重建缓存。
func TestCacheBuilderRebuildIfChanged(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, tmpDir)
	cachePath := filepath.Join(tmpDir, "cache.db")

	// 第一次构建完整缓存
	builder := &CacheBuilder{
		CachePath: cachePath,
		DataDir:   dataDir,
	}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("Initial BuildFullCache() failed: %v", err)
	}

	// 获取初始缓存
	initialCache, _ := LoadCacheFile(cachePath)
	initialMessageCount := initialCache.TotalMessages

	// 添加新数据
	newTime := time.Now().Add(1 * time.Minute)
	appendProjectRecord(t, dataDir, "/tmp/test-project", "session-3", newTime)
	projectPath := filepath.Join(dataDir, "projects", "test-project", "session.jsonl")
	if err := os.Chtimes(projectPath, newTime, newTime); err != nil {
		t.Fatalf("更新 projects 文件修改时间失败: %v", err)
	}

	err := builder.RebuildIfChanged()

	if err != nil {
		t.Fatalf("RebuildIfChanged() failed: %v", err)
	}

	// 验证缓存已更新
	updatedCache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("Load updated cache failed: %v", err)
	}

	// 应该有更多消息
	if updatedCache.TotalMessages <= initialMessageCount {
		t.Errorf("TotalMessages = %d, should be > %d", updatedCache.TotalMessages, initialMessageCount)
	}

	// 更新时间应该更新
	if !updatedCache.LastUpdate.After(initialCache.LastUpdate) {
		t.Error("LastUpdate should be after initial cache time")
	}
}

func TestCacheBuilderIncrementalProjectFileReuse(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, tmpDir)
	projectDir := filepath.Join(dataDir, "projects", "test-project")
	secondPath := filepath.Join(projectDir, "session-2.jsonl")
	baseTime := time.Now().Add(-1 * time.Hour)
	if err := os.WriteFile(secondPath, []byte(projectRecordJSON("/tmp/test-project", "session-extra", baseTime)+"\n"), 0644); err != nil {
		t.Fatalf("写入第二个 projects jsonl 失败: %v", err)
	}

	cachePath := filepath.Join(tmpDir, "cache.db")
	builder := &CacheBuilder{CachePath: cachePath, DataDir: dataDir}
	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("Initial BuildFullCache() failed: %v", err)
	}

	initialCache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("Load initial cache failed: %v", err)
	}
	if len(initialCache.ProjectFiles) != 2 {
		t.Fatalf("ProjectFiles=%d, want 2", len(initialCache.ProjectFiles))
	}

	firstPath := filepath.Join(projectDir, "session.jsonl")
	appendProjectRecord(t, dataDir, "/tmp/test-project", "session-new", time.Now())
	newTime := time.Now().Add(2 * time.Minute)
	if err := os.Chtimes(firstPath, newTime, newTime); err != nil {
		t.Fatalf("更新第一个 projects 文件修改时间失败: %v", err)
	}

	if err := builder.BuildFullCache(); err != nil {
		t.Fatalf("Second BuildFullCache() failed: %v", err)
	}

	updatedCache, err := LoadCacheFile(cachePath)
	if err != nil {
		t.Fatalf("Load updated cache failed: %v", err)
	}
	if updatedCache.TotalMessages != 4 {
		t.Fatalf("TotalMessages=%d, want 4", updatedCache.TotalMessages)
	}

	relSecond, err := filepath.Rel(dataDir, secondPath)
	if err != nil {
		t.Fatalf("Rel second path failed: %v", err)
	}
	relSecond = filepath.ToSlash(relSecond)
	if updatedCache.ProjectFiles[relSecond] == nil {
		t.Fatalf("Missing cache entry for unchanged file %s", relSecond)
	}
	if updatedCache.ProjectFiles[relSecond].ModTimeUnix != initialCache.ProjectFiles[relSecond].ModTimeUnix {
		t.Fatalf("Unchanged file metadata was not reused")
	}
}

// TestCacheBuilderNeedsRebuild 测试缓存重建检查
func TestCacheBuilderNeedsRebuild(t *testing.T) {
	tests := []struct {
		name             string
		cacheLastUpdate  time.Time
		dataLastModified time.Time
		cacheRulesHash   string
		wantNeedsRebuild bool
	}{
		{
			name:             "缓存是最新的，不需要重建",
			cacheLastUpdate:  time.Now().Add(-1 * time.Hour),
			dataLastModified: time.Now().Add(-2 * time.Hour),
			wantNeedsRebuild: false,
		},
		{
			name:             "规则变更，需要重建",
			cacheLastUpdate:  time.Now().Add(-1 * time.Hour),
			dataLastModified: time.Now().Add(-2 * time.Hour),
			cacheRulesHash:   "stale",
			wantNeedsRebuild: true,
		},
		{
			name:             "数据更新过，需要重建",
			cacheLastUpdate:  time.Now().Add(-2 * time.Hour),
			dataLastModified: time.Now().Add(-1 * time.Hour),
			wantNeedsRebuild: true,
		},
		{
			name:             "缓存版本过旧，需要重建",
			cacheLastUpdate:  time.Now().Add(-1 * time.Hour),
			dataLastModified: time.Now().Add(-2 * time.Hour),
			wantNeedsRebuild: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tmpDir := t.TempDir()
			cachePath := filepath.Join(tmpDir, "cache.db")

			// 创建缓存文件
			rulesHash, err := currentBashRulesHash()
			if err != nil {
				t.Fatalf("Setup: load rules hash failed: %v", err)
			}
			cache := &CacheFile{
				Version:       CacheVersion,
				LastUpdate:    tt.cacheLastUpdate,
				BashRulesHash: rulesHash,
			}
			if tt.cacheRulesHash != "" {
				cache.BashRulesHash = tt.cacheRulesHash
			}
			if tt.name == "缓存版本过旧，需要重建" {
				cache.Version = "1.0"
			}
			if err := cache.Save(cachePath); err != nil {
				t.Fatalf("Setup: Save cache failed: %v", err)
			}

			// 创建数据文件
			dataDir := filepath.Join(tmpDir, "data")
			if err := os.MkdirAll(dataDir, 0755); err != nil {
				t.Fatalf("Setup: Create data dir failed: %v", err)
			}
			projectPath := filepath.Join(dataDir, "projects", "test-project", "session.jsonl")
			if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
				t.Fatalf("Setup: Create projects dir failed: %v", err)
			}
			if err := os.WriteFile(projectPath, []byte("test"), 0644); err != nil {
				t.Fatalf("Setup: Write projects jsonl failed: %v", err)
			}
			// 设置文件修改时间
			if err := os.Chtimes(projectPath, tt.dataLastModified, tt.dataLastModified); err != nil {
				t.Fatalf("Setup: Set file time failed: %v", err)
			}

			builder := &CacheBuilder{
				CachePath: cachePath,
				DataDir:   dataDir,
			}

			// Act
			needsRebuild := builder.NeedsRebuild()

			// Assert
			if needsRebuild != tt.wantNeedsRebuild {
				t.Errorf("NeedsRebuild() = %v, want %v", needsRebuild, tt.wantNeedsRebuild)
			}
		})
	}
}

// TestCacheBuilderGetLastDataModified 测试获取数据最后修改时间
func TestCacheBuilderGetLastDataModified(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")

	// 创建数据目录和文件
	if err := os.MkdirAll(filepath.Join(dataDir, "projects"), 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	// 创建多个文件，设置不同时间
	baseTime := time.Now().Add(-24 * time.Hour)

	files := map[string]time.Time{
		"history.jsonl":       baseTime,
		"stats-cache.json":    baseTime.Add(1 * time.Hour),
		"debug/test.log":      baseTime.Add(2 * time.Hour),
		"projects/test.jsonl": baseTime.Add(3 * time.Hour),
	}

	for file, modTime := range files {
		path := filepath.Join(dataDir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("创建文件失败: %v", err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("设置文件时间失败: %v", err)
		}
	}

	builder := &CacheBuilder{DataDir: dataDir}

	// Act
	lastMod, err := builder.GetLastDataModified()

	// Assert
	if err != nil {
		t.Fatalf("GetLastDataModified() failed: %v", err)
	}

	// 应该返回最新的修改时间
	expected := baseTime.Add(3 * time.Hour)
	if !lastMod.Equal(expected) && !lastMod.Round(time.Second).Equal(expected.Round(time.Second)) {
		t.Errorf("GetLastDataModified() = %v, want %v", lastMod, expected)
	}
}
