package main

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var toolFailureExpr = regexp.MustCompile(`(?i)(error|failed|exception|traceback|timed out|timeout|permission denied|no such file|command failed|exit code|is_error|\"success\"\s*:\s*false|失败|错误|异常|超时|权限|不存在)`)

func parseToolResults(record ProjectRecord, timestamp time.Time, projectName string, pendingTools map[string]pendingToolCall, agg *ProjectAggregate) {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(record.Message, &msg); err != nil {
		return
	}

	contentRaw := bytes.TrimSpace(msg.Content)
	if len(contentRaw) == 0 || contentRaw[0] != '[' {
		return
	}

	var contents []AssistantContent
	if err := json.Unmarshal(contentRaw, &contents); err != nil {
		return
	}

	for _, content := range contents {
		if content.Type != "tool_result" || content.ToolUseID == "" {
			continue
		}

		call, ok := pendingTools[content.ToolUseID]
		if !ok {
			call = pendingToolCall{
				ID:          content.ToolUseID,
				Tool:        "unknown",
				Model:       "unknown",
				Project:     projectName,
				SessionID:   record.SessionID,
				AgentID:     record.AgentID,
				IsSidechain: record.IsSidechain,
				Timestamp:   timestamp,
			}
		}

		kind, failed := classifyToolResult(content)
		preview := toolResultPreview(content.Content)

		if !ok {
			addToolCallLocked(agg, call.Tool, call.Model)
			addAgentToolCallLocked(agg, call)
		}
		addToolResultLocked(agg, call, failed, kind, preview, timestamp)

		delete(pendingTools, content.ToolUseID)
	}
}

func addToolCallLocked(agg *ProjectAggregate, tool string, model string) {
	stat := ensureToolStat(agg, tool)
	stat.CallCount++
	modelStat := ensureToolModelStat(agg, tool, model)
	modelStat.CallCount++
}

func addToolResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, kind string, preview string, timestamp time.Time) {
	stat := ensureToolStat(agg, call.Tool)
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	if failed {
		stat.FailureCount++
		modelStat.FailureCount++
		addAgentToolResultLocked(agg, call, true, false)
		addCommandOrFileResultLocked(agg, call, true, false)
		agg.ToolFailureKinds[kind]++
		if len(agg.ToolAnalysisFailureSamples()) < 30 {
			agg.ToolAnalysisAddFailureSample(ToolFailureSample{
				Tool:           call.Tool,
				Model:          call.Model,
				Kind:           kind,
				Project:        call.Project,
				SessionID:      call.SessionID,
				Timestamp:      timestamp.Format(time.RFC3339),
				ContentPreview: preview,
			})
		}
		return
	}
	stat.SuccessCount++
	modelStat.SuccessCount++
	addAgentToolResultLocked(agg, call, false, false)
	addCommandOrFileResultLocked(agg, call, false, false)
}

func addMissingToolResultLocked(agg *ProjectAggregate, call pendingToolCall) {
	stat := ensureToolStat(agg, call.Tool)
	stat.MissingResultCount++
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	modelStat.MissingResultCount++
	addAgentToolResultLocked(agg, call, false, true)
	addCommandOrFileResultLocked(agg, call, false, true)
}

func recordRuntimeEventLocked(agg *ProjectAggregate, record ProjectRecord, timestamp time.Time, projectName string) {
	eventType := record.Type
	if eventType == "" {
		eventType = "unknown"
	}

	if eventType == "attachment" {
		var attachment struct {
			Type       string  `json:"type"`
			HookName   string  `json:"hookName"`
			HookEvent  string  `json:"hookEvent"`
			Command    string  `json:"command"`
			Stdout     string  `json:"stdout"`
			Stderr     string  `json:"stderr"`
			ExitCode   int     `json:"exitCode"`
			DurationMs int     `json:"durationMs"`
			Filename   string  `json:"filename"`
			Used       float64 `json:"used"`
			Total      float64 `json:"total"`
			Remaining  float64 `json:"remaining"`
			Skills     []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"skills"`
		}
		if err := json.Unmarshal(record.Attachment, &attachment); err == nil && attachment.Type != "" {
			eventType = "attachment:" + attachment.Type
			switch attachment.Type {
			case "hook_success", "hook_cancelled", "hook_non_blocking_error":
				recordHookEventLocked(agg, attachment.Type, attachment.HookName, attachment.HookEvent, attachment.Command, attachment.Stderr, attachment.DurationMs)
			case "invoked_skills":
				for _, skill := range attachment.Skills {
					recordSkillLocked(agg, skill.Name, skill.Path)
				}
			case "opened_file_in_ide":
				recordOpenedFileLocked(agg, attachment.Filename)
			case "budget_usd":
				recordBudgetLocked(agg, attachment.Used, attachment.Total, attachment.Remaining)
			case "queued_command":
				// Counted through event type below.
			}
			if len(agg.EventSamples) < 40 && (strings.HasPrefix(attachment.Type, "hook_") || attachment.Type == "invoked_skills" || attachment.Type == "queued_command") {
				addEventSampleLocked(agg, eventType, projectName, record.SessionID, timestamp, string(record.Attachment))
			}
		}
	}

	agg.EventTypes[eventType]++
	switch eventType {
	case "attachment:queued_command":
		// Derived count comes from EventTypes in finalize.
	case "attachment:plan_mode", "attachment:plan_mode_exit":
		// Derived count comes from EventTypes in finalize.
	case "permission-mode":
		mode := record.PermissionMode
		if mode == "" {
			mode = rawStringField(record.Content, "mode")
		}
		if mode == "" {
			mode = rawStringField(record.Message, "permissionMode")
		}
		if mode == "" {
			mode = "unknown"
		}
		agg.PermissionModes[mode]++
	case "agent-name":
		name := record.Name
		if name == "" {
			name = rawStringField(record.Content, "name")
		}
		if name == "" {
			name = rawStringField(record.Message, "name")
		}
		if name != "" {
			agent := ensureAgentStat(agg, record.AgentID, record.IsSidechain)
			agent.AgentName = name
		}
	}

	agent := ensureAgentStat(agg, record.AgentID, record.IsSidechain)
	if record.Type == "assistant" || record.Type == "user" {
		agent.MessageCount++
	}
	if record.SessionID != "" {
		key := agent.AgentID
		if agg.AgentSessions[key] == nil {
			agg.AgentSessions[key] = make(map[string]bool)
		}
		if !agg.AgentSessions[key][record.SessionID] {
			agg.AgentSessions[key][record.SessionID] = true
			agent.SessionCount++
		}
	}
}

func recordHookEventLocked(agg *ProjectAggregate, attachmentType, hookName, hookEvent, command, stderr string, durationMs int) {
	if hookName == "" {
		hookName = "unknown"
	}
	key := hookName + "\x00" + hookEvent
	if agg.HookStats[key] == nil {
		agg.HookStats[key] = &HookStatItem{HookName: hookName, HookEvent: hookEvent}
	}
	stat := agg.HookStats[key]
	stat.TotalCount++
	switch attachmentType {
	case "hook_success":
		stat.SuccessCount++
	case "hook_cancelled":
		stat.CancelledCount++
	default:
		stat.ErrorCount++
		if stderr != "" {
			stat.LastError = previewString(stderr, 240)
		}
	}
	if command != "" {
		stat.LastCommand = previewString(command, 160)
	}
	if durationMs > 0 {
		previous := float64(stat.TotalCount - 1)
		stat.AvgDurationMs = (stat.AvgDurationMs*previous + float64(durationMs)) / float64(stat.TotalCount)
	}
}

func recordSkillLocked(agg *ProjectAggregate, name, path string) {
	if name == "" {
		name = "unknown"
	}
	if agg.SkillStats[name] == nil {
		agg.SkillStats[name] = &SkillStatItem{Name: name, Path: path}
	}
	agg.SkillStats[name].Count++
	if agg.SkillStats[name].Path == "" {
		agg.SkillStats[name].Path = path
	}
}

func recordOpenedFileLocked(agg *ProjectAggregate, path string) {
	if path == "" {
		return
	}
	if agg.OpenedFiles[path] == nil {
		agg.OpenedFiles[path] = &FileAccessStat{Path: path}
	}
	agg.OpenedFiles[path].Count++
}

func recordBudgetLocked(agg *ProjectAggregate, used, total, remaining float64) {
	if agg.BudgetSummary == nil {
		agg.BudgetSummary = &BudgetSummary{}
	}
	agg.BudgetSummary.LatestUsed = used
	agg.BudgetSummary.LatestTotal = total
	agg.BudgetSummary.LatestRemaining = remaining
	agg.BudgetSummary.EventCount++
	if used > agg.BudgetSummary.MaxUsed {
		agg.BudgetSummary.MaxUsed = used
	}
}

func addEventSampleLocked(agg *ProjectAggregate, eventType, project, sessionID string, timestamp time.Time, raw string) {
	agg.EventSamples = append(agg.EventSamples, EventSample{
		Type:           eventType,
		Project:        project,
		SessionID:      sessionID,
		Timestamp:      timestamp.Format(time.RFC3339),
		ContentPreview: previewString(raw, 240),
	})
}

func addAgentToolCallLocked(agg *ProjectAggregate, call pendingToolCall) {
	agent := ensureAgentStat(agg, call.AgentID, call.IsSidechain)
	agent.ToolCallCount++
	if call.IsSidechain {
		return
	}
}

func addAgentToolResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, missing bool) {
	agent := ensureAgentStat(agg, call.AgentID, call.IsSidechain)
	if failed {
		agent.ToolFailureCount++
	}
	if missing {
		agent.MissingResultCount++
	}
}

func ensureAgentStat(agg *ProjectAggregate, agentID string, isSidechain bool) *AgentStatItem {
	if agg.AgentStats == nil {
		agg.AgentStats = make(map[string]*AgentStatItem)
	}
	if agentID == "" {
		if isSidechain {
			agentID = "sidechain:unknown"
		} else {
			agentID = "main"
		}
	}
	if agg.AgentStats[agentID] == nil {
		agg.AgentStats[agentID] = &AgentStatItem{AgentID: agentID, IsSidechain: isSidechain}
	}
	if isSidechain {
		agg.AgentStats[agentID].IsSidechain = true
	}
	return agg.AgentStats[agentID]
}

func ensureToolStat(agg *ProjectAggregate, tool string) *ToolStatItem {
	if agg.ToolStats == nil {
		agg.ToolStats = make(map[string]*ToolStatItem)
	}
	if tool == "" {
		tool = "unknown"
	}
	if agg.ToolStats[tool] == nil {
		agg.ToolStats[tool] = &ToolStatItem{Tool: tool}
	}
	return agg.ToolStats[tool]
}

func ensureToolModelStat(agg *ProjectAggregate, tool string, model string) *ToolModelStatItem {
	if agg.ToolModelStats == nil {
		agg.ToolModelStats = make(map[string]*ToolModelStatItem)
	}
	if tool == "" {
		tool = "unknown"
	}
	if model == "" {
		model = "unknown"
	}
	key := model + "\x00" + tool
	if agg.ToolModelStats[key] == nil {
		agg.ToolModelStats[key] = &ToolModelStatItem{Model: model, Tool: tool}
	}
	return agg.ToolModelStats[key]
}

func classifyToolResult(content AssistantContent) (string, bool) {
	if content.IsError {
		return "explicit_error", true
	}

	text := toolResultText(content.Content)
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, `"success":false`) || strings.Contains(lower, `"success": false`):
		return "api_failure", true
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout") || strings.Contains(lower, "超时"):
		return "timeout", true
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "权限"):
		return "permission", true
	case strings.Contains(lower, "no such file") || strings.Contains(lower, "不存在"):
		return "not_found", true
	case strings.Contains(lower, "exit code") || strings.Contains(lower, "command failed"):
		return "command_failed", true
	case toolFailureExpr.MatchString(text):
		return "error_text", true
	default:
		return "", false
	}
}

func toolResultPreview(raw json.RawMessage) string {
	text := toolResultText(raw)
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		return text[:240]
	}
	return text
}

func toolResultText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func (agg *ProjectAggregate) ToolAnalysisFailureSamples() []ToolFailureSample {
	if agg.ToolAnalysis == nil {
		return nil
	}
	return agg.ToolAnalysis.FailureSamples
}

func (agg *ProjectAggregate) ToolAnalysisAddFailureSample(sample ToolFailureSample) {
	if agg.ToolAnalysis == nil {
		agg.ToolAnalysis = &ToolAnalysisData{}
	}
	agg.ToolAnalysis.FailureSamples = append(agg.ToolAnalysis.FailureSamples, sample)
}
