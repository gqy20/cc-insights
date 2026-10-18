package main

import (
	"os"
	"path/filepath"
	"testing"
)

const piFixture = `{"type":"session","version":3,"id":"sess-pi-1","timestamp":"2026-07-03T10:00:00.000Z","cwd":"/tmp/pi-project"}
{"type":"model_change","id":"m1","parentId":null,"timestamp":"2026-07-03T10:00:00.100Z","provider":"glm","modelId":"glm-5v-turbo"}
{"type":"message","id":"u1","parentId":"m1","timestamp":"2026-07-03T10:00:01.000Z","message":{"role":"user","content":[{"type":"text","text":"列出目录"}],"timestamp":1770013201000}}
{"type":"message","id":"a1","parentId":"u1","timestamp":"2026-07-03T10:00:02.000Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"思考"},{"type":"toolCall","id":"call-bash-1","name":"bash","arguments":{"command":"ls -la"}}]}}
{"type":"message","id":"a1r","parentId":"a1","timestamp":"2026-07-03T10:00:03.000Z","message":{"role":"toolResult","toolCallId":"call-bash-1","toolName":"bash","isError":false,"content":[{"type":"text","text":"file1\nfile2"}]}}
{"type":"message","id":"a2","parentId":"a1r","timestamp":"2026-07-03T10:00:04.000Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"call-bash-2","name":"bash","arguments":{"command":"false"}}]}}
{"type":"message","id":"a2r","parentId":"a2","timestamp":"2026-07-03T10:00:05.000Z","message":{"role":"toolResult","toolCallId":"call-bash-2","toolName":"bash","isError":true,"content":[{"type":"text","text":"Process exited with code 1"}]}}
{"type":"message","id":"a3","parentId":"a2r","timestamp":"2026-07-03T10:00:06.000Z","message":{"role":"assistant","content":[{"type":"toolCall","id":"call-missing","name":"read","arguments":{"path":"/tmp/x"}}]}}
{"type":"compaction","id":"c1","timestamp":"2026-07-03T10:00:07.000Z"}
`

func writePiFixture(t *testing.T, dataDir string) string {
	t.Helper()
	dir := filepath.Join(dataDir, "agent", "sessions", "--tmp-pi-project--")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	path := filepath.Join(dir, "2026-07-03T10-00-00-000Z_sess-pi-1.jsonl")
	if err := os.WriteFile(path, []byte(piFixture), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	return path
}

func TestParsePiSessionFileAggregate(t *testing.T) {
	path := writePiFixture(t, t.TempDir())

	agg := newProjectAggregate()
	parsePiSessionFileAggregateFull(path, TimeFilter{}, agg)
	agg.finalize()

	if agg.DailyActivity["2026-07-03"] != 3 {
		t.Fatalf("DailyActivity=%d, want 3 assistant messages", agg.DailyActivity["2026-07-03"])
	}
	if stat := agg.ProjectStats["/tmp/pi-project"]; stat == nil || stat.MessageCount != 3 {
		t.Fatalf("ProjectStats=%+v, want 3 messages", stat)
	}
	if m := agg.ModelUsage["glm/glm-5v-turbo"]; m == nil || m.Count != 3 {
		t.Fatalf("ModelUsage=%+v, want count=3", m)
	}
	bash := agg.ToolStats["Bash"]
	if bash == nil || bash.CallCount != 2 || bash.SuccessCount != 1 || bash.FailureCount != 1 {
		t.Fatalf("Bash stat=%+v, want calls=2 success=1 failure=1", bash)
	}
	read := agg.ToolStats["Read"]
	if read == nil || read.MissingResultCount != 1 {
		t.Fatalf("Read stat=%+v, want missing=1", read)
	}
	// isError=true + exit code 文本 → 确认失败 exit_code_1
	found := false
	for _, stat := range agg.FailureReasons {
		if stat.Reason == "exit_code_1" && stat.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("FailureReasons=%+v, want bash/exit_code_1", agg.FailureReasons)
	}
	if agg.EventTypes["compaction"] != 1 {
		t.Fatalf("compaction events=%d, want 1", agg.EventTypes["compaction"])
	}
}

func TestParsePiTailIncremental(t *testing.T) {
	incDir, fullDir := t.TempDir(), t.TempDir()
	incPath := writePiFixture(t, incDir)
	fullPath := writePiFixture(t, fullDir)

	aggInc := newProjectAggregate()
	offset, snap := parsePiSessionFileAggregateFull(incPath, TimeFilter{}, aggInc)
	aggInc.finalize()

	appendix := `{"type":"message","id":"a4","parentId":"a3","timestamp":"2026-07-03T10:00:20.000Z","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}
{"type":"message","id":"a3r","parentId":"a3","timestamp":"2026-07-03T10:00:21.000Z","message":{"role":"toolResult","toolCallId":"call-missing","toolName":"read","isError":false,"content":[{"type":"text","text":"content of x"}]}}
`
	for _, p := range []string{incPath, fullPath} {
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("append open failed: %v", err)
		}
		f.WriteString(appendix)
		f.Close()
	}

	tailBase := projectFileAggregateToAggregate(aggregateToProjectFileAggregate(aggInc))
	parsePiSessionFileTail(incPath, offset, snap, TimeFilter{}, tailBase)
	tailBase.finalize()

	aggFull := newProjectAggregate()
	parsePiSessionFileAggregateFull(fullPath, TimeFilter{}, aggFull)
	aggFull.finalize()

	if tailBase.DailyActivity["2026-07-03"] != aggFull.DailyActivity["2026-07-03"] {
		t.Fatalf("DailyActivity inc=%d full=%d", tailBase.DailyActivity["2026-07-03"], aggFull.DailyActivity["2026-07-03"])
	}
	for _, tool := range []string{"Bash", "Read"} {
		incStat, fullStat := tailBase.ToolStats[tool], aggFull.ToolStats[tool]
		if incStat == nil || fullStat == nil {
			t.Fatalf("tool %s missing: inc=%v full=%v", tool, incStat, fullStat)
		}
		if incStat.CallCount != fullStat.CallCount || incStat.SuccessCount != fullStat.SuccessCount ||
			incStat.FailureCount != fullStat.FailureCount || incStat.MissingResultCount != fullStat.MissingResultCount {
			t.Fatalf("tool %s inc=%+v full=%+v", tool, *incStat, *fullStat)
		}
	}
}

// opencode fixture：session + message + tool part（error 与 completed 各一）
func writeOpenCodeFixture(t *testing.T, dataDir string) {
	t.Helper()
	sesDir := filepath.Join(dataDir, "storage", "session", "hash1")
	msgDir := filepath.Join(dataDir, "storage", "message", "ses_oc1")
	partDir := filepath.Join(dataDir, "storage", "part", "msg_oc_a1")
	for _, d := range []string{sesDir, msgDir, partDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
	}
	os.WriteFile(filepath.Join(sesDir, "ses_oc1.json"), []byte(`{"id":"ses_oc1","directory":"/tmp/oc-project","time":{"created":1770013200000,"updated":1770013300000}}`), 0644)
	os.WriteFile(filepath.Join(msgDir, "msg_oc_u0.json"), []byte(`{"id":"msg_oc_u0","sessionID":"ses_oc1","role":"user","model":{"providerID":"minimax","modelID":"MiniMax-M2.1"},"time":{"created":1770013200000}}`), 0644)
	os.WriteFile(filepath.Join(msgDir, "msg_oc_a1.json"), []byte(`{"id":"msg_oc_a1","sessionID":"ses_oc1","role":"assistant","agent":"build","time":{"created":1770013201000}}`), 0644)
	os.WriteFile(filepath.Join(partDir, "prt_1.json"), []byte(`{"id":"prt_1","sessionID":"ses_oc1","messageID":"msg_oc_a1","type":"tool","tool":"bash","callID":"oc_call_1","state":{"status":"completed","input":"{\"command\":\"ls\"}","output":"a\nb","time":{"start":1770013202000,"end":1770013202500}}}`), 0644)
	os.WriteFile(filepath.Join(partDir, "prt_2.json"), []byte(`{"id":"prt_2","sessionID":"ses_oc1","messageID":"msg_oc_a1","type":"tool","tool":"read","callID":"oc_call_2","state":{"status":"error","error":"File not found: /tmp/missing.md","time":{"start":1770013203000,"end":1770013203100}}}`), 0644)
}

func TestParseOpenCodeStorage(t *testing.T) {
	dataDir := t.TempDir()
	writeOpenCodeFixture(t, dataDir)

	agg := newProjectAggregate()
	idx := buildOpenCodeIndexInto(dataDir, agg)
	if idx == nil || len(idx.sessions) != 1 || len(idx.messages) != 2 {
		t.Fatalf("index=%+v", idx)
	}
	agg.finalize()

	if agg.DailyActivity["2026-02-02"] != 1 {
		t.Fatalf("DailyActivity=%v, want 1 assistant message on 2026-02-02 (ms 1770013201000)", agg.DailyActivity)
	}
	// assistant 消息本身无 model，应继承 session 内 user 消息的模型
	if m := agg.ModelUsage["minimax/MiniMax-M2.1"]; m == nil || m.Count != 1 {
		t.Fatalf("ModelUsage=%+v, want minimax/MiniMax-M2.1 count=1", agg.ModelUsage)
	}
	if stat := agg.ProjectStats["/tmp/oc-project"]; stat == nil || stat.MessageCount != 1 {
		t.Fatalf("ProjectStats=%+v", stat)
	}

	// part 解析
	partAgg := newProjectAggregate()
	partDir := filepath.Join(dataDir, "storage", "part", "msg_oc_a1")
	entries, _ := os.ReadDir(partDir)
	for _, e := range entries {
		parseOpenCodePartFile("storage/part/msg_oc_a1/"+e.Name(), filepath.Join(partDir, e.Name()), idx, partAgg)
	}
	partAgg.finalize()

	bash := partAgg.ToolStats["Bash"]
	if bash == nil || bash.CallCount != 1 || bash.SuccessCount != 1 {
		t.Fatalf("Bash stat=%+v, want calls=1 success=1", bash)
	}
	read := partAgg.ToolStats["Read"]
	if read == nil || read.FailureCount != 1 {
		t.Fatalf("Read stat=%+v, want failure=1 (status=error)", read)
	}
	found := false
	for _, stat := range partAgg.FailureReasons {
		if stat.Reason == "not_found" {
			found = true
		}
	}
	if !found {
		t.Fatalf("FailureReasons=%+v, want file/not_found", partAgg.FailureReasons)
	}
}

func TestNormalizeOpenCodeInput(t *testing.T) {
	// 双重编码字符串 + filePath 归一化
	out := normalizeOpenCodeInput([]byte(`"{\"filePath\":\"/tmp/a.md\"}"`))
	if want := `{"file_path":"/tmp/a.md"}`; string(out) != want {
		t.Fatalf("normalize=%s, want %s", out, want)
	}
	// 对象形态 + filePath
	out = normalizeOpenCodeInput([]byte(`{"filePath":"/tmp/b.go"}`))
	if want := `{"file_path":"/tmp/b.go"}`; string(out) != want {
		t.Fatalf("normalize=%s, want %s", out, want)
	}
	// 普通命令对象保持不变
	out = normalizeOpenCodeInput([]byte(`{"command":"ls"}`))
	if string(out) != `{"command":"ls"}` {
		t.Fatalf("normalize=%s, want unchanged", out)
	}
}
