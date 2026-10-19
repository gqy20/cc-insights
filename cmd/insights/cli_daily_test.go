package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// daily digest：行 schema、设备名、原子写、幂等全量快照。
func TestDailyDigestRowsAndFile(t *testing.T) {
	// 用 pi fixture 构建一个可用渠道缓存（cfg 指向临时目录）
	dir := t.TempDir()
	writePiFixture(t, dir)
	orig := cfg
	cfg = Config{DataDir: dir, CacheDir: filepath.Join(dir, "cache"), Provider: ProviderPi}
	defer func() { cfg = orig }()

	rows, err := buildDailyDigestRows("dev-x")
	if err != nil {
		t.Fatalf("buildDailyDigestRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1 (single fixture day)", len(rows))
	}
	row := rows[0]
	if row.V != dailyDigestSchemaVersion || row.Device != "dev-x" || row.Provider != string(ProviderPi) {
		t.Fatalf("row identity = %+v", row)
	}
	if row.Date != "2026-07-03" || row.Messages != 3 {
		t.Fatalf("row = %+v, want date=2026-07-03 messages=3", row)
	}
	if row.Models["glm/glm-5v-turbo"].Count != 3 {
		t.Fatalf("models = %+v, want glm count=3", row.Models)
	}
	if row.TZ == "" || row.ExportedAt == "" {
		t.Fatalf("tz/exported_at missing: %+v", row)
	}
	// v2 维度：质量/时段/项目
	if row.V != 2 {
		t.Fatalf("schema v=%d, want 2", row.V)
	}
	if row.Failures != 1 || row.Missing != 1 {
		t.Fatalf("failures=%d missing=%d, want 1/1 (exit_code_1 + 无结果 read)", row.Failures, row.Missing)
	}
	if row.Reasons["bash/exit_code_1"] != 1 {
		t.Fatalf("reasons=%v, want bash/exit_code_1", row.Reasons)
	}
	if row.Hourly[10] != 3 {
		t.Fatalf("hourly[10]=%d, want 3 messages", row.Hourly[10])
	}
	if row.Projects["/tmp/pi-project"].Messages != 3 {
		t.Fatalf("projects=%v, want /tmp/pi-project messages=3", row.Projects)
	}

	// 原子写 + 可逐行解析
	out := filepath.Join(dir, "digest.jsonl")
	if err := writeDailyDigestFile(out, rows, false); err != nil {
		t.Fatalf("writeDailyDigestFile: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed dailyDigestRow
	if err := json.Unmarshal([]byte(strings.SplitN(string(data), "\n", 2)[0]), &parsed); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if parsed.Device != "dev-x" {
		t.Fatalf("re-parsed device=%s", parsed.Device)
	}
	if _, err := os.Stat(out + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file should not remain after atomic write")
	}
}

func TestDigestDeviceName(t *testing.T) {
	if digestDeviceName("override") != "override" {
		t.Fatal("override device name not honored")
	}
	if digestDeviceName("") == "" {
		t.Fatal("default device name should fall back to hostname, not empty")
	}
}

// 合并写入语义：旧行保留（源数据消失的天不丢）、冲突新行胜、损坏行容忍、--rewrite 清理。
func TestDailyDigestMergeWrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "digest.jsonl")

	old := dailyDigestRow{
		V: 1, Device: "dev-x", Provider: "claude", Date: "2026-09-01",
		Messages: 10, ExportedAt: "2026-10-01T00:00:00+08:00",
	}
	oldOtherDevice := dailyDigestRow{
		V: 1, Device: "dev-y", Provider: "claude", Date: "2026-09-01",
		Messages: 7, ExportedAt: "2026-10-01T00:00:00+08:00",
	}
	writeRows := func(rows []dailyDigestRow) {
		f, err := os.Create(out)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		enc := json.NewEncoder(f)
		for _, r := range rows {
			if err := enc.Encode(r); err != nil {
				t.Fatalf("encode: %v", err)
			}
		}
		f.Close()
	}
	writeRows([]dailyDigestRow{old, oldOtherDevice})

	// 本次导出：同键新行（messages 修正为 11）+ 损坏行混入旧文件再测容忍。
	fresh := dailyDigestRow{
		V: 2, Device: "dev-x", Provider: "claude", Date: "2026-09-01",
		Messages: 11, ExportedAt: "2026-10-19T12:00:00+08:00",
	}
	freshToday := dailyDigestRow{
		V: 2, Device: "dev-x", Provider: "zcode", Date: "2026-10-19",
		Messages: 100, ExportedAt: "2026-10-19T12:00:00+08:00",
	}
	if err := os.WriteFile(out, []byte("{broken\n"), 0644); err != nil {
		t.Fatalf("append broken line: %v", err)
	}
	// 旧文件现在 = 两行有效 + 一行损坏（broken 覆盖了文件，重新构造完整场景）
	writeRows([]dailyDigestRow{old, oldOtherDevice})
	fh, _ := os.OpenFile(out, os.O_APPEND|os.O_WRONLY, 0644)
	fh.WriteString("{broken\n")
	fh.Close()

	if err := writeDailyDigestFile(out, []dailyDigestRow{fresh, freshToday}, false); err != nil {
		t.Fatalf("merge write: %v", err)
	}

	merged, err := readDailyDigestRows(out)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	got := map[string]dailyDigestRow{}
	for _, r := range merged {
		got[r.Device+"/"+r.Provider+"/"+r.Date] = r
	}
	if len(merged) != 3 {
		t.Fatalf("merged rows=%d, want 3 (old claude day + other device + fresh zcode day)", len(merged))
	}
	// 同键冲突：新行胜（messages 11，exported_at 更新）
	if r := got["dev-x/claude/2026-09-01"]; r.Messages != 11 || r.ExportedAt != fresh.ExportedAt {
		t.Fatalf("conflict row = %+v, want fresh (messages=11)", r)
	}
	// 源数据消失的旧行（dev-x 的 2026-08-15 不在本次导出中）：保留
	// （本场景由 oldOtherDevice 代表——它不在本次导出，必须原样保留）
	if r := got["dev-y/claude/2026-09-01"]; r.Messages != 7 {
		t.Fatalf("other-device row = %+v, want preserved messages=7", r)
	}
	// 新增行
	if r := got["dev-x/zcode/2026-10-19"]; r.Messages != 100 {
		t.Fatalf("fresh row = %+v, want messages=100", r)
	}

	// --rewrite：退回全量快照，未出现在本次导出的键被清掉
	if err := writeDailyDigestFile(out, []dailyDigestRow{fresh, freshToday}, true); err != nil {
		t.Fatalf("rewrite write: %v", err)
	}
	rewritten, err := readDailyDigestRows(out)
	if err != nil {
		t.Fatalf("re-read rewrite: %v", err)
	}
	if len(rewritten) != 2 {
		t.Fatalf("rewritten rows=%d, want 2 (snapshot semantics)", len(rewritten))
	}
}

func TestMergeDailyDigestRowsClockGuard(t *testing.T) {
	// 防御旧文件时钟倒拨：冲突时 exported_at 新者胜
	oldRow := dailyDigestRow{V: 2, Device: "d", Provider: "p", Date: "2026-10-01", Messages: 5, ExportedAt: "2026-10-20T00:00:00+08:00"}
	freshRow := dailyDigestRow{V: 2, Device: "d", Provider: "p", Date: "2026-10-01", Messages: 9, ExportedAt: "2026-10-19T00:00:00+08:00"}
	merged := mergeDailyDigestRows([]dailyDigestRow{oldRow}, []dailyDigestRow{freshRow})
	if len(merged) != 1 || merged[0].Messages != 5 {
		t.Fatalf("merged=%+v, want old row to win when exported_at is newer", merged)
	}
}
