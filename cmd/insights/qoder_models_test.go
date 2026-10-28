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
	// 隔离真实状态库（catalog 会用 chat_model_preferences 兜底窗口），只测日志来源。
	t.Setenv("APPDATA", t.TempDir())
	// 真实排版：model_config 同行带 api_key/url（官方已脱敏），只取 key/display_name/窗口。
	writeQoderRunLog(t, root, "2026-10-24T11-36-33-668+08-00-aaaaaa-p1", strings.Join([]string{
		`2026-10-24T11:36:35 info starting worker`,
		`[QoderInferRequest details] model_config={"key":"smodel","display_name":"Sonus","model":"","format":"openai","is_vl":true,"is_reasoning":true,"api_key":"[redacted]","url":"","source":"system","max_input_tokens":180000}, custom_model=null`,
		`unrelated line with no markers`,
	}, "\n"))
	writeQoderRunLog(t, root, "2026-10-27T17-27-02-489+08-00-bbbbbb-p2", strings.Join([]string{
		`[QoderInferRequest details] model_config={"key":"qfmodel","display_name":"Qwen3.8-Flash","api_key":"[redacted]","max_input_tokens":180000}, custom_model=null`,
		`[Routing] Reasoning: Validated model 'dfmodel' (DeepSeek-Flash) from catalog`,
		// 同一个 run 日志内后面的行更新真名，应该覆盖前面的；但不带窗口的行不能清掉窗口。
		`[QoderInferRequest details] model_config={"key":"qfmodel","display_name":"Qwen3.8-Flash-v2","api_key":"[redacted]"}`,
	}, "\n"))

	catalog := qoderModelCatalog(qoderModelLogRootsForTest(root))
	if got := catalog["qfmodel"].Name; got != "Qwen3.8-Flash-v2" {
		t.Fatalf("qfmodel.Name = %q, want latest in-file name", got)
	}
	if got := catalog["smodel"].Name; got != "Sonus" {
		t.Fatalf("smodel.Name = %q, want Sonus", got)
	}
	if got := catalog["dfmodel"].Name; got != "DeepSeek-Flash" {
		t.Fatalf("dfmodel.Name = %q, want DeepSeek-Flash (routing fallback)", got)
	}
	// 窗口只来自 config 行；routing 行不带，也不该清掉已有值。
	if got := catalog["smodel"].MaxInputTokens; got != 180000 {
		t.Fatalf("smodel.MaxInputTokens = %d, want 180000", got)
	}
	if got := catalog["qfmodel"].MaxInputTokens; got != 180000 {
		t.Fatalf("qfmodel.MaxInputTokens = %d, want 180000（v2 行无窗口不得清零）", got)
	}
	if got := catalog["dfmodel"].MaxInputTokens; got != 0 {
		t.Fatalf("dfmodel.MaxInputTokens = %d, want 0（routing 行不带窗口）", got)
	}
	for alias, info := range catalog {
		if strings.Contains(info.Name, "api_key") || strings.Contains(info.Name, "redacted") {
			t.Fatalf("alias %q leaked raw log text: %q", alias, info.Name)
		}
	}
}

func TestQoderModelCatalogNewestRunWins(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", t.TempDir())
	writeQoderRunLog(t, root, "2026-10-01T00-00-00-000+08-00-old-p1",
		`model_config={"key":"qmodel_38max","display_name":"Qwen3.7-Max","max_input_tokens":100000}`)
	writeQoderRunLog(t, root, "2026-10-27T00-00-00-000+08-00-new-p2",
		`model_config={"key":"qmodel_38max","display_name":"Qwen3.8-Max","max_input_tokens":180000}`)

	catalog := qoderModelCatalog(qoderModelLogRootsForTest(root))
	if got := catalog["qmodel_38max"].Name; got != "Qwen3.8-Max" {
		t.Fatalf("qmodel_38max.Name = %q, want name from the newest run", got)
	}
	if got := catalog["qmodel_38max"].MaxInputTokens; got != 180000 {
		t.Fatalf("qmodel_38max.MaxInputTokens = %d, want window from the newest run", got)
	}
}

func TestQoderModelCatalogMissingLogsDegradesToEmpty(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
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
	t.Setenv("APPDATA", t.TempDir())
	roots := qoderModelLogRootsForTest(t.TempDir())
	first := qoderModelCatalog(roots)
	if _, ok := first["__sentinel__"]; ok {
		t.Fatal("fresh catalog should not contain the sentinel")
	}
	first["__sentinel__"] = qoderModelInfo{Name: "written"}
	second := qoderModelCatalog(roots)
	if second["__sentinel__"].Name != "written" {
		t.Fatal("catalog should be reused per roots, not rescanned")
	}
	if other := qoderModelCatalog(qoderModelLogRootsForTest(t.TempDir())); len(other) != 0 {
		t.Fatalf("other roots got %v, want an isolated catalog", other)
	}
}

func TestApplyQoderModelsFillsNamesAndEstimates(t *testing.T) {
	data := &DashboardData{
		ModelUsage: []ModelUsageItem{
			{Model: "qfmodel"},
			{Model: "claude-opus-4-6"},
		},
		CostAnalysis: &CostAnalysisData{
			ByModel: []CostModelStat{
				{Model: "smodel", ContextRatioSum: 3.0},
				{Model: "gpt-5.1-codex-max"},
				{Model: "qfmodel", ContextRatioSum: 0.5},
				{Model: "nmodel", ContextRatioSum: 2.0}, // catalog 里没有窗口：不估算
			},
		},
	}
	applyQoderModels(data, map[string]qoderModelInfo{
		"qfmodel": {Name: "Qwen3.8-Flash", MaxInputTokens: 180000},
		"smodel":  {Name: "Sonus", MaxInputTokens: 180000},
		"nmodel":  {Name: "NoWindow"},
	})

	if got := data.ModelUsage[0].ModelName; got != "Qwen3.8-Flash" {
		t.Fatalf("ModelUsage[0].ModelName = %q", got)
	}
	if got := data.ModelUsage[1].ModelName; got != "" {
		t.Fatalf("non-qoder model got %q, want untouched", got)
	}
	byModel := data.CostAnalysis.ByModel
	if byModel[0].ModelName != "Sonus" {
		t.Fatalf("ByModel[0].ModelName = %q", byModel[0].ModelName)
	}
	if got := byModel[0].EstInputTokens; got != 540000 {
		t.Fatalf("smodel.EstInputTokens = %d, want 540000 (3.0×180000)", got)
	}
	if got := byModel[2].EstInputTokens; got != 90000 {
		t.Fatalf("qfmodel.EstInputTokens = %d, want 90000 (0.5×180000)", got)
	}
	if byModel[1].ModelName != "" || byModel[1].EstInputTokens != 0 {
		t.Fatalf("non-qoder cost model touched: %+v", byModel[1])
	}
	if byModel[3].ModelName != "NoWindow" || byModel[3].EstInputTokens != 0 {
		t.Fatalf("windowless alias should get name only: %+v", byModel[3])
	}
	if got := data.CostAnalysis.Totals.EstInputTokens; got != 630000 {
		t.Fatalf("Totals.EstInputTokens = %d, want 630000", got)
	}

	// 空 catalog 必须是纯 no-op（无日志时统计照常输出，也不清已填充的估算）。
	applyQoderModels(data, map[string]qoderModelInfo{})
	if data.ModelUsage[0].ModelName != "Qwen3.8-Flash" {
		t.Fatal("empty catalog must not clear already-resolved names")
	}
	if data.CostAnalysis.Totals.EstInputTokens != 630000 {
		t.Fatal("empty catalog must not clear estimates")
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
	catalog := map[string]qoderModelInfo{}
	line := `[QoderInferRequest details] model_config={"key":"a1","display_name":"Alpha","max_input_tokens":111000}` +
		` model_config={"key":"a2","display_name":"Beta","max_input_tokens":222000}` +
		` padding=` + strings.Repeat("z", 1024)
	scanQoderModelMarker(catalog, line, qoderModelConfigMarker, qoderModelConfigRe, true)
	if catalog["a1"].Name != "Alpha" || catalog["a2"].Name != "Beta" {
		t.Fatalf("catalog = %v, want both matches on one line found", catalog)
	}
	if catalog["a1"].MaxInputTokens != 111000 || catalog["a2"].MaxInputTokens != 222000 {
		t.Fatalf("windows = %v/%v, want 111000/222000", catalog["a1"].MaxInputTokens, catalog["a2"].MaxInputTokens)
	}
	// Routing 行只更新真名，不带窗口也不清窗口。
	routing := `[Routing] Reasoning: Validated model 'a1' (Alpha-2) from catalog`
	scanQoderModelMarker(catalog, routing, qoderModelRoutingMarker, qoderRoutingRe, false)
	if catalog["a1"].Name != "Alpha-2" || catalog["a1"].MaxInputTokens != 111000 {
		t.Fatalf("routing updated window: %+v", catalog["a1"])
	}
	// 超长 marker 落在窗口截断处也不能崩，命中不到就是不加。
	before := len(catalog)
	scanQoderModelMarker(catalog, "model_config="+strings.Repeat("z", 2048), qoderModelConfigMarker, qoderModelConfigRe, true)
	if len(catalog) != before {
		t.Fatalf("catalog = %v, want noise to add nothing", catalog)
	}
}

// qoderModelLogRootsForTest 绕开真实 ~/.qoder，只指向测试目录。
func qoderModelLogRootsForTest(root string) []string {
	return []string{root}
}
