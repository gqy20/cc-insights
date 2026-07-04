package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var codexExitCodeExpr = regexp.MustCompile(`(?i)(?:exit code|exited with code|exit status)\s+(-?\d+)`)

type codexRecord struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionState struct {
	SessionID    string
	Project      string
	CurrentModel string
	StartedAt    time.Time
	PendingTools map[string]*codexPendingTool
	ActivePlan   bool
}

type codexPendingTool struct {
	pendingToolCall
	ResultRecorded bool
	WeakResultSeen bool
	WeakFailed     bool
	WeakPreview    string
	WeakTimestamp  time.Time
}

type codexPatchChange struct {
	Type        string `json:"type"`
	MovePath    string `json:"move_path"`
	UnifiedDiff string `json:"unified_diff"`
	Content     string `json:"content"`
}

func parseCodexSessionFileAggregate(filePath string, tf TimeFilter, agg *ProjectAggregate) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	state := codexSessionState{
		SessionID:    filePath,
		Project:      "Unknown",
		CurrentModel: "unknown",
		PendingTools: make(map[string]*codexPendingTool),
	}

	decoder := json.NewDecoder(f)
	for {
		var record codexRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		timestamp, hasTimestamp := parseProjectRecordTimestamp(record.Timestamp)

		switch record.Type {
		case "session_meta":
			parseCodexSessionMeta(record.Payload, &state, timestamp)
		case "turn_context":
			parseCodexTurnContext(record.Payload, &state)
		}

		if hasTimestamp && !tf.Contains(timestamp) {
			continue
		}
		if !hasTimestamp && hasTimeFilter(tf) {
			continue
		}

		recordCodexEventType(agg, record.Type)
		if hasTimestamp {
			recordCodexEventType(ensureDailyRuntimeAggregate(agg, timestamp.Format("2006-01-02")), record.Type)
		}

		switch record.Type {
		case "session_meta":
			recordCodexSessionSeen(agg, &state, timestamp)
		case "turn_context":
			recordCodexSessionSeen(agg, &state, timestamp)
		case "response_item":
			parseCodexResponseItem(record.Payload, &state, timestamp, agg)
		case "event_msg":
			parseCodexEventMessage(record.Payload, &state, timestamp, agg)
		}
	}

	for _, pending := range state.PendingTools {
		if pending.ResultRecorded {
			continue
		}
		if pending.WeakResultSeen {
			recordCodexToolResult(agg, pending, codexWeakClassification(pending), pending.WeakPreview, pending.WeakTimestamp, 0)
			continue
		}
		recordCodexMissingTool(agg, pending.pendingToolCall)
	}
}

func parseCodexSessionMeta(raw json.RawMessage, state *codexSessionState, fallback time.Time) {
	var payload struct {
		ID        string `json:"id"`
		SessionID string `json:"session_id"`
		Timestamp string `json:"timestamp"`
		Cwd       string `json:"cwd"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	state.SessionID = firstNonEmpty(payload.ID, payload.SessionID, state.SessionID)
	if payload.Cwd != "" {
		state.Project = payload.Cwd
	}
	if ts, ok := parseProjectRecordTimestamp(payload.Timestamp); ok {
		state.StartedAt = ts
	} else if !fallback.IsZero() {
		state.StartedAt = fallback
	}
}

func parseCodexTurnContext(raw json.RawMessage, state *codexSessionState) {
	var payload struct {
		Cwd   string `json:"cwd"`
		Model string `json:"model"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	if payload.Cwd != "" {
		state.Project = payload.Cwd
	}
	if payload.Model != "" {
		state.CurrentModel = payload.Model
	}
}

func parseCodexResponseItem(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var payload struct {
		Type      string          `json:"type"`
		Role      string          `json:"role"`
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Input     json.RawMessage `json:"input"`
		CallID    string          `json:"call_id"`
		Output    json.RawMessage `json:"output"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}

	switch payload.Type {
	case "message":
		if payload.Role == "assistant" {
			recordCodexAssistantMessage(agg, state, timestamp)
		} else if payload.Role == "user" {
			recordCodexUserMessage(agg, state, timestamp)
		}
	case "function_call", "custom_tool_call":
		input := codexToolInputRaw(payload.Arguments, payload.Input)
		recordCodexToolCall(agg, state, payload.Name, payload.CallID, input, timestamp)
		if payload.Name == "update_plan" {
			recordCodexUpdatePlanEvent(agg, state, input, timestamp)
		}
	case "web_search_call":
		recordCodexToolCall(agg, state, "web_search", payload.CallID, nil, timestamp)
	case "tool_search_call":
		recordCodexToolCall(agg, state, "tool_search", payload.CallID, nil, timestamp)
	case "function_call_output", "custom_tool_call_output":
		recordCodexWeakToolResult(state, payload.CallID, payload.Output, timestamp)
	}
}

func parseCodexEventMessage(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &head) != nil {
		return
	}
	recordCodexEventType(agg, "event_msg:"+head.Type)
	if !timestamp.IsZero() {
		recordCodexEventType(ensureDailyRuntimeAggregate(agg, timestamp.Format("2006-01-02")), "event_msg:"+head.Type)
	}

	switch head.Type {
	case "agent_message":
		recordCodexSessionSeen(agg, state, timestamp)
	case "user_message":
		recordCodexUserMessage(agg, state, timestamp)
	case "task_started":
		recordCodexSessionSeen(agg, state, timestamp)
	case "task_complete":
		recordCodexTaskComplete(raw, state, timestamp, agg)
	case "context_compacted":
		recordCodexContextCompacted(state, timestamp, agg)
	case "token_count":
		recordCodexTokenCount(raw, state, timestamp, agg)
	case "exec_command_end":
		recordCodexExecCommandEnd(raw, state, timestamp, agg)
	case "patch_apply_end":
		recordCodexPatchApplyEnd(raw, state, timestamp, agg)
	case "mcp_tool_call_end":
		recordCodexMCPToolCallEnd(raw, state, timestamp, agg)
	case "turn_aborted":
		recordCodexTurnAborted(raw, state, timestamp, agg)
	}
}

func recordCodexAssistantMessage(agg *ProjectAggregate, state *codexSessionState, timestamp time.Time) {
	if timestamp.IsZero() {
		return
	}
	projectName := codexProject(state)
	sessionID := codexSessionID(state)
	dateKey := timestamp.Format("2006-01-02")

	if agg.ProjectStats[projectName] == nil {
		agg.ProjectStats[projectName] = &ProjectStatItem{Project: projectName}
	}
	agg.ProjectStats[projectName].MessageCount++
	weekday := int(timestamp.Weekday())
	agg.WeekdayData[(weekday+6)%7].MessageCount++
	agg.DailyActivity[dateKey]++
	if agg.DailyProjectCounts[dateKey] == nil {
		agg.DailyProjectCounts[dateKey] = make(map[string]int)
	}
	agg.DailyProjectCounts[dateKey][projectName]++
	if sessionID != "" {
		if agg.DailySessions[dateKey] == nil {
			agg.DailySessions[dateKey] = make(map[string]bool)
		}
		agg.DailySessions[dateKey][sessionID] = true
	}
	hour := timestamp.Hour()
	agg.HourlyCounts[hour]++
	dailyHourlyCounts := agg.DailyHourlyCounts[dateKey]
	dailyHourlyCounts[hour]++
	agg.DailyHourlyCounts[dateKey] = dailyHourlyCounts

	stat := ensureSessionStat(agg, sessionID, projectName)
	if stat != nil {
		recordSessionTimestamp(stat, timestamp)
		stat.AssistantMessageCount++
		stat.MessageCount++
	}
}

func recordCodexUserMessage(agg *ProjectAggregate, state *codexSessionState, timestamp time.Time) {
	stat := ensureSessionStat(agg, codexSessionID(state), codexProject(state))
	if stat == nil {
		return
	}
	recordSessionTimestamp(stat, timestamp)
	stat.UserMessageCount++
	stat.MessageCount++
}

func recordCodexSessionSeen(agg *ProjectAggregate, state *codexSessionState, timestamp time.Time) {
	stat := ensureSessionStat(agg, codexSessionID(state), codexProject(state))
	if stat == nil {
		return
	}
	if !state.StartedAt.IsZero() {
		recordSessionTimestamp(stat, state.StartedAt)
	}
	recordSessionTimestamp(stat, timestamp)
}

func recordCodexToolCall(agg *ProjectAggregate, state *codexSessionState, tool, callID string, input json.RawMessage, timestamp time.Time) {
	if callID == "" {
		callID = tool + ":" + timestamp.Format(time.RFC3339Nano)
	}
	if tool == "" {
		tool = "unknown"
	}
	call := pendingToolCall{
		ID:        callID,
		Tool:      tool,
		Model:     codexModel(state),
		Project:   codexProject(state),
		SessionID: codexSessionID(state),
		Timestamp: timestamp,
		Input:     input,
	}
	state.PendingTools[callID] = &codexPendingTool{pendingToolCall: call}
	for _, target := range codexAggregatesForCall(agg, call) {
		addToolCallLocked(target, call.Tool, call.Model)
		addAgentToolCallLocked(target, call)
		addSessionToolCallLocked(target, call)
		recordStructuredToolInputLocked(target, &call)
	}
	state.PendingTools[callID].pendingToolCall = call
}

func recordCodexWeakToolResult(state *codexSessionState, callID string, output json.RawMessage, timestamp time.Time) {
	if callID == "" {
		return
	}
	pending := state.PendingTools[callID]
	if pending == nil || pending.ResultRecorded {
		return
	}
	text := rawCodexString(output)
	pending.WeakResultSeen = true
	pending.WeakPreview = previewString(text, 240)
	pending.WeakTimestamp = timestamp
	pending.WeakFailed = codexOutputFailed(text)
}

func recordCodexExecCommandEnd(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var payload struct {
		CallID   string `json:"call_id"`
		Command  string `json:"command"`
		Status   string `json:"status"`
		ExitCode *int   `json:"exit_code"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		Duration int64  `json:"duration"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	pending := ensureCodexPendingForResult(state, payload.CallID, "exec_command", timestamp)
	if len(pending.Input) == 0 && payload.Command != "" {
		pending.Input = mustJSONRaw(map[string]string{"cmd": payload.Command})
		for _, target := range codexAggregatesForCall(agg, pending.pendingToolCall) {
			recordStructuredToolInputLocked(target, &pending.pendingToolCall)
		}
	}
	failed := strings.EqualFold(payload.Status, "failed") || (payload.ExitCode != nil && *payload.ExitCode != 0)
	classification := toolFailureClassification{}
	if failed {
		classification = newFailureClassification("command_failed", "bash", "exit_code")
	}
	preview := previewString(strings.TrimSpace(payload.Stderr+"\n"+payload.Stdout), 240)
	recordCodexToolResult(agg, pending, classification, preview, timestamp, payload.Duration)
	delete(state.PendingTools, payload.CallID)
}

func recordCodexPatchApplyEnd(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var payload struct {
		CallID  string                      `json:"call_id"`
		Success *bool                       `json:"success"`
		Status  string                      `json:"status"`
		Stdout  string                      `json:"stdout"`
		Stderr  string                      `json:"stderr"`
		Changes map[string]codexPatchChange `json:"changes"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	pending := ensureCodexPendingForResult(state, payload.CallID, "apply_patch", timestamp)
	failed := payload.Success != nil && !*payload.Success
	if strings.EqualFold(payload.Status, "failed") {
		failed = true
	}
	classification := toolFailureClassification{}
	if failed {
		classification = newFailureClassification("tool_error", "tool", "apply_patch_failed")
	}
	preview := previewString(strings.TrimSpace(payload.Stderr+"\n"+payload.Stdout), 240)
	recordCodexPatchFileOperations(agg, pending, payload.Changes, failed)
	recordCodexToolResult(agg, pending, classification, preview, timestamp, 0)
	delete(state.PendingTools, payload.CallID)
}

func recordCodexMCPToolCallEnd(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var payload struct {
		CallID string          `json:"call_id"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	pending := ensureCodexPendingForResult(state, payload.CallID, "mcp__unknown__unknown", timestamp)
	failed := codexResultHasError(payload.Result)
	classification := toolFailureClassification{}
	if failed {
		classification = newFailureClassification("api_failure", "mcp", "mcp_error")
	}
	recordCodexToolResult(agg, pending, classification, toolResultPreview(payload.Result), timestamp, 0)
	delete(state.PendingTools, payload.CallID)
}

func recordCodexTurnAborted(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var payload struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	stat := ensureSessionStat(agg, codexSessionID(state), codexProject(state))
	if stat == nil {
		return
	}
	recordSessionTimestamp(stat, timestamp)
	stat.Outcome = "aborted"
	stat.StopReason = previewString(payload.Reason, 120)
}

func recordCodexTaskComplete(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var payload struct {
		DurationMs int64 `json:"duration_ms"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	recordCodexSessionSeen(agg, state, timestamp)
	call := pendingToolCall{
		Project:   codexProject(state),
		SessionID: codexSessionID(state),
		Timestamp: timestamp,
	}
	for _, target := range codexAggregatesForCall(agg, call) {
		recordTurnDurationLocked(target, payload.DurationMs, 0, call.SessionID, call.Project, timestamp)
	}
	if state.ActivePlan {
		for _, target := range codexAggregatesForCall(agg, call) {
			recordPlanModeExitLocked(target, "task_complete", call.SessionID)
		}
		state.ActivePlan = false
	}
}

func recordCodexContextCompacted(state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	call := pendingToolCall{
		Project:   codexProject(state),
		SessionID: codexSessionID(state),
		Timestamp: timestamp,
	}
	for _, target := range codexAggregatesForCall(agg, call) {
		ensureCodexContextStats(target).ContextCompactions++
	}
}

func recordCodexTokenCount(raw json.RawMessage, state *codexSessionState, timestamp time.Time, agg *ProjectAggregate) {
	var payload struct {
		Info struct {
			LastTokenUsage struct {
				InputTokens           int `json:"input_tokens"`
				CachedInputTokens     int `json:"cached_input_tokens"`
				OutputTokens          int `json:"output_tokens"`
				ReasoningOutputTokens int `json:"reasoning_output_tokens"`
				TotalTokens           int `json:"total_tokens"`
			} `json:"last_token_usage"`
		} `json:"info"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	tokens := payload.Info.LastTokenUsage.TotalTokens
	if tokens <= 0 {
		return
	}
	model := codexModel(state)
	dateKey := timestamp.Format("2006-01-02")
	projectName := codexProject(state)
	sessionID := codexSessionID(state)
	for _, target := range []*ProjectAggregate{
		agg,
		ensureDailyRuntimeAggregate(agg, dateKey),
		ensureDailyProjectRuntimeAggregate(agg, dateKey, projectName),
		ensureDailySessionRuntimeAggregate(agg, dateKey, sessionID),
	} {
		recordModelUsageLocked(target, model, tokens)
		recordModelTokenBreakdownLocked(target, model, payload.Info.LastTokenUsage.InputTokens, payload.Info.LastTokenUsage.CachedInputTokens, payload.Info.LastTokenUsage.OutputTokens, payload.Info.LastTokenUsage.ReasoningOutputTokens)
		recordCodexSessionTokenUsage(target, sessionID, projectName, model, timestamp, payload.Info.LastTokenUsage.InputTokens, payload.Info.LastTokenUsage.OutputTokens+payload.Info.LastTokenUsage.ReasoningOutputTokens, tokens)
		recordCodexCostUsageLocked(target, sessionID, projectName, model, payload.Info.LastTokenUsage.InputTokens, payload.Info.LastTokenUsage.CachedInputTokens, payload.Info.LastTokenUsage.OutputTokens+payload.Info.LastTokenUsage.ReasoningOutputTokens, tokens)
		recordCodexContextTokenUsage(target, payload.Info.LastTokenUsage.InputTokens, payload.Info.LastTokenUsage.CachedInputTokens, payload.Info.LastTokenUsage.ReasoningOutputTokens, tokens)
	}
	if agg.DailyModelCounts[dateKey] == nil {
		agg.DailyModelCounts[dateKey] = make(map[string]int)
	}
	agg.DailyModelCounts[dateKey][model]++
	if agg.DailyModelTokens[dateKey] == nil {
		agg.DailyModelTokens[dateKey] = make(map[string]int)
	}
	agg.DailyModelTokens[dateKey][model] += tokens
}

func recordCodexCostUsageLocked(agg *ProjectAggregate, sessionID, projectName, model string, inputTokens, cachedInputTokens, outputTokens, totalTokens int) {
	modelStat := ensureCostModelStat(agg, model)
	modelStat.RequestCount++
	modelStat.InputTokens += inputTokens
	modelStat.OutputTokens += outputTokens
	modelStat.CacheReadTokens += cachedInputTokens
	modelStat.TotalTokens += totalTokens

	projectStat := ensureCostProjectStat(agg, projectName)
	projectStat.RequestCount++
	projectStat.InputTokens += inputTokens
	projectStat.OutputTokens += outputTokens
	projectStat.TotalTokens += totalTokens

	if sessionID != "" {
		sessionStat := ensureCostSessionStat(agg, sessionID, projectName)
		sessionStat.RequestCount++
		sessionStat.InputTokens += inputTokens
		sessionStat.OutputTokens += outputTokens
		sessionStat.TotalTokens += totalTokens
		if sessionStat.Model == "" {
			sessionStat.Model = model
		} else if sessionStat.Model != model {
			sessionStat.Model = "mixed"
		}
	}
}

func recordCodexSessionTokenUsage(agg *ProjectAggregate, sessionID, projectName, model string, timestamp time.Time, inputTokens, outputTokens, totalTokens int) {
	stat := ensureSessionStat(agg, sessionID, projectName)
	if stat == nil {
		return
	}
	stat.TotalTokens += totalTokens
	stat.InputTokens += inputTokens
	stat.OutputTokens += outputTokens
	if stat.ModelsUsed == nil {
		stat.ModelsUsed = make(map[string]int)
	}
	stat.ModelsUsed[model]++
	recordSessionTimestamp(stat, timestamp)
}

func recordCodexToolResult(agg *ProjectAggregate, pending *codexPendingTool, classification toolFailureClassification, preview string, timestamp time.Time, durationMs int64) {
	if pending == nil || pending.ResultRecorded {
		return
	}
	pending.ResultRecorded = true
	resultSize := len(preview)
	for _, target := range codexAggregatesForCall(agg, pending.pendingToolCall) {
		addToolResultLocked(target, pending.pendingToolCall, classification, preview, timestamp, durationMs, resultSize)
	}
}

func recordCodexUpdatePlanEvent(agg *ProjectAggregate, state *codexSessionState, input json.RawMessage, timestamp time.Time) {
	if len(input) == 0 {
		return
	}
	var payload struct {
		Plan []struct {
			Step   string `json:"step"`
			Status string `json:"status"`
		} `json:"plan"`
	}
	if json.Unmarshal(input, &payload) != nil || len(payload.Plan) == 0 {
		return
	}
	call := pendingToolCall{
		Project:   codexProject(state),
		SessionID: codexSessionID(state),
		Timestamp: timestamp,
	}
	for _, target := range codexAggregatesForCall(agg, call) {
		if state.ActivePlan {
			recordPlanModeReentryLocked(target, call.SessionID)
		} else {
			recordPlanModeLocked(target, "", false, call.SessionID, call.Project)
		}
		recordCodexPlanSteps(target, payload.Plan)
	}
	state.ActivePlan = true
}

func recordCodexPatchFileOperations(agg *ProjectAggregate, pending *codexPendingTool, changes map[string]codexPatchChange, failed bool) {
	if pending == nil || len(changes) == 0 {
		return
	}
	for _, target := range codexAggregatesForCall(agg, pending.pendingToolCall) {
		for path, change := range changes {
			operation := codexPatchFileOperation(change.Type)
			recordCodexFileOperation(target, operation, path, pending.Model, failed)
			added, deleted := codexPatchLineDelta(change)
			recordCodexFileDiff(target, path, added, deleted)
			if change.MovePath != "" && change.MovePath != path {
				recordCodexFileOperation(target, "Write", change.MovePath, pending.Model, failed)
				recordCodexFileDiff(target, change.MovePath, 0, 0)
			}
		}
	}
}

func recordCodexFileOperation(agg *ProjectAggregate, operation, path, model string, failed bool) {
	if strings.TrimSpace(path) == "" {
		return
	}
	key := operation + "\x00" + path
	if agg.FileOperationStats[key] == nil {
		agg.FileOperationStats[key] = &FileOperationStat{Operation: operation, Path: path}
	}
	stat := agg.FileOperationStats[key]
	stat.CallCount++
	mStat := ensureFileOperationModelStat(agg, operation, path, model)
	mStat.CallCount++
	recordFileHotStatLocked(agg, operation, path)
	if failed {
		stat.FailureCount++
		mStat.FailureCount++
		updateFileHotResultLocked(agg, path, true, false)
		if operation == "Edit" {
			recordFileEditFailureLocked(agg, path, "tool", "apply_patch_failed")
		}
		return
	}
	stat.SuccessCount++
	mStat.SuccessCount++
	updateFileHotResultLocked(agg, path, false, false)
}

func recordCodexFileDiff(agg *ProjectAggregate, path string, addedLines, deletedLines int) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if agg.FileDiffStats == nil {
		agg.FileDiffStats = make(map[string]*FileDiffAgg)
	}
	if agg.FileDiffStats[path] == nil {
		agg.FileDiffStats[path] = &FileDiffAgg{Path: path, Extension: fileExtensionLabel(path)}
	}
	stat := agg.FileDiffStats[path]
	stat.PatchEvents++
	stat.AddedLines += addedLines
	stat.DeletedLines += deletedLines
}

func recordCodexPlanSteps(agg *ProjectAggregate, steps []struct {
	Step   string `json:"step"`
	Status string `json:"status"`
}) {
	if len(steps) == 0 {
		return
	}
	if agg.PlanStepAgg == nil {
		agg.PlanStepAgg = &PlanStepAgg{
			StatusCounts: make(map[string]int),
			StepCounts:   make(map[string]*PlanStepHotAgg),
		}
	}
	agg.PlanStepAgg.UpdateEvents++
	agg.PlanStepAgg.TotalSteps += len(steps)
	if len(steps) > agg.PlanStepAgg.MaxStepsPerUpdate {
		agg.PlanStepAgg.MaxStepsPerUpdate = len(steps)
	}
	for _, step := range steps {
		status := normalizeCodexPlanStatus(step.Status)
		agg.PlanStepAgg.StatusCounts[status]++
		label := previewString(step.Step, 160)
		if label == "" {
			label = "(empty)"
		}
		if agg.PlanStepAgg.StepCounts[label] == nil {
			agg.PlanStepAgg.StepCounts[label] = &PlanStepHotAgg{Step: label}
		}
		item := agg.PlanStepAgg.StepCounts[label]
		item.Count++
		item.LastStatus = status
	}
}

func recordCodexContextTokenUsage(agg *ProjectAggregate, inputTokens, cachedInputTokens, reasoningTokens, totalTokens int) {
	context := ensureCodexContextStats(agg)
	context.TokenEvents++
	context.TotalCachedInputTokens += cachedInputTokens
	context.TotalReasoningTokens += reasoningTokens
	if inputTokens > context.MaxInputTokens {
		context.MaxInputTokens = inputTokens
	}
	if totalTokens > context.MaxTotalTokens {
		context.MaxTotalTokens = totalTokens
	}
}

func ensureCodexContextStats(agg *ProjectAggregate) *ContextAnalysisData {
	if agg.ContextStats == nil {
		agg.ContextStats = &ContextAnalysisData{}
	}
	return agg.ContextStats
}

func codexPatchFileOperation(changeType string) string {
	switch strings.ToLower(strings.TrimSpace(changeType)) {
	case "add":
		return "Write"
	case "update":
		return "Edit"
	case "delete", "remove":
		return "Write"
	case "move", "rename":
		return "Edit"
	default:
		return "Edit"
	}
}

func codexPatchLineDelta(change codexPatchChange) (int, int) {
	if change.UnifiedDiff != "" {
		return countUnifiedDiffLines(change.UnifiedDiff)
	}
	switch strings.ToLower(strings.TrimSpace(change.Type)) {
	case "add":
		return countTextLines(change.Content), 0
	case "delete", "remove":
		return 0, countTextLines(change.Content)
	default:
		return 0, 0
	}
}

func countUnifiedDiffLines(diff string) (int, int) {
	added, deleted := 0, 0
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			deleted++
		}
	}
	return added, deleted
}

func countTextLines(text string) int {
	if text == "" {
		return 0
	}
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func fileExtensionLabel(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return "(none)"
	}
	return ext
}

func normalizeCodexPlanStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "unknown"
	}
	return status
}

func recordCodexMissingTool(agg *ProjectAggregate, call pendingToolCall) {
	for _, target := range codexAggregatesForCall(agg, call) {
		addMissingToolResultLocked(target, call)
	}
}

func ensureCodexPendingForResult(state *codexSessionState, callID, fallbackTool string, timestamp time.Time) *codexPendingTool {
	if callID != "" {
		if pending := state.PendingTools[callID]; pending != nil {
			return pending
		}
	}
	call := pendingToolCall{
		ID:        callID,
		Tool:      fallbackTool,
		Model:     codexModel(state),
		Project:   codexProject(state),
		SessionID: codexSessionID(state),
		Timestamp: timestamp,
	}
	pending := &codexPendingTool{pendingToolCall: call}
	if callID != "" {
		state.PendingTools[callID] = pending
	}
	return pending
}

func codexAggregatesForCall(agg *ProjectAggregate, call pendingToolCall) []*ProjectAggregate {
	dateKey := call.Timestamp.Format("2006-01-02")
	if call.Timestamp.IsZero() {
		return []*ProjectAggregate{agg}
	}
	return []*ProjectAggregate{
		agg,
		ensureDailyRuntimeAggregate(agg, dateKey),
		ensureDailyProjectRuntimeAggregate(agg, dateKey, call.Project),
		ensureDailySessionRuntimeAggregate(agg, dateKey, call.SessionID),
	}
}

func codexArgumentsRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		trimmed := strings.TrimSpace(s)
		if json.Valid([]byte(trimmed)) {
			return json.RawMessage(trimmed)
		}
		return mustJSONRaw(map[string]string{"input": s})
	}
	return raw
}

func codexToolInputRaw(arguments, input json.RawMessage) json.RawMessage {
	if len(arguments) != 0 {
		return codexArgumentsRaw(arguments)
	}
	return codexArgumentsRaw(input)
}

func codexWeakClassification(pending *codexPendingTool) toolFailureClassification {
	if pending == nil || !pending.WeakFailed {
		return toolFailureClassification{}
	}
	return newFailureClassification("error_text", failureCategoryForTool(pending.Tool), "error_text")
}

func codexOutputFailed(text string) bool {
	matches := codexExitCodeExpr.FindStringSubmatch(text)
	if len(matches) == 2 {
		code, err := strconv.Atoi(matches[1])
		return err == nil && code != 0
	}
	return toolFailureExpr.MatchString(text)
}

func codexResultHasError(raw json.RawMessage) bool {
	lower := strings.ToLower(string(raw))
	return strings.Contains(lower, `"error"`) || strings.Contains(lower, `"is_error":true`) || strings.Contains(lower, `"success":false`)
}

func rawCodexString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func mustJSONRaw(v interface{}) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func recordCodexEventType(agg *ProjectAggregate, eventType string) {
	if eventType == "" {
		return
	}
	if agg.EventTypes == nil {
		agg.EventTypes = make(map[string]int)
	}
	agg.EventTypes[eventType]++
}

func codexSessionID(state *codexSessionState) string {
	return firstNonEmpty(state.SessionID, "unknown")
}

func codexProject(state *codexSessionState) string {
	return firstNonEmpty(state.Project, "Unknown")
}

func codexModel(state *codexSessionState) string {
	return firstNonEmpty(state.CurrentModel, "unknown")
}
