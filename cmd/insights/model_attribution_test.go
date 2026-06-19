package main

import (
	"testing"
)

// ============================================================
// P1 — command/bash 模型交叉索引
// ============================================================

func TestModelAttribution_BashCommandRecord(t *testing.T) {
	agg := newProjectAggregate()
	call := pendingToolCall{
		Tool:    "Bash",
		Model:   "claude-sonnet-4.5",
		Input:   jsonRaw(`{"command": "git status"}`),
		Project: "/p",
	}
	recordStructuredToolInputLocked(agg, &call)
	addCommandOrFileResultLocked(agg, call, false, false)

	stat := ensureBashCommandModelStat(agg, "git", "claude-sonnet-4.5")
	if stat.CallCount != 1 || stat.SuccessCount != 1 {
		t.Errorf("git model stat = %+v, want CallCount=1 SuccessCount=1", stat)
	}
	if len(agg.BashCommandModelStats) != 1 {
		t.Errorf("BashCommandModelStats len = %d, want 1", len(agg.BashCommandModelStats))
	}
	if _, ok := agg.BashCommandModelStats["claude-opus\x00git"]; ok {
		t.Error("opus git entry should not exist (different model)")
	}
}

func TestModelAttribution_BashCommandResultDistributes(t *testing.T) {
	agg := newProjectAggregate()
	call := pendingToolCall{
		Tool:          "Bash",
		Model:         "claude-sonnet-4.5",
		Input:         jsonRaw(`{"command": "cd /tmp && pnpm test"}`),
		CommandName:   "cd",
		ChainCommands: []string{"cd", "pnpm"},
		Project:       "/p",
	}
	recordStructuredToolInputLocked(agg, &call)
	addCommandOrFileResultLocked(agg, call, true, false) // 失败全局分发

	cdStat := ensureBashCommandModelStat(agg, "cd", "claude-sonnet-4.5")
	pnpmStat := ensureBashCommandModelStat(agg, "pnpm", "claude-sonnet-4.5")
	if cdStat.FailureCount != 1 || pnpmStat.FailureCount != 1 {
		t.Errorf("model failure distribute: cd=%d pnpm=%d, want 1/1",
			cdStat.FailureCount, pnpmStat.FailureCount)
	}
}

// ============================================================
// P1 — file 操作模型交叉索引
// ============================================================

func TestModelAttribution_FileOperationRecord(t *testing.T) {
	agg := newProjectAggregate()
	call := pendingToolCall{
		Tool:    "Read",
		Model:   "claude-sonnet-4.5",
		Input:   jsonRaw(`{"file_path": "/a.go"}`),
		Project: "/p",
	}
	recordStructuredToolInputLocked(agg, &call)
	addCommandOrFileResultLocked(agg, call, false, true) // missing

	stat := ensureFileOperationModelStat(agg, "Read", "/a.go", "claude-sonnet-4.5")
	if stat.CallCount != 1 || stat.MissingResultCount != 1 {
		t.Errorf("file op model stat = %+v, want CallCount=1 Missing=1", stat)
	}
}

// ============================================================
// P1 — agent 模型交叉索引
// ============================================================

func TestModelAttribution_AgentRecord(t *testing.T) {
	agg := newProjectAggregate()
	call := pendingToolCall{
		Tool:        "Task",
		Model:       "claude-sonnet-4.5",
		AgentID:     "general-purpose",
		IsSidechain: true,
		Project:     "/p",
	}
	addAgentToolCallLocked(agg, call)
	addAgentToolResultLocked(agg, call, true, false)

	stat := ensureAgentModelStat(agg, "general-purpose", "claude-sonnet-4.5", true)
	if stat.ToolCallCount != 1 || stat.ToolFailureCount != 1 || !stat.IsSidechain {
		t.Errorf("agent model stat = %+v, want 1 call/1 fail/sidechain", stat)
	}
}

// ============================================================
// P2 — session 诚实集合
// ============================================================

func TestModelAttribution_SessionModelsUsed(t *testing.T) {
	agg := newProjectAggregate()
	calls := []pendingToolCall{
		{SessionID: "s1", Project: "/p", Model: "claude-sonnet-4.5"},
		{SessionID: "s1", Project: "/p", Model: "claude-sonnet-4.5"},
		{SessionID: "s1", Project: "/p", Model: "claude-opus-4"},
	}
	for _, c := range calls {
		addSessionToolCallLocked(agg, c)
	}
	stat := ensureSessionStat(agg, "s1", "/p")
	if stat.ModelsUsed["claude-sonnet-4.5"] != 2 || stat.ModelsUsed["claude-opus-4"] != 1 {
		t.Errorf("ModelsUsed = %+v, want sonnet=2 opus=1", stat.ModelsUsed)
	}
	finalizeSessionItem(stat)
	if stat.PrimaryModel != "claude-sonnet-4.5" {
		t.Errorf("PrimaryModel = %q, want claude-sonnet-4.5", stat.PrimaryModel)
	}
}

func TestPrimarySessionModel_TieBreakByName(t *testing.T) {
	got := primarySessionModel(map[string]int{"zzz": 1, "aaa": 1})
	if got != "aaa" {
		t.Errorf("tie-break primary = %q, want aaa", got)
	}
}

// ============================================================
// merge 跨 model 去重累加
// ============================================================

func TestModelAttribution_MergeDedup(t *testing.T) {
	a := newProjectAggregate()
	b := newProjectAggregate()
	for _, agg := range []*ProjectAggregate{a, b} {
		call := pendingToolCall{
			Tool:    "Bash",
			Model:   "claude-sonnet-4.5",
			Input:   jsonRaw(`{"command": "git status"}`),
			Project: "/p",
		}
		recordStructuredToolInputLocked(agg, &call)
		addCommandOrFileResultLocked(agg, call, false, false)
	}
	mergeProjectAggregate(a, b)
	stat := ensureBashCommandModelStat(a, "git", "claude-sonnet-4.5")
	if stat.CallCount != 2 || stat.SuccessCount != 2 {
		t.Errorf("after merge git model stat = %+v, want 2/2", stat)
	}
	if len(a.BashCommandModelStats) != 1 {
		t.Errorf("BashCommandModelStats len after merge = %d, want 1 (deduped)", len(a.BashCommandModelStats))
	}
}

// ============================================================
// finalize 生成 ByModel
// ============================================================

func TestModelAttribution_FinalizeByModel(t *testing.T) {
	agg := newProjectAggregate()
	for _, m := range []string{"claude-sonnet-4.5", "claude-opus-4"} {
		call := pendingToolCall{
			Tool:    "Bash",
			Model:   m,
			Input:   jsonRaw(`{"command": "git status"}`),
			Project: "/p",
		}
		recordStructuredToolInputLocked(agg, &call)
	}
	agg.finalizeCommandAnalysis()
	if agg.CommandAnalysis == nil {
		t.Fatal("CommandAnalysis nil")
	}
	if len(agg.CommandAnalysis.ByModel) != 2 {
		t.Errorf("CommandAnalysis.ByModel len = %d, want 2", len(agg.CommandAnalysis.ByModel))
	}
}

// ============================================================
// P3 — filterModels 从 ByModel 重建主列表
// ============================================================

func TestFilterModels_RebuildFromByModel(t *testing.T) {
	data := &DashboardData{
		CommandAnalysis: &CommandAnalysisData{
			ByModel: []BashCommandModelStat{
				{Model: "claude-sonnet-4.5", CommandName: "git", CallCount: 10, FailureCount: 2},
				{Model: "claude-opus-4", CommandName: "git", CallCount: 5},
			},
			BashCommands: []BashCommandStat{{CommandName: "git", CallCount: 15}},
		},
		FileAnalysis: &FileAnalysisData{
			ByModel: []FileOperationModelStat{
				{Model: "claude-sonnet-4.5", Operation: "Read", Path: "/a.go", CallCount: 3},
				{Model: "claude-opus-4", Operation: "Read", Path: "/b.go", CallCount: 1},
			},
		},
		AgentAnalysis: &AgentAnalysisData{
			ByModel: []AgentModelStat{
				{Model: "claude-sonnet-4.5", AgentID: "a1", ToolCallCount: 4},
				{Model: "claude-opus-4", AgentID: "a2", ToolCallCount: 1},
			},
		},
	}
	filterModels(data, "sonnet")

	if len(data.CommandAnalysis.BashCommands) != 1 || data.CommandAnalysis.BashCommands[0].CallCount != 10 {
		t.Errorf("BashCommands rebuilt = %+v, want 1 item CallCount=10", data.CommandAnalysis.BashCommands)
	}
	if len(data.CommandAnalysis.ByModel) != 1 || data.CommandAnalysis.ByModel[0].Model != "claude-sonnet-4.5" {
		t.Errorf("CommandAnalysis.ByModel = %+v, want only sonnet", data.CommandAnalysis.ByModel)
	}
	if len(data.CommandAnalysis.FileOperations) != 1 || data.CommandAnalysis.FileOperations[0].Path != "/a.go" {
		t.Errorf("FileOperations rebuilt = %+v, want /a.go", data.CommandAnalysis.FileOperations)
	}
	if len(data.AgentAnalysis.ByModel) != 1 || data.AgentAnalysis.ByModel[0].AgentID != "a1" {
		t.Errorf("AgentAnalysis.ByModel = %+v, want only a1", data.AgentAnalysis.ByModel)
	}
}

// ============================================================
// P3 — filterSessions 按 ModelsUsed 过滤（诚实集合）
// ============================================================

func TestFilterSessions_ByModelsUsed(t *testing.T) {
	data := &DashboardData{
		SessionAnalysis: &SessionAnalysisData{
			Sessions: []SessionAnalysisItem{
				{SessionID: "s1", ModelsUsed: map[string]int{"claude-sonnet-4.5": 3, "claude-opus-4": 1}},
				{SessionID: "s2", ModelsUsed: map[string]int{"claude-opus-4": 2}},
			},
		},
	}
	filterSessions(data, AnalysisFilter{Model: "sonnet"})
	if len(data.SessionAnalysis.Sessions) != 1 || data.SessionAnalysis.Sessions[0].SessionID != "s1" {
		t.Errorf("Sessions after model filter = %+v, want only s1 (used sonnet, mixed kept)",
			data.SessionAnalysis.Sessions)
	}
}

// ============================================================
// P3 — applyPrecisionGuard：model 筛选保留有归因的模块
// ============================================================

func TestApplyPrecisionGuard_KeepsModelAttributedModules(t *testing.T) {
	data := &DashboardData{
		ToolAnalysis:     &ToolAnalysisData{},
		FailureAnalysis:  &FailureAnalysisData{},
		SessionAnalysis:  &SessionAnalysisData{Sessions: []SessionAnalysisItem{{SessionID: "s1"}}},
		FileAnalysis:     &FileAnalysisData{},
		AgentAnalysis:    &AgentAnalysisData{},
		EventAnalysis:    &EventAnalysisData{},
		SkillAnalysis:    &SkillAnalysisData{},
		TaskPlanAnalysis: &TaskPlanAnalysisData{},
	}
	applyPrecisionGuard(data, AnalysisFilter{Model: "sonnet"})
	if data.SessionAnalysis == nil || data.FileAnalysis == nil || data.AgentAnalysis == nil {
		t.Error("model filter should keep session/file/agent analysis (have model index)")
	}
	if data.EventAnalysis != nil || data.SkillAnalysis != nil || data.TaskPlanAnalysis != nil {
		t.Error("model filter should clear event/skill/taskplan (no model attribution anchor)")
	}
}

// ============================================================
// 快照往返：覆盖缓存序列化/反序列化路径
// （QueryByTimeRange 经 projectFileAggregateToAggregate 还原每日快照）
// 防止 model map 在快照往返中丢失导致 ByModel=[] 的回归
// ============================================================

func TestModelAttribution_SnapshotRoundTrip(t *testing.T) {
	agg := newProjectAggregate()
	for _, m := range []string{"claude-sonnet-4.5", "claude-opus-4"} {
		call := pendingToolCall{
			Tool:    "Bash",
			Model:   m,
			Input:   jsonRaw(`{"command": "git status"}`),
			Project: "/p",
		}
		recordStructuredToolInputLocked(agg, &call)
		addCommandOrFileResultLocked(agg, call, false, false)
	}
	// 正向快照 → 反向还原（模拟缓存命中路径）
	snapshot := aggregateToProjectFileAggregate(agg)
	restored := projectFileAggregateToAggregate(snapshot)
	restored.finalizeCommandAnalysis()

	if restored.CommandAnalysis == nil || len(restored.CommandAnalysis.ByModel) != 2 {
		t.Fatalf("round-trip CommandAnalysis.ByModel len = %d, want 2 (model maps must survive snapshot restore)",
			len(restored.CommandAnalysis.ByModel))
	}
	for _, m := range restored.CommandAnalysis.ByModel {
		if m.CallCount != 1 {
			t.Errorf("round-trip model %s CallCount = %d, want 1", m.Model, m.CallCount)
		}
	}
}
