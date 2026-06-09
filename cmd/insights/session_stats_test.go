package main

import (
	"reflect"
	"testing"
	"time"
)

// TestParseSessionStatsWithTimeFilter 测试带时间过滤的会话统计
// 重写后使用 ParseProjectsConcurrentOnce + extractSessionStatsFromAggregate
func TestParseSessionStatsWithTimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := createTestDataDir(t, tmpDir)

	origDataDir := cfg.DataDir
	cfg.DataDir = dataDir
	defer func() { cfg.DataDir = origDataDir }()

	// Arrange - 准备时间过滤器（最近7天）
	now := time.Now()
	start := now.AddDate(0, 0, -7)
	end := now.Add(2 * time.Hour)
	filter := TimeFilter{
		Start: &start,
		End:   &end,
	}

	// Act - 调用函数（内部使用 aggregate 而非冗余遍历）
	stats, err := ParseSessionStatsWithFilter(filter)

	// Assert
	if err != nil {
		t.Fatalf("ParseSessionStatsWithFilter() error = %v", err)
	}

	// 验证 fixture 中的两条会话记录
	if stats.TotalSessions != 2 {
		t.Errorf("Filtered TotalSessions = %d, want 2", stats.TotalSessions)
	}

	t.Logf("最近7天会话统计: 总数=%d", stats.TotalSessions)
}

// TestSessionStatsTypes 测试数据类型
func TestSessionStatsTypes(t *testing.T) {
	stats := SessionStats{
		TotalSessions:   1309,
		PeakDate:        "2026-01-04",
		PeakCount:       23,
		ValleyDate:      "2025-11-19",
		ValleyCount:     2,
		DailySessionMap: map[string]int{"2026-01-04": 23, "2025-11-19": 2},
	}

	// 验证类型
	expectedType := reflect.TypeOf(map[string]int{})
	actualType := reflect.TypeOf(stats.DailySessionMap)

	if actualType != expectedType {
		t.Errorf("DailySessionMap type mismatch: got %v, want %v", actualType, expectedType)
	}
}
