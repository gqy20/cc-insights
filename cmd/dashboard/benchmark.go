//go:build !prod

package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	// 解析参数
	dataDir := "../data"
	for i, arg := range os.Args {
		if arg == "-data" && i+1 < len(os.Args) {
			dataDir = os.Args[i+1]
		}
	}
	cfg.DataDir = dataDir

	fmt.Println("=== Claude Code Dashboard 性能测试 ===")
	fmt.Printf("数据目录: %s\n\n", dataDir)

	// 测试 7 天数据
	fmt.Println("测试时间范围: 最近 7 天")
	tf := NewTimeFilterFromPreset(Range7Days)

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
