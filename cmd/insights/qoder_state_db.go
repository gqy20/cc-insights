package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Qoder 状态库（main.sqlite）是**只读增强层**，不是第二个数据源：
// 实测它只有 4 个会话 / 56 条消息，与 transcript 同源且不占优，消息与工具计数
// 全部以 transcript 为准。它独占的是 transcript 里没有的 turn 级信号：
//
//   - chat_session_messages.status：completed / failed / interrupted（权威 turn 结局）
//   - assistant payload_json.durationMs：整轮耗时（Qoder 不写 Claude 那种 system/turn_duration 行）
//
// 刻意不接入的两类，是为了避免与 transcript 派生指标重复计数：
// tool part 的 durationMs（已可由 tool_use→tool_result 时间差得到）与
// turn_file_change_files 的增删行（已从 Edit/Write 入参得到）。

const (
	qoderStateDBName    = "main.sqlite"
	qoderAppStorageName = "com.qoder.app.stable"
)

// qoderStateDBCandidates 列出各平台的候选状态库路径。
// Windows/macOS 路径已在本机与官方布局核实；Linux 路径是 new-api 侧常见约定，
// **未核实**，探测不到就静默跳过，不影响 transcript 主链路。
func qoderStateDBCandidates() []string {
	homeDir, _ := os.UserHomeDir()
	var dirs []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		dirs = append(dirs, filepath.Join(appData, qoderAppStorageName))
	} else {
		dirs = append(dirs, filepath.Join(homeDir, "AppData", "Roaming", qoderAppStorageName))
	}
	dirs = append(dirs,
		filepath.Join(homeDir, "Library", "Application Support", qoderAppStorageName),
		filepath.Join(homeDir, ".config", qoderAppStorageName),
	)
	candidates := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir != "" {
			candidates = append(candidates, filepath.Join(dir, qoderStateDBName))
		}
	}
	return candidates
}

// qoderStateDBPath 返回第一个存在的状态库路径；都没有则空串（增强层整体跳过）。
func qoderStateDBPath() string {
	for _, candidate := range qoderStateDBCandidates() {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

type qoderSessionMeta struct {
	cwd   string
	title string
}

type qoderAssistantPayload struct {
	DurationMs int64 `json:"durationMs"`
	TurnMetric struct {
		DurationMs int64 `json:"durationMs"`
		TurnCount  int   `json:"turnCount"`
	} `json:"turnMetrics"`
}

func (p qoderAssistantPayload) duration() int64 {
	if p.DurationMs > 0 {
		return p.DurationMs
	}
	return p.TurnMetric.DurationMs
}

// recordQoderStateDBLocked 读取状态库并把 turn 级信号并入聚合。
// 任何失败都由调用方降级为 Warn：transcript 才是主数据源。
func recordQoderStateDBLocked(agg *ProjectAggregate, dbPath string, tf TimeFilter) error {
	if dbPath == "" {
		return nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	db, err := sql.Open("sqlite", sqliteReadOnlyURI(dbPath))
	if err != nil {
		return err
	}
	defer db.Close()

	sessions, err := loadQoderSessionMeta(db)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return nil
	}
	rows, err := db.Query(`select session_id, status, created_at, payload_json
from chat_session_messages
where role = 'assistant'
order by created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var sessionID, status, payloadJSON string
		var createdAt int64
		if err := rows.Scan(&sessionID, &status, &createdAt, &payloadJSON); err != nil {
			return err
		}
		timestamp := time.UnixMilli(createdAt)
		if !tf.Contains(timestamp) {
			continue
		}
		recordQoderTurnLocked(agg, sessions, sessionID, status, timestamp, payloadJSON)
	}
	return rows.Err()
}

func loadQoderSessionMeta(db *sql.DB) (map[string]qoderSessionMeta, error) {
	rows, err := db.Query(`select session_id, coalesce(cwd, ''), coalesce(title, '') from chat_sessions`)
	if err != nil {
		// 状态库版本不含该表时整体跳过，不视为错误。
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	sessions := make(map[string]qoderSessionMeta)
	for rows.Next() {
		var sessionID, cwd, title string
		if err := rows.Scan(&sessionID, &cwd, &title); err != nil {
			return nil, err
		}
		sessions[sessionID] = qoderSessionMeta{cwd: cwd, title: title}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func recordQoderTurnLocked(agg *ProjectAggregate, sessions map[string]qoderSessionMeta, sessionID, status string, timestamp time.Time, payloadJSON string) {
	projectName := "Unknown"
	if meta, ok := sessions[sessionID]; ok && meta.cwd != "" {
		projectName = meta.cwd
	}
	dateKey := timestamp.Format("2006-01-02")

	// turn 结局：transcript 里只有 tool_result 级别的 is_error，
	// 整轮 failed/interrupted 是状态库独有的信号。
	switch status {
	case "failed", "interrupted":
		event := "qoder_turn_" + status
		recordEventCount(agg, event)
		recordEventCount(ensureDailyRuntimeAggregate(agg, dateKey), event)
	}

	var payload qoderAssistantPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return
	}
	durationMs := payload.duration()
	if durationMs <= 0 {
		return
	}
	recordTurnDurationLocked(agg, durationMs, payload.TurnMetric.TurnCount, sessionID, projectName, timestamp)
	recordTurnDurationLocked(ensureDailyRuntimeAggregate(agg, dateKey), durationMs, payload.TurnMetric.TurnCount, sessionID, projectName, timestamp)
	recordTurnDurationLocked(ensureDailyProjectRuntimeAggregate(agg, dateKey, projectName), durationMs, payload.TurnMetric.TurnCount, sessionID, projectName, timestamp)
	recordTurnDurationLocked(ensureDailySessionRuntimeAggregate(agg, dateKey, sessionID), durationMs, payload.TurnMetric.TurnCount, sessionID, projectName, timestamp)
}

func recordEventCount(agg *ProjectAggregate, event string) {
	if agg.EventTypes == nil {
		agg.EventTypes = make(map[string]int)
	}
	agg.EventTypes[event]++
}

// contextCategorySnapshot 是 chat_session_context_usage.snapshot_json 的按需切片。
// token 字段被官方门控为 0（tokenCountsAvailable=false），只有占比可用。
type contextCategorySnapshot struct {
	Categories []struct {
		Name       string  `json:"name"`
		Percentage float64 `json:"percentage"`
	} `json:"categories"`
}

// recordQoderContextCategories 把上下文构成占比并入聚合。
// 同名类别取观测到的最高占比：max 是单调量，跨文件/跨天/跨渠道合并都不会失真。
// 必须同时写入按天快照——时间过滤查询会从按天快照重建分析层，只写顶层会让
// 带 -p 的查询读不到这份数据（与 peak_context_ratio 同一个坑）。
// tf 在无缓存的直接解析路径上过滤快照行：窗口外的行顶层与按天快照都不写，
// 避免 -p 7d 的上下文构成混进更早的天（缓存构建路径传 TimeFilter{} 全量采集）。
func recordQoderContextCategories(agg *ProjectAggregate, dbPath string, tf TimeFilter) error {
	if dbPath == "" {
		return nil
	}
	db, err := sql.Open("sqlite", sqliteReadOnlyURI(dbPath))
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`select snapshot_json, updated_at from chat_session_context_usage`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var raw string
		var updatedAt int64
		if err := rows.Scan(&raw, &updatedAt); err != nil {
			return err
		}
		timestamp := time.UnixMilli(updatedAt).Local()
		if !tf.Contains(timestamp) {
			continue
		}
		var snapshot contextCategorySnapshot
		if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
			continue
		}
		if len(snapshot.Categories) == 0 {
			continue
		}
		targets := []*ProjectAggregate{agg}
		if dateKey := timestamp.Format("2006-01-02"); dateKey != "0001-01-01" {
			targets = append(targets, ensureDailyRuntimeAggregate(agg, dateKey))
		}
		for _, target := range targets {
			stats := ensureContextStats(target)
			if stats.CategoryShares == nil {
				stats.CategoryShares = make(map[string]float64, len(snapshot.Categories))
			}
			for _, category := range snapshot.Categories {
				if category.Name == "" || category.Percentage <= 0 {
					continue
				}
				if category.Percentage > stats.CategoryShares[category.Name] {
					stats.CategoryShares[category.Name] = category.Percentage
				}
			}
		}
	}
	return rows.Err()
}

// formatContextCategoryShares 渲染上下文构成Top-N，供 CLI 单行展示。
func formatContextCategoryShares(shares map[string]float64, limit int) string {
	if len(shares) == 0 {
		return ""
	}
	type entry struct {
		name  string
		value float64
	}
	entries := make([]entry, 0, len(shares))
	for name, value := range shares {
		entries = append(entries, entry{name: name, value: value})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].value != entries[j].value {
			return entries[i].value > entries[j].value
		}
		return entries[i].name < entries[j].name
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	parts := make([]string, 0, len(entries))
	for _, item := range entries {
		parts = append(parts, fmt.Sprintf("%s %.0f%%", item.name, item.value*100))
	}
	return strings.Join(parts, " · ")
}
