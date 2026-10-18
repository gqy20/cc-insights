package main

import (
	"fmt"
)

// 跨 provider 合并视图：--provider all。
// 逐 provider 加载/刷新各自缓存（含 tail 增量），在内存合并为一次性
// CacheFile，不落盘。QueryByTimeRange 的 runtime 聚合路径会从
// DailyStats + DailyRuntime* 重建完整分析层，因此合并只需这三组字段。

var allMergeProviderList = []Provider{ProviderClaude, ProviderCodex, ProviderOpenCode, ProviderPi}

// loadMergedAllProvidersCache 加载所有可用 provider 的缓存并合并。
func loadMergedAllProvidersCache() (*CacheFile, error) {
	if cfg.DataDirExplicit {
		return nil, fmt.Errorf("--provider all 不支持自定义 --data（各渠道使用各自默认数据目录）")
	}
	var merged *CacheFile
	included := make([]string, 0, len(allMergeProviderList))
	for _, provider := range allMergeProviderList {
		sub := cfg
		sub.Provider = provider
		sub.DataDir = defaultDataDirForProvider(provider)
		sub.DataDirExplicit = false
		cache, err := refreshCacheForConfig(sub, false)
		if err != nil {
			Warn("合并视图跳过 provider", "provider", string(provider), "error", err.Error())
			continue
		}
		if cache == nil || len(cache.DailyStats) == 0 {
			continue
		}
		merged = mergeCacheFiles(merged, cache)
		included = append(included, string(provider))
	}
	if merged == nil {
		return nil, fmt.Errorf("未发现任何可用数据源（claude/codex/opencode/pi）")
	}
	merged.Provider = string(ProviderAll)
	merged.BuildID = "all:" + fmt.Sprint(included)
	Info("合并视图", "providers", fmt.Sprint(included))
	return merged, nil
}

// mergeCacheFiles 把 src 合并进 dst（不修改 src，返回可安全使用的 dst）。
// 只合并 QueryByTimeRange 依赖的字段；ProjectStats/ModelUsage/ToolStats/
// 分析结构体均由查询期从 DailyStats 与 runtime 聚合重建。
func mergeCacheFiles(dst, src *CacheFile) *CacheFile {
	if dst == nil {
		out := &CacheFile{
			Version:             src.Version,
			Provider:            src.Provider,
			BashRulesHash:       src.BashRulesHash,
			LastUpdate:          src.LastUpdate,
			DailyStats:          make(map[string]*DayAggregate, len(src.DailyStats)),
			DailyRuntime:        make(map[string]ProjectFileAggregate, len(src.DailyRuntime)),
			DailyProjectRuntime: make(map[string]map[string]ProjectFileAggregate, len(src.DailyProjectRuntime)),
			DailySessionRuntime: make(map[string]map[string]ProjectFileAggregate, len(src.DailySessionRuntime)),
		}
		mergeCacheFiles(out, src)
		return out
	}
	if src.LastUpdate.After(dst.LastUpdate) {
		dst.LastUpdate = src.LastUpdate
	}
	for date, day := range src.DailyStats {
		if existing := dst.DailyStats[date]; existing != nil {
			existing.MessageCount += day.MessageCount
			existing.SessionCount += day.SessionCount
			existing.ToolCallCount += day.ToolCallCount
			existing.HourlyCounts = addIntArrays24(existing.HourlyCounts, day.HourlyCounts)
			existing.ProjectCounts = mergeAddIntMap(existing.ProjectCounts, day.ProjectCounts)
			existing.ModelCounts = mergeAddIntMap(existing.ModelCounts, day.ModelCounts)
			existing.ModelTokens = mergeAddIntMap(existing.ModelTokens, day.ModelTokens)
			continue
		}
		dayCopy := *day
		dayCopy.ProjectCounts = copyIntMap(day.ProjectCounts)
		dayCopy.ModelCounts = copyIntMap(day.ModelCounts)
		dayCopy.ModelTokens = copyIntMap(day.ModelTokens)
		dayCopy.HourlyCounts = day.HourlyCounts // 数组值，直接拷贝
		dst.DailyStats[date] = &dayCopy
	}
	mergeDailyRuntime(dst.DailyRuntime, src.DailyRuntime)
	mergeDailyProjectRuntime(dst.DailyProjectRuntime, src.DailyProjectRuntime)
	mergeDailySessionRuntime(dst.DailySessionRuntime, src.DailySessionRuntime)
	return dst
}

// mergeDailyRuntime 按 date 合并 runtime 快照：转 ProjectAggregate 合并后回写。
func mergeDailyRuntime(dst, src map[string]ProjectFileAggregate) {
	for date, snapshot := range src {
		if existing, ok := dst[date]; ok {
			mergedAgg := projectFileAggregateToAggregate(existing)
			mergeProjectAggregate(mergedAgg, projectFileAggregateToAggregate(snapshot))
			dst[date] = aggregateToProjectFileAggregate(mergedAgg)
		} else {
			dst[date] = snapshot
		}
	}
}

func mergeDailyProjectRuntime(dst, src map[string]map[string]ProjectFileAggregate) {
	for date, projects := range src {
		if dst[date] == nil {
			dst[date] = make(map[string]ProjectFileAggregate, len(projects))
		}
		mergeDailyRuntime(dst[date], projects)
	}
}

func mergeDailySessionRuntime(dst, src map[string]map[string]ProjectFileAggregate) {
	mergeDailyProjectRuntime(dst, src)
}

func addIntArrays24(dst, src [24]int) [24]int {
	for i := range src {
		dst[i] += src[i]
	}
	return dst
}

// mergeAddIntMap 把 src 逐键加到 dst（dst 为 nil 时新建）。
func mergeAddIntMap(dst, src map[string]int) map[string]int {
	if dst == nil {
		dst = make(map[string]int, len(src))
	}
	for k, v := range src {
		dst[k] += v
	}
	return dst
}
