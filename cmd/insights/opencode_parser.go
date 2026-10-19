package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// opencode 数据源：~/.local/share/opencode/storage/{session,message,part}/
// 每条记录一个 JSON 文件（非 JSONL）：
//   - session/<projHash>/<ses>.json：directory（项目路径）、time.created/updated
//   - message/<ses>/<msg>.json：role、model{providerID,modelID}、time.created
//   - part/<msg>/<part>.json：type=text|reasoning|step-start|step-finish|tool
//
// 解析模型：message/ 与 session/ 每次构建重建轻量索引（文件小、成本可控）
// 并完成消息级计数；part/（尤其 type=tool）作为文件级增量缓存单元，
// state.status/error 是结构化失败信号。无 token 用量数据。

type openCodeSessionInfo struct {
	Directory string
	CreatedMs int64
	UpdatedMs int64
}

type openCodeMessageInfo struct {
	SessionID string
	Role      string
	Model     string
	Agent     string
	CreatedMs int64
}

type openCodeIndex struct {
	sessions map[string]openCodeSessionInfo
	messages map[string]openCodeMessageInfo
}

// buildOpenCodeIndexInto 扫描 session/ 与 message/ 树建立索引，并把消息级
// 统计（消息数、模型使用、session 生命周期）计入 aggregate。
// ponytail: 每次全量重建都会重读 message/ 树（无文件级缓存）；消息文件
// 很小（数百字节），量级到 10 万条时再考虑把索引本身缓存。
func buildOpenCodeIndexInto(dataDir string, agg *ProjectAggregate) *openCodeIndex {
	idx := &openCodeIndex{
		sessions: make(map[string]openCodeSessionInfo),
		messages: make(map[string]openCodeMessageInfo),
	}
	storageDir := filepath.Join(dataDir, "storage")

	// session 元数据
	_ = filepath.WalkDir(filepath.Join(storageDir, "session"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		var s struct {
			ID        string `json:"id"`
			Directory string `json:"directory"`
			Time      struct {
				Created int64 `json:"created"`
				Updated int64 `json:"updated"`
			} `json:"time"`
		}
		if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &s) == nil && s.ID != "" {
			idx.sessions[s.ID] = openCodeSessionInfo{Directory: s.Directory, CreatedMs: s.Time.Created, UpdatedMs: s.Time.Updated}
		}
		return nil
	})

	// message 元数据 + 消息计数。
	// opencode 把 model 存在 user 消息上（assistant 常为空），
	// 按时间序把 session 内最近一次 model 传播给后续 assistant 消息。
	type rawMessage struct {
		id, sessionID, role, agent, model string
		createdMs                         int64
	}
	var rawMessages []rawMessage
	_ = filepath.WalkDir(filepath.Join(storageDir, "message"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		var m struct {
			ID        string `json:"id"`
			SessionID string `json:"sessionID"`
			Role      string `json:"role"`
			Agent     string `json:"agent"`
			Model     struct {
				ProviderID string `json:"providerID"`
				ModelID    string `json:"modelID"`
			} `json:"model"`
			Time struct {
				Created int64 `json:"created"`
			} `json:"time"`
		}
		if data, err := os.ReadFile(path); err != nil || json.Unmarshal(data, &m) != nil || m.ID == "" {
			return nil
		}
		model := m.Model.ModelID
		if m.Model.ProviderID != "" {
			model = m.Model.ProviderID + "/" + m.Model.ModelID
		}
		rawMessages = append(rawMessages, rawMessage{id: m.ID, sessionID: m.SessionID, role: m.Role, agent: m.Agent, model: model, createdMs: m.Time.Created})
		return nil
	})
	sort.Slice(rawMessages, func(i, j int) bool {
		if rawMessages[i].createdMs != rawMessages[j].createdMs {
			return rawMessages[i].createdMs < rawMessages[j].createdMs
		}
		return rawMessages[i].id < rawMessages[j].id
	})
	sessionModels := make(map[string]string)
	for _, m := range rawMessages {
		if m.model != "" {
			sessionModels[m.sessionID] = m.model
		}
		model := m.model
		if model == "" {
			model = sessionModels[m.sessionID]
		}
		idx.messages[m.id] = openCodeMessageInfo{
			SessionID: m.sessionID,
			Role:      m.role,
			Model:     model,
			Agent:     m.agent,
			CreatedMs: m.createdMs,
		}
		if m.role == "assistant" {
			ts := msToTime(m.createdMs)
			recordAssistantMessage(agg, idx.projectFor(m.sessionID), m.sessionID, model, ts)
			recordPiModelUsage(agg, model, ts, nil)
		}
	}

	// session 生命周期
	for id, s := range idx.sessions {
		stat := ensureSessionStat(agg, id, idx.projectFor(id))
		if stat == nil {
			continue
		}
		recordSessionTimestamp(stat, msToTime(s.CreatedMs))
		recordSessionTimestamp(stat, msToTime(s.UpdatedMs))
	}
	return idx
}

func (idx *openCodeIndex) projectFor(sessionID string) string {
	if s, ok := idx.sessions[sessionID]; ok && s.Directory != "" {
		return s.Directory
	}
	return "Unknown"
}

// openCodePartUsage part 文件的可解析字段（tool 与 step-finish 共用）。
type openCodePartUsage struct {
	ID        string `json:"id"`
	MessageID string `json:"messageID"`
	Type      string `json:"type"`
	Tool      string `json:"tool"`
	CallID    string `json:"callID"`
	State     struct {
		Status string          `json:"status"`
		Input  json.RawMessage `json:"input"`
		Output string          `json:"output"`
		Error  string          `json:"error"`
		Time   struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		} `json:"time"`
	} `json:"state"`
	Tokens struct {
		Input     int `json:"input"`
		Output    int `json:"output"`
		Reasoning int `json:"reasoning"`
		Cache     struct {
			Read  int `json:"read"`
			Write int `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
}

// parseOpenCodePartFile 解析单个 part/<msgID>/<part>.json 文件（增量缓存单元）。
func parseOpenCodePartFile(relPath, filePath string, idx *openCodeIndex, agg *ProjectAggregate) {
	var p openCodePartUsage
	data, err := os.ReadFile(filePath)
	if err != nil || json.Unmarshal(data, &p) != nil {
		return
	}
	if p.Type == "step-finish" {
		recordOpenCodeStepUsage(agg, idx, &p)
		return
	}
	if p.Type != "tool" {
		return
	}
	recordOpenCodeToolPart(agg, idx, &p)
}

// recordOpenCodeToolPart type=tool 的 part：自含调用+结果。
func recordOpenCodeToolPart(agg *ProjectAggregate, idx *openCodeIndex, p *openCodePartUsage) {
	msg := idx.messages[p.MessageID]
	tool := agentToolName(p.Tool)
	ts := msToTime(p.State.Time.Start)
	if ts.IsZero() {
		ts = msToTime(msg.CreatedMs)
	}
	call := pendingToolCall{
		ID:        p.CallID,
		Tool:      tool,
		Model:     msg.Model,
		Project:   idx.projectFor(msg.SessionID),
		SessionID: msg.SessionID,
		Timestamp: ts,
		Input:     normalizeOpenCodeInput(p.State.Input),
	}
	if call.Model == "" {
		call.Model = "unknown"
	}
	durationMs := p.State.Time.End - p.State.Time.Start
	text := p.State.Output
	if p.State.Error != "" {
		text = p.State.Error
	}

	classification := toolFailureClassification{}
	if p.State.Status == "error" || p.State.Error != "" {
		classification = classifyToolResult(tool, AssistantContent{
			Type:    "tool_result",
			IsError: true,
			Content: json.RawMessage(strconv.Quote(text)),
		})
		classification.Basis = "status"
	} else if text != "" {
		classification = classifyToolResult(tool, AssistantContent{
			Type:    "tool_result",
			Content: json.RawMessage(strconv.Quote(text)),
		})
	}

	for _, target := range codexAggregatesForCall(agg, call) {
		addToolCallLocked(target, call.Tool, call.Model)
		addAgentToolCallLocked(target, call)
		addSessionToolCallLocked(target, call)
		recordStructuredToolInputLocked(target, &call)
		addToolResultLocked(target, call, classification, previewString(text, 240), ts, durationMs, len(text))
	}
}

// normalizeOpenCodeInput 归一化 opencode 工具输入：
// input 可能是 JSON 字符串或对象；filePath 键统一改写为 file_path，
// 以复用 Claude 侧的命令/文件分析。
func normalizeOpenCodeInput(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return []byte("{}")
	}
	// 双重编码：字符串形态先解引号
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return raw
		}
		raw = json.RawMessage(s)
		if len(raw) == 0 {
			return []byte("{}")
		}
	}
	if raw[0] != '{' && raw[0] != '[' {
		return json.RawMessage(`{"input":` + string(raw) + `}`)
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	if v, ok := m["filePath"]; ok && m["file_path"] == nil {
		m["file_path"] = v
		delete(m, "filePath")
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

func msToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// recordOpenCodeStepUsage 把 step-finish part 的 tokens 记入模型/成本/
// 会话/上下文统计（opencode 的 usage 载体，与 pi message.usage 同构）。
// ponytail: 无时间戳（part 无 time 字段），用所属消息的创建时间近似。
func recordOpenCodeStepUsage(agg *ProjectAggregate, idx *openCodeIndex, p *openCodePartUsage) {
	msg := idx.messages[p.MessageID]
	model := msg.Model
	if model == "" {
		model = "unknown"
	}
	ts := msToTime(msg.CreatedMs)
	total := p.Tokens.Input + p.Tokens.Output + p.Tokens.Reasoning + p.Tokens.Cache.Read + p.Tokens.Cache.Write
	projectName := idx.projectFor(msg.SessionID)
	sessionID := msg.SessionID
	outputTokens := p.Tokens.Output + p.Tokens.Reasoning

	if !ts.IsZero() {
		dateKey := ts.Format("2006-01-02")
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
		Project: projectName, SessionID: sessionID, Model: model, Timestamp: ts,
	}) {
		recordModelUsageLocked(target, model, total)
		recordModelTokenBreakdownLocked(target, model, p.Tokens.Input, p.Tokens.Cache.Read, p.Tokens.Output, p.Tokens.Reasoning)
		recordCodexCostUsageLocked(target, sessionID, projectName, model, p.Tokens.Input, p.Tokens.Cache.Read, outputTokens, total)
		recordCodexSessionTokenUsage(target, sessionID, projectName, model, ts, p.Tokens.Input, outputTokens, total)
		recordCodexContextTokenUsage(target, p.Tokens.Input, p.Tokens.Cache.Read, p.Tokens.Reasoning, total)
	}
}
