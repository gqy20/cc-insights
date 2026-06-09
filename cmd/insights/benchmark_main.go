//go:build bench

package main

import (
	"flag"
	"fmt"
	"time"
)

func main() {
	rangePreset := flag.String("range", "7d", "测试时间范围: 7d 或 all")
	flag.Parse()

	fmt.Println("=== Claude Code Dashboard 性能测试 ===")
	fmt.Printf("数据目录: %s\n\n", cfg.DataDir)

	preset := Range7Days
	if *rangePreset == "all" {
		preset = RangeAll
	}
	fmt.Printf("测试时间范围: %s\n", *rangePreset)
	tf := NewTimeFilterFromPreset(preset)

	// 测试 history.jsonl 解析
	fmt.Println("\n1. history.jsonl 解析测试:")
	start := time.Now()
	cmdStats, _, err := ParseHistoryConcurrent(tf)
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		elapsed := time.Since(start)
		fmt.Printf("   ✓ 耗时: %.2fs\n", elapsed.Seconds())
		fmt.Printf("   ✓ 命令数: %d\n", len(cmdStats))
		fmt.Printf("   ✓ 总记录: %d\n", sumCounts(cmdStats))
	}

	// 测试 debug 日志解析
	fmt.Println("\n2. debug/ 日志解析测试:")
	start = time.Now()
	toolStats, err := ParseDebugLogsConcurrent(tf)
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		elapsed := time.Since(start)
		fmt.Printf("   ✓ 耗时: %.2fs\n", elapsed.Seconds())
		fmt.Printf("   ✓ 工具数: %d\n", len(toolStats))
		fmt.Printf("   ✓ 总调用: %d\n", sumToolCounts(toolStats))
	}

	fmt.Println("\n=== 测试完成 ===")
}

func sumCounts(stats []CommandStats) int {
	total := 0
	for _, s := range stats {
		total += s.Count
	}
	return total
}

func sumToolCounts(stats []MCPToolStats) int {
	total := 0
	for _, s := range stats {
		total += s.Count
	}
	return total
}
