package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordCodexSQLiteHooksDedupesTraceRows(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, codexHookLogDBName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
create table logs (
	id integer primary key autoincrement,
	ts integer not null,
	ts_nanos integer not null,
	level text not null,
	target text not null,
	feedback_log_body text,
	module_path text,
	file text,
	line integer,
	thread_id text,
	process_uuid text,
	estimated_bytes integer not null default 0
)`)
	if err != nil {
		t.Fatalf("create logs table: %v", err)
	}

	ts := time.Now().Unix()
	body := "session_loop{thread_id=thread-1}:submission_dispatch{cwd=/tmp/project-alpha}:turn{turn.id=turn-1}:session_task.run:run_turn:run_hooks_and_record_inputs: registering event source with poller"
	for i := 0; i < 3; i++ {
		insertCodexHookLog(t, db, ts, int64(i), "log", "thread-1", body)
	}
	stopBody := "session_loop{thread_id=thread-1}:submission_dispatch{cwd=/tmp/project-alpha}:turn{turn.id=turn-1}:session_task.run:run_turn:run_turn_stop_hooks: deregistering event source from poller"
	insertCodexHookLog(t, db, ts, 4, "log", "thread-1", stopBody)
	insertCodexHookLog(t, db, ts, 5, "codex_app_server::outgoing_message", "", "app-server event: hook/started targeted_connections=1")
	insertCodexHookLog(t, db, ts, 6, "codex_app_server::outgoing_message", "", "app-server event: hook/completed targeted_connections=1")

	agg := newProjectAggregate()
	if err := recordCodexSQLiteHooksLocked(agg, dataDir, TimeFilter{}); err != nil {
		t.Fatalf("recordCodexSQLiteHooksLocked: %v", err)
	}
	agg.finalize()

	inputKey := "CodexHookTrace\x00run_hooks_and_record_inputs"
	inputStat := agg.HookStats[inputKey]
	if inputStat == nil {
		t.Fatalf("missing hook stat %q", inputKey)
	}
	if inputStat.TotalCount != 1 || inputStat.SuccessCount != 1 || inputStat.ErrorCount != 0 {
		t.Fatalf("input hook stat = total %d success %d error %d, want 1/1/0", inputStat.TotalCount, inputStat.SuccessCount, inputStat.ErrorCount)
	}

	stopKey := "CodexHookTrace\x00run_turn_stop_hooks"
	stopStat := agg.HookStats[stopKey]
	if stopStat == nil || stopStat.TotalCount != 1 || stopStat.SuccessCount != 1 {
		t.Fatalf("stop hook stat = %#v, want one success", stopStat)
	}
	if agg.EventTypes["codex_hook_trace:run_hooks_and_record_inputs"] != 1 {
		t.Fatalf("input hook event count = %d, want 1", agg.EventTypes["codex_hook_trace:run_hooks_and_record_inputs"])
	}
	if agg.EventTypes["codex_hook/started"] != 1 || agg.EventTypes["codex_hook/completed"] != 1 {
		t.Fatalf("app-server hook event counts = started %d completed %d, want 1/1", agg.EventTypes["codex_hook/started"], agg.EventTypes["codex_hook/completed"])
	}

	dateKey := time.Unix(ts, 0).Format("2006-01-02")
	daily := agg.DailyRuntime[dateKey]
	if daily == nil {
		t.Fatalf("missing daily runtime for %s", dateKey)
	}
	if daily.HookStats[inputKey].TotalCount != 1 {
		t.Fatalf("daily input hook count = %d, want 1", daily.HookStats[inputKey].TotalCount)
	}
	if daily.EventTypes["codex_hook_trace:run_turn_stop_hooks"] != 1 {
		t.Fatalf("daily stop event count = %d, want 1", daily.EventTypes["codex_hook_trace:run_turn_stop_hooks"])
	}

	session := agg.SessionStatsMap["thread-1"]
	if session == nil {
		t.Fatal("missing session hook association")
	}
	if session.Project != "project-alpha" {
		t.Fatalf("session project = %q, want project-alpha", session.Project)
	}
	if session.HookBreakdown["run_hooks_and_record_inputs"].TotalCount != 1 {
		t.Fatalf("session input hook count = %d, want 1", session.HookBreakdown["run_hooks_and_record_inputs"].TotalCount)
	}
}

func TestCodexHookCWDReCapturesWindowsDrivePath(t *testing.T) {
	// Windows 盘符路径含冒号，正则不能因排除 : 而截断成 "C"。
	body := "session_loop{thread_id=thread-1}:submission_dispatch{cwd=C:\\Users\\dev\\project-win}:turn{turn.id=turn-1}"
	if got := codexHookRegexpValue(codexHookCWDRe, body); got != `C:\Users\dev\project-win` {
		t.Fatalf("cwd capture = %q, want full drive path", got)
	}
}

func insertCodexHookLog(t *testing.T, db *sql.DB, ts, tsNanos int64, target, threadID, body string) {
	t.Helper()
	_, err := db.Exec(`
insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id, estimated_bytes)
values (?, ?, 'TRACE', ?, ?, ?, length(?))`, ts, tsNanos, target, body, threadID, body)
	if err != nil {
		t.Fatalf("insert log: %v", err)
	}
}
