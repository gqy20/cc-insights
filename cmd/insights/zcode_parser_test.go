package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createZCodeFixtureDB(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	dbPath := zcodeDBPathForDataDir(dataDir)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatalf("创建 db 目录失败: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	// 与真实库同构的最小 schema（只保留 zcode 解析用到的列）。
	statements := []string{
		`create table session (id text primary key, directory text, path text, title text, parent_id text)`,
		`create table message (id text primary key, session_id text, time_created integer, time_updated integer, data text, sequence integer)`,
		`create table part (id text primary key, message_id text, session_id text, time_created integer, time_updated integer, data text, sequence integer)`,
		`create table model_usage (id text primary key, session_id text, model_id text, status text, error_type text,
started_at integer, completed_at integer, duration_ms integer, time_to_first_token_ms integer,
input_tokens integer, output_tokens integer, reasoning_tokens integer,
cache_creation_input_tokens integer, cache_read_input_tokens integer, computed_total_tokens integer)`,
		`create table tool_usage (id text primary key, session_id text, turn_id text, tool_call_id text,
tool_name text, status text, error_type text, exit_code integer, destructive integer)`,
		`create table todo (session_id text, content text, status text, priority text, time_created integer)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}

	// fixture 日期固定 2026-06-15：主会话 + 子代理会话（parent_id 归并）。
	base := time.Date(2026, 6, 15, 10, 0, 0, 0, time.Local)
	project := `D:\work\zcode-demo`
	sub := `D:\work\zcode-sub`
	ms := func(t time.Time) string { return fmt.Sprintf("%d", t.UnixMilli()) }
	inserts := []string{
		`insert into session (id, directory, path, title, parent_id) values ('sess-z1', '` + project + `', '` + project + `', '主会话标题', '')`,
		`insert into session (id, directory, path, title, parent_id) values ('sess-sub1', '` + sub + `', '` + sub + `', '子代理任务', 'sess-z1')`,
		// 主会话 assistant：token 记账主源（message.data.tokens，全历史覆盖）。
		`insert into message (id, session_id, time_created, sequence, data) values ('msg-a1', 'sess-z1', ` + ms(base) + `, 1,
'{"role":"assistant","modelID":"GLM-5.3","agent":"zcode-agent","path":{"cwd":"D:\\work\\zcode-demo"},"tokens":{"total":1200,"input":1000,"output":200,"reasoning":50,"cache":{"read":400,"write":0}}}')`,
		`insert into message (id, session_id, time_created, sequence, data) values ('msg-u1', 'sess-z1', ` + ms(base.Add(-time.Minute)) + `, 0,
'{"role":"user","agent":"zcode-agent","contextSnapshot":{"envInfo":{"cwd":"D:\\work\\zcode-demo"}}}')`,
		// 子代理会话的 assistant（agent=zcode-Explore → sidechain，session 归并到主会话）。
		`insert into message (id, session_id, time_created, sequence, data) values ('msg-sub-a1', 'sess-sub1', ` + ms(base.Add(2*time.Minute)) + `, 1,
'{"role":"assistant","modelID":"GLM-5.3-Flash","agent":"zcode-Explore","path":{"cwd":"D:\\work\\zcode-sub"},"tokens":{"total":300,"input":250,"output":50,"reasoning":0,"cache":{"read":100,"write":0}}}')`,
		// 工具：Bash 成功（state.time 真实时长 3000ms，列差故意放大证明取 state.time）
		`insert into part (id, message_id, session_id, time_created, time_updated, data) values
('p-1', 'msg-a1', 'sess-z1', ` + ms(base.Add(time.Second)) + `, ` + ms(base.Add(60*time.Second)) + `,
'{"type":"tool","callID":"call-z1","tool":"Bash","state":{"status":"completed","input":{"command":"cd /tmp && pnpm test"},"output":"tests passed","time":{"start":` + ms(base.Add(time.Second)) + `,"end":` + ms(base.Add(4*time.Second)) + `}}}')`,
		// Read 失败：error 文本 timed out → timeout/timeout
		`insert into part (id, message_id, session_id, time_created, time_updated, data) values
('p-2', 'msg-a1', 'sess-z1', ` + ms(base.Add(5*time.Second)) + `, ` + ms(base.Add(6*time.Second)) + `,
'{"type":"tool","callID":"call-z2","tool":"Read","state":{"status":"error","input":{"file_path":"D:\\work\\missing.go"},"error":"Tool execution timed out after 30000ms"}}')`,
		// 子代理会话里的工具：归并主会话 + agent 归因
		`insert into part (id, message_id, session_id, time_created, time_updated, data) values
('p-4', 'msg-sub-a1', 'sess-sub1', ` + ms(base.Add(3*time.Minute)) + `, ` + fmt.Sprintf("%d", base.Add(3*time.Minute).UnixMilli()+2000) + `,
'{"type":"tool","callID":"call-z4","tool":"Grep","state":{"status":"completed","input":{"pattern":"foo"},"output":"matched","time":{"start":` + ms(base.Add(3*time.Minute)) + `,"end":` + fmt.Sprintf("%d", base.Add(3*time.Minute).UnixMilli()+100) + `}}}')`,
		// 非工具 part（过滤）
		`insert into part (id, message_id, session_id, time_created, time_updated, data) values
('p-3', 'msg-a1', 'sess-z1', ` + ms(base) + `, ` + ms(base) + `, '{"type":"text","text":"hello"}')`,
		// compaction → ContextCompactions
		`insert into part (id, message_id, session_id, time_created, time_updated, data) values
('p-5', 'msg-a1', 'sess-z1', ` + ms(base.Add(10*time.Minute)) + `, ` + ms(base.Add(10*time.Minute)) + `, '{"type":"compaction","auto":true,"compactReason":"context_limit"}')`,
		// model_usage：完成请求的 duration/TTFT（token 已由 message 记账，不重复）
		`insert into model_usage (id, session_id, model_id, status, started_at, duration_ms, time_to_first_token_ms, computed_total_tokens)
values ('mu-1', 'sess-z1', 'GLM-5.3', 'completed', ` + ms(base) + `, 2000, 800, 0)`,
		// 模型侧失败 → 事件信号
		`insert into model_usage (id, session_id, model_id, status, started_at, duration_ms, computed_total_tokens)
values ('mu-2', 'sess-z1', 'GLM-5.3', 'error', ` + ms(base.Add(20*time.Second)) + `, 0, 0)`,
		// tool_usage：call-z2 无 exit_code（timeout 文本分类）；call-z5 有 exit_code=3
		`insert into tool_usage (id, session_id, tool_call_id, tool_name, status, error_type, exit_code, destructive) values
('tu-1', 'sess-z1', 'call-z2', 'Read', 'error', 'tool_timeout', 0, 0)`,
		`insert into part (id, message_id, session_id, time_created, time_updated, data) values
('p-6', 'msg-a1', 'sess-z1', ` + ms(base.Add(30*time.Second)) + `, ` + ms(base.Add(31*time.Second)) + `,
'{"type":"tool","callID":"call-z5","tool":"Bash","state":{"status":"error","input":{"command":"make build"},"error":"make: *** [build] Error 3"}}')`,
		`insert into tool_usage (id, session_id, tool_call_id, tool_name, status, error_type, exit_code, destructive) values
('tu-2', 'sess-z1', 'call-z5', 'Bash', 'error', 'tool_execution_failed', 3, 1)`,
		// todo：主会话 + 子代理会话（归并）
		`insert into todo (session_id, content, status, priority, time_created) values ('sess-z1', 'task a', 'completed', 'high', ` + ms(base) + `)`,
		`insert into todo (session_id, content, status, priority, time_created) values ('sess-z1', 'task b', 'pending', 'high', ` + ms(base) + `)`,
		`insert into todo (session_id, content, status, priority, time_created) values ('sess-sub1', 'sub task', 'completed', 'high', ` + ms(base) + `)`,
	}
	for _, stmt := range inserts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("插入数据失败: %v", err)
		}
	}
	return dbPath
}

func TestParseZCodeDBAggregate(t *testing.T) {
	dbPath := createZCodeFixtureDB(t)

	agg := newProjectAggregate()
	if err := parseZCodeDBAggregate(dbPath, TimeFilter{}, agg); err != nil {
		t.Fatalf("parseZCodeDBAggregate: %v", err)
	}
	agg.finalize()

	// 消息计数：主会话 assistant 1 + 子代理 assistant 1（归并后同 session）。
	if got := agg.DailyActivity["2026-06-15"]; got != 2 {
		t.Fatalf("daily assistant messages = %d, want 2", got)
	}
	session := agg.SessionStatsMap["sess-z1"]
	if session == nil || session.MessageCount != 3 {
		t.Fatalf("session message count = %#v, want 3 (assistant+user+subagent assistant)", session)
	}
	if agg.SessionStatsMap["sess-sub1"] != nil {
		t.Fatal("subagent session should merge into parent sess-z1")
	}
	if session.Project != `D:\work\zcode-demo` {
		t.Fatalf("session project = %q, want fixture cwd", session.Project)
	}
	if session.Title != "主会话标题" || session.TitleSource != "ai-title" {
		t.Fatalf("session title = %q/%q, want ai-title from session table", session.Title, session.TitleSource)
	}

	// 工具调用：Bash 成功 + Read 超时失败 + Grep 成功 + Bash exit_code 失败。
	if stat := agg.ToolStats["Bash"]; stat == nil || stat.CallCount != 2 || stat.FailureCount != 1 {
		t.Fatalf("Bash stats = %#v, want 2 calls 1 failure", agg.ToolStats["Bash"])
	}
	if stat := agg.ToolStats["Read"]; stat == nil || stat.FailureCount != 1 {
		t.Fatalf("Read stats = %#v, want 1 failure", agg.ToolStats["Read"])
	}

	// 失败细分：Read → timeout/timeout；Bash → bash/exit_code_3（tool_usage 结构化信号）。
	reason := func(tool string) string {
		for _, item := range agg.FailureAnalysis.ByToolReason {
			if item.Tool == tool {
				return item.Category + "/" + item.Reason
			}
		}
		return ""
	}
	if got := reason("Read"); got != "timeout/timeout" {
		t.Fatalf("Read failure reason = %q, want timeout/timeout", got)
	}
	if got := reason("Bash"); got != "bash/exit_code_3" {
		t.Fatalf("Bash failure reason = %q, want bash/exit_code_3", got)
	}

	// Bash 命令文本进入命令分析（链式 cd && pnpm）。
	if agg.CommandAnalysis == nil {
		t.Fatal("CommandAnalysis should not be nil")
	}
	foundPnpm := false
	for _, cmd := range agg.CommandAnalysis.BashCommands {
		if cmd.CommandName == "pnpm" {
			foundPnpm = true
		}
	}
	if !foundPnpm {
		t.Fatalf("BashCommands should contain 'pnpm' from chain command, got %+v", agg.CommandAnalysis.BashCommands)
	}

	// token 记账主源：message.tokens（主会话 1200 + 子代理 300）。
	modelStat := agg.CostModelStats["GLM-5.3"]
	if modelStat == nil || modelStat.RequestCount != 1 || modelStat.TotalTokens != 1200 {
		t.Fatalf("GLM-5.3 stat = %#v, want 1 request 1200 tokens", agg.CostModelStats["GLM-5.3"])
	}
	flashStat := agg.CostModelStats["GLM-5.3-Flash"]
	if flashStat == nil || flashStat.TotalTokens != 300 {
		t.Fatalf("GLM-5.3-Flash stat = %#v, want 300 tokens", agg.CostModelStats["GLM-5.3-Flash"])
	}
	if usage := agg.ModelUsage["GLM-5.3"]; usage == nil || usage.Tokens != 1200 || usage.InputTokens != 1000 {
		t.Fatalf("ModelUsage = %#v, want GLM-5.3 1200 tokens / 1000 input", agg.ModelUsage["GLM-5.3"])
	}

	// 请求时长/TTFT 来自 model_usage（token 不重复计）。
	if modelStat.TotalRoundTripMs != 2000 {
		t.Fatalf("TotalRoundTripMs = %d, want 2000", modelStat.TotalRoundTripMs)
	}
	if modelStat.TotalTTFTMs != 800 {
		t.Fatalf("TotalTTFTMs = %d, want 800", modelStat.TotalTTFTMs)
	}

	// 模型侧失败 → 事件信号；destructive 工具 → 风险事件。
	if agg.EventTypes["zcode:model_error:unknown"] != 1 {
		t.Fatalf("model error event = %d, want 1", agg.EventTypes["zcode:model_error:unknown"])
	}
	if agg.EventTypes["zcode:destructive_tool"] != 1 {
		t.Fatalf("destructive tool event = %d, want 1", agg.EventTypes["zcode:destructive_tool"])
	}

	// compaction → 上下文压缩计数。
	if context := agg.ContextStats; context == nil || context.ContextCompactions != 1 {
		t.Fatalf("context compactions = %#v, want 1", agg.ContextStats)
	}

	// 子代理工具归并主会话且 agent 归因生效。
	if stat := agg.ToolStats["Grep"]; stat == nil || stat.CallCount != 1 {
		t.Fatalf("Grep stats = %#v, want 1 call from subagent session", agg.ToolStats["Grep"])
	}
	if agg.AgentAnalysis == nil {
		t.Fatal("AgentAnalysis should not be nil")
	}
	sidechainCalls := 0
	for _, agent := range agg.AgentAnalysis.Agents {
		if agent.AgentID == "Explore" && agent.IsSidechain {
			sidechainCalls = agent.ToolCallCount
		}
	}
	if sidechainCalls != 1 {
		t.Fatalf("Explore sidechain tool calls = %d, want 1", sidechainCalls)
	}
}

func TestParseZCodeDBAggregateTaskPlan(t *testing.T) {
	dbPath := createZCodeFixtureDB(t)

	agg := newProjectAggregate()
	if err := parseZCodeDBAggregate(dbPath, TimeFilter{}, agg); err != nil {
		t.Fatalf("parseZCodeDBAggregate: %v", err)
	}
	agg.finalize()

	tasks := agg.TaskPlanAnalysis.Tasks
	if tasks.TotalTasks != 3 {
		t.Fatalf("total tasks = %d, want 3", tasks.TotalTasks)
	}
	if tasks.TotalSessions != 1 {
		t.Fatalf("task sessions = %d, want 1 (subagent todo merged)", tasks.TotalSessions)
	}
	if tasks.CompletionRate < 66.6 || tasks.CompletionRate > 66.7 {
		t.Fatalf("completion rate = %.1f, want 66.7", tasks.CompletionRate)
	}
	// 按天快照携带 TaskAgg：时间过滤查询经快照重建 TaskPlanAnalysis 的原料。
	daily := agg.DailyRuntime["2026-06-15"]
	if daily.TaskAgg == nil || daily.TaskAgg.TotalTasks != 3 {
		t.Fatalf("daily TaskAgg = %#v, want 3 tasks on 2026-06-15", daily.TaskAgg)
	}
	// 序列化-还原 roundtrip 后 TaskAgg 存活（缓存增量/合并的原料）。
	restored := projectFileAggregateToAggregate(aggregateToProjectFileAggregate(agg))
	if restored.TaskAgg == nil || restored.TaskAgg.TotalTasks != 3 {
		t.Fatalf("restored TaskAgg = %#v, want 3 tasks", restored.TaskAgg)
	}
	// 合并进目标聚合（mergeProjectAggregate 路径）。
	dst := newProjectAggregate()
	mergeProjectAggregate(dst, restored)
	if dst.TaskAgg == nil || dst.TaskAgg.TotalTasks != 3 || dst.TaskAgg.StatusCounts["completed"] != 2 {
		t.Fatalf("merged TaskAgg = %#v, want 3 tasks / 2 completed", dst.TaskAgg)
	}
}

func TestResolveProviderDetectsZCodeDataDir(t *testing.T) {
	dataDir := t.TempDir()
	if got := resolveProvider(ProviderAuto, dataDir); got != ProviderClaude {
		t.Fatalf("empty dir resolves to %q, want claude", got)
	}
	dbDir := filepath.Join(dataDir, "cli", "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "db.sqlite"), []byte("x"), 0644); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if got := resolveProvider(ProviderAuto, dataDir); got != ProviderZcode {
		t.Fatalf("zcode dir resolves to %q, want zcode", got)
	}
	if got := defaultDataDirForProvider(ProviderZcode); filepath.Base(got) != ".zcode" {
		t.Fatalf("zcode default dir = %q, want ~/.zcode", got)
	}
}
