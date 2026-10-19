package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func createOpenCodeFixtureDB(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	dbPath := openCodeDBPathForDataDir(dataDir)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`create table session (id text primary key, directory text, path text, title text, parent_id text,
time_created integer, time_updated integer)`,
		`create table message (id text primary key, session_id text, time_created integer, time_updated integer, data text)`,
		`create table part (id text primary key, message_id text, session_id text, time_created integer, time_updated integer, data text)`,
		`create table todo (session_id text, status text, time_created integer)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}

	base := time.Date(2026, 6, 15, 10, 0, 0, 0, time.Local)
	project := `D:\work\oc-demo`
	sub := `D:\work\oc-sub`
	ms := func(t time.Time) string { return fmt.Sprintf("%d", t.UnixMilli()) }
	inserts := []string{
		`insert into session (id, directory, title, parent_id, time_created, time_updated) values
('ses-oc1', '` + project + `', '主会话标题', '', ` + ms(base.Add(-time.Hour)) + `, ` + ms(base.Add(time.Hour)) + `)`,
		// 子代理会话（parent_id 归并到主会话）
		`insert into session (id, directory, title, parent_id, time_created, time_updated) values
('ses-oc-sub', '` + sub + `', '子代理任务', 'ses-oc1', ` + ms(base) + `, ` + ms(base.Add(30*time.Minute)) + `)`,
		// user 消息带模型（opencode 约定：model 存 user 消息，按时序传播给 assistant）
		`insert into message (id, session_id, time_created, data) values
('m-u1', 'ses-oc1', ` + ms(base) + `, '{"role":"user","agent":"build","model":{"providerID":"glm","modelID":"glm-5v-turbo"}}')`,
		`insert into message (id, session_id, time_created, data) values
('m-a1', 'ses-oc1', ` + ms(base.Add(10*time.Second)) + `, '{"role":"assistant","agent":"build"}')`,
		// 子代理会话的 assistant（agent=explore → sidechain）
		`insert into message (id, session_id, time_created, data) values
('m-sub-a1', 'ses-oc-sub', ` + ms(base.Add(time.Minute)) + `, '{"role":"assistant","agent":"explore"}')`,
		// tool part：bash 成功（state.time 真实时长）
		`insert into part (id, message_id, session_id, time_created, data) values
('p-1', 'm-a1', 'ses-oc1', ` + ms(base.Add(15*time.Second)) + `,
'{"type":"tool","callID":"call-oc1","tool":"bash","state":{"status":"completed","input":{"command":"cd /tmp && pnpm test"},"output":"tests passed","time":{"start":` + ms(base.Add(15*time.Second)) + `,"end":` + ms(base.Add(18*time.Second)) + `}}}')`,
		// tool part：read 失败（error 状态）
		`insert into part (id, message_id, session_id, time_created, data) values
('p-2', 'm-a1', 'ses-oc1', ` + ms(base.Add(20*time.Second)) + `,
'{"type":"tool","callID":"call-oc2","tool":"read","state":{"status":"error","input":{"filePath":"D:\\work\\missing.go"},"error":"no such file or directory"}}')`,
		// 子代理的工具调用（归并主会话 + explore 归因）
		`insert into part (id, message_id, session_id, time_created, data) values
('p-3', 'm-sub-a1', 'ses-oc-sub', ` + ms(base.Add(70*time.Second)) + `,
'{"type":"tool","callID":"call-oc3","tool":"grep","state":{"status":"completed","input":{"pattern":"foo"},"output":"matched","time":{"start":` + ms(base.Add(70*time.Second)) + `,"end":` + fmt.Sprintf("%d", base.Add(70*time.Second).UnixMilli()+100) + `}}}')`,
		// step-finish part：token 载体
		`insert into part (id, message_id, session_id, time_created, data) values
('p-4', 'm-a1', 'ses-oc1', ` + ms(base.Add(30*time.Second)) + `,
'{"type":"step-finish","reason":"stop","cost":0,"tokens":{"total":11199,"input":66,"output":52,"reasoning":0,"cache":{"read":0,"write":11081}}}')`,
		// compaction
		`insert into part (id, message_id, session_id, time_created, data) values
('p-5', 'm-a1', 'ses-oc1', ` + ms(base.Add(10*time.Minute)) + `, '{"type":"compaction"}')`,
		// todo：主会话 + 子代理（归并）
		`insert into todo (session_id, status, time_created) values ('ses-oc1', 'completed', ` + ms(base) + `)`,
		`insert into todo (session_id, status, time_created) values ('ses-oc1', 'pending', ` + ms(base) + `)`,
		`insert into todo (session_id, status, time_created) values ('ses-oc-sub', 'completed', ` + ms(base) + `)`,
	}
	for _, stmt := range inserts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("插入数据失败: %v", err)
		}
	}
	return dataDir
}

func TestParseOpenCodeDBAggregate(t *testing.T) {
	dataDir := createOpenCodeFixtureDB(t)
	dbPath := openCodeDBPathForDataDir(dataDir)

	agg := newProjectAggregate()
	if err := parseOpenCodeDBAggregate(dbPath, TimeFilter{}, agg); err != nil {
		t.Fatalf("parseOpenCodeDBAggregate: %v", err)
	}
	agg.finalize()

	// 消息计数：主会话 1 + 子代理 1（归并），模型传播 glm/glm-5v-turbo。
	if got := agg.DailyActivity["2026-06-15"]; got != 2 {
		t.Fatalf("daily assistant messages = %d, want 2", got)
	}
	session := agg.SessionStatsMap["ses-oc1"]
	if session == nil {
		t.Fatal("session ses-oc1 missing")
	}
	if agg.SessionStatsMap["ses-oc-sub"] != nil {
		t.Fatal("subagent session should merge into parent")
	}
	if session.Project != `D:\work\oc-demo` {
		t.Fatalf("session project = %q, want root directory", session.Project)
	}
	if session.Title != "主会话标题" || session.TitleSource != "ai-title" {
		t.Fatalf("title = %q/%q, want ai-title from session table", session.Title, session.TitleSource)
	}

	// 工具：bash 成功（agentToolName 大写化）+ read 失败 + grep 来自子代理。
	if stat := agg.ToolStats["Bash"]; stat == nil || stat.CallCount != 1 || stat.SuccessCount != 1 {
		t.Fatalf("Bash stats = %#v, want 1 call 1 success", agg.ToolStats["Bash"])
	}
	if stat := agg.ToolStats["Read"]; stat == nil || stat.FailureCount != 1 {
		t.Fatalf("Read stats = %#v, want 1 failure", agg.ToolStats["Read"])
	}
	if stat := agg.ToolStats["Grep"]; stat == nil || stat.CallCount != 1 {
		t.Fatalf("Grep stats = %#v, want 1 call from subagent session", agg.ToolStats["Grep"])
	}
	// 模型传播：工具归因到 glm/glm-5v-turbo。
	if stat := agg.ToolModelStats["glm/glm-5v-turbo\x00Bash"]; stat == nil || stat.CallCount != 1 {
		t.Fatalf("tool model attribution = %#v, want glm/glm-5v-turbo", agg.ToolModelStats)
	}

	// step-finish token 记账。
	if usage := agg.ModelUsage["glm/glm-5v-turbo"]; usage == nil || usage.Tokens != 11199 || usage.InputTokens != 66 {
		t.Fatalf("ModelUsage = %#v, want tokens=11199 input=66", agg.ModelUsage["glm/glm-5v-turbo"])
	}
	if stat := agg.CostModelStats["glm/glm-5v-turbo"]; stat == nil || stat.TotalTokens != 11199 {
		t.Fatalf("CostModelStats = %#v, want 11199 tokens", agg.CostModelStats["glm/glm-5v-turbo"])
	}

	// 失败分类：read 的 error 文本 "no such file or directory" → bash/not_found 类。
	reason := ""
	for _, item := range agg.FailureAnalysis.ByToolReason {
		if item.Tool == "Read" {
			reason = item.Category + "/" + item.Reason
		}
	}
	if reason == "" {
		t.Fatalf("Read failure reason missing: %+v", agg.FailureAnalysis.ByToolReason)
	}

	// 子代理工具的 agent 归因。
	sidechain := 0
	for _, agent := range agg.AgentAnalysis.Agents {
		if agent.AgentID == "explore" && agent.IsSidechain {
			sidechain = agent.ToolCallCount
		}
	}
	if sidechain != 1 {
		t.Fatalf("explore sidechain calls = %d, want 1", sidechain)
	}

	// todo 按天归因（子代理归并）。
	tasks := agg.TaskPlanAnalysis.Tasks
	if tasks.TotalTasks != 3 || tasks.TotalSessions != 1 {
		t.Fatalf("Tasks = %#v, want 3 tasks / 1 session", tasks)
	}

	// compaction 信号。
	if agg.ContextStats == nil || agg.ContextStats.ContextCompactions != 1 {
		t.Fatalf("context compactions = %#v, want 1", agg.ContextStats)
	}
}

func TestListLogFileInfosPrefersOpenCodeDB(t *testing.T) {
	dataDir := createOpenCodeFixtureDB(t)
	files, err := listLogFileInfos(ProviderOpenCode, dataDir)
	if err != nil {
		t.Fatalf("listLogFileInfos: %v", err)
	}
	if len(files) != 1 || files[0].RelPath != openCodeDBName {
		t.Fatalf("files = %+v, want single opencode.db entry", files)
	}
	if !filepath.IsAbs(files[0].AbsPath) {
		t.Fatalf("abs path = %q", files[0].AbsPath)
	}
}
