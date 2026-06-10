package main

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var toolFailureExpr = regexp.MustCompile(`(?i)(error|failed|exception|traceback|timed out|timeout|permission denied|no such file|command failed|exit code|is_error|\"success\"\s*:\s*false|失败|错误|异常|超时|权限|不存在)`)
var exitCodeExpr = regexp.MustCompile(`(?i)(exit code|exited with code|exit status)\s+(-?\d+)`)
var httpStatusExpr = regexp.MustCompile(`\b(4\d\d|5\d\d)\b`)

type toolFailureClassification struct {
	Kind     string
	Category string
	Reason   string
	Failed   bool
}

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

		classification := classifyToolResult(call.Tool, content)
		preview := toolResultPreview(content.Content)

		if !ok {
			addToolCallLocked(agg, call.Tool, call.Model)
			addAgentToolCallLocked(agg, call)
			addSessionToolCallLocked(agg, call)
		}
		addToolResultLocked(agg, call, classification, preview, timestamp)

		delete(pendingTools, content.ToolUseID)
	}
}

func addToolCallLocked(agg *ProjectAggregate, tool string, model string) {
	stat := ensureToolStat(agg, tool)
	stat.CallCount++
	modelStat := ensureToolModelStat(agg, tool, model)
	modelStat.CallCount++
}

func addToolResultLocked(agg *ProjectAggregate, call pendingToolCall, classification toolFailureClassification, preview string, timestamp time.Time) {
	stat := ensureToolStat(agg, call.Tool)
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	if classification.Failed {
		stat.FailureCount++
		modelStat.FailureCount++
		addAgentToolResultLocked(agg, call, true, false)
		addSessionToolResultLocked(agg, call, true, false)
		addCommandOrFileResultLocked(agg, call, true, false)
		recordFailureAnalysisLocked(agg, call, classification)
		if len(agg.FailureSamples) < 30 {
			agg.FailureSamples = append(agg.FailureSamples, ToolFailureSample{
				Tool:           call.Tool,
				Model:          call.Model,
				Kind:           classification.Kind,
				Category:       classification.Category,
				Reason:         classification.Reason,
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
	addSessionToolResultLocked(agg, call, false, false)
	addCommandOrFileResultLocked(agg, call, false, false)
}

func addMissingToolResultLocked(agg *ProjectAggregate, call pendingToolCall) {
	stat := ensureToolStat(agg, call.Tool)
	stat.MissingResultCount++
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	modelStat.MissingResultCount++
	addAgentToolResultLocked(agg, call, false, true)
	addSessionToolResultLocked(agg, call, false, true)
	addCommandOrFileResultLocked(agg, call, false, true)
}

func recordRuntimeEventLocked(agg *ProjectAggregate, record ProjectRecord, timestamp time.Time, projectName string) {
	recordSessionRecordLocked(agg, record, timestamp, projectName)

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
				recordBudgetLocked(agg, attachment.Used, attachment.Total, attachment.Remaining, timestamp, projectName, record.SessionID)
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

func recordBudgetLocked(agg *ProjectAggregate, used, total, remaining float64, timestamp time.Time, projectName string, sessionID string) {
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
	agg.BudgetTimeline = append(agg.BudgetTimeline, BudgetTimelineItem{
		Timestamp: timestamp.Format(time.RFC3339),
		Project:   projectName,
		SessionID: sessionID,
		Used:      used,
		Total:     total,
		Remaining: remaining,
	})
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

func recordFailureAnalysisLocked(agg *ProjectAggregate, call pendingToolCall, classification toolFailureClassification) {
	ensureFailureReasonStat(agg, classification.Category, classification.Reason).Count++
	ensureFailureToolReasonStat(agg, call.Tool, classification.Category, classification.Reason).Count++
	ensureFailureModelReasonStat(agg, call.Model, classification.Category, classification.Reason).Count++
}

func ensureFailureReasonStat(agg *ProjectAggregate, category, reason string) *FailureReasonStat {
	if agg.FailureReasons == nil {
		agg.FailureReasons = make(map[string]*FailureReasonStat)
	}
	category, reason = normalizeFailureCategoryReason(category, reason)
	key := category + "\x00" + reason
	if agg.FailureReasons[key] == nil {
		agg.FailureReasons[key] = &FailureReasonStat{Category: category, Reason: reason}
	}
	return agg.FailureReasons[key]
}

func ensureFailureToolReasonStat(agg *ProjectAggregate, tool, category, reason string) *FailureToolReasonStat {
	if agg.FailureToolReasons == nil {
		agg.FailureToolReasons = make(map[string]*FailureToolReasonStat)
	}
	if tool == "" {
		tool = "unknown"
	}
	category, reason = normalizeFailureCategoryReason(category, reason)
	key := tool + "\x00" + category + "\x00" + reason
	if agg.FailureToolReasons[key] == nil {
		agg.FailureToolReasons[key] = &FailureToolReasonStat{Tool: tool, Category: category, Reason: reason}
	}
	return agg.FailureToolReasons[key]
}

func ensureFailureModelReasonStat(agg *ProjectAggregate, model, category, reason string) *FailureModelReasonStat {
	if agg.FailureModelReasons == nil {
		agg.FailureModelReasons = make(map[string]*FailureModelReasonStat)
	}
	if model == "" {
		model = "unknown"
	}
	category, reason = normalizeFailureCategoryReason(category, reason)
	key := model + "\x00" + category + "\x00" + reason
	if agg.FailureModelReasons[key] == nil {
		agg.FailureModelReasons[key] = &FailureModelReasonStat{Model: model, Category: category, Reason: reason}
	}
	return agg.FailureModelReasons[key]
}

func normalizeFailureCategoryReason(category, reason string) (string, string) {
	if category == "" {
		category = "unknown"
	}
	if reason == "" {
		reason = "unknown"
	}
	return category, reason
}

func classifyToolResult(tool string, content AssistantContent) toolFailureClassification {
	if content.IsError {
		return newFailureClassification("explicit_error", failureCategoryForTool(tool), "explicit_error")
	}

	text := toolResultText(content.Content)
	if text == "" {
		return toolFailureClassification{}
	}
	lower := strings.ToLower(text)
	if category, reason, ok := classifyDetailedFailure(tool, lower); ok {
		return newFailureClassification("", category, reason)
	}
	switch {
	case strings.Contains(lower, `"success":false`) || strings.Contains(lower, `"success": false`):
		return newFailureClassification("api_failure", "api", "success_false")
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout") || strings.Contains(lower, "超时"):
		return newFailureClassification("timeout", "timeout", "timeout")
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "权限"):
		return newFailureClassification("permission", "permission", "permission_denied")
	case strings.Contains(lower, "no such file") || strings.Contains(lower, "不存在"):
		return newFailureClassification("not_found", "file", "not_found")
	case strings.Contains(lower, "exit code") || strings.Contains(lower, "command failed"):
		return newFailureClassification("command_failed", "bash", "command_failed")
	case toolFailureExpr.MatchString(text):
		return newFailureClassification("error_text", failureCategoryForTool(tool), "error_text")
	default:
		return toolFailureClassification{}
	}
}

func newFailureClassification(kind, category, reason string) toolFailureClassification {
	if category == "" {
		category = "unknown"
	}
	if reason == "" {
		reason = kind
	}
	if kind == "" {
		kind = kindFromCategoryReason(category, reason)
	}
	return toolFailureClassification{
		Kind:     kind,
		Category: category,
		Reason:   reason,
		Failed:   true,
	}
}

func classifyDetailedFailure(tool string, lower string) (string, string, bool) {
	if category, reason, ok := classifyCommonFailure(lower); ok {
		return category, reason, true
	}

	switch {
	case tool == "Bash":
		return classifyBashFailure(lower)
	case tool == "Edit" || tool == "MultiEdit" || tool == "Write" || tool == "Read":
		return classifyFileToolFailure(lower)
	case strings.HasPrefix(tool, "mcp__"):
		return classifyMCPFailure(lower)
	case strings.Contains(strings.ToLower(tool), "web"):
		return classifyWebFailure(lower)
	default:
		return "", "", false
	}
}

func classifyCommonFailure(lower string) (string, string, bool) {
	switch {
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "operation not permitted") || strings.Contains(lower, "权限"):
		return "permission", "permission_denied", true
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "超时"):
		return "timeout", "timeout", true
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") || strings.Contains(lower, "429"):
		return "rate_limit", "rate_limit_429", true
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "invalid api key") || strings.Contains(lower, "authentication"):
		return "auth", "auth_error", true
	case strings.Contains(lower, "model not found") || strings.Contains(lower, "unknown model"):
		return "model", "model_not_found", true
	default:
		return "", "", false
	}
}

func classifyBashFailure(lower string) (string, string, bool) {
	if matches := exitCodeExpr.FindStringSubmatch(lower); len(matches) == 3 {
		return "bash", "exit_code_" + matches[2], true
	}
	switch {
	case strings.Contains(lower, "command not found"):
		return "bash", "command_not_found", true
	case strings.Contains(lower, "no such file or directory"):
		return "bash", "not_found", true
	case strings.Contains(lower, "test failed") || strings.Contains(lower, "tests failed") || strings.Contains(lower, "failing tests") || strings.Contains(lower, "failed tests"):
		return "bash", "test_failure", true
	case strings.Contains(lower, "killed") || strings.Contains(lower, "signal: killed"):
		return "bash", "killed", true
	case strings.Contains(lower, "command failed"):
		return "bash", "command_failed", true
	default:
		return "", "", false
	}
}

func classifyFileToolFailure(lower string) (string, string, bool) {
	switch {
	case strings.Contains(lower, "old_string") || strings.Contains(lower, "old string") || strings.Contains(lower, "string to replace") || strings.Contains(lower, "no match") || strings.Contains(lower, "not found in file"):
		return "edit", "old_string_mismatch", true
	case strings.Contains(lower, "no such file") || strings.Contains(lower, "file not found") || strings.Contains(lower, "cannot find") || strings.Contains(lower, "不存在"):
		return "file", "not_found", true
	case strings.Contains(lower, "is a directory") || strings.Contains(lower, "not a file"):
		return "file", "not_regular_file", true
	case strings.Contains(lower, "binary file") || strings.Contains(lower, "invalid utf") || strings.Contains(lower, "encoding"):
		return "file", "encoding_or_binary", true
	default:
		return "", "", false
	}
}

func classifyMCPFailure(lower string) (string, string, bool) {
	if category, reason, ok := classifyWebFailure(lower); ok {
		return category, reason, true
	}
	switch {
	case strings.Contains(lower, "schema") || strings.Contains(lower, "invalid params") || strings.Contains(lower, "validation"):
		return "mcp", "schema_error", true
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "econnreset") || strings.Contains(lower, "enotfound"):
		return "mcp", "network_error", true
	case strings.Contains(lower, "tool not found") || strings.Contains(lower, "method not found"):
		return "mcp", "tool_not_found", true
	default:
		return "", "", false
	}
}

func classifyWebFailure(lower string) (string, string, bool) {
	switch {
	case strings.Contains(lower, "dns") || strings.Contains(lower, "no such host") || strings.Contains(lower, "enotfound"):
		return "web", "dns_error", true
	case strings.Contains(lower, "tls") || strings.Contains(lower, "certificate") || strings.Contains(lower, "x509"):
		return "web", "tls_error", true
	case strings.Contains(lower, "network") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "econnreset"):
		return "web", "network_error", true
	}
	if matches := httpStatusExpr.FindStringSubmatch(lower); len(matches) == 2 {
		return "web", "http_" + matches[1], true
	}
	return "", "", false
}

func failureCategoryForTool(tool string) string {
	switch {
	case tool == "Bash":
		return "bash"
	case tool == "Edit" || tool == "MultiEdit":
		return "edit"
	case tool == "Read" || tool == "Write":
		return "file"
	case strings.HasPrefix(tool, "mcp__"):
		return "mcp"
	case strings.Contains(strings.ToLower(tool), "web"):
		return "web"
	default:
		return "tool"
	}
}

func kindFromCategoryReason(category, reason string) string {
	switch reason {
	case "not_found":
		return "not_found"
	case "permission_denied":
		return "permission"
	case "timeout":
		return "timeout"
	case "command_failed":
		return "command_failed"
	case "success_false":
		return "api_failure"
	case "error_text":
		return "error_text"
	case "explicit_error":
		return "explicit_error"
	}
	if reason == "" {
		return category
	}
	if category == "" || strings.HasPrefix(reason, category+"_") {
		return reason
	}
	return category + "_" + reason
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
