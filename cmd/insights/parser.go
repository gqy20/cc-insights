package main

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"regexp"
	"sort"
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
		dateKey := call.Timestamp.Format("2006-01-02")
		if call.Timestamp.IsZero() {
			dateKey = timestamp.Format("2006-01-02")
		}
		dailyAgg := ensureDailyRuntimeAggregate(agg, dateKey)

		classification := classifyToolResult(call.Tool, content)
		preview := toolResultPreview(content.Content)

		// M5: compute duration and result size
		durationMs := timestamp.Sub(call.Timestamp).Milliseconds()
		resultSize := len(toolResultText(content.Content))

		if !ok {
			addToolCallLocked(agg, call.Tool, call.Model)
			addAgentToolCallLocked(agg, call)
			addSessionToolCallLocked(agg, call)
			addToolCallLocked(dailyAgg, call.Tool, call.Model)
			addAgentToolCallLocked(dailyAgg, call)
			addSessionToolCallLocked(dailyAgg, call)
		}
		addToolResultLocked(agg, call, classification, preview, timestamp, durationMs, resultSize)
		addToolResultLocked(dailyAgg, call, classification, preview, timestamp, durationMs, resultSize)

		delete(pendingTools, content.ToolUseID)
	}
}

func addToolCallLocked(agg *ProjectAggregate, tool string, model string) {
	stat := ensureToolStat(agg, tool)
	stat.CallCount++
	modelStat := ensureToolModelStat(agg, tool, model)
	modelStat.CallCount++
}

func addToolResultLocked(agg *ProjectAggregate, call pendingToolCall, classification toolFailureClassification, preview string, timestamp time.Time, durationMs int64, resultSize int) {
	stat := ensureToolStat(agg, call.Tool)
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	if classification.Failed {
		stat.FailureCount++
		modelStat.FailureCount++
		addAgentToolResultLocked(agg, call, true, false)
		addSessionToolResultLocked(agg, call, true, false)
		addCommandOrFileResultLocked(agg, call, true, false)
		recordFailureAnalysisLocked(agg, call, classification)
		// file_analysis: 按文件记录 Edit 失败原因
		if (call.Tool == "Edit" || call.Tool == "MultiEdit") && call.FileOpKey != "" {
			if parts := strings.SplitN(call.FileOpKey, "\x00", 2); len(parts) == 2 {
				recordFileEditFailureLocked(agg, parts[1], classification.Category, classification.Reason)
			}
		}
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
		// M5: record performance for error result
		recordToolPerformanceLocked(agg, call, durationMs, resultSize, true, false)
		return
	}
	stat.SuccessCount++
	modelStat.SuccessCount++
	addAgentToolResultLocked(agg, call, false, false)
	addSessionToolResultLocked(agg, call, false, false)
	addCommandOrFileResultLocked(agg, call, false, false)
	// M5: record performance for success result
	recordToolPerformanceLocked(agg, call, durationMs, resultSize, false, false)
}

func addMissingToolResultLocked(agg *ProjectAggregate, call pendingToolCall) {
	stat := ensureToolStat(agg, call.Tool)
	stat.MissingResultCount++
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	modelStat.MissingResultCount++
	addAgentToolResultLocked(agg, call, false, true)
	addSessionToolResultLocked(agg, call, false, true)
	addCommandOrFileResultLocked(agg, call, false, true)
	// M5: record missing result
	recordToolPerformanceLocked(agg, call, -1, 0, false, true)
}

// --- file_analysis: 按文件记录 Edit 失败原因 ---

func recordFileEditFailureLocked(agg *ProjectAggregate, path, category, reason string) {
	if agg.FileEditFailures == nil {
		agg.FileEditFailures = make(map[string]*FileEditFailureAgg)
	}
	if agg.FileEditFailures[path] == nil {
		agg.FileEditFailures[path] = &FileEditFailureAgg{
			Path:         path,
			ReasonCounts: make(map[string]int),
		}
	}
	agg.FileEditFailures[path].TotalFailures++
	reasonKey := reason
	if reasonKey == "" {
		reasonKey = category
	}
	if reasonKey == "" {
		reasonKey = "unknown"
	}
	agg.FileEditFailures[path].ReasonCounts[reasonKey]++
}

// --- file_analysis: file-history-snapshot 事件 ---

func recordFileHistorySnapshotLocked(agg *ProjectAggregate, rawAttachment json.RawMessage, sessionID string) {
	var snap struct {
		Snapshot struct {
			TrackedFileBackups map[string]struct {
				BackupFileName string `json:"backupFileName"`
				Version        int    `json:"version"`
				BackupTime     string `json:"backupTime"`
			} `json:"trackedFileBackups"`
		} `json:"snapshot"`
		IsSnapshotUpdate bool `json:"isSnapshotUpdate"`
	}
	if err := json.Unmarshal(rawAttachment, &snap); err != nil {
		return
	}
	for filePath := range snap.Snapshot.TrackedFileBackups {
		if agg.FileSnapshotStats == nil {
			agg.FileSnapshotStats = make(map[string]*FileSnapshotAgg)
		}
		if agg.FileSnapshotStats[filePath] == nil {
			agg.FileSnapshotStats[filePath] = &FileSnapshotAgg{
				Path:       filePath,
				SessionSet: make(map[string]bool),
			}
		}
		s := agg.FileSnapshotStats[filePath]
		s.SnapshotCount++
		backup := snap.Snapshot.TrackedFileBackups[filePath]
		if backup.Version > s.MaxVersion {
			s.MaxVersion = backup.Version
		}
		if sessionID != "" && !s.SessionSet[sessionID] {
			s.SessionSet[sessionID] = true
		}
		if snap.IsSnapshotUpdate {
			s.IsUpdateCount++
		}
	}
}

// --- file_analysis: edited_text_file attachment ---

func recordEditedTextFileLocked(agg *ProjectAggregate, filename string, rawAttachment json.RawMessage) {
	if filename == "" || rawAttachment == nil {
		return
	}
	var edited struct {
		Snippet string `json:"snippet"`
	}
	if err := json.Unmarshal(rawAttachment, &edited); err != nil {
		return
	}
	if agg.FileEditedStats == nil {
		agg.FileEditedStats = make(map[string]*FileEditedAgg)
	}
	stat := agg.FileEditedStats[filename]
	if stat == nil {
		stat = &FileEditedAgg{Path: filename}
		agg.FileEditedStats[filename] = stat
	}
	stat.EditCount++
	lineCount := countLines(edited.Snippet)
	stat.TotalLines += lineCount
	stat.TotalChars += int64(len(edited.Snippet))
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for _, ch := range s {
		if ch == '\n' {
			n++
		}
	}
	return n
}

// --- end file_analysis ---

// --- task_plan_analysis (Milestone 4): Plan event recording functions ---

// ensurePlanModeAgg lazy-init PlanModeAgg on ProjectAggregate
func ensurePlanModeAgg(agg *ProjectAggregate) *PlanModeAgg {
	if agg.PlanModeAgg == nil {
		agg.PlanModeAgg = &PlanModeAgg{
			ExitReasons:   make(map[string]int),
			PlanFilePaths: make(map[string]*PlanFileAgg),
			SessionSet:    make(map[string]bool),
		}
	}
	return agg.PlanModeAgg
}

// recordPlanModeLocked records a plan_mode entry event
func recordPlanModeLocked(agg *ProjectAggregate, planFilePath string, planExists bool, sessionID, cwd string) {
	p := ensurePlanModeAgg(agg)
	p.EntryCount++
	if sessionID != "" && !p.SessionSet[sessionID] {
		p.SessionSet[sessionID] = true
	}
	// Track referenced plan file path for uniqueness counting
	if planFilePath != "" {
		if p.PlanFilePaths[planFilePath] == nil {
			p.PlanFilePaths[planFilePath] = &PlanFileAgg{
				FilePath:    planFilePath,
				FileName:    filepath.Base(planFilePath),
				RefSessions: make(map[string]bool),
			}
		}
	}
}

// recordPlanModeExitLocked records a plan_mode_exit event
func recordPlanModeExitLocked(agg *ProjectAggregate, exitReason, sessionID string) {
	p := ensurePlanModeAgg(agg)
	p.ExitCount++
	if exitReason == "" {
		exitReason = "unknown"
	}
	p.ExitReasons[exitReason]++
}

// recordPlanModeReentryLocked records a plan_mode_reentry event
func recordPlanModeReentryLocked(agg *ProjectAggregate, sessionID string) {
	p := ensurePlanModeAgg(agg)
	p.ReentryCount++
}

// recordPlanFileReferenceLocked records a plan_file_reference (has full markdown content!)
func recordPlanFileReferenceLocked(agg *ProjectAggregate, planFilePath, planContent, sessionID string) {
	if planFilePath == "" {
		return
	}
	p := ensurePlanModeAgg(agg)
	if p.PlanFilePaths[planFilePath] == nil {
		p.PlanFilePaths[planFilePath] = &PlanFileAgg{
			FilePath:    planFilePath,
			FileName:    filepath.Base(planFilePath),
			RefSessions: make(map[string]bool),
		}
	}
	pf := p.PlanFilePaths[planFilePath]
	// Only store content once (first reference wins)
	if pf.PlanContent == "" && planContent != "" {
		pf.PlanContent = planContent
		pf.CharCount = len(planContent)
		pf.LineCount = strings.Count(planContent, "\n") + 1
		pf.HasCode = strings.Contains(planContent, "```")
		if len(planContent) > 200 {
			pf.Preview = strings.TrimSpace(planContent[:200])
		} else {
			pf.Preview = strings.TrimSpace(planContent)
		}
	}
	// Dedup ref count per session
	if sessionID != "" && !pf.RefSessions[sessionID] {
		pf.RefSessions[sessionID] = true
		pf.RefCount++
	}
}

// recordGoalStatusLocked records a goal_status event (capped at 50 to prevent unbounded growth)
func recordGoalStatusLocked(agg *ProjectAggregate, met, sentinel bool, condition, sessionID string, timestamp time.Time) {
	if agg.GoalStatusAgg == nil {
		agg.GoalStatusAgg = &GoalStatusAgg{}
	}
	agg.GoalStatusAgg.Items = append(agg.GoalStatusAgg.Items, GoalStatusItem{
		SessionID: sessionID,
		Met:       met,
		Sentinel:  sentinel,
		Condition: condition,
		Timestamp: timestamp.Format(time.RFC3339),
	})
	if len(agg.GoalStatusAgg.Items) > 50 {
		agg.GoalStatusAgg.Items = agg.GoalStatusAgg.Items[:50]
	}
}

// recordReminderLocked records task_reminder or todo_reminder frequency
func recordReminderLocked(agg *ProjectAggregate, kind, sessionID, cwd string) {
	if agg.ReminderAgg == nil {
		agg.ReminderAgg = &ReminderAgg{
			TaskSessionCounts:   make(map[string]int),
			TodoSessionCounts:   make(map[string]int),
			TaskSessionProjects: make(map[string]string),
			TodoSessionProjects: make(map[string]string),
		}
	}
	r := agg.ReminderAgg
	if kind == "task" {
		r.TaskReminderCount++
		r.TaskSessionCounts[sessionID]++
		if _, exists := r.TaskSessionProjects[sessionID]; !exists {
			r.TaskSessionProjects[sessionID] = cwd
		}
	} else {
		r.TodoReminderCount++
		r.TodoSessionCounts[sessionID]++
		if _, exists := r.TodoSessionProjects[sessionID]; !exists {
			r.TodoSessionProjects[sessionID] = cwd
		}
	}
}

// --- end task_plan_analysis ---

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
			Snapshot json.RawMessage `json:"snapshot"` // file-analysis: for file-history-snapshot
			// task_plan_analysis (Milestone 4): plan/task attachment fields
			PlanFilePath string `json:"planFilePath"` // plan_mode, plan_file_reference
			PlanExists   bool   `json:"planExists"`   // plan_mode
			ReminderType string `json:"reminderType"` // plan_mode
			IsSubAgent   bool   `json:"isSubAgent"`   // plan_mode
			PlanContent  string `json:"planContent"`  // plan_file_reference (full markdown!)
			ExitReason   string `json:"exitReason"`   // plan_mode_exit
			Met          bool   `json:"met"`          // goal_status
			Sentinel     bool   `json:"sentinel"`     // goal_status
			Condition    string `json:"condition"`    // goal_status
			ItemCount    int    `json:"itemCount"`    // task_reminder, todo_reminder
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
			case "file-history-snapshot":
				recordFileHistorySnapshotLocked(agg, record.Attachment, record.SessionID)
			case "edited_text_file":
			case "plan_mode":
				recordPlanModeLocked(agg, attachment.PlanFilePath, attachment.PlanExists, record.SessionID, record.Cwd)
			case "plan_mode_exit":
				recordPlanModeExitLocked(agg, attachment.ExitReason, record.SessionID)
			case "plan_mode_reentry":
				recordPlanModeReentryLocked(agg, record.SessionID)
			case "plan_file_reference":
				recordPlanFileReferenceLocked(agg, attachment.PlanFilePath, attachment.PlanContent, record.SessionID)
			case "goal_status":
				recordGoalStatusLocked(agg, attachment.Met, attachment.Sentinel, attachment.Condition, record.SessionID, timestamp)
			case "task_reminder":
				recordReminderLocked(agg, "task", record.SessionID, record.Cwd)
			case "todo_reminder":
				recordReminderLocked(agg, "todo", record.SessionID, record.Cwd)
				recordEditedTextFileLocked(agg, attachment.Filename, record.Attachment)
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
	return previewString(text, 240)
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

// === M5: Tool Performance & Quality Analysis ===

const maxSlowCalls = 20 // Top-N slowest calls to retain globally

// buildToolPerfCategoryKey constructs the M5 category key and display parts.
// Returns (categoryKey, baseTool, subKey, sampleInput)
func buildToolPerfCategoryKey(call pendingToolCall) (string, string, string, string) {
	switch call.Tool {
	case "Bash":
		subKey := call.CommandName
		if subKey == "" {
			subKey = "unknown"
		}
		sampleInput := ""
		if len(call.Input) > 0 {
			var input struct {
				Command string `json:"command"`
			}
			if json.Unmarshal(call.Input, &input) == nil && input.Command != "" {
				sampleInput = previewString(input.Command, 120)
			}
		}
		return "Bash\x00" + subKey, "Bash", subKey, sampleInput
	case "Read", "Edit", "Write", "MultiEdit":
		if call.FileOpKey != "" {
			parts := strings.SplitN(call.FileOpKey, "\x00", 2)
			subKey := ""
			if len(parts) == 2 {
				subKey = parts[1]
			}
			sampleInput := truncatePath(subKey)
			return call.FileOpKey, call.Tool, subKey, sampleInput
		}
		return call.Tool + "\x00", call.Tool, "", ""
	default:
		// MCP tools like mcp__jina__web_search are already granular enough
		return call.Tool + "\x00", call.Tool, "", ""
	}
}

func truncatePath(path string) string {
	if len(path) <= 80 {
		return path
	}
	return "..." + path[len(path)-77:]
}

func ensureToolPerfAgg(agg *ProjectAggregate, categoryKey string) *ToolPerfAgg {
	if agg.ToolPerfStats == nil {
		agg.ToolPerfStats = make(map[string]*ToolPerfAgg)
	}
	if agg.ToolPerfStats[categoryKey] == nil {
		agg.ToolPerfStats[categoryKey] = &ToolPerfAgg{
			MinDurationMs: math.MaxInt64,
		}
	}
	return agg.ToolPerfStats[categoryKey]
}

// recordToolPerformanceLocked records one paired tool_use->tool_result event.
func recordToolPerformanceLocked(agg *ProjectAggregate, call pendingToolCall, durationMs int64, resultSize int, isError bool, isMissing bool) {
	categoryKey, baseTool, _, sampleInput := buildToolPerfCategoryKey(call)
	stat := ensureToolPerfAgg(agg, categoryKey)

	stat.CallCount++
	switch {
	case isMissing:
		stat.MissingCount++
	case isError:
		stat.ErrorCount++
	default:
		stat.SuccessCount++
	}

	if durationMs >= 0 {
		stat.TotalDurationMs += durationMs
		if durationMs < stat.MinDurationMs {
			stat.MinDurationMs = durationMs
		}
		if durationMs > stat.MaxDurationMs {
			stat.MaxDurationMs = durationMs
		}
	}

	stat.TotalResultSize += int64(resultSize)
	if resultSize == 0 {
		stat.EmptyResults++
	}

	if stat.SampleInput == "" && sampleInput != "" {
		stat.SampleInput = sampleInput
	}

	// Maintain global slowest-calls Top-N (only for paired calls with valid duration)
	if durationMs >= 0 && !isMissing {
		recordSlowCallLocked(agg, call, baseTool, categoryKey, durationMs, isError, resultSize)
	}
}

func recordSlowCallLocked(agg *ProjectAggregate, call pendingToolCall, baseTool, categoryKey string, durationMs int64, isError bool, resultSize int) {
	item := ToolSlowCallItem{
		Tool:        call.Tool,
		Category:    formatCategoryDisplay(baseTool, categoryKey),
		DurationMs:  durationMs,
		IsError:     isError,
		ResultSize:  resultSize,
		Timestamp:   call.Timestamp.Format(time.RFC3339),
		SampleInput: extractSampleInputForSlow(call),
	}

	if len(agg.SlowestCalls) < maxSlowCalls {
		agg.SlowestCalls = append(agg.SlowestCalls, item)
		sortSlowCallsDesc(agg.SlowestCalls)
	} else if durationMs > agg.SlowestCalls[len(agg.SlowestCalls)-1].DurationMs {
		agg.SlowestCalls[len(agg.SlowestCalls)-1] = item
		sortSlowCallsDesc(agg.SlowestCalls)
	}
}

func sortSlowCallsDesc(calls []ToolSlowCallItem) {
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].DurationMs > calls[j].DurationMs
	})
}

func formatCategoryDisplay(baseTool, categoryKey string) string {
	idx := strings.IndexByte(categoryKey, '\x00')
	if idx < 0 {
		return baseTool
	}
	subKey := categoryKey[idx+1:]
	if subKey == "" {
		return baseTool
	}
	return baseTool + ":" + subKey
}

func extractSampleInputForSlow(call pendingToolCall) string {
	if call.CommandName != "" {
		var input struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(call.Input, &input) == nil && input.Command != "" {
			return previewString(input.Command, 120)
		}
	}
	if call.FileOpKey != "" {
		parts := strings.SplitN(call.FileOpKey, "\x00", 2)
		if len(parts) == 2 {
			return truncatePath(parts[1])
		}
	}
	return ""
}
