package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestParseProjectsConcurrentOnce 测试一次遍历并发解析所有项目统计
// 这是核心聚合函数，取代了以下已删除的独立解析函数：
//   - ParseProjectStatsWithFilter（项目统计）
//   - ParseProjectStatsByWeekday（星期统计）
//   - ParseDailyActivityFromProjects（每日活动）
//   - ParseHourlyCountsFromProjects（小时统计）
//   - ParseModelUsageFromProjects（模型使用）
//   - ParseWorkHoursStats（工作时段）
func TestParseProjectsConcurrentOnce(t *testing.T) {
	// 检查实际数据目录是否存在
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "projects")); os.IsNotExist(err) {
		t.Skip("跳过：projects 数据目录不存在")
	}

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

// TestAssistantMessageThinkingType 测试 AssistantMessage 支持 thinking 类型内容
// 实际 Claude Code 数据中 assistant 消息的 content 包含 thinking 类型
func TestAssistantMessageThinkingType(t *testing.T) {
	// Arrange: 模拟实际数据中的 thinking 类型 JSON
	thinkingJSON := `{
		"id": "msg_test",
		"type": "message",
		"role": "assistant",
		"model": "mimo-v2.5-pro",
		"content": [
			{
				"type": "thinking",
				"thinking": "让我分析这个问题...",
				"signature": ""
			},
			{
				"type": "text",
				"text": "这是回复内容"
			}
		],
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_read_input_tokens": 0
		}
	}`

	// Act: 解析 JSON
	var msg AssistantMessage
	err := json.Unmarshal([]byte(thinkingJSON), &msg)

	// Assert: 解析不应出错
	if err != nil {
		t.Fatalf("Failed to parse assistant message: %v", err)
	}

	// Assert: 应该能识别模型
	if msg.Model != "mimo-v2.5-pro" {
		t.Errorf("Expected model 'mimo-v2.5-pro', got '%s'", msg.Model)
	}

	// Assert: 应该能提取 thinking 内容（通过 Thinking 字段）
	foundThinking := false
	foundText := false
	for _, c := range msg.Content {
		if c.Type == "thinking" && c.Thinking != "" {
			foundThinking = true
			if c.Thinking != "让我分析这个问题..." {
				t.Errorf("Thinking content mismatch, got: %s", c.Thinking)
			}
		}
		if c.Type == "text" && c.Text == "这是回复内容" {
			foundText = true
		}
	}

	if !foundText {
		t.Error("无法提取 text 类型内容")
	}

	if !foundThinking {
		t.Error("无法提取 thinking 类型内容")
	}
}

// TestParseSessionIndex 测试 sessions-index.json 解析功能
// Claude Code 新增了 sessions-index.json 提供更准确的会话统计
func TestParseSessionIndex(t *testing.T) {
	// Arrange: 模拟 sessions-index.json 数据
	sessionIndexJSON := `{
		"version": 1,
		"entries": [
			{
				"sessionId": "abc-123",
				"fullPath": "/test/abc-123.jsonl",
				"fileMtime": 1700000000000,
				"firstPrompt": "测试提示",
				"summary": "测试会话",
				"messageCount": 10,
				"created": "2026-01-07T13:02:00.128Z",
				"modified": "2026-01-07T13:03:41.212Z",
				"projectPath": "/home/test/project",
				"isSidechain": false
			},
			{
				"sessionId": "def-456",
				"fullPath": "/test/def-456.jsonl",
				"fileMtime": 1700000100000,
				"firstPrompt": "另一个提示",
				"summary": "另一个会话",
				"messageCount": 25,
				"created": "2026-01-08T10:00:00.000Z",
				"modified": "2026-01-08T10:30:00.000Z",
				"projectPath": "/home/test/project",
				"isSidechain": false
			}
		]
	}`

	// Act: 解析 JSON（当前应该失败，因为 ParseSessionIndex 尚未实现）
	var result SessionIndexResult
	err := json.Unmarshal([]byte(sessionIndexJSON), &result)

	// Assert: 解析不应出错（结构定义正确）
	if err != nil {
		t.Fatalf("Failed to parse session index: %v", err)
	}

	// Assert: 验证基本字段
	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}

	if result.Entries[0].SessionID != "abc-123" {
		t.Errorf("Expected session ID 'abc-123', got '%s'", result.Entries[0].SessionID)
	}

	if result.Entries[0].MessageCount != 10 {
		t.Errorf("Expected message count 10, got %d", result.Entries[0].MessageCount)
	}

	// 🔴 红阶段: 验证能从实际项目路径解析 sessions-index.json
	// 使用临时目录创建测试文件
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "sessions-index.json")
	if err := os.WriteFile(indexPath, []byte(sessionIndexJSON), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	index, err := ParseSessionIndex(tmpDir)
	if err != nil {
		t.Errorf("❌ FAILED: ParseSessionIndex error: %v (expected success)", err)
		return
	}

	if index == nil {
		t.Error("❌ FAILED: ParseSessionIndex returned nil")
		return
	}

	if len(index.Entries) != 2 {
		t.Errorf("❌ FAILED: Expected 2 entries from file, got %d", len(index.Entries))
	}

	// 验证统计功能
	totalMessages := 0
	for _, entry := range index.Entries {
		totalMessages += entry.MessageCount
	}
	if totalMessages != 35 { // 10 + 25
		t.Errorf("❌ FAILED: Total message count expected 35, got %d", totalMessages)
	}

	t.Logf("✅ SessionIndex 解析成功: %d 个会话, 总消息数 %d", len(index.Entries), totalMessages)
}

// TestParseSessionStatsWithFilter_UseAggregate 测试 ParseSessionStatsWithFilter 使用 aggregate 而非冗余遍历
// 重写后应调用 ParseProjectsConcurrentOnce + extractSessionStatsFromAggregate
func TestParseSessionStatsWithFilter_UseAggregate(t *testing.T) {
	// Arrange: 创建最小化测试数据
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "projects", "p")
	os.MkdirAll(projDir, 0755)

	// 2 个不同 session = 2 个会话
	for i, sid := range []string{"s1", "s2"} {
		content := fmt.Sprintf(
			`{"type":"assistant","message":{"role":"assistant","model":"m-1","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":5,"output_tokens":10}},"timestamp":"2026-01-0%dT10:00:00Z","cwd":"/tmp","sessionId":"%s"}`+"\n",
			i+1, sid,
		)
		os.WriteFile(filepath.Join(projDir, sid+".jsonl"), []byte(content), 0644)
	}

	origDataDir := cfg.DataDir
	cfg.DataDir = tmpDir
	defer func() { cfg.DataDir = origDataDir }()

	tf := TimeFilter{Start: nil, End: nil}

	// Act
	stats, err := ParseSessionStatsWithFilter(tf)

	// Assert
	if err != nil {
		t.Fatalf("ParseSessionStatsWithFilter failed: %v", err)
	}
	if stats == nil {
		t.Fatal("Returned nil stats")
	}

	// 应有 2 个会话（来自 aggregate.DailySessions，非冗余遍历）
	if stats.TotalSessions != 2 {
		t.Errorf("TotalSessions=%d, expected=2 (should come from aggregate, not redundant parse)", stats.TotalSessions)
	}

	t.Logf("✅ ParseSessionStatsWithFilter 正确使用 aggregate: sessions=%d", stats.TotalSessions)
}
