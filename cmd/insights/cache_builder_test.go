package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestCacheBuilderBuildFullCache 测试构建完整缓存
func TestCacheBuilderBuildFullCache(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()

	// 创建测试数据目录
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("创建测试数据目录失败: %v", err)
	}

	// 创建最小的测试数据
	historyPath := filepath.Join(dataDir, "history.jsonl")
	historyContent := `{"display":"test","timestamp":` + strconv.FormatInt(time.Now().UnixMilli(), 10) + `,"project":"test-project"}
{"display":"/help","timestamp":` + strconv.FormatInt(time.Now().UnixMilli(), 10) + `,"project":"test-project"}
`
	if err := os.WriteFile(historyPath, []byte(historyContent), 0644); err != nil {
		t.Fatalf("创建测试history.jsonl失败: %v", err)
	}

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
	if cache.Version == "" {
		t.Error("Cache version is empty")
	}

	if cache.TotalMessages == 0 {
		t.Error("TotalMessages should be > 0")
	}
}

// TestCacheBuilderIncrementalUpdate 测试增量更新缓存
func TestCacheBuilderIncrementalUpdate(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	cachePath := filepath.Join(tmpDir, "cache.db")

	// 创建数据目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("创建数据目录失败: %v", err)
	}

	// 创建初始数据
	baseTime := time.Now().Add(-2 * 24 * time.Hour)
	historyPath := filepath.Join(dataDir, "history.jsonl")
	initialContent := `{"display":"old","timestamp":` + strconv.FormatInt(baseTime.UnixMilli(), 10) + `,"project":"test"}
`
	if err := os.WriteFile(historyPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("创建初始数据失败: %v", err)
	}

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
	newContent := `
{"display":"new1","timestamp":` + strconv.FormatInt(newTime.UnixMilli(), 10) + `,"project":"test"}
{"display":"new2","timestamp":` + strconv.FormatInt(newTime.Add(time.Second).UnixMilli(), 10) + `,"project":"test"}
`
	if err := os.WriteFile(historyPath, []byte(initialContent+newContent), 0644); err != nil {
		t.Fatalf("添加新数据失败: %v", err)
	}

	// Act - 增量更新
	err := builder.IncrementalUpdate()

	// Assert
	if err != nil {
		t.Fatalf("IncrementalUpdate() failed: %v", err)
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

// TestCacheBuilderNeedsRebuild 测试缓存重建检查
func TestCacheBuilderNeedsRebuild(t *testing.T) {
	tests := []struct {
		name             string
		cacheLastUpdate  time.Time
		dataLastModified time.Time
		wantNeedsRebuild bool
	}{
		{
			name:             "缓存是最新的，不需要重建",
			cacheLastUpdate:  time.Now().Add(-1 * time.Hour),
			dataLastModified: time.Now().Add(-2 * time.Hour),
			wantNeedsRebuild: false,
		},
		{
			name:             "数据更新过，需要重建",
			cacheLastUpdate:  time.Now().Add(-2 * time.Hour),
			dataLastModified: time.Now().Add(-1 * time.Hour),
			wantNeedsRebuild: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tmpDir := t.TempDir()
			cachePath := filepath.Join(tmpDir, "cache.db")

			// 创建缓存文件
			cache := &CacheFile{
				Version:    "1.0",
				LastUpdate: tt.cacheLastUpdate,
			}
			if err := cache.Save(cachePath); err != nil {
				t.Fatalf("Setup: Save cache failed: %v", err)
			}

			// 创建数据文件
			dataDir := filepath.Join(tmpDir, "data")
			if err := os.MkdirAll(dataDir, 0755); err != nil {
				t.Fatalf("Setup: Create data dir failed: %v", err)
			}
			historyPath := filepath.Join(dataDir, "history.jsonl")
			if err := os.WriteFile(historyPath, []byte("test"), 0644); err != nil {
				t.Fatalf("Setup: Write history failed: %v", err)
			}
			// 设置文件修改时间
			if err := os.Chtimes(historyPath, tt.dataLastModified, tt.dataLastModified); err != nil {
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
