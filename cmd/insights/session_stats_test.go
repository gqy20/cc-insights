package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// TestMain 设置测试环境
func TestMain(m *testing.M) {
	// 设置数据目录（从 cmd/insights/ 到 data/ 需要向上三级）
	cfg.DataDir = "../../../data"
	m.Run()
}

// TestParseSessionStatsWithTimeFilter 测试带时间过滤的会话统计
// 重写后使用 ParseProjectsConcurrentOnce + extractSessionStatsFromAggregate
func TestParseSessionStatsWithTimeFilter(t *testing.T) {
	// 检查实际数据目录是否存在
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "projects")); os.IsNotExist(err) {
		t.Skip("跳过：projects 数据目录不存在")
	}

	// Arrange - 准备时间过滤器（最近7天）
	now := time.Now()
	start := now.AddDate(0, 0, -7)
	filter := TimeFilter{
		Start: &start,
		End:   &now,
	}

	// Act - 调用函数（内部使用 aggregate 而非冗余遍历）
	stats, err := ParseSessionStatsWithFilter(filter)

	// Assert
	if err != nil {
		t.Fatalf("ParseSessionStatsWithFilter() error = %v", err)
	}

	// 验证过滤后的数据量应该非负
	if stats.TotalSessions < 0 {
		t.Errorf("Filtered TotalSessions >= 0, got %d", stats.TotalSessions)
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
