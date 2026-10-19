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
	if err := writeDailyDigestFile(out, rows); err != nil {
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
