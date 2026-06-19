package main

import (
	"encoding/json"
	"sort"
	"time"
)

func ensureSessionStat(agg *ProjectAggregate, sessionID, project string) *SessionAnalysisItem {
	if sessionID == "" {
		return nil
	}
	if agg.SessionStatsMap == nil {
		agg.SessionStatsMap = make(map[string]*SessionAnalysisItem)
	}
	if agg.SessionStatsMap[sessionID] == nil {
		agg.SessionStatsMap[sessionID] = &SessionAnalysisItem{
			SessionID: sessionID,
			Project:   project,
			Outcome:   "unknown",
		}
	}
	stat := agg.SessionStatsMap[sessionID]
	if stat.Project == "" || stat.Project == "Unknown" {
		stat.Project = project
	}
	return stat
}

func mergeSessionStat(dst, src *SessionAnalysisItem) {
	if dst.Project == "" || dst.Project == "Unknown" {
		dst.Project = src.Project
	}
	if src.TitleSource == "custom-title" || dst.Title == "" {
		dst.Title = src.Title
		dst.TitleSource = src.TitleSource
	}
	if dst.FirstPromptPreview == "" {
		dst.FirstPromptPreview = src.FirstPromptPreview
	}
	if src.LastPromptPreview != "" {
		dst.LastPromptPreview = src.LastPromptPreview
	}
	if dst.StartedAt == "" || (src.StartedAt != "" && src.StartedAt < dst.StartedAt) {
		dst.StartedAt = src.StartedAt
	}
	if dst.EndedAt == "" || src.EndedAt > dst.EndedAt {
		dst.EndedAt = src.EndedAt
	}
	dst.DurationMs += src.DurationMs
	dst.SystemDurationMs += src.SystemDurationMs
	dst.MessageCount += src.MessageCount
	dst.AssistantMessageCount += src.AssistantMessageCount
	dst.UserMessageCount += src.UserMessageCount
	dst.SystemEventCount += src.SystemEventCount
	dst.ToolCallCount += src.ToolCallCount
	dst.ToolFailureCount += src.ToolFailureCount
	dst.MissingResultCount += src.MissingResultCount
	dst.TotalTokens += src.TotalTokens
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.PermissionModeChanges += src.PermissionModeChanges
	dst.ModeChanges += src.ModeChanges
	dst.PlanModeCount += src.PlanModeCount
	dst.PlanModeExitCount += src.PlanModeExitCount
	dst.QueueOperationCount += src.QueueOperationCount
	dst.HookCount += src.HookCount
	dst.HookErrorCount += src.HookErrorCount
	if src.LastPermissionMode != "" {
		dst.LastPermissionMode = src.LastPermissionMode
	}
	if src.LastMode != "" {
		dst.LastMode = src.LastMode
	}
	if dst.QueueOperationsSample == "" {
		dst.QueueOperationsSample = src.QueueOperationsSample
	}
	if src.StopReason != "" {
		dst.StopReason = src.StopReason
	}
	if src.PreventedContinuation {
		dst.PreventedContinuation = true
	}
	if src.Outcome != "" && src.Outcome != "unknown" {
		dst.Outcome = src.Outcome
	}
	for model, count := range src.ModelsUsed {
		if dst.ModelsUsed == nil {
			dst.ModelsUsed = make(map[string]int)
		}
		dst.ModelsUsed[model] += count
	}
}

func recordSessionRecordLocked(agg *ProjectAggregate, record ProjectRecord, timestamp time.Time, projectName string) {
	stat := ensureSessionStat(agg, record.SessionID, projectName)
	if stat == nil {
		return
	}
	recordSessionTimestamp(stat, timestamp)

	switch record.Type {
	case "assistant":
		stat.AssistantMessageCount++
		stat.MessageCount++
	case "user":
		stat.UserMessageCount++
		stat.MessageCount++
	case "system":
		stat.SystemEventCount++
		recordSessionSystemEvent(stat, record)
	case "last-prompt":
		prompt := record.LastPrompt
		if prompt == "" {
			prompt = rawStringField(record.Content, "lastPrompt")
		}
		if prompt == "" {
			prompt = rawStringField(record.Message, "lastPrompt")
		}
		if prompt != "" {
			preview := previewString(prompt, 220)
			if stat.FirstPromptPreview == "" {
				stat.FirstPromptPreview = preview
			}
			stat.LastPromptPreview = preview
			if stat.Title == "" {
				stat.Title = previewString(prompt, 80)
				stat.TitleSource = "last-prompt"
			}
		}
	case "ai-title":
		recordSessionTitle(stat, "ai-title", firstNonEmpty(record.AITitle, rawStringField(record.Content, "aiTitle"), rawStringField(record.Message, "aiTitle")))
	case "custom-title":
		recordSessionTitle(stat, "custom-title", firstNonEmpty(record.CustomTitle, rawStringField(record.Content, "customTitle"), rawStringField(record.Message, "customTitle")))
	case "queue-operation":
		operation := firstNonEmpty(record.Operation, rawStringField(record.Content, "operation"), rawStringField(record.Message, "operation"))
		content := firstNonEmpty(rawStringField(record.Content, "content"), rawStringField(record.Message, "content"), rawJSONText(record.Content))
		recordSessionQueueOperation(agg, stat, operation, content)
	case "permission-mode":
		mode := firstNonEmpty(record.PermissionMode, rawStringField(record.Content, "mode"), rawStringField(record.Message, "permissionMode"))
		if mode == "" {
			mode = "unknown"
		}
		stat.PermissionModeChanges++
		stat.LastPermissionMode = mode
	case "mode":
		mode := firstNonEmpty(record.Mode, rawStringField(record.Content, "mode"), rawStringField(record.Message, "mode"))
		if mode == "" {
			mode = "unknown"
		}
		stat.ModeChanges++
		stat.LastMode = mode
	case "result":
		recordSessionResult(stat, record)
	case "started":
		if stat.Outcome == "unknown" {
			stat.Outcome = "started"
		}
	case "attachment":
		recordSessionAttachment(stat, record)
	}
}

func recordSessionTimestamp(stat *SessionAnalysisItem, timestamp time.Time) {
	if timestamp.IsZero() {
		return
	}
	value := timestamp.Format(time.RFC3339)
	if stat.StartedAt == "" || value < stat.StartedAt {
		stat.StartedAt = value
	}
	if stat.EndedAt == "" || value > stat.EndedAt {
		stat.EndedAt = value
	}
}

func recordSessionTitle(stat *SessionAnalysisItem, source, title string) {
	if title == "" {
		return
	}
	if stat.TitleSource == "custom-title" && source != "custom-title" {
		return
	}
	stat.Title = previewString(title, 120)
	stat.TitleSource = source
}

func recordSessionQueueOperation(agg *ProjectAggregate, stat *SessionAnalysisItem, operation, content string) {
	if operation == "" {
		operation = "unknown"
	}
	stat.QueueOperationCount++
	if agg.SessionQueueOps == nil {
		agg.SessionQueueOps = make(map[string]int)
	}
	agg.SessionQueueOps[operation]++
	if stat.QueueOperationsSample == "" && content != "" {
		stat.QueueOperationsSample = previewString(content, 160)
	}
}

func recordSessionSystemEvent(stat *SessionAnalysisItem, record ProjectRecord) {
	if record.Subtype == "turn_duration" {
		stat.SystemDurationMs += int64(record.DurationMs)
		if record.MessageCount > stat.MessageCount {
			stat.MessageCount = record.MessageCount
		}
	}
	if record.Subtype == "stop_hook_summary" {
		stat.HookCount += record.HookCount
		stat.HookErrorCount += len(record.HookErrors)
		if record.StopReason != "" {
			stat.StopReason = record.StopReason
		}
		if record.PreventedContinuation {
			stat.PreventedContinuation = true
			stat.Outcome = "interrupted"
		}
	}
}

func recordSessionResult(stat *SessionAnalysisItem, record ProjectRecord) {
	status := firstNonEmpty(record.Status, rawStringField(record.Content, "status"), rawStringField(record.Message, "status"))
	if status == "" {
		status = firstNonEmpty(rawStringField(record.Content, "result"), rawStringField(record.Message, "result"))
	}
	if status != "" {
		stat.Outcome = normalizeSessionOutcome(status)
	}
}

func recordSessionAttachment(stat *SessionAnalysisItem, record ProjectRecord) {
	var attachment struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(record.Attachment, &attachment); err != nil {
		return
	}
	switch attachment.Type {
	case "plan_mode":
		stat.PlanModeCount++
	case "plan_mode_exit":
		stat.PlanModeExitCount++
	}
}

func addSessionToolCallLocked(agg *ProjectAggregate, call pendingToolCall) {
	stat := ensureSessionStat(agg, call.SessionID, call.Project)
	if stat == nil {
		return
	}
	stat.ToolCallCount++
	if stat.ModelsUsed == nil {
		stat.ModelsUsed = make(map[string]int)
	}
	stat.ModelsUsed[call.Model]++
}

func addSessionToolResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, missing bool) {
	stat := ensureSessionStat(agg, call.SessionID, call.Project)
	if stat == nil {
		return
	}
	if failed {
		stat.ToolFailureCount++
	}
	if missing {
		stat.MissingResultCount++
	}
}

func addSessionCostUsageLocked(agg *ProjectAggregate, record ProjectRecord, projectName string, inputTokens, outputTokens, totalTokens int) {
	stat := ensureSessionStat(agg, record.SessionID, projectName)
	if stat == nil {
		return
	}
	stat.InputTokens += inputTokens
	stat.OutputTokens += outputTokens
	stat.TotalTokens += totalTokens
}

func (agg *ProjectAggregate) finalizeSessionAnalysis() {
	analysis := &SessionAnalysisData{
		Sessions:        make([]SessionAnalysisItem, 0, len(agg.SessionStatsMap)),
		QueueOperations: make([]QueueOperationStat, 0),
		Titles:          make([]SessionTitleStat, 0),
	}
	outcomes := make(map[string]int)
	titles := make(map[string]int)

	for _, stat := range agg.SessionStatsMap {
		statCopy := *stat
		finalizeSessionItem(&statCopy)
		analysis.Sessions = append(analysis.Sessions, statCopy)
		outcomes[statCopy.Outcome]++
		if statCopy.TitleSource != "" {
			titles[statCopy.TitleSource]++
		}
	}

	sort.Slice(analysis.Sessions, func(i, j int) bool {
		if analysis.Sessions[i].EndedAt == analysis.Sessions[j].EndedAt {
			return analysis.Sessions[i].SessionID < analysis.Sessions[j].SessionID
		}
		return analysis.Sessions[i].EndedAt > analysis.Sessions[j].EndedAt
	})
	analysis.TopFailures = append([]SessionAnalysisItem(nil), analysis.Sessions...)
	sort.Slice(analysis.TopFailures, func(i, j int) bool {
		if analysis.TopFailures[i].ToolFailureCount == analysis.TopFailures[j].ToolFailureCount {
			return analysis.TopFailures[i].ToolCallCount > analysis.TopFailures[j].ToolCallCount
		}
		return analysis.TopFailures[i].ToolFailureCount > analysis.TopFailures[j].ToolFailureCount
	})
	analysis.LongRunning = append([]SessionAnalysisItem(nil), analysis.Sessions...)
	sort.Slice(analysis.LongRunning, func(i, j int) bool {
		return analysis.LongRunning[i].DurationMs > analysis.LongRunning[j].DurationMs
	})
	for outcome, count := range outcomes {
		analysis.Outcomes = append(analysis.Outcomes, SessionOutcomeStat{Outcome: outcome, Count: count})
	}
	sort.Slice(analysis.Outcomes, func(i, j int) bool {
		if analysis.Outcomes[i].Count == analysis.Outcomes[j].Count {
			return analysis.Outcomes[i].Outcome < analysis.Outcomes[j].Outcome
		}
		return analysis.Outcomes[i].Count > analysis.Outcomes[j].Count
	})
	for operation, count := range agg.SessionQueueOps {
		analysis.QueueOperations = append(analysis.QueueOperations, QueueOperationStat{Operation: operation, Count: count})
	}
	sort.Slice(analysis.QueueOperations, func(i, j int) bool {
		if analysis.QueueOperations[i].Count == analysis.QueueOperations[j].Count {
			return analysis.QueueOperations[i].Operation < analysis.QueueOperations[j].Operation
		}
		return analysis.QueueOperations[i].Count > analysis.QueueOperations[j].Count
	})
	for source, count := range titles {
		analysis.Titles = append(analysis.Titles, SessionTitleStat{Source: source, Count: count})
	}
	sort.Slice(analysis.Titles, func(i, j int) bool {
		return analysis.Titles[i].Count > analysis.Titles[j].Count
	})
	limitSessionAnalysis(analysis)
	agg.SessionAnalysis = analysis
}

func finalizeSessionItem(stat *SessionAnalysisItem) {
	if stat.StartedAt != "" && stat.EndedAt != "" {
		if start, err := time.Parse(time.RFC3339, stat.StartedAt); err == nil {
			if end, err := time.Parse(time.RFC3339, stat.EndedAt); err == nil && end.After(start) {
				stat.DurationMs = end.Sub(start).Milliseconds()
			}
		}
	}
	if stat.DurationMs == 0 {
		stat.DurationMs = stat.SystemDurationMs
	}
	if stat.Outcome == "" || stat.Outcome == "unknown" || stat.Outcome == "started" {
		switch {
		case stat.PreventedContinuation:
			stat.Outcome = "interrupted"
		case stat.ToolFailureCount > 0 || stat.HookErrorCount > 0:
			stat.Outcome = "failed"
		case stat.MessageCount > 0 || stat.ToolCallCount > 0:
			stat.Outcome = "completed"
		default:
			stat.Outcome = "unknown"
		}
	}
	if stat.Title == "" && stat.LastPromptPreview != "" {
		stat.Title = previewString(stat.LastPromptPreview, 80)
		stat.TitleSource = "last-prompt"
	}
	if len(stat.ModelsUsed) > 0 {
		stat.PrimaryModel = primarySessionModel(stat.ModelsUsed)
	}
}

// primarySessionModel 返回 session 内使用次数最多的模型（平手按模型名排序）。
func primarySessionModel(modelsUsed map[string]int) string {
	best := ""
	bestCount := -1
	for model, count := range modelsUsed {
		if count > bestCount || (count == bestCount && model < best) {
			best = model
			bestCount = count
		}
	}
	return best
}

func limitSessionAnalysis(analysis *SessionAnalysisData) {
	if len(analysis.Sessions) > 100 {
		analysis.Sessions = analysis.Sessions[:100]
	}
	if len(analysis.TopFailures) > 30 {
		analysis.TopFailures = analysis.TopFailures[:30]
	}
	if len(analysis.LongRunning) > 30 {
		analysis.LongRunning = analysis.LongRunning[:30]
	}
}

func normalizeSessionOutcome(status string) string {
	switch status {
	case "success", "succeeded", "completed", "complete", "ok":
		return "completed"
	case "failed", "failure", "error":
		return "failed"
	case "cancelled", "canceled", "aborted", "interrupted":
		return "interrupted"
	default:
		return status
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
