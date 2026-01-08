package main

import (
	"reflect"
	"testing"
	"time"
)

// TestMain 设置测试环境
func TestMain(m *testing.M) {
	// 设置数据目录（从 cmd/dashboard/ 到 data/ 需要向上三级）
	cfg.DataDir = "../../../data"
	m.Run()
}

// TestParseSessionStats 测试解析会话统计
func TestParseSessionStats(t *testing.T) {
	// Arrange - 准备测试数据（使用实际数据结构）

	// Act - 调用函数
	stats, err := ParseSessionStats()

	// Assert - 验证结果
	if err != nil {
		t.Fatalf("ParseSessionStats() error = %v", err)
	}

	// 验证基本字段
	if stats.TotalSessions <= 0 {
		t.Errorf("TotalSessions > 0, got %d", stats.TotalSessions)
	}

	if stats.PeakDate == "" {
		t.Error("PeakDate should not be empty")
	}

	if stats.PeakCount <= 0 {
		t.Errorf("PeakCount > 0, got %d", stats.PeakCount)
	}

	if stats.ValleyDate == "" {
		t.Error("ValleyDate should not be empty")
	}

	if stats.ValleyCount < 0 {
		t.Errorf("ValleyCount >= 0, got %d", stats.ValleyCount)
	}

	if len(stats.DailySessionMap) == 0 {
		t.Error("DailySessionMap should not be empty")
	}

	t.Logf("会话统计: 总数=%d, 峰值=%s(%d), 谷值=%s(%d)",
		stats.TotalSessions, stats.PeakDate, stats.PeakCount, stats.ValleyDate, stats.ValleyCount)
}

// TestParseSessionStatsWithTimeFilter 测试带时间过滤的会话统计
func TestParseSessionStatsWithTimeFilter(t *testing.T) {
	// Arrange - 准备时间过滤器（最近7天）
	now := time.Now()
	start := now.AddDate(0, 0, -7)
	filter := TimeFilter{
		Start: &start,
		End:   &now,
	}

	// Act - 调用函数
	stats, err := ParseSessionStatsWithFilter(filter)

	// Assert
	if err != nil {
		t.Fatalf("ParseSessionStatsWithFilter() error = %v", err)
	}

	// 验证过滤后的数据量应该小于或等于总量
	if stats.TotalSessions < 0 {
		t.Errorf("Filtered TotalSessions >= 0, got %d", stats.TotalSessions)
	}

	t.Logf("最近7天会话统计: 总数=%d", stats.TotalSessions)
}

// TestGetDailySessionTrend 测试获取每日会话趋势
func TestGetDailySessionTrend(t *testing.T) {
	// Arrange
	// Act
	dates, counts, err := GetDailySessionTrend()

	// Assert
	if err != nil {
		t.Fatalf("GetDailySessionTrend() error = %v", err)
	}

	if len(dates) != len(counts) {
		t.Errorf("dates and counts length mismatch: %d vs %d", len(dates), len(counts))
	}

	if len(dates) == 0 {
		t.Error("Should have at least one day of data")
	}

	// 验证日期顺序
	for i := 1; i < len(dates); i++ {
		if dates[i] < dates[i-1] {
			t.Errorf("Dates not in ascending order: %s < %s", dates[i], dates[i-1])
		}
	}

	t.Logf("会话趋势: %d 天数据", len(dates))
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
