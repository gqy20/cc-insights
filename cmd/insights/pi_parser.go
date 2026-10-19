package main

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
	"time"
)

// pi agent 数据源：~/.pi/agent/sessions/<encoded-cwd>/<timestamp>_<uuid>.jsonl
// 每个文件一个 append-only session（与 codex 同构，复用 tail 增量机制）。
// 记录类型：session / model_change / thinking_level_change / message / compaction。
// message.role: user | assistant | toolResult（toolResult 带 toolCallId + isError，
// 是结构化失败信号；无 token 用量数据，tok/cost 维度为空）。

type piRecord struct {
	Type      string    `json:"type"`
	ID        string    `json:"id"`
	Timestamp string    `json:"timestamp"`
	Cwd       string    `json:"cwd"`
	Provider  string    `json:"provider"`
	ModelID   string    `json:"modelId"`
	Message   piMessage `json:"message"`
}

type piMessage struct {
	Role       string      `json:"role"`
	ToolCallID string      `json:"toolCallId"`
	ToolName   string      `json:"toolName"`
	IsError    bool        `json:"isError"`
	Content    []piContent `json:"content"`
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	Usage      *piUsage    `json:"usage"`
}

// piUsage pi assistant 消息自带的 token 用量（api 系统各异，字段已归一）。
type piUsage struct {
	Input       int `json:"input"`
	Output      int `json:"output"`
	CacheRead   int `json:"cacheRead"`
	CacheWrite  int `json:"cacheWrite"`
	TotalTokens int `json:"totalTokens"`
	Reasoning   int `json:"reasoning"`
}

type piContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// piState 复用 codexSessionState 结构（SessionID/Project/CurrentModel/PendingTools）。
type piState = codexSessionState

func parsePiSessionFileAggregateFull(filePath string, tf TimeFilter, agg *ProjectAggregate) (int64, *SessionTailState) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, nil
	}
	defer f.Close()

	state := piState{
		SessionID:    filePath,
		Project:      "Unknown",
		CurrentModel: "unknown",
		PendingTools: make(map[string]*codexPendingTool),
	}
	consumed, err := scanPiRecords(f, &state, tf, agg)
	if err != nil {
		Warn("pi JSONL 文件读取失败", "path", filePath, "error", err.Error())
	}
	finalizePiPendings(agg, &state)
	return consumed, snapshotSessionTailState(&state)
}

// parsePiSessionFileTail 从 offset 续解析增长部分（append-only session）。
func parsePiSessionFileTail(filePath string, offset int64, snap *SessionTailState, tf TimeFilter, base *ProjectAggregate) (int64, *SessionTailState) {
	f, err := os.Open(filePath)
	if err != nil {
		return offset, snap
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, snap
	}
	state := restoreSessionTailState(snap)
	consumed, err := scanPiRecords(f, state, tf, base)
	if err != nil {
		Warn("pi JSONL tail 读取失败", "path", filePath, "error", err.Error())
	}
	finalizePiPendings(base, state)
	return offset + consumed, snapshotSessionTailState(state)
}

func scanPiRecords(f *os.File, state *piState, tf TimeFilter, agg *ProjectAggregate) (int64, error) {
	scanner := newJSONLScanner(f)
	for scanner.Scan() {
		var record piRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		timestamp, hasTimestamp := parseProjectRecordTimestamp(record.Timestamp)

		switch record.Type {
		case "session":
			if record.Cwd != "" {
				state.Project = record.Cwd
			}
			if record.ID != "" {
				state.SessionID = record.ID
			}
			if state.StartedAt.IsZero() && hasTimestamp {
				state.StartedAt = timestamp
			}
			recordCodexSessionSeen(agg, state, timestamp)
		case "model_change":
			if record.ModelID != "" {
				model := record.ModelID
				if record.Provider != "" {
					model = record.Provider + "/" + record.ModelID
				}
				state.CurrentModel = model
			}
		}

		if hasTimestamp && !tf.Contains(timestamp) {
			continue
		}
		if !hasTimestamp && hasTimeFilter(tf) {
			continue
		}

		switch record.Type {
		case "message":
			recordPiMessage(agg, state, &record, timestamp)
		case "compaction":
			recordCodexEventType(agg, "compaction")
		}
	}
	return scanner.Consumed(), scanner.Err()
}

func recordPiMessage(agg *ProjectAggregate, state *piState, record *piRecord, timestamp time.Time) {
	msg := &record.Message
	switch msg.Role {
	case "assistant":
		recordCodexAssistantMessage(agg, state, timestamp)
		// 消息自带 model/provider 时优先，口径与 model_change 一致（provider/model）
		model := ""
		if msg.Model != "" {
			model = msg.Model
			if msg.Provider != "" {
				model = msg.Provider + "/" + msg.Model
			}
		}
		if model == "" {
			model = codexModel(state)
		}
		recordPiTokenUsage(agg, state, model, msg, timestamp)
		for i := range msg.Content {
			if msg.Content[i].Type == "toolCall" {
				recordPiToolCall(agg, state, &msg.Content[i], timestamp)
			}
		}
	case "user":
		recordCodexUserMessage(agg, state, timestamp)
	case "toolResult":
		recordPiToolResult(agg, state, msg, timestamp)
	}
}

func recordPiModelUsage(agg *ProjectAggregate, model string, timestamp time.Time, usage *piUsage) {
	if model == "" || model == "unknown" {
		return
	}
	if agg.ModelUsage == nil {
		agg.ModelUsage = make(map[string]*ModelUsageItem)
	}
	if agg.ModelUsage[model] == nil {
		agg.ModelUsage[model] = &ModelUsageItem{Model: model}
	}
	agg.ModelUsage[model].Count++
	if !timestamp.IsZero() {
		dateKey := timestamp.Format("2006-01-02")
		if agg.DailyModelCounts[dateKey] == nil {
			agg.DailyModelCounts[dateKey] = make(map[string]int)
		}
		agg.DailyModelCounts[dateKey][model]++
	}
	if usage == nil {
		return
	}
	total := usage.TotalTokens
	if total == 0 && (usage.Input > 0 || usage.Output > 0) {
		total = usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
	}
	mu := agg.ModelUsage[model]
	mu.Tokens += total
	mu.InputTokens += usage.Input
	mu.OutputTokens += usage.Output
	mu.CachedInputTokens += usage.CacheRead
	mu.ReasoningOutputTokens += usage.Reasoning
	if !timestamp.IsZero() {
		dateKey := timestamp.Format("2006-01-02")
		if agg.DailyModelTokens[dateKey] == nil {
			agg.DailyModelTokens[dateKey] = make(map[string]int)
		}
		agg.DailyModelTokens[dateKey][model] += total
	}
}

// recordPiTokenUsage 把 assistant 消息自带的 usage 记入模型/成本/会话/上下文
// 四类统计，路由到全局与日级 runtime 聚合（与 codex token_count 事件同构）。
func recordPiTokenUsage(agg *ProjectAggregate, state *piState, model string, msg *piMessage, timestamp time.Time) {
	u := msg.Usage
	if u == nil {
		// 无 usage 时仍记模型次数与日级计数（digest 依赖）
		recordPiModelUsage(agg, model, timestamp, nil)
		return
	}
	total := u.TotalTokens
	if total == 0 && (u.Input > 0 || u.Output > 0) {
		total = u.Input + u.Output + u.CacheRead + u.CacheWrite
	}
	outputTokens := u.Output + u.Reasoning
	projectName := codexProject(state)
	sessionID := codexSessionID(state)
	if !timestamp.IsZero() {
		dateKey := timestamp.Format("2006-01-02")
		if agg.DailyModelCounts[dateKey] == nil {
			agg.DailyModelCounts[dateKey] = make(map[string]int)
		}
		agg.DailyModelCounts[dateKey][model]++
		if agg.DailyModelTokens[dateKey] == nil {
			agg.DailyModelTokens[dateKey] = make(map[string]int)
		}
		agg.DailyModelTokens[dateKey][model] += total
	}
	for _, target := range codexAggregatesForCall(agg, pendingToolCall{
		Project: projectName, SessionID: sessionID, Model: model, Timestamp: timestamp,
	}) {
		recordModelUsageLocked(target, model, total)
		recordModelTokenBreakdownLocked(target, model, u.Input, u.CacheRead, u.Output, u.Reasoning)
		recordCodexCostUsageLocked(target, sessionID, projectName, model, u.Input, u.CacheRead, outputTokens, total)
		recordCodexSessionTokenUsage(target, sessionID, projectName, model, timestamp, u.Input, outputTokens, total)
		recordCodexContextTokenUsage(target, u.Input, u.CacheRead, u.Reasoning, total)
	}
}

func recordPiToolCall(agg *ProjectAggregate, state *piState, content *piContent, timestamp time.Time) {
	tool := content.Name
	if tool == "" {
		tool = "unknown"
	}
	callID := content.ID
	if callID == "" {
		callID = tool + ":" + timestamp.Format(time.RFC3339Nano)
	}
	input := content.Arguments
	if len(input) == 0 {
		input = []byte("{}")
	}
	call := pendingToolCall{
		ID:        callID,
		Tool:      agentToolName(tool),
		Model:     codexModel(state),
		Project:   codexProject(state),
		SessionID: codexSessionID(state),
		Timestamp: timestamp,
		Input:     input,
	}
	pending := &codexPendingTool{pendingToolCall: call}
	state.PendingTools[callID] = pending
	for _, target := range codexAggregatesForCall(agg, call) {
		addToolCallLocked(target, call.Tool, call.Model)
		addAgentToolCallLocked(target, call)
		addSessionToolCallLocked(target, call)
		recordStructuredToolInputLocked(target, &call)
	}
}

func recordPiToolResult(agg *ProjectAggregate, state *piState, msg *piMessage, timestamp time.Time) {
	callID := msg.ToolCallID
	pending := state.PendingTools[callID]
	if pending == nil {
		return
	}
	text := piMessageText(msg)
	// isError 是结构化失败信号（确认失败）；文本分类复用 Claude 规则细化 reason
	classification := toolFailureClassification{}
	if msg.IsError {
		classification = classifyToolResult(pending.Tool, AssistantContent{
			Type:    "tool_result",
			IsError: true,
			Content: json.RawMessage(strconv.Quote(text)),
		})
		classification.Basis = "is_error"
	} else if text != "" {
		classification = classifyToolResult(pending.Tool, AssistantContent{
			Type:    "tool_result",
			Content: json.RawMessage(strconv.Quote(text)),
		})
	}
	recordCodexToolResult(agg, pending, classification, previewString(text, 240), timestamp, 0)
	delete(state.PendingTools, callID)
}

func finalizePiPendings(agg *ProjectAggregate, state *piState) {
	for _, pending := range state.PendingTools {
		if pending.ResultRecorded {
			continue
		}
		recordCodexMissingTool(agg, pending.pendingToolCall)
		pending.WasMissingRecorded = true
	}
}

// agentToolName 把 pi/opencode 工具名归一到 Claude 惯例，便于跨 provider 对比。
func agentToolName(tool string) string {
	switch tool {
	case "bash":
		return "Bash"
	case "read":
		return "Read"
	case "write":
		return "Write"
	case "edit":
		return "Edit"
	case "grep":
		return "Grep"
	case "glob":
		return "Glob"
	case "ls":
		return "LS"
	default:
		return tool
	}
}

func piMessageText(msg *piMessage) string {
	out := make([]byte, 0, 256)
	for i := range msg.Content {
		if msg.Content[i].Type == "text" {
			if len(out) > 0 {
				out = append(out, '\n')
			}
			out = append(out, msg.Content[i].Text...)
		}
	}
	if len(out) > 4096 {
		out = out[:4096]
	}
	return string(out)
}
