package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCacheFileCreation 测试缓存文件的创建和保存
func TestCacheFileCreation(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	cache := &CacheFile{
		Version:    "1.0",
		LastUpdate: time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
		TimeRange: TimeRange{
			Start: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 1, 8, 23, 59, 59, 0, time.UTC),
		},
		DailyStats: map[string]*DayAggregate{
			"2026-01-08": {
				Date:          "2026-01-08",
				MessageCount:  100,
				SessionCount:  5,
				ToolCallCount: 20,
			},
		},
		TotalMessages: 1000,
		TotalSessions: 50,
	}

	// Act
	err := cache.Save(cachePath)

	// Assert
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}
}

// TestCacheFileLoad 测试缓存文件的加载
func TestCacheFileLoad(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.db")

	originalCache := &CacheFile{
		Version:       "1.0",
		LastUpdate:    time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
		TotalMessages: 1000,
		TotalSessions: 50,
		DailyStats: map[string]*DayAggregate{
			"2026-01-08": {
				Date:          "2026-01-08",
				MessageCount:  100,
				SessionCount:  5,
				ToolCallCount: 20,
			},
		},
	}

	// 保存原始缓存
	if err := originalCache.Save(cachePath); err != nil {
		t.Fatalf("Setup: Save() failed: %v", err)
	}

	// Act
	loadedCache, err := LoadCacheFile(cachePath)

	// Assert
	if err != nil {
		t.Fatalf("LoadCacheFile() failed: %v", err)
	}

	if loadedCache == nil {
		t.Fatal("Loaded cache is nil")
	}

	// 验证版本
	if loadedCache.Version != originalCache.Version {
		t.Errorf("Version = %s, want %s", loadedCache.Version, originalCache.Version)
	}

	// 验证统计数据
	if loadedCache.TotalMessages != originalCache.TotalMessages {
		t.Errorf("TotalMessages = %d, want %d", loadedCache.TotalMessages, originalCache.TotalMessages)
	}

	if loadedCache.TotalSessions != originalCache.TotalSessions {
		t.Errorf("TotalSessions = %d, want %d", loadedCache.TotalSessions, originalCache.TotalSessions)
	}

	// 验证每日统计
	dayStats, exists := loadedCache.DailyStats["2026-01-08"]
	if !exists {
		t.Fatal("DailyStats[2026-01-08] does not exist")
	}

	if dayStats.MessageCount != 100 {
		t.Errorf("MessageCount = %d, want 100", dayStats.MessageCount)
	}
}

// TestCacheFileNotExists 测试加载不存在的缓存文件
func TestCacheFileNotExists(t *testing.T) {
	// Arrange
	cachePath := "/tmp/nonexistent_cache_12345.db"

	// Act
	_, err := LoadCacheFile(cachePath)

	// Assert
	if err == nil {
		t.Error("Expected error for nonexistent cache file, got nil")
	}
}

// TestCacheFileQueryByTimeRange 测试按时间范围查询缓存数据
func TestCacheFileQueryByTimeRange(t *testing.T) {
	// Arrange
	cache := &CacheFile{
		Version:    "1.0",
		LastUpdate: time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
		DailyStats: map[string]*DayAggregate{
			"2026-01-06": {Date: "2026-01-06", MessageCount: 50, SessionCount: 2},
			"2026-01-07": {Date: "2026-01-07", MessageCount: 80, SessionCount: 3},
			"2026-01-08": {Date: "2026-01-08", MessageCount: 100, SessionCount: 5},
		},
	}

	start := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 8, 23, 59, 59, 0, time.UTC)

	// Act
	result := cache.QueryByTimeRange(start, end)

	// Assert
	if result == nil {
		t.Fatal("QueryByTimeRange() returned nil")
	}

	// 应该只包含 2026-01-07 和 2026-01-08
	expectedMessages := 80 + 100 // 180
	if result.TotalMessages != expectedMessages {
		t.Errorf("TotalMessages = %d, want %d", result.TotalMessages, expectedMessages)
	}

	expectedSessions := 3 + 5 // 8
	if result.TotalSessions != expectedSessions {
		t.Errorf("TotalSessions = %d, want %d", result.TotalSessions, expectedSessions)
	}
}

// TestCacheFileIsExpired 测试缓存过期检查
func TestCacheFileIsExpired(t *testing.T) {
	tests := []struct {
		name             string
		cacheLastUpdate  time.Time
		dataLastModified time.Time
		wantExpired      bool
	}{
		{
			name:             "缓存较新，未过期",
			cacheLastUpdate:  time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
			dataLastModified: time.Date(2026, 1, 8, 11, 0, 0, 0, time.UTC),
			wantExpired:      false,
		},
		{
			name:             "缓存较旧，已过期",
			cacheLastUpdate:  time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC),
			dataLastModified: time.Date(2026, 1, 8, 11, 0, 0, 0, time.UTC),
			wantExpired:      true,
		},
		{
			name:             "时间相同，未过期",
			cacheLastUpdate:  time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
			dataLastModified: time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC),
			wantExpired:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cache := &CacheFile{
				LastUpdate: tt.cacheLastUpdate,
			}

			// Act
			expired := cache.IsExpired(tt.dataLastModified)

			// Assert
			if expired != tt.wantExpired {
				t.Errorf("IsExpired() = %v, want %v", expired, tt.wantExpired)
			}
		})
	}
}

// TestDayAggregateAddRecord 测试DayAggregate添加记录
func TestDayAggregateAddRecord(t *testing.T) {
	// Arrange
	dayAgg := &DayAggregate{
		Date:          "2026-01-08",
		MessageCount:  0,
		SessionCount:  0,
		ToolCallCount: 0,
		HourlyCounts:  [24]int{},
		ProjectCounts: make(map[string]int),
	}

	// Act - 添加一条消息记录
	dayAgg.AddMessage("project-a", 10)

	// Assert
	if dayAgg.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", dayAgg.MessageCount)
	}

	if dayAgg.HourlyCounts[10] != 1 {
		t.Errorf("HourlyCounts[10] = %d, want 1", dayAgg.HourlyCounts[10])
	}

	if dayAgg.ProjectCounts["project-a"] != 1 {
		t.Errorf("ProjectCounts[project-a] = %d, want 1", dayAgg.ProjectCounts["project-a"])
	}
}

// TestTimeRangeContains 测试时间范围包含检查
func TestTimeRangeContains(t *testing.T) {
	tests := []struct {
		name          string
		tr            TimeRange
		testTime      time.Time
		wantContained bool
	}{
		{
			name: "时间在范围内",
			tr: TimeRange{
				Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
			},
			testTime:      time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			wantContained: true,
		},
		{
			name: "时间在范围前",
			tr: TimeRange{
				Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
			},
			testTime:      time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			wantContained: false,
		},
		{
			name: "时间在范围后",
			tr: TimeRange{
				Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
			},
			testTime:      time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			wantContained: false,
		},
		{
			name: "边界条件-开始时间",
			tr: TimeRange{
				Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
			},
			testTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantContained: true,
		},
		{
			name: "边界条件-结束时间",
			tr: TimeRange{
				Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
			},
			testTime:      time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
			wantContained: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			contained := tt.tr.Contains(tt.testTime)

			// Assert
			if contained != tt.wantContained {
				t.Errorf("Contains() = %v, want %v", contained, tt.wantContained)
			}
		})
	}
}
