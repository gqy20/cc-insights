package main

import (
	"testing"
	"time"
)

// TestParseProjectStats 测试项目统计解析功能
func TestParseProjectStats(t *testing.T) {
	// Arrange: 创建测试数据
	tf := TimeFilter{Start: nil, End: nil}

	// Act: 执行解析
	stats, err := ParseProjectStatsWithFilter(tf)

	// Assert: 验证结果
	if err != nil {
		t.Fatalf("ParseProjectStatsWithFilter failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}

	// 验证有数据
	if len(stats.Projects) == 0 {
		t.Log("Warning: No project stats found (may be expected if no data exists)")
	}

	// 验证数据结构
	for _, proj := range stats.Projects {
		if proj.Project == "" {
			t.Error("Project name should not be empty")
		}
		if proj.SessionCount < 0 {
			t.Errorf("SessionCount should be >= 0, got %d", proj.SessionCount)
		}
		if proj.MessageCount < 0 {
			t.Errorf("MessageCount should be >= 0, got %d", proj.MessageCount)
		}
	}

	t.Logf("Found %d projects", len(stats.Projects))
}

// TestParseProjectStatsWithTimeFilter 测试时间过滤功能
func TestParseProjectStatsWithTimeFilter(t *testing.T) {
	// Arrange: 创建7天范围
	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)
	tf := TimeFilter{
		Start: &sevenDaysAgo,
		End:   &now,
	}

	// Act: 执行解析
	stats, err := ParseProjectStatsWithFilter(tf)

	// Assert: 验证结果
	if err != nil {
		t.Fatalf("ParseProjectStatsWithFilter failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}

	t.Logf("Found %d projects in last 7 days", len(stats.Projects))
}

// TestProjectStatsByWeekday 测试星期分布统计
func TestProjectStatsByWeekday(t *testing.T) {
	// Arrange
	tf := TimeFilter{Start: nil, End: nil}

	// Act
	stats, err := ParseProjectStatsByWeekday(tf)

	// Assert
	if err != nil {
		t.Fatalf("ParseProjectStatsByWeekday failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}

	// 验证7天数据
	if len(stats.WeekdayData) != 7 {
		t.Errorf("Expected 7 weekdays, got %d", len(stats.WeekdayData))
	}

	// 验证数据格式
	for i, wd := range stats.WeekdayData {
		if wd.Weekday < 0 || wd.Weekday > 6 {
			t.Errorf("Weekday should be 0-6, got %d at index %d", wd.Weekday, i)
		}
		if wd.MessageCount < 0 {
			t.Errorf("MessageCount should be >= 0, got %d", wd.MessageCount)
		}
	}

	t.Logf("Weekday distribution: %v", stats.WeekdayData)
}

// TestParseDailyActivityFromProjects 测试从projects生成每日活动数据
func TestParseDailyActivityFromProjects(t *testing.T) {
	// Arrange
	tf := TimeFilter{Start: nil, End: nil}

	// Act
	activity, err := ParseDailyActivityFromProjects(tf)

	// Assert
	if err != nil {
		t.Fatalf("ParseDailyActivityFromProjects failed: %v", err)
	}

	if activity == nil {
		t.Fatal("Expected non-nil activity")
	}

	if len(activity) == 0 {
		t.Log("Warning: No daily activity found (may be expected if no data exists)")
	}

	// 验证数据格式
	for _, day := range activity {
		if day.Date == "" {
			t.Error("Date should not be empty")
		}
		if day.MessageCount < 0 {
			t.Errorf("MessageCount should be >= 0, got %d", day.MessageCount)
		}
		if day.SessionCount < 0 {
			t.Errorf("SessionCount should be >= 0, got %d", day.SessionCount)
		}
	}

	t.Logf("Found %d days of activity", len(activity))
}

// TestParseHourlyCountsFromProjects 测试从projects生成小时统计
func TestParseHourlyCountsFromProjects(t *testing.T) {
	// Arrange
	tf := TimeFilter{Start: nil, End: nil}

	// Act
	counts, err := ParseHourlyCountsFromProjects(tf)

	// Assert
	if err != nil {
		t.Fatalf("ParseHourlyCountsFromProjects failed: %v", err)
	}

	if counts == nil {
		t.Fatal("Expected non-nil counts")
	}

	// 验证24小时数据
	if len(counts) != 24 {
		t.Errorf("Expected 24 hours, got %d", len(counts))
	}

	// 验证数据格式
	for hour, count := range counts {
		if hour < 0 || hour > 23 {
			t.Errorf("Hour should be 0-23, got %d", hour)
		}
		if count < 0 {
			t.Errorf("Count should be >= 0, got %d for hour %d", count, hour)
		}
	}

	total := 0
	for _, count := range counts {
		total += count
	}
	t.Logf("Total hourly messages: %d", total)
}
