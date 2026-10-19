package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const codexHookLogDBName = "logs_2.sqlite"

// sqliteReadOnlyURI 构造跨平台合法的 sqlite 只读 file URI。
// Windows 路径（C:\Users\...）需先转为带前导斜杠的正斜杠形式，
// 否则渲染成 file:C:%5CUsers... 这类非法 URI，sqlite 驱动会拒绝。
func sqliteReadOnlyURI(dbPath string) string {
	uriPath := filepath.ToSlash(dbPath)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dbURL := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro&_busy_timeout=1000"}
	return dbURL.String()
}

var (
	codexHookThreadRe = regexp.MustCompile(`thread_id=([0-9a-fA-F-]+)`)
	codexHookTurnRe   = regexp.MustCompile(`turn\.id=([0-9a-fA-F-]+)`)
	// cwd= 位于 {...} 内，排除 } 即可；再排除 : 会把 Windows 盘符路径
	// （cwd=C:\Users\...）截断成 "C"，导致项目归因错误。
	codexHookCWDRe    = regexp.MustCompile(`cwd=([^}]+)`)
)

type codexHookLogRow struct {
	TS       int64
	TSNanos  int64
	Target   string
	ThreadID string
	Body     string
}

func recordCodexSQLiteHooksLocked(agg *ProjectAggregate, dataDir string, tf TimeFilter) error {
	dbPath := filepath.Join(dataDir, codexHookLogDBName)
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	// Windows 路径（C:\Users\...）需先转为带前导斜杠的正斜杠形式，
	// 否则渲染成 file:C:%5CUsers... 这类非法 URI，sqlite 驱动会拒绝。
	db, err := sql.Open("sqlite", sqliteReadOnlyURI(dbPath))
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`
select ts, ts_nanos, coalesce(target, ''), coalesce(thread_id, ''), coalesce(feedback_log_body, '')
from logs
where feedback_log_body like '%run_hooks_and_record_inputs%'
   or feedback_log_body like '%run_turn_stop_hooks%'
   or feedback_log_body like 'app-server event: hook/%'
order by ts, ts_nanos, id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	for rows.Next() {
		var row codexHookLogRow
		if err := rows.Scan(&row.TS, &row.TSNanos, &row.Target, &row.ThreadID, &row.Body); err != nil {
			return err
		}
		t := codexHookLogTime(row.TS, row.TSNanos)
		if !tf.Contains(t) {
			continue
		}
		recordCodexHookLogRowLocked(agg, row, seen, t)
	}
	return rows.Err()
}

func recordCodexHookLogRowLocked(agg *ProjectAggregate, row codexHookLogRow, seen map[string]bool, t time.Time) {
	body := row.Body
	if strings.HasPrefix(body, "app-server event: hook/") {
		event := strings.TrimPrefix(body, "app-server event: ")
		if idx := strings.IndexByte(event, ' '); idx >= 0 {
			event = event[:idx]
		}
		if event != "" {
			agg.EventTypes["codex_"+event]++
			ensureDailyRuntimeAggregate(agg, t.Format("2006-01-02")).EventTypes["codex_"+event]++
		}
		return
	}

	hookEvent := ""
	switch {
	case strings.Contains(body, "run_hooks_and_record_inputs"):
		hookEvent = "run_hooks_and_record_inputs"
	case strings.Contains(body, "run_turn_stop_hooks"):
		hookEvent = "run_turn_stop_hooks"
	default:
		return
	}

	threadID := firstNonEmpty(row.ThreadID, codexHookRegexpValue(codexHookThreadRe, body))
	turnID := codexHookRegexpValue(codexHookTurnRe, body)
	key := codexHookDedupeKey(hookEvent, threadID, turnID, row)
	if seen[key] {
		return
	}
	seen[key] = true

	agg.EventTypes["codex_hook_trace:"+hookEvent]++
	dailyAgg := ensureDailyRuntimeAggregate(agg, t.Format("2006-01-02"))
	dailyAgg.EventTypes["codex_hook_trace:"+hookEvent]++
	recordCodexHookTraceLocked(agg, threadID, codexHookProjectName(body), hookEvent)
	recordCodexHookTraceLocked(dailyAgg, threadID, codexHookProjectName(body), hookEvent)
}

func recordCodexHookTraceLocked(agg *ProjectAggregate, sessionID, project, hookEvent string) {
	const hookName = "CodexHookTrace"
	key := hookName + "\x00" + hookEvent
	if agg.HookStats[key] == nil {
		agg.HookStats[key] = &HookStatItem{HookName: hookName, HookEvent: hookEvent}
	}
	stat := agg.HookStats[key]
	stat.TotalCount++
	stat.SuccessCount++
	if sessionID != "" {
		if stat.SessionIDs == nil {
			stat.SessionIDs = make(map[string]bool)
		}
		stat.SessionIDs[sessionID] = true
	}

	session := ensureSessionStat(agg, sessionID, project)
	if session == nil {
		return
	}
	if session.HookBreakdown == nil {
		session.HookBreakdown = make(map[string]*SessionHookStat)
	}
	sessionHook := session.HookBreakdown[hookEvent]
	if sessionHook == nil {
		sessionHook = &SessionHookStat{HookName: hookName, HookEvent: hookEvent}
		session.HookBreakdown[hookEvent] = sessionHook
	}
	sessionHook.TotalCount++
	sessionHook.SuccessCount++
}

func codexHookDedupeKey(hookEvent, threadID, turnID string, row codexHookLogRow) string {
	if threadID != "" && turnID != "" {
		return hookEvent + "\x00" + threadID + "\x00" + turnID
	}
	if threadID != "" {
		return hookEvent + "\x00" + threadID + "\x00" + fmt.Sprint(row.TS)
	}
	return fmt.Sprintf("%s\x00%d\x00%d", hookEvent, row.TS, row.TSNanos)
}

func codexHookRegexpValue(re *regexp.Regexp, s string) string {
	matches := re.FindStringSubmatch(s)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func codexHookProjectName(body string) string {
	cwd := codexHookRegexpValue(codexHookCWDRe, body)
	if cwd == "" {
		return ""
	}
	return filepath.Base(cwd)
}

func codexHookLogTime(ts, tsNanos int64) time.Time {
	if tsNanos >= int64(time.Second) || tsNanos <= -int64(time.Second) {
		tsNanos %= int64(time.Second)
	}
	return time.Unix(ts, tsNanos)
}
