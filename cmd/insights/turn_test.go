package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestRequestRoundTripMs 验证单请求 round-trip 的边界处理。
func TestRequestRoundTripMs(t *testing.T) {
	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	last := map[string]time.Time{"s": base}

	cases := []struct {
		name    string
		session string
		ts      time.Time
		hasTs   bool
		want    int64
	}{
		{"normal", "s", base.Add(20 * time.Second), true, 20000},
		{"no prev", "other", base.Add(20 * time.Second), true, 0},
		{"no timestamp", "s", base, false, 0},
		{"empty session", "", base.Add(5 * time.Second), true, 0},
		{"reverse", "s", base.Add(-5 * time.Second), true, 0},
		{"suspended >30min", "s", base.Add(31 * time.Minute), true, 0},
	}
	for _, c := range cases {
		got := requestRoundTripMs(last, c.session, c.ts, c.hasTs)
		if got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

// TestTurnDurationParsingAndFinalize 验证 system/turn_duration 行被解析、四层累积，并 finalize 输出。
func TestTurnDurationParsingAndFinalize(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "turn-proj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := turnDurationRecord("/tmp/turn-proj", "s1", base, 60000, 10) + "\n" +
		turnDurationRecord("/tmp/turn-proj", "s1", base.Add(time.Minute), 120000, 20) + "\n" +
		turnDurationRecord("/tmp/turn-proj", "s2", base.Add(2*time.Minute), 30000, 5) + "\n"
	if err := os.WriteFile(filepath.Join(projectDir, "s.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if agg.ToolPerformance == nil {
		t.Fatal("ToolPerformance nil, finalizeTurnDuration not applied")
	}
	tp := agg.ToolPerformance
	if tp.TurnCount != 3 {
		t.Fatalf("TurnCount=%d want 3", tp.TurnCount)
	}
	// avg = (60000+120000+30000)/3 = 70000
	if tp.AvgTurnDurationMs != 70000 {
		t.Fatalf("AvgTurnDurationMs=%f want 70000", tp.AvgTurnDurationMs)
	}
	if tp.MaxTurnDurationMs != 120000 {
		t.Fatalf("MaxTurnDurationMs=%d want 120000", tp.MaxTurnDurationMs)
	}
	if len(tp.SlowTurns) != 3 {
		t.Fatalf("SlowTurns len=%d want 3", len(tp.SlowTurns))
	}
	if tp.SlowTurns[0].DurationMs != 120000 {
		t.Fatalf("SlowTurns not desc, [0]=%d want 120000", tp.SlowTurns[0].DurationMs)
	}
}

// TestRoundTripAndThroughput 验证单请求 round-trip 透传到 cost，并派生吞吐量。
// 第一条 assistant 前驱是 user(10s)，第二条前驱是第一条 assistant(15s)。
func TestRoundTripAndThroughput(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	projectDir := filepath.Join(dataDir, "projects", "rt-proj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	content := userRecord("/tmp/rt-proj", "s1", base) + "\n" +
		costAssistantRecord("/tmp/rt-proj", "s1", base.Add(10*time.Second), "model-x", false, "", 50, 100, 0, 0, 0, 0) + "\n" +
		costAssistantRecord("/tmp/rt-proj", "s1", base.Add(25*time.Second), "model-x", false, "", 50, 50, 0, 0, 0, 0) + "\n"
	if err := os.WriteFile(filepath.Join(projectDir, "s.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	agg, err := ParseProjectsConcurrentOnceFromDir(TimeFilter{}, dataDir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if agg.CostAnalysis == nil || len(agg.CostAnalysis.ByModel) == 0 {
		t.Fatalf("CostAnalysis empty: %+v", agg.CostAnalysis)
	}
	stat := agg.CostAnalysis.ByModel[0]
	if stat.Model != "model-x" {
		t.Fatalf("model=%q want model-x", stat.Model)
	}
	if stat.RequestCount != 2 {
		t.Fatalf("RequestCount=%d want 2", stat.RequestCount)
	}
	// 10000 + 15000 = 25000ms
	if stat.TotalRoundTripMs != 25000 {
		t.Fatalf("TotalRoundTripMs=%d want 25000", stat.TotalRoundTripMs)
	}
	// AvgRoundTripMs = 25000/2 = 12500
	if stat.AvgRoundTripMs != 12500 {
		t.Fatalf("AvgRoundTripMs=%f want 12500", stat.AvgRoundTripMs)
	}
	// OutputTokensPerSec = 150 / 25 = 6.0
	if stat.OutputTokensPerSec != 6.0 {
		t.Fatalf("OutputTokensPerSec=%f want 6.0", stat.OutputTokensPerSec)
	}
}

// TestTurnAndRoundTripCacheRoundTrip 验证 turn 字段 + cost round-trip 字段经
// aggregateToProjectFileAggregate / projectFileAggregateToAggregate 往返不丢失（最易漏的同步点）。
func TestTurnAndRoundTripCacheRoundTrip(t *testing.T) {
	agg := newProjectAggregate()
	agg.TurnCount = 5
	agg.TurnTotalDurationMs = 250000
	agg.TurnMaxDurationMs = 90000
	agg.SlowTurns = []TurnSlowItem{{SessionID: "s1", Project: "p", DurationMs: 90000, MessageCount: 7}}

	ms := ensureCostModelStat(agg, "m")
	ms.RequestCount = 3
	ms.OutputTokens = 300
	ms.TotalRoundTripMs = 12345

	file := aggregateToProjectFileAggregate(agg)
	restored := projectFileAggregateToAggregate(file)

	if restored.TurnCount != 5 {
		t.Fatalf("TurnCount restore=%d want 5", restored.TurnCount)
	}
	if restored.TurnTotalDurationMs != 250000 {
		t.Fatalf("TurnTotalDurationMs restore=%d want 250000", restored.TurnTotalDurationMs)
	}
	if restored.TurnMaxDurationMs != 90000 {
		t.Fatalf("TurnMaxDurationMs restore=%d want 90000", restored.TurnMaxDurationMs)
	}
	if len(restored.SlowTurns) != 1 || restored.SlowTurns[0].DurationMs != 90000 {
		t.Fatalf("SlowTurns restore=%+v", restored.SlowTurns)
	}
	rms := restored.CostModelStats["m"]
	if rms == nil {
		t.Fatal("CostModelStats[m] nil after restore")
	}
	if rms.TotalRoundTripMs != 12345 {
		t.Fatalf("CostModelStat TotalRoundTripMs restore=%d want 12345", rms.TotalRoundTripMs)
	}
}

// turnDurationRecord 构造一条 system/turn_duration 行
func turnDurationRecord(cwd, sessionID string, ts time.Time, durationMs, messageCount int) string {
	return `{"type":"system","subtype":"turn_duration","cwd":"` + cwd + `","sessionId":"` + sessionID + `","gitBranch":"main","timestamp":"` +
		ts.Format(time.RFC3339Nano) + `","durationMs":` + strconv.Itoa(durationMs) + `,"messageCount":` + strconv.Itoa(messageCount) + `}`
}

// userRecord 构造一条真人 user 行（非 tool_result），用于设置 round-trip 起点
func userRecord(cwd, sessionID string, ts time.Time) string {
	return `{"type":"user","cwd":"` + cwd + `","sessionId":"` + sessionID + `","timestamp":"` +
		ts.Format(time.RFC3339Nano) + `","message":{"role":"user","content":"hi"}}`
}
