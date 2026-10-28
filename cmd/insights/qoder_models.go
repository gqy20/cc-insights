package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
)

// qoderModelCatalogNeeded 判断这次分析是否可能含 Qoder 数据，只有可能时才付扫描成本。
// 只用 Claude/Codex 的调用因此完全不受影响。
func qoderModelCatalogNeeded(provider Provider) bool {
	return provider == ProviderQoder || provider == ProviderAll
}

// qoderModelCatalog 返回别名 -> 真实模型名。按 (数据目录集合) 记忆：web 是常驻进程，
// 会在一次进程里切多个 provider，同一批目录只扫一次。
func qoderModelCatalog(roots []string) map[string]string {
	key := strings.Join(roots, "\n")
	qoderModelCatalogMu.Lock()
	defer qoderModelCatalogMu.Unlock()
	if catalog, ok := qoderModelCatalogCache[key]; ok {
		return catalog
	}
	merged := map[string]string{}
	for _, root := range roots {
		runsDir := filepath.Join(root, "logs", qoderModelRunsSubdir)
		for _, logPath := range selectQoderModelLogs(runsDir, qoderModelScanBudgetBytes) {
			applyQoderModelLog(merged, logPath)
		}
	}
	qoderModelCatalogCache[key] = merged
	return merged
}

var (
	qoderModelCatalogMu    sync.Mutex
	qoderModelCatalogCache = map[string]map[string]string{}
)

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

func applyQoderModelLog(catalog map[string]string, logPath string) {
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
			scanQoderModelMarker(catalog, line, qoderModelRoutingMarker, qoderRoutingRe)
			scanQoderModelMarker(catalog, line, qoderModelConfigMarker, qoderModelConfigRe)
		}
		if err != nil {
			return
		}
	}
}

// scanQoderModelMarker 对每个 marker 之后的有界窗口跑 re，命中即写入 catalog。
// 窗口把正则成本钉死：日志行里可能带完整请求体，对整行跑正则会随行长放大。
func scanQoderModelMarker(catalog map[string]string, line, marker string, re *regexp.Regexp) {
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
			if name := strings.TrimSpace(match[2]); name != "" {
				catalog[match[1]] = name
			}
		}
		offset = start
	}
}

// decorateQoderModelNames 在渲染边缘给模型条目补真实名称。只有分析里可能出现
// Qoder 数据时才扫描日志，Claude/Codex 的调用完全不付这个成本。
// 挂在两个 dashboard 构建器的出口，CLI 与 web 的每条命令路径都覆盖到。
func decorateQoderModelNames(data *DashboardData) {
	if data == nil {
		return
	}
	provider := Provider(data.Provider)
	if !qoderModelCatalogNeeded(provider) {
		return
	}
	applyQoderModelNames(data, qoderModelCatalog(qoderModelLogRoots(provider, cfg.DataDir)))
}

// applyQoderModelNames 按 catalog 填充 ModelName。别名不是 Qoder 专属命名，
// Claude/GPT 之类的模型名查不到就是空串，不影响它们。
func applyQoderModelNames(data *DashboardData, catalog map[string]string) {
	if data == nil || len(catalog) == 0 {
		return
	}
	for i := range data.ModelUsage {
		if name := catalog[data.ModelUsage[i].Model]; name != "" {
			data.ModelUsage[i].ModelName = name
		}
	}
	if data.CostAnalysis == nil {
		return
	}
	for i := range data.CostAnalysis.ByModel {
		if name := catalog[data.CostAnalysis.ByModel[i].Model]; name != "" {
			data.CostAnalysis.ByModel[i].ModelName = name
		}
	}
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
