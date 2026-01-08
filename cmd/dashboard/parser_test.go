package main

import (
	"strconv"
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
		hourInt, err := strconv.Atoi(hour)
		if err != nil || hourInt < 0 || hourInt > 23 {
			t.Errorf("Hour should be 00-23, got %s", hour)
		}
		if count < 0 {
			t.Errorf("Count should be >= 0, got %d for hour %s", count, hour)
		}
	}

	total := 0
	for _, count := range counts {
		total += count
	}
	t.Logf("Total hourly messages: %d", total)
}

// TestParseModelUsageFromProjects 测试从projects生成模型使用统计
func TestParseModelUsageFromProjects(t *testing.T) {
	// Arrange
	tf := TimeFilter{Start: nil, End: nil}

	// Act
	usage, err := ParseModelUsageFromProjects(tf)

	// Assert
	if err != nil {
		t.Fatalf("ParseModelUsageFromProjects failed: %v", err)
	}

	if usage == nil {
		t.Fatal("Expected non-nil usage")
	}

	if len(usage) == 0 {
		t.Log("Warning: No model usage found (may be expected if no data exists)")
	}

	// 验证数据格式
	totalRequests := 0
	for _, item := range usage {
		if item.Model == "" {
			t.Error("Model should not be empty")
		}
		if item.Count < 0 {
			t.Errorf("Count should be >= 0, got %d", item.Count)
		}
		if item.Tokens < 0 {
			t.Errorf("Tokens should be >= 0, got %d", item.Tokens)
		}
		totalRequests += item.Count
	}

	t.Logf("Found %d models", len(usage))
	t.Logf("Total requests: %d", totalRequests)
	for _, item := range usage {
		t.Logf("  %s: %d requests, %d tokens", item.Model, item.Count, item.Tokens)
	}
}

// TestParseWorkHoursStats 测试工作时段统计
func TestParseWorkHoursStats(t *testing.T) {
	// Arrange
	tf := TimeFilter{Start: nil, End: nil}

	// Act
	stats, err := ParseWorkHoursStats(tf)

	// Assert
	if err != nil {
		t.Fatalf("ParseWorkHoursStats failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}

	t.Logf("工作时段统计:")
	t.Logf("  工作时段(9-18点): %d 次", stats.WorkHoursCount)
	t.Logf("  非工作时段: %d 次", stats.OffHoursCount)
	t.Logf("  工作时段占比: %.1f%%", stats.WorkHoursRatio)
	t.Logf("  峰值小时: %d点 (%d 次)", stats.PeakHour, stats.PeakHourCount)
}

// TestParseProjectsConcurrentOnce 测试一次遍历并发解析所有项目统计
func TestParseProjectsConcurrentOnce(t *testing.T) {
	// Arrange
	tf := TimeFilter{Start: nil, End: nil}

	// Act
	aggregate, err := ParseProjectsConcurrentOnce(tf)

	// Assert
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnce failed: %v", err)
	}

	if aggregate == nil {
		t.Fatal("Expected non-nil aggregate")
	}

	// 验证项目统计
	if aggregate.ProjectStats == nil {
		t.Error("ProjectStats should not be nil")
	}
	if len(aggregate.ProjectStats) == 0 {
		t.Error("Expected at least one project")
	}

	// 验证星期统计
	if aggregate.WeekdayStats == nil {
		t.Error("WeekdayStats should not be nil")
	}

	// 验证每日活动
	if aggregate.DailyActivityList == nil {
		t.Error("DailyActivityList should not be nil")
	}

	// 验证小时统计
	if aggregate.HourlyData == nil {
		t.Error("HourlyData should not be nil")
	}
	if len(aggregate.HourlyData) != 24 {
		t.Errorf("Expected 24 hours, got %d", len(aggregate.HourlyData))
	}

	// 验证模型使用
	if aggregate.ModelUsageList == nil {
		t.Error("ModelUsageList should not be nil")
	}

	// 验证工作时段统计
	if aggregate.WorkHoursStats == nil {
		t.Error("WorkHoursStats should not be nil")
	}

	// 数据一致性检查
	totalMessages := 0
	for _, proj := range aggregate.Projects {
		totalMessages += proj.MessageCount
	}

	totalFromDaily := 0
	for _, day := range aggregate.DailyActivityList {
		totalFromDaily += day.MessageCount
	}

	if totalMessages != totalFromDaily {
		t.Errorf("数据不一致: 项目总计=%d, 每日总计=%d", totalMessages, totalFromDaily)
	}

	t.Logf("✅ 一次遍历成功获取所有统计数据:")
	t.Logf("  项目数: %d", len(aggregate.Projects))
	t.Logf("  总消息数: %d", totalMessages)
	t.Logf("  天数: %d", len(aggregate.DailyActivityList))
	t.Logf("  模型数: %d", len(aggregate.ModelUsageList))
}
