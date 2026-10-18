package main

import (
	"os"
	"testing"
)

// 金标准：tail 增量解析结果 == 全量重扫结果。
// 尾部包含新消息、新工具调用+结果，以及上一轮 EOF 被记为 missing 的
// call-missing 的真实结果（验证 missing 修正路径）。
const codexTailAppendix = `{"timestamp":"2026-07-03T10:00:20Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"more"}]}}
{"timestamp":"2026-07-03T10:00:21Z","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-missing","command":"pwd","status":"completed","exit_code":0,"stdout":"/tmp","stderr":"","duration":50}}
{"timestamp":"2026-07-03T10:00:22Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-tail-new","arguments":"{\"cmd\":\"ls\"}"}}
{"timestamp":"2026-07-03T10:00:23Z","type":"event_msg","payload":{"type":"exec_command_end","call_id":"call-tail-new","command":"ls","status":"completed","exit_code":0,"stdout":"a\n","stderr":"","duration":10}}
`

func TestCodexTailIncrementalMatchesFullReparse(t *testing.T) {
	incDir, fullDir := t.TempDir(), t.TempDir()
	incPath := writeCodexSessionFixture(t, incDir)
	fullPath := writeCodexSessionFixture(t, fullDir)

	// 第一轮：只解析前缀
	aggInc := newProjectAggregate()
	offset, snap := parseCodexSessionFileAggregateFull(incPath, TimeFilter{}, aggInc)
	prefixSize := fileSize(t, incPath)
	if offset != prefixSize {
		t.Fatalf("offset=%d, want file size %d", offset, prefixSize)
	}
	if snap == nil || len(snap.Pendings) == 0 {
		t.Fatalf("snapshot pendings = %+v, want unresolved call-missing", snap)
	}
	aggInc.finalize()
	if stat := aggInc.ToolStats["exec_command"]; stat == nil || stat.MissingResultCount != 1 {
		t.Fatalf("prefix exec_command = %+v, want missing=1", stat)
	}

	// 追加尾部
	for _, path := range []string{incPath, fullPath} {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("open append failed: %v", err)
		}
		if _, err := f.WriteString(codexTailAppendix); err != nil {
			t.Fatalf("append failed: %v", err)
		}
		f.Close()
	}

	// 增量：tail 续解析（base 复用 aggInc 的缓存形态）
	tailBase := projectFileAggregateToAggregate(aggregateToProjectFileAggregate(aggInc))
	newOffset, newSnap := parseCodexSessionFileTail(incPath, offset, snap, TimeFilter{}, tailBase)
	if newOffset <= offset {
		t.Fatalf("newOffset=%d should advance past %d", newOffset, offset)
	}
	tailBase.finalize()

	// 全量重扫
	aggFull := newProjectAggregate()
	parseCodexSessionFileAggregate(fullPath, TimeFilter{}, aggFull)
	aggFull.finalize()

	// 对比关键聚合
	if aggFull.DailyActivity["2026-07-03"] != tailBase.DailyActivity["2026-07-03"] {
		t.Fatalf("DailyActivity inc=%d full=%d", tailBase.DailyActivity["2026-07-03"], aggFull.DailyActivity["2026-07-03"])
	}
	for _, tool := range []string{"exec_command", "apply_patch"} {
		incStat, fullStat := tailBase.ToolStats[tool], aggFull.ToolStats[tool]
		if incStat == nil || fullStat == nil {
			t.Fatalf("tool %s missing: inc=%v full=%v", tool, incStat, fullStat)
		}
		if incStat.CallCount != fullStat.CallCount || incStat.SuccessCount != fullStat.SuccessCount ||
			incStat.FailureCount != fullStat.FailureCount || incStat.MissingResultCount != fullStat.MissingResultCount {
			t.Fatalf("tool %s inc=%+v full=%+v", tool, *incStat, *fullStat)
		}
	}
	// call-missing 的真实结果应修正 missing 计数
	if stat := tailBase.ToolStats["exec_command"]; stat.MissingResultCount != 0 || stat.SuccessCount != 3 {
		t.Fatalf("exec_command after tail = %+v, want missing=0 success=3", *stat)
	}
	incReasons, fullReasons := failureReasonMap(tailBase), failureReasonMap(aggFull)
	for key, count := range fullReasons {
		if incReasons[key] != count {
			t.Fatalf("failure reason %s inc=%d full=%d (inc=%v full=%v)", key, incReasons[key], count, incReasons, fullReasons)
		}
	}
	if len(incReasons) != len(fullReasons) {
		t.Fatalf("reason sets differ: inc=%v full=%v", incReasons, fullReasons)
	}
	_ = newSnap
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s failed: %v", path, err)
	}
	return info.Size()
}

func failureReasonMap(agg *ProjectAggregate) map[string]int {
	out := make(map[string]int)
	for key, stat := range agg.FailureReasons {
		out[key] = stat.Count
	}
	return out
}

// jsonlScanner 的偏移跟踪：消费字节数必须精确到文件末尾（含无换行的末行）。
func TestJSONLScannerConsumedOffset(t *testing.T) {
	path := t.TempDir() + "/offset.jsonl"
	content := "line1\nline2\n" // 12 bytes
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer f.Close()
	scanner := newJSONLScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	if count != 2 || scanner.Consumed() != 12 {
		t.Fatalf("lines=%d consumed=%d, want 2/12", count, scanner.Consumed())
	}

	// 无换行末行：consumed 仍覆盖全部字节，追加后从该偏移续读
	path2 := t.TempDir() + "/partial.jsonl"
	os.WriteFile(path2, []byte("a\nb"), 0644)
	f2, _ := os.Open(path2)
	scanner2 := newJSONLScanner(f2)
	n2 := 0
	for scanner2.Scan() {
		n2++
	}
	f2.Close()
	if n2 != 2 || scanner2.Consumed() != 3 {
		t.Fatalf("partial: lines=%d consumed=%d, want 2/3", n2, scanner2.Consumed())
	}
	// 追加 "c\n" 后从 consumed 续读，应只读到 "c"
	f3, _ := os.OpenFile(path2, os.O_APPEND|os.O_WRONLY, 0644)
	f3.WriteString("c\n")
	f3.Close()
	f4, _ := os.Open(path2)
	defer f4.Close()
	f4.Seek(scanner2.Consumed(), 0)
	scanner3 := newJSONLScanner(f4)
	var lines []string
	for scanner3.Scan() {
		lines = append(lines, string(scanner3.Bytes()))
	}
	if len(lines) != 1 || lines[0] != "c" {
		t.Fatalf("resume lines = %v, want [c]", lines)
	}
}
