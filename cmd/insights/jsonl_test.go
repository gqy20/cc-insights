package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestParseProjectFile_SkipsNULCorruptedLine 回归测试:
// 一条被 NUL 字节污染的损坏行(崩溃/截断写入的产物)不得让解析陷入死循环,
// 且不应影响同文件中其余有效行的解析。
//
// 背景:流式 json.Decoder 在遇到非法字节时既不返回 EOF 也不前移读取位置,
// 配合“遇错 continue”的循环会 100% CPU 死循环——实测一条坏行即可让 cache
// 构建挂死 30 分钟以上。改用逐行 bufio.Scanner + json.Unmarshal 后可安全跳过。
func TestParseProjectFile_SkipsNULCorruptedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")

	good := `{"type":"assistant","timestamp":"2026-07-01T10:00:00Z","sessionId":"s1","cwd":"/p","message":{}}`
	// 模拟真实损坏样本:大量前导 NUL 字节 + 半截 JSON 尾巴
	corrupt := strings.Repeat("\x00", 4000) + `chat history","sessionId":"s1"}`
	content := good + "\n" + corrupt + "\n" + good + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	agg := newProjectAggregate()
	done := make(chan struct{})
	go func() {
		parseProjectFileAggregate(path, TimeFilter{}, agg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("解析含 NUL 损坏行的文件超过 10s 未返回(疑似死循环回归)")
	}

	total := 0
	for _, c := range agg.HourlyCounts {
		total += c
	}
	if total != 2 {
		t.Fatalf("有效 assistant 消息数 = %d, want 2(损坏行应被跳过,其余有效行正常解析)", total)
	}
}
