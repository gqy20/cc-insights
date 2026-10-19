package main

// zcode agent 数据源：~/.zcode/cli/db/db.sqlite（只读，含 -wal/-shm 的活跃库）。
// 结构化程度高于 JSONL 渠道，各表分工：
//   - session: directory/title/parent_id → 项目归因、会话标题、子代理会话归并
//   - message: role/modelID/path.cwd/agent/tokens → 消息计数、token 记账（全历史主源）、
//     子代理归因（agent != zcode-agent）
//   - part(type=tool): callID/state{status,input,output,error,time} → 工具调用、
//     命令文本（Bash 命令族）与结构化失败；真实执行时长在 state.time（列会被批量回写污染）
//   - part(type=compaction): context_limit 压缩 → 上下文诊断
//   - tool_usage: 按 callID 关联 → exit_code/error_type/destructive 失败细分与风险信号
//   - model_usage: 请求级 duration/TTFT/状态 → 吞吐、首 token 延迟与模型侧失败
//     （仅覆盖启用之后的时段，token 记账不依赖它）
//   - todo: 任务状态 → TaskPlanAnalysis
// 单一 sqlite 文件即数据源：listLogFileInfos 把 db.sqlite 作为唯一文件条目
// （size/mtime 合并 -wal/-shm 作为变更信号），复用文件级缓存增量机制。

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const zcodeMainAgent = "zcode-agent"

type zcodeSessionInfo struct {
	Directory string
	Title     string
	ParentID  string
}

type zcodeTokenUsage struct {
	Total     int `json:"total"`
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

type zcodeMessageData struct {
	Role    string `json:"role"`
	ModelID string `json:"modelID"`
	Agent   string `json:"agent"`
	Model   struct {
		ModelID string `json:"modelID"`
	} `json:"model"`
	Path struct {
		Cwd string `json:"cwd"`
	} `json:"path"`
	ContextSnapshot struct {
		EnvInfo struct {
			Cwd string `json:"cwd"`
		} `json:"envInfo"`
	} `json:"contextSnapshot"`
	Tokens zcodeTokenUsage `json:"tokens"`
}

type zcodeToolPartData struct {
	Type   string `json:"type"`
	CallID string `json:"callID"`
	Tool   string `json:"tool"`
	State  struct {
		Status string          `json:"status"`
		Input  json.RawMessage `json:"input"`
		Output string          `json:"output"`
		Error  string          `json:"error"`
		Time   *struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		} `json:"time"`
	} `json:"state"`
}

// zcodeToolUsageInfo 来自 tool_usage 表（按 callID 关联），结构化失败与风险信号。
type zcodeToolUsageInfo struct {
	Status      string
	ErrorType   string
	ExitCode    sql.NullInt64
	Destructive bool
}

type zcodeMessageMeta struct {
	Model   string
	Project string
	Agent   string
}

func parseZCodeDBAggregate(dbPath string, tf TimeFilter, agg *ProjectAggregate) error {
	db, err := sql.Open("sqlite", sqliteReadOnlyURI(dbPath))
	if err != nil {
		return err
	}
	defer db.Close()

	sessions, err := loadZCodeSessions(db)
	if err != nil {
		return err
	}
	toolUsage, err := loadZCodeToolUsage(db)
	if err != nil {
		return err
	}
	msgMeta, err := recordZCodeMessages(db, sessions, tf, agg)
	if err != nil {
		return err
	}
	if err := recordZCodeToolParts(db, sessions, msgMeta, toolUsage, tf, agg); err != nil {
		return err
	}
	if err := recordAgentCompactions(db, sessions, tf, agg); err != nil {
		return err
	}
	if err := recordAgentTodos(db, sessions, tf, agg); err != nil {
		return err
	}
	return recordZCodeModelUsage(db, sessions, tf, agg)
}

func loadZCodeSessions(db *sql.DB) (map[string]*zcodeSessionInfo, error) {
	rows, err := db.Query(`select id, coalesce(directory, ''), coalesce(path, ''),
coalesce(title, ''), coalesce(parent_id, '') from session`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := make(map[string]*zcodeSessionInfo, 256)
	for rows.Next() {
		var id, directory, path, title, parentID string
		if err := rows.Scan(&id, &directory, &path, &title, &parentID); err != nil {
			return nil, err
		}
		sessions[id] = &zcodeSessionInfo{
			Directory: firstNonEmpty(directory, path),
			Title:     title,
			ParentID:  parentID,
		}
	}
	return sessions, rows.Err()
}

// loadZCodeToolUsage 加载 tool_usage 表（按 tool_call_id 索引）：结构化失败与风险信号。
func loadZCodeToolUsage(db *sql.DB) (map[string]zcodeToolUsageInfo, error) {
	rows, err := db.Query(`select tool_call_id, coalesce(status, ''), coalesce(error_type, ''),
exit_code, coalesce(destructive, 0) from tool_usage`)
	if err != nil {
		// 表缺失属于旧版本 zcode：返回空索引，失败分类回退到 error 文本。
		return map[string]zcodeToolUsageInfo{}, nil
	}
	defer rows.Close()
	usage := make(map[string]zcodeToolUsageInfo, 65536)
	for rows.Next() {
		var callID, status, errorType string
		var exitCode sql.NullInt64
		var destructive int
		if err := rows.Scan(&callID, &status, &errorType, &exitCode, &destructive); err != nil {
			return nil, err
		}
		if callID == "" {
			continue
		}
		usage[callID] = zcodeToolUsageInfo{
			Status:      status,
			ErrorType:   errorType,
			ExitCode:    exitCode,
			Destructive: destructive != 0,
		}
	}
	return usage, rows.Err()
}

// resolveZCodeRootSession 子代理 session（parent_id 链）归并到顶层会话，
// 与 claude 的 sidechain 消息归入主会话语义对齐（实测 parent 链为单层）。
func resolveZCodeRootSession(sessions map[string]*zcodeSessionInfo, sessionID string) string {
	for depth := 0; depth < 5; depth++ {
		info := sessions[sessionID]
		if info == nil || info.ParentID == "" || info.ParentID == sessionID {
			return sessionID
		}
		sessionID = info.ParentID
	}
	return sessionID
}

func zcodeProjectFor(sessions map[string]*zcodeSessionInfo, sessionID, cwd string) string {
	if cwd != "" {
		return cwd
	}
	if info := sessions[sessionID]; info != nil {
		return firstNonEmpty(info.Directory, "Unknown")
	}
	return "Unknown"
}

func recordZCodeMessages(db *sql.DB, sessions map[string]*zcodeSessionInfo, tf TimeFilter, agg *ProjectAggregate) (map[string]zcodeMessageMeta, error) {
	rows, err := db.Query(`select id, session_id, coalesce(time_created, 0), data from message`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msgMeta := make(map[string]zcodeMessageMeta, 65536)
	rootSessions := make(map[string]bool, len(sessions))
	for rows.Next() {
		var id, sessionID string
		var tsMs int64
		var raw []byte
		if err := rows.Scan(&id, &sessionID, &tsMs, &raw); err != nil {
			return nil, err
		}
		var msg zcodeMessageData
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
		}
		ts := time.UnixMilli(tsMs)
		project := zcodeProjectFor(sessions, sessionID, firstNonEmpty(msg.Path.Cwd, msg.ContextSnapshot.EnvInfo.Cwd))
		model := firstNonEmpty(msg.ModelID, msg.Model.ModelID, "unknown")
		msgMeta[id] = zcodeMessageMeta{
			Model:   model,
			Project: project,
			Agent:   msg.Agent,
		}

		if !tf.Contains(ts) {
			continue
		}
		rootID := resolveZCodeRootSession(sessions, sessionID)
		switch msg.Role {
		case "assistant":
			recordAssistantMessage(agg, project, rootID, model, ts)
			// token 记账主源：message.data.tokens 覆盖全历史（model_usage 只覆盖启用后时段）。
			recordZCodeTokenUsageLocked(agg, rootID, project, model, ts, msg.Tokens)
		case "user":
			recordZCodeUserMessage(agg, rootID, project, ts)
		}
		if !rootSessions[rootID] {
			rootSessions[rootID] = true
			recordZCodeSessionTitle(agg, sessions, rootID)
		}
	}
	return msgMeta, rows.Err()
}

// recordZCodeSessionTitle 用 session 表的显式标题（AI 生成摘要）作为会话标题，
// 优先级高于 last-prompt 推断（在首个消息统计入账后调用一次）。
func recordZCodeSessionTitle(agg *ProjectAggregate, sessions map[string]*zcodeSessionInfo, rootID string) {
	info := sessions[rootID]
	if info == nil || info.Title == "" {
		return
	}
	stat := agg.SessionStatsMap[rootID]
	if stat == nil || stat.TitleSource != "" {
		return
	}
	recordSessionTitle(stat, "ai-title", info.Title)
}

func recordZCodeUserMessage(agg *ProjectAggregate, sessionID, project string, ts time.Time) {
	if ts.IsZero() {
		return
	}
	stat := ensureSessionStat(agg, sessionID, project)
	if stat == nil {
		return
	}
	recordSessionTimestamp(stat, ts)
	stat.UserMessageCount++
	stat.MessageCount++
}

func recordZCodeToolParts(db *sql.DB, sessions map[string]*zcodeSessionInfo, msgMeta map[string]zcodeMessageMeta, toolUsage map[string]zcodeToolUsageInfo, tf TimeFilter, agg *ProjectAggregate) error {
	// json_extract 过滤 type=tool：宁可全表扫 JSON，也不依赖字段顺序（实测有带/不带空格两种序列化）。
	rows, err := db.Query(`select message_id, session_id, coalesce(time_created, 0), coalesce(time_updated, 0), data
from part where json_extract(data, '$.type') = 'tool'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var messageID, sessionID string
		var createdMs, updatedMs int64
		var raw []byte
		if err := rows.Scan(&messageID, &sessionID, &createdMs, &updatedMs, &raw); err != nil {
			return err
		}
		var part zcodeToolPartData
		if err := json.Unmarshal(raw, &part); err != nil || part.Tool == "" {
			continue
		}
		meta := msgMeta[messageID]
		project := zcodeProjectFor(sessions, sessionID, meta.Project)
		// 真实执行起止在 state.time：time_created/time_updated 列会被批量回写污染，仅作回退。
		callTs := time.UnixMilli(createdMs)
		durationMs := updatedMs - createdMs
		if part.State.Time != nil {
			if part.State.Time.Start > 0 {
				callTs = time.UnixMilli(part.State.Time.Start)
			}
			if part.State.Time.Start > 0 && part.State.Time.End > 0 {
				durationMs = part.State.Time.End - part.State.Time.Start
			}
		}
		if !tf.Contains(callTs) {
			continue
		}
		rootID := resolveZCodeRootSession(sessions, sessionID)
		agent := firstNonEmpty(meta.Agent, zcodeMainAgent)
		call := pendingToolCall{
			ID:   part.CallID,
			Tool: part.Tool,
			// 子代理（zcode-Explore/general-purpose 等）的工具调用归入主会话，agent 维度单独统计。
			AgentID:     agentValueFor(agent),
			IsSidechain: agent != zcodeMainAgent,
			Model:       firstNonEmpty(meta.Model, "unknown"),
			Project:     project,
			SessionID:   rootID,
			Timestamp:   callTs,
			Input:       part.State.Input,
		}
		if call.ID == "" {
			call.ID = part.Tool + ":" + callTs.Format(time.RFC3339Nano)
		}
		usage := toolUsage[part.CallID]
		targets := codexAggregatesForCall(agg, call)
		for _, target := range targets {
			addToolCallLocked(target, call.Tool, call.Model)
			addAgentToolCallLocked(target, call)
			addSessionToolCallLocked(target, call)
			recordStructuredToolInputLocked(target, &call)
			if usage.Destructive {
				target.EventTypes["zcode:destructive_tool"]++
			}
		}

		resultTs := callTs.Add(time.Duration(durationMs) * time.Millisecond)
		switch part.State.Status {
		case "completed":
			preview := previewString(part.State.Output, 240)
			for _, target := range targets {
				addToolResultLocked(target, call, toolFailureClassification{}, preview, resultTs, durationMs, len(part.State.Output))
			}
		case "error":
			// status=error 是结构化失败信号（Basis=field），细分原因见 classifyZCodeToolError。
			classification := classifyZCodeToolError(call.Tool, part.State.Error, usage)
			preview := previewString(firstNonEmpty(part.State.Error, part.State.Output), 240)
			for _, target := range targets {
				addToolResultLocked(target, call, classification, preview, resultTs, durationMs, len(preview))
			}
		default:
			// running 等未闭合状态：计入 missing（与 claude/codex 的缺失语义一致）。
			for _, target := range targets {
				addMissingToolResultLocked(target, call)
			}
		}
	}
	return rows.Err()
}

// agentValueFor 子代理名去掉 zcode- 前缀，与 claude 渠道的 agent 展示风格对齐。
func agentValueFor(agent string) string {
	if agent == zcodeMainAgent {
		return ""
	}
	return strings.TrimPrefix(agent, "zcode-")
}

// classifyZCodeToolError 细分 zcode 工具失败原因。
// 优先级：结构化 error_type/exit_code（tool_usage 表，Basis=field）
// → error 文本关键词（复用 claude 渠道分类器）→ 兜底 error_status。
func classifyZCodeToolError(tool, errorText string, usage zcodeToolUsageInfo) toolFailureClassification {
	lower := strings.ToLower(strings.TrimSpace(errorText))
	switch {
	case usage.ErrorType == "tool_timeout" || strings.Contains(lower, "timed out"):
		return newFailureClassification("timeout", "timeout", "timeout")
	case usage.ExitCode.Valid && usage.ExitCode.Int64 != 0:
		return newFailureClassification("command_failed", "bash", fmt.Sprintf("exit_code_%d", usage.ExitCode.Int64))
	case usage.ErrorType == "permission_denied" || strings.Contains(lower, "permission denied"):
		return newFailureClassification("permission", "permission", "permission_denied")
	case usage.ErrorType == "tool_cancelled" || strings.Contains(lower, "was cancelled"):
		// 用户/系统中断，与 claude 的 explicit_error（用户拒绝工具调用）语义对齐。
		return newFailureClassification("explicit_error", failureCategoryForTool(tool), "user_cancelled")
	}
	if category, reason, ok := classifyDetailedFailure(tool, lower); ok {
		return newFailureClassification("", category, reason)
	}
	return newFailureClassification("tool_error", failureCategoryForTool(tool), "error_status")
}

func recordAgentCompactions(db *sql.DB, sessions map[string]*zcodeSessionInfo, tf TimeFilter, agg *ProjectAggregate) error {
	rows, err := db.Query(`select session_id, coalesce(time_created, 0), data from part where json_extract(data, '$.type') = 'compaction'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sessionID string
		var tsMs int64
		var raw []byte
		if err := rows.Scan(&sessionID, &tsMs, &raw); err != nil {
			return err
		}
		ts := time.UnixMilli(tsMs)
		if !tf.Contains(ts) {
			continue
		}
		call := pendingToolCall{
			SessionID: resolveZCodeRootSession(sessions, sessionID),
			Project:   zcodeProjectFor(sessions, sessionID, ""),
			Timestamp: ts,
		}
		for _, target := range codexAggregatesForCall(agg, call) {
			ensureCodexContextStats(target).ContextCompactions++
		}
	}
	return rows.Err()
}

func recordZCodeModelUsage(db *sql.DB, sessions map[string]*zcodeSessionInfo, tf TimeFilter, agg *ProjectAggregate) error {
	rows, err := db.Query(`select session_id, coalesce(model_id, ''), coalesce(started_at, 0),
coalesce(duration_ms, 0), coalesce(time_to_first_token_ms, 0), coalesce(status, ''),
coalesce(error_type, '') from model_usage`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var sessionID, model, status, errorType string
		var startedMs, durationMs, ttftMs int64
		if err := rows.Scan(&sessionID, &model, &startedMs, &durationMs, &ttftMs, &status, &errorType); err != nil {
			return err
		}
		ts := time.UnixMilli(startedMs)
		if !tf.Contains(ts) {
			continue
		}
		rootID := resolveZCodeRootSession(sessions, sessionID)
		project := zcodeProjectFor(sessions, sessionID, "")
		if status == "error" {
			// 模型侧失败（server_error/timeout/rate_limited/context_exceeded 等）：事件信号供下钻。
			event := "zcode:model_error:" + firstNonEmpty(errorType, "unknown")
			dateKey := ts.Format("2006-01-02")
			agg.EventTypes[event]++
			ensureDailyRuntimeAggregate(agg, dateKey).EventTypes[event]++
			continue
		}
		if durationMs <= 0 {
			continue
		}
		// 请求级真实耗时/首 token 延迟 → 吞吐与延迟诊断（token 记账在 message 扫描完成）。
		recordZCodeRequestDurationLocked(agg, rootID, project, model, ts, durationMs, ttftMs)
	}
	return rows.Err()
}

func recordZCodeTokenUsageLocked(agg *ProjectAggregate, sessionID, project, model string, ts time.Time, tokens zcodeTokenUsage) {
	if tokens.Total <= 0 {
		return
	}
	if model == "" {
		model = "unknown"
	}
	outputWithReasoning := tokens.Output + tokens.Reasoning
	dateKey := ts.Format("2006-01-02")
	for _, target := range []*ProjectAggregate{
		agg,
		ensureDailyRuntimeAggregate(agg, dateKey),
		ensureDailyProjectRuntimeAggregate(agg, dateKey, project),
		ensureDailySessionRuntimeAggregate(agg, dateKey, sessionID),
	} {
		recordModelUsageLocked(target, model, tokens.Total)
		recordModelTokenBreakdownLocked(target, model, tokens.Input, tokens.Cache.Read, tokens.Output, tokens.Reasoning)
		recordCodexSessionTokenUsage(target, sessionID, project, model, ts, tokens.Input, outputWithReasoning, tokens.Total)
		recordCodexCostUsageLocked(target, sessionID, project, model, tokens.Input, tokens.Cache.Read, outputWithReasoning, tokens.Total)
		recordCodexContextTokenUsage(target, tokens.Input, tokens.Cache.Read, tokens.Reasoning, tokens.Total)
	}
	if agg.DailyModelCounts[dateKey] == nil {
		agg.DailyModelCounts[dateKey] = make(map[string]int)
	}
	agg.DailyModelCounts[dateKey][model]++
	if agg.DailyModelTokens[dateKey] == nil {
		agg.DailyModelTokens[dateKey] = make(map[string]int)
	}
	agg.DailyModelTokens[dateKey][model] += tokens.Total
}

func recordZCodeRequestDurationLocked(agg *ProjectAggregate, sessionID, project, model string, ts time.Time, durationMs, ttftMs int64) {
	if model == "" {
		model = "unknown"
	}
	dateKey := ts.Format("2006-01-02")
	for _, target := range []*ProjectAggregate{
		agg,
		ensureDailyRuntimeAggregate(agg, dateKey),
		ensureDailyProjectRuntimeAggregate(agg, dateKey, project),
		ensureDailySessionRuntimeAggregate(agg, dateKey, sessionID),
	} {
		ensureCostModelStat(target, model).TotalRoundTripMs += durationMs
		ensureCostProjectStat(target, project).TotalRoundTripMs += durationMs
		if sessionID != "" {
			ensureCostSessionStat(target, sessionID, project).TotalRoundTripMs += durationMs
		}
		if ttftMs > 0 {
			ensureCostModelStat(target, model).TotalTTFTMs += ttftMs
		}
	}
}

// recordAgentTodos 把 todo 表按 time_created 归因写入 TaskAgg（顶层 + 按天快照），
// 子代理 session 的 todo 归并到主会话。zcode 与 opencode（sqlite 布局）的 todo 表同构，共用。
// 时间过滤查询经按天快照重建 TaskPlanAnalysis。
func recordAgentTodos(db *sql.DB, sessions map[string]*zcodeSessionInfo, tf TimeFilter, agg *ProjectAggregate) error {
	rows, err := db.Query(`select session_id, coalesce(status, ''), coalesce(time_created, 0) from todo`)
	if err != nil {
		// 表缺失属旧版本 zcode：无任务信号，跳过。
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var sessionID, status string
		var tsMs int64
		if err := rows.Scan(&sessionID, &status, &tsMs); err != nil {
			return err
		}
		if status == "" {
			status = "pending"
		}
		ts := time.UnixMilli(tsMs)
		if !tf.Contains(ts) {
			continue
		}
		rootID := resolveZCodeRootSession(sessions, sessionID)
		recordTaskAggLocked(agg, rootID, status)
		recordTaskAggLocked(ensureDailyRuntimeAggregate(agg, ts.Format("2006-01-02")), rootID, status)
	}
	return rows.Err()
}

// sqliteDataFileInfos 把 sqlite 库文件的 size/mtime 合并 -wal/-shm 统计：
// 活跃库的写入先落 WAL，库本体 mtime 可能不变，合并后才能作为变更信号。
// zcode（db.sqlite）与 opencode（opencode.db）共用。
func sqliteDataFileInfos(files []projectFileInfo) []projectFileInfo {
	for i := range files {
		base := files[i].AbsPath
		var size int64
		modTime := files[i].ModTime
		for _, suffix := range []string{"", "-wal", "-shm"} {
			info, err := os.Stat(base + suffix)
			if err != nil {
				continue
			}
			size += info.Size()
			if m := info.ModTime().UnixNano(); m > modTime {
				modTime = m
			}
		}
		if size > 0 {
			files[i].Size = size
		}
		files[i].ModTime = modTime
	}
	return files
}

func zcodeDBPathForDataDir(dataDir string) string {
	return filepath.Join(dataDir, "cli", "db", "db.sqlite")
}
