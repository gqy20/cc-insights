package main

import (
	"path/filepath"
	"testing"
	"time"
)

// 合并视图：两个 provider 的缓存合并后，查询期数字应等于各自之和。
func TestMergeCacheFilesSumsProviders(t *testing.T) {
	// provider A：claude fixture
	dirA := t.TempDir()
	dataA := createTestDataDir(t, dirA)
	cacheA := &CacheFile{}
	builderA := &CacheBuilder{CachePath: filepath.Join(dirA, "c.db"), DataDir: dataA}
	aggA, _, _, _, err := builderA.buildProjectAggregateIncremental(dataA, nil)
	if err != nil {
		t.Fatalf("build A: %v", err)
	}
	cacheA = cacheFileFromAggregate(t, aggA, "claude")

	// provider B：pi fixture
	dirB := t.TempDir()
	writePiFixture(t, dirB)
	builderB := &CacheBuilder{CachePath: filepath.Join(dirB, "c.db"), DataDir: dirB, Provider: ProviderPi}
	aggB, _, _, _, err := builderB.buildProjectAggregateIncremental(dirB, nil)
	if err != nil {
		t.Fatalf("build B: %v", err)
	}
	cacheB := cacheFileFromAggregate(t, aggB, "pi")

	msgsA, toolsA := sumDailyMessages(cacheA), sumRuntimeToolCalls(cacheA)
	msgsB, toolsB := sumDailyMessages(cacheB), sumRuntimeToolCalls(cacheB)
	if msgsA == 0 || toolsB == 0 {
		t.Fatalf("fixtures empty: A msgs=%d tools=%d, B msgs=%d tools=%d", msgsA, toolsA, msgsB, toolsB)
	}

	merged := mergeCacheFiles(nil, cacheA)
	merged = mergeCacheFiles(merged, cacheB)
	merged.Provider = string(ProviderAll)

	if got := sumDailyMessages(merged); got != msgsA+msgsB {
		t.Fatalf("merged messages=%d, want %d", got, msgsA+msgsB)
	}
	if got := sumRuntimeToolCalls(merged); got != toolsA+toolsB {
		t.Fatalf("merged tool calls=%d, want %d", got, toolsA+toolsB)
	}

	// 查询期（QueryByTimeRange → runtime 聚合）同样守恒
	q := merged.QueryByTimeRange(time.Time{}, time.Now().Add(24*time.Hour))
	if q.ToolAnalysis == nil {
		t.Fatal("merged ToolAnalysis nil after query")
	}
	if q.ToolAnalysis.TotalCalls != toolsA+toolsB {
		t.Fatalf("queried tool calls=%d, want %d", q.ToolAnalysis.TotalCalls, toolsA+toolsB)
	}
	if q.ModelUsage["glm/glm-5v-turbo"] == nil {
		t.Fatalf("merged models=%+v, want pi fixture model glm/glm-5v-turbo", q.ModelUsage)
	}
}

// cacheFileFromAggregate 从聚合构建最小可查询 CacheFile（DailyStats + runtime 映射）。
func cacheFileFromAggregate(t *testing.T, agg *ProjectAggregate, provider string) *CacheFile {
	t.Helper()
	agg.finalize()
	cache := &CacheFile{
		Version:             CacheVersion,
		Provider:            provider,
		DailyStats:          make(map[string]*DayAggregate),
		DailyRuntime:        make(map[string]ProjectFileAggregate),
		DailyProjectRuntime: make(map[string]map[string]ProjectFileAggregate),
		DailySessionRuntime: make(map[string]map[string]ProjectFileAggregate),
	}
	total := 0
	for date, count := range agg.DailyActivity {
		total += count
		cache.DailyStats[date] = &DayAggregate{
			Date:          date,
			MessageCount:  count,
			HourlyCounts:  agg.DailyHourlyCounts[date],
			ProjectCounts: copyIntMap(agg.DailyProjectCounts[date]),
			ModelCounts:   copyIntMap(agg.DailyModelCounts[date]),
		}
	}
	cache.TotalMessages = total
	for date, runtimeAgg := range agg.DailyRuntime {
		if runtimeAgg != nil {
			cache.DailyRuntime[date] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
		}
	}
	for date, projects := range agg.DailyProjectRuntime {
		for project, runtimeAgg := range projects {
			if cache.DailyProjectRuntime[date] == nil {
				cache.DailyProjectRuntime[date] = make(map[string]ProjectFileAggregate)
			}
			cache.DailyProjectRuntime[date][project] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
		}
	}
	for date, sessions := range agg.DailySessionRuntime {
		for sessionID, runtimeAgg := range sessions {
			if cache.DailySessionRuntime[date] == nil {
				cache.DailySessionRuntime[date] = make(map[string]ProjectFileAggregate)
			}
			cache.DailySessionRuntime[date][sessionID] = aggregateToProjectFileAggregateWithDaily(runtimeAgg, false)
		}
	}
	return cache
}

func sumDailyMessages(c *CacheFile) int {
	total := 0
	for _, day := range c.DailyStats {
		total += day.MessageCount
	}
	return total
}

func sumRuntimeToolCalls(c *CacheFile) int {
	agg := newProjectAggregate()
	for _, snapshot := range c.DailyRuntime {
		mergeProjectAggregate(agg, projectFileAggregateToAggregate(snapshot))
	}
	total := 0
	for _, stat := range agg.ToolStats {
		total += stat.CallCount
	}
	return total
}
