package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestParseProjectsConcurrentOnce_CostAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "cost-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("创建测试项目目录失败: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := costAssistantRecord("/tmp/cost-project", "session-a", base, "claude-sonnet-4.5", false, "", 100, 40, 300, 20, 2, 1) + "\n" +
		costAssistantRecord("/tmp/cost-project", "session-a", base.Add(time.Minute), "claude-sonnet-4.5", false, "", 50, 30, 200, 10, 0, 0) + "\n" +
		costAssistantRecord("/tmp/cost-project", "session-b", base.Add(2*time.Minute), "glm-5v-turbo", true, "agent-cost", 80, 25, 40, 5, 1, 0) + "\n" +
		attachmentRecord("/tmp/cost-project", "session-a", base.Add(3*time.Minute), `{"type":"budget_usd","used":0.42,"total":10,"remaining":9.58}`) + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 jsonl 失败: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("ParseProjectsConcurrentOnceFromDir failed: %v", err)
	}

	if agg.CostAnalysis == nil {
		t.Fatal("CostAnalysis should not be nil")
	}

	totals := agg.CostAnalysis.Totals
	if totals.RequestCount != 3 {
		t.Fatalf("RequestCount=%d, want 3", totals.RequestCount)
	}
	if totals.InputTokens != 230 || totals.OutputTokens != 95 || totals.CacheReadInputTokens != 540 || totals.CacheCreationInputTokens != 35 {
		t.Fatalf("unexpected totals: %+v", totals)
	}
	if totals.ServerToolUseRequests != 4 {
		t.Fatalf("ServerToolUseRequests=%d, want 4", totals.ServerToolUseRequests)
	}
	if totals.TotalTokens != 900 {
		t.Fatalf("TotalTokens=%d, want 900", totals.TotalTokens)
	}
	if totals.BillableInputTokens != 265 {
		t.Fatalf("BillableInputTokens=%d, want 265", totals.BillableInputTokens)
	}

	if len(agg.CostAnalysis.ByModel) < 2 || agg.CostAnalysis.ByModel[0].Model != "claude-sonnet-4.5" {
		t.Fatalf("ByModel=%+v, want claude-sonnet-4.5 first", agg.CostAnalysis.ByModel)
	}
	if agg.CostAnalysis.ByModel[0].TotalTokens != 750 {
		t.Fatalf("claude total=%d, want 750", agg.CostAnalysis.ByModel[0].TotalTokens)
	}
	if agg.CostAnalysis.ByModel[0].AvgOutputTokens != 35 {
		t.Fatalf("claude avg output=%f, want 35", agg.CostAnalysis.ByModel[0].AvgOutputTokens)
	}

	if len(agg.CostAnalysis.BySession) < 2 || agg.CostAnalysis.BySession[0].SessionID != "session-a" {
		t.Fatalf("BySession=%+v, want session-a first", agg.CostAnalysis.BySession)
	}
	if agg.CostAnalysis.BySession[0].Model != "claude-sonnet-4.5" {
		t.Fatalf("session-a model=%q, want claude-sonnet-4.5", agg.CostAnalysis.BySession[0].Model)
	}

	agents := make(map[string]CostAgentStat)
	for _, item := range agg.CostAnalysis.ByAgent {
		agents[item.AgentID] = item
	}
	if agents["main"].TotalTokens != 750 {
		t.Fatalf("main agent tokens=%d, want 750", agents["main"].TotalTokens)
	}
	if agents["agent-cost"].TotalTokens != 150 || !agents["agent-cost"].IsSidechain {
		t.Fatalf("agent-cost=%+v, want sidechain with 150 tokens", agents["agent-cost"])
	}

	if len(agg.CostAnalysis.BudgetTimeline) != 1 {
		t.Fatalf("BudgetTimeline len=%d, want 1", len(agg.CostAnalysis.BudgetTimeline))
	}
	if agg.CostAnalysis.BudgetTimeline[0].Used != 0.42 {
		t.Fatalf("budget used=%f, want 0.42", agg.CostAnalysis.BudgetTimeline[0].Used)
	}
}

func costAssistantRecord(cwd, sessionID string, ts time.Time, model string, isSidechain bool, agentID string, input, output, cacheRead, cacheCreation, webSearch, webFetch int) string {
	sidechain := "false"
	if isSidechain {
		sidechain = "true"
	}
	agentField := ""
	if agentID != "" {
		agentField = `,"agentId":"` + agentID + `"`
	}
	return `{"type":"assistant","cwd":"` + cwd + `","sessionId":"` + sessionID + `","isSidechain":` + sidechain + agentField + `,"timestamp":"` + ts.Format(time.RFC3339Nano) + `","message":{"model":"` + model + `","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":` +
		strconv.Itoa(input) + `,"output_tokens":` + strconv.Itoa(output) + `,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) + `,"cache_creation_input_tokens":` + strconv.Itoa(cacheCreation) + `,"server_tool_use":{"web_search_requests":` + strconv.Itoa(webSearch) + `,"web_fetch_requests":` + strconv.Itoa(webFetch) + `}}}}`
}
