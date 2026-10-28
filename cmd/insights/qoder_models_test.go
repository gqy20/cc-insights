package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Qoder transcript 里的 message.model 是内部别名，真实模型名只出现在运行日志
// （logs/runs/<启动时间戳>-*/qodercli.log）。真名只在渲染边缘套用，聚合 key 保持别名，
// 套用不到就退回别名——所以这些测试同时覆盖"解析"和"取不到时的退化"。

func writeQoderRunLog(t *testing.T, root, runName, body string) {
	t.Helper()
	dir := filepath.Join(root, "logs", "runs", runName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	path := filepath.Join(dir, qoderModelLogName)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write log failed: %v", err)
	}
}

func TestQoderModelCatalogParsesRunLogs(t *testing.T) {
	root := t.TempDir()
	// 真实排版：model_config 同行带 api_key/url（官方已脱敏），只取 key 与 display_name。
	writeQoderRunLog(t, root, "2026-10-24T11-36-33-668+08-00-aaaaaa-p1", strings.Join([]string{
		`2026-10-24T11:36:35 info starting worker`,
		`[QoderInferRequest details] model_config={"key":"smodel","display_name":"Sonus","model":"","format":"openai","is_vl":true,"is_reasoning":true,"api_key":"[redacted]","url":"","source":"system","max_input_tokens":180000}, custom_model=null`,
		`unrelated line with no markers`,
	}, "\n"))
	writeQoderRunLog(t, root, "2026-10-27T17-27-02-489+08-00-bbbbbb-p2", strings.Join([]string{
		`[QoderInferRequest details] model_config={"key":"qfmodel","display_name":"Qwen3.8-Flash","api_key":"[redacted]","max_input_tokens":180000}, custom_model=null`,
		`[Routing] Reasoning: Validated model 'dfmodel' (DeepSeek-Flash) from catalog`,
		// 同一个 run 日志内后面的行更新，应该覆盖前面的。
		`[QoderInferRequest details] model_config={"key":"qfmodel","display_name":"Qwen3.8-Flash-v2","api_key":"[redacted]"}`,
	}, "\n"))

	catalog := qoderModelCatalog(qoderModelLogRootsForTest(root))
	if got := catalog["qfmodel"]; got != "Qwen3.8-Flash-v2" {
		t.Fatalf("qfmodel = %q, want latest in-file name", got)
	}
	if got := catalog["smodel"]; got != "Sonus" {
		t.Fatalf("smodel = %q, want Sonus", got)
	}
	if got := catalog["dfmodel"]; got != "DeepSeek-Flash" {
		t.Fatalf("dfmodel = %q, want DeepSeek-Flash (routing fallback)", got)
	}
	for alias, name := range catalog {
		if strings.Contains(name, "api_key") || strings.Contains(name, "redacted") {
			t.Fatalf("alias %q leaked raw log text: %q", alias, name)
		}
	}
}

func TestQoderModelCatalogNewestRunWins(t *testing.T) {
	root := t.TempDir()
	writeQoderRunLog(t, root, "2026-10-01T00-00-00-000+08-00-old-p1",
		`model_config={"key":"qmodel_38max","display_name":"Qwen3.7-Max"}`)
	writeQoderRunLog(t, root, "2026-10-27T00-00-00-000+08-00-new-p2",
		`model_config={"key":"qmodel_38max","display_name":"Qwen3.8-Max"}`)

	catalog := qoderModelCatalog(qoderModelLogRootsForTest(root))
	if got := catalog["qmodel_38max"]; got != "Qwen3.8-Max" {
		t.Fatalf("qmodel_38max = %q, want name from the newest run", got)
	}
}

func TestQoderModelCatalogMissingLogsDegradesToEmpty(t *testing.T) {
	catalog := qoderModelCatalog(qoderModelLogRootsForTest(t.TempDir()))
	if len(catalog) != 0 {
		t.Fatalf("catalog = %v, want empty when no run logs exist", catalog)
	}

	// run 目录在、日志文件缺：同样只是没有映射，不能报错或 panic。
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "logs", "runs", "2026-10-27T00-00-00-000+08-00-x-p9"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if got := qoderModelCatalog(qoderModelLogRootsForTest(root)); len(got) != 0 {
		t.Fatalf("catalog = %v, want empty when log file is absent", got)
	}
}

func TestSelectQoderModelLogsKeepsNewestWithinBudget(t *testing.T) {
	root := t.TempDir()
	runsDir := filepath.Join(root, "logs", "runs")
	bigBody := strings.Repeat("x", 5000) + "\n"
	// 旧日志里有映射，新日志单独就超预算：跳过它，剩下的继续扫。
	writeQoderRunLog(t, root, "2026-10-01T00-00-00-000+08-00-old-p1",
		`model_config={"key":"smodel","display_name":"Sonus"}`+"\n")
	writeQoderRunLog(t, root, "2026-10-27T00-00-00-000+08-00-new-p2", bigBody)

	paths := selectQoderModelLogs(runsDir, 4500)
	if len(paths) != 1 {
		t.Fatalf("selected %d logs, want the oversized newest one skipped: %v", len(paths), paths)
	}
	if !strings.Contains(paths[0], "old-p1") {
		t.Fatalf("selected %q, want the older in-budget log", paths[0])
	}

	// 预算充足时按旧→新返回，保证"后面的覆盖前面的"成立。
	all := selectQoderModelLogs(runsDir, 1<<20)
	if len(all) != 2 {
		t.Fatalf("selected %d logs, want both: %v", len(all), all)
	}
	if !strings.Contains(all[0], "old-p1") || !strings.Contains(all[1], "new-p2") {
		t.Fatalf("order = %v, want oldest first", all)
	}
}

func TestQoderModelCatalogMemoizedPerRoots(t *testing.T) {
	roots := qoderModelLogRootsForTest(t.TempDir())
	first := qoderModelCatalog(roots)
	if _, ok := first["__sentinel__"]; ok {
		t.Fatal("fresh catalog should not contain the sentinel")
	}
	first["__sentinel__"] = "written"
	second := qoderModelCatalog(roots)
	if second["__sentinel__"] != "written" {
		t.Fatal("catalog should be reused per roots, not rescanned")
	}
	if other := qoderModelCatalog(qoderModelLogRootsForTest(t.TempDir())); len(other) != 0 {
		t.Fatalf("other roots got %v, want an isolated catalog", other)
	}
}

func TestApplyQoderModelNamesFillsModelUsageAndCostViews(t *testing.T) {
	data := &DashboardData{
		ModelUsage: []ModelUsageItem{
			{Model: "qfmodel"},
			{Model: "claude-opus-4-6"},
		},
		CostAnalysis: &CostAnalysisData{
			ByModel: []CostModelStat{{Model: "smodel"}, {Model: "gpt-5.1-codex-max"}},
		},
	}
	applyQoderModelNames(data, map[string]string{"qfmodel": "Qwen3.8-Flash", "smodel": "Sonus"})

	if got := data.ModelUsage[0].ModelName; got != "Qwen3.8-Flash" {
		t.Fatalf("ModelUsage[0].ModelName = %q", got)
	}
	if got := data.ModelUsage[1].ModelName; got != "" {
		t.Fatalf("non-qoder model got %q, want untouched", got)
	}
	if got := data.CostAnalysis.ByModel[0].ModelName; got != "Sonus" {
		t.Fatalf("CostAnalysis.ByModel[0].ModelName = %q", got)
	}
	if got := data.CostAnalysis.ByModel[1].ModelName; got != "" {
		t.Fatalf("non-qoder cost model got %q, want untouched", got)
	}

	// 空 catalog 必须是纯 no-op（无日志时统计照常输出）。
	applyQoderModelNames(data, map[string]string{})
	if data.ModelUsage[0].ModelName != "Qwen3.8-Flash" {
		t.Fatal("empty catalog must not clear already-resolved names")
	}
}

func TestQoderModelCatalogNeededOnlyForQoderProviders(t *testing.T) {
	for _, provider := range []Provider{ProviderQoder, ProviderAll} {
		if !qoderModelCatalogNeeded(provider) {
			t.Fatalf("%s should scan the model catalog", provider)
		}
	}
	// 别的 provider 完全不付扫描成本，也不该被 Qoder 别名表影响。
	for _, provider := range []Provider{ProviderClaude, ProviderCodex, ProviderPi, ProviderOpenCode} {
		if qoderModelCatalogNeeded(provider) {
			t.Fatalf("%s should not scan the model catalog", provider)
		}
	}
	if qoderModelCatalogNeeded(ProviderAuto) {
		t.Fatal("unresolved auto should not scan the model catalog")
	}
}

func TestModelDisplayLabelKeepsAliasForDrilldown(t *testing.T) {
	// 别名是 --model 过滤和统计 key，标签里必须一起给出，否则报表看得懂却下钻不了。
	if got := modelDisplayLabel("qfmodel", "Qwen3.8-Flash"); got != "Qwen3.8-Flash (qfmodel)" {
		t.Fatalf("label = %q", got)
	}
	if got := modelDisplayLabel("qfmodel", ""); got != "qfmodel" {
		t.Fatalf("label = %q, want bare alias when unresolved", got)
	}
	if got := modelDisplayLabel("Ultimate", "Ultimate"); got != "Ultimate" {
		t.Fatalf("label = %q, want no duplicate when name equals alias", got)
	}
}

func TestScanQoderModelMarkerWindowsMultipleMatches(t *testing.T) {
	catalog := map[string]string{}
	line := `[QoderInferRequest details] model_config={"key":"a1","display_name":"Alpha"}` +
		` model_config={"key":"a2","display_name":"Beta"}` +
		` padding=` + strings.Repeat("z", 1024)
	scanQoderModelMarker(catalog, line, qoderModelConfigMarker, qoderModelConfigRe)
	if catalog["a1"] != "Alpha" || catalog["a2"] != "Beta" {
		t.Fatalf("catalog = %v, want both matches on one line found", catalog)
	}
	// 超长 marker 落在窗口截断处也不能崩，命中不到就是不加。
	before := len(catalog)
	scanQoderModelMarker(catalog, "model_config="+strings.Repeat("z", 2048), qoderModelConfigMarker, qoderModelConfigRe)
	if len(catalog) != before {
		t.Fatalf("catalog = %v, want noise to add nothing", catalog)
	}
}

// qoderModelLogRootsForTest 绕开真实 ~/.qoder，只指向测试目录。
func qoderModelLogRootsForTest(root string) []string {
	return []string{root}
}
