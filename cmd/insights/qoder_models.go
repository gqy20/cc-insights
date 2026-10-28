package main

import (
	"bufio"
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Qoder transcript 的 message.model 是内部别名（qfmodel / smodel / dfmodel /
// qmodel_38max …），真实模型名只出现在运行日志里：
//
//	[QoderInferRequest details] model_config={"key":"qfmodel","display_name":"Qwen3.8-Flash",...}
//	[Routing] Reasoning: Validated model 'qfmodel' (Qwen3.8-Flash) from catalog
//
// 真名只在渲染边缘套用，聚合 key 始终是别名：日志按 run 目录滚动、随时可能被清掉，
// 若把真名当统计 key，同一模型会在缓存里裂成两个桶（旧文件按真名、新文件按别名）。
// 套用不到就退回别名——展示层退化，统计口径不变。
//
// 注意 model_config 同行还带 api_key/url 字段（官方已脱敏），这里只取 key 与
// display_name 两项，且不保留整行文本。
const (
	qoderModelLogName       = "qodercli.log"
	qoderModelRunsSubdir    = "runs"
	qoderModelConfigMarker  = "model_config="
	qoderModelRoutingMarker = "Validated model '"
	qoderModelWindowBytes   = 512
	qoderModelReaderBytes   = 256 * 1024
	// 单次进程最多扫描的日志字节数。实测本机 65MB 全量扫描约 118ms，
	// 预算 192MB 把最坏情况钉在半秒内；超预算的旧日志跳过，真名退回别名。
	qoderModelScanBudgetBytes = int64(192 << 20)
)

// 两条线索都从 marker 之后锚定：窗口化匹配保证正则只看它前面那一小段，
// 而 model_config 里 "key" 恰好紧邻 "display_name" 是 Qoder 日志的稳定排版。
var (
	qoderModelConfigRe = regexp.MustCompile(`^\{"key":"([A-Za-z0-9_.-]+)","display_name":"([^"]+)"`)
	qoderRoutingRe     = regexp.MustCompile(`^([A-Za-z0-9_.-]+)' \(([^)]+)\) from catalog`)
	// max_input_tokens 是请求时实际下发的输入窗口，估算输入 token 的锚点。
	qoderModelWindowRe = regexp.MustCompile(`"max_input_tokens":([0-9]+)`)
)

// qoderModelInfo 是 catalog 条目：真实名称 + 输入窗口。
// Name 只影响展示；MaxInputTokens 与 transcript 的 context_usage_ratio 相乘
// 估算输入侧 token（Qoder 官方不下发 token，输出侧没有任何锚点，不做估算）。
type qoderModelInfo struct {
	Name           string
	MaxInputTokens int64
}

// qoderModelCatalogNeeded 判断这次分析是否可能含 Qoder 数据，只有可能时才付扫描成本。
// 只用 Claude/Codex 的调用因此完全不受影响。
func qoderModelCatalogNeeded(provider Provider) bool {
	return provider == ProviderQoder || provider == ProviderAll
}

// qoderModelCatalog 返回别名 -> 模型信息。按 (数据目录集合) 记忆：web 是常驻进程，
// 会在一次进程里切多个 provider，同一批目录只扫一次。
func qoderModelCatalog(roots []string) map[string]qoderModelInfo {
	key := strings.Join(roots, "\n")
	qoderModelCatalogMu.Lock()
	defer qoderModelCatalogMu.Unlock()
	if catalog, ok := qoderModelCatalogCache[key]; ok {
		return catalog
	}
	merged := map[string]qoderModelInfo{}
	for _, root := range roots {
		runsDir := filepath.Join(root, "logs", qoderModelRunsSubdir)
		for _, logPath := range selectQoderModelLogs(runsDir, qoderModelScanBudgetBytes) {
			applyQoderModelLog(merged, logPath)
		}
	}
	applyQoderStateDBContextWindows(merged)
	qoderModelCatalogCache[key] = merged
	return merged
}

var (
	qoderModelCatalogMu    sync.Mutex
	qoderModelCatalogCache = map[string]map[string]qoderModelInfo{}
)

// applyQoderStateDBContextWindows 用状态库 chat_model_preferences.context_window
// 补日志里没出现的别名。日志的 max_input_tokens 是请求时实际配置（qfmodel=180k、
// dfmodel=1M），优先级更高；状态库的数值更像 UI 默认（实测清一色 400k），只在
// 日志完全缺席该别名时兜底。窗口两个来源都没有时估算退化为不展示。
func applyQoderStateDBContextWindows(catalog map[string]qoderModelInfo) {
	dbPath := qoderStateDBPath()
	if dbPath == "" {
		return
	}
	db, err := sql.Open("sqlite", sqliteReadOnlyURI(dbPath))
	if err != nil {
		return
	}
	defer db.Close()
	rows, err := db.Query(`select model_key, context_window from chat_model_preferences where context_window > 0`)
	if err != nil {
		return // 表缺失或版本差异，静默跳过
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var window int64
		if err := rows.Scan(&key, &window); err != nil {
			return
		}
		if key == "" || window <= 0 {
			continue
		}
		info := catalog[key]
		if info.MaxInputTokens == 0 {
			info.MaxInputTokens = window
			catalog[key] = info
		}
	}
}

// qoderModelLogRoots 返回要扫描的数据目录：Qoder 默认目录，以及 provider 解析为
// qoder 时显式指定的 --data 目录。
func qoderModelLogRoots(provider Provider, dataDir string) []string {
	roots := make([]string, 0, 2)
	seen := make(map[string]bool, 2)
	appendRoot := func(root string) {
		if root == "" {
			return
		}
		root = filepath.Clean(root)
		if seen[root] {
			return
		}
		seen[root] = true
		roots = append(roots, root)
	}
	appendRoot(defaultDataDirForProvider(ProviderQoder))
	if provider == ProviderQoder {
		appendRoot(dataDir)
	}
	return roots
}

// selectQoderModelLogs 从最新往旧累计日志大小、取到预算为止，再按旧→新返回，
// 让"后出现的覆盖先出现的"这条新鲜度规则成立。
//
// run 目录名以启动时间戳开头，字典序即时间序。取不下某个文件时跳过它继续找更小的旧
// 日志，而不是整体放弃——真名缺失只是退回别名，不该因此丢掉整批可用线索。
func selectQoderModelLogs(runsDir string, budget int64) []string {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(entries))
	sizes := make(map[string]int64, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(runsDir, entry.Name(), qoderModelLogName)
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			continue
		}
		paths = append(paths, path)
		sizes[path] = info.Size()
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i] > paths[j] })

	chosen := make([]string, 0, len(paths))
	var used int64
	for _, path := range paths {
		size := sizes[path]
		if used+size > budget {
			continue
		}
		used += size
		chosen = append(chosen, path)
	}
	sort.Strings(chosen)
	return chosen
}

func applyQoderModelLog(catalog map[string]qoderModelInfo, logPath string) {
	file, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer file.Close()

	// ReadString 而不是 Scanner：Scanner 遇到超长行会报错并放弃整个文件，
	// 这里一条日志再长也只是它自己那一条解析不到。
	reader := bufio.NewReaderSize(file, qoderModelReaderBytes)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			scanQoderModelLine(catalog, line)
		}
		if err != nil {
			return
		}
	}
}

// scanQoderModelLine 消费一行里的全部线索：config 行同时提取真名与窗口，
// Routing 行只提取真名（不触碰已有窗口——窗口以请求时配置为准，Routing 行不带）。
func scanQoderModelLine(catalog map[string]qoderModelInfo, line string) {
	scanQoderModelMarker(catalog, line, qoderModelConfigMarker, qoderModelConfigRe, true)
	scanQoderModelMarker(catalog, line, qoderModelRoutingMarker, qoderRoutingRe, false)
}

// scanQoderModelMarker 对每个 marker 之后的有界窗口跑 re，命中即写入 catalog。
// 窗口把正则成本钉死：日志行里可能带完整请求体，对整行跑正则会随行长放大。
func scanQoderModelMarker(catalog map[string]qoderModelInfo, line, marker string, re *regexp.Regexp, withWindow bool) {
	if !strings.Contains(line, marker) {
		return
	}
	for offset := 0; ; {
		idx := strings.Index(line[offset:], marker)
		if idx < 0 {
			return
		}
		start := offset + idx + len(marker)
		window := line[start:]
		if len(window) > qoderModelWindowBytes {
			window = window[:qoderModelWindowBytes]
		}
		if match := re.FindStringSubmatch(window); match != nil {
			info := catalog[match[1]]
			if name := strings.TrimSpace(match[2]); name != "" {
				info.Name = name
			}
			if withWindow {
				if w := qoderModelWindowRe.FindStringSubmatch(window); w != nil {
					if v, err := strconv.ParseInt(w[1], 10, 64); err == nil && v > 0 {
						info.MaxInputTokens = v
					}
				}
			}
			catalog[match[1]] = info
		}
		offset = start
	}
}

// decorateQoderModels 在渲染边缘给模型条目补真实名称与估算输入 token。
// 只有分析里可能出现 Qoder 数据时才扫描日志，Claude/Codex 的调用完全不付这个成本。
// 挂在三个 DashboardData 构建器的出口，CLI 与 web 的每条命令路径都覆盖到。
func decorateQoderModels(data *DashboardData) {
	if data == nil {
		return
	}
	provider := Provider(data.Provider)
	if !qoderModelCatalogNeeded(provider) {
		return
	}
	applyQoderModels(data, qoderModelCatalog(qoderModelLogRoots(provider, cfg.DataDir)))
}

// applyQoderModels 按 catalog 填充真实名称与估算输入 token。估算 =
// 该模型 Σ(context_usage_ratio) × 窗口：解析层只落 ratio 和这一"原料"，
// 乘法留在渲染层，窗口配置变化或日志缺失都不会污染缓存里的统计口径。
// 别名不是 Qoder 专属命名，Claude/GPT 之类的模型名查不到就是零值，不影响它们。
func applyQoderModels(data *DashboardData, catalog map[string]qoderModelInfo) {
	if data == nil || len(catalog) == 0 {
		return
	}
	for i := range data.ModelUsage {
		if info, ok := catalog[data.ModelUsage[i].Model]; ok && info.Name != "" {
			data.ModelUsage[i].ModelName = info.Name
		}
	}
	if data.CostAnalysis == nil {
		return
	}
	for i := range data.CostAnalysis.ByModel {
		stat := &data.CostAnalysis.ByModel[i]
		info, ok := catalog[stat.Model]
		if !ok {
			continue
		}
		if info.Name != "" {
			stat.ModelName = info.Name
		}
		if info.MaxInputTokens > 0 && stat.ContextRatioSum > 0 {
			stat.EstInputTokens = int(math.Round(stat.ContextRatioSum * float64(info.MaxInputTokens)))
		}
	}
	var estTotal int
	for _, stat := range data.CostAnalysis.ByModel {
		estTotal += stat.EstInputTokens
	}
	data.CostAnalysis.Totals.EstInputTokens = estTotal
}

// modelDisplayLabel 渲染用标签：查到真名就带上别名括号，查不到就只显示原值。
// 括号里的别名不能省——统计 key 和 --model 过滤用的都是别名，只显示真名会让用户
// 看到报表却找不到下钻参数。
func modelDisplayLabel(model, modelName string) string {
	if modelName == "" || modelName == model {
		return model
	}
	return modelName + " (" + model + ")"
}
