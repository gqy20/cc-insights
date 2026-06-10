package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ParseSessionIndex 解析 sessions-index.json 文件
// 返回会话索引数据，可用于更准确的会话统计
func ParseSessionIndex(projectPath string) (*SessionIndexResult, error) {
	if projectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}

	indexPath := filepath.Join(projectPath, "sessions-index.json")
	f, err := os.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("open sessions-index.json: %w", err)
	}
	defer f.Close()

	var result SessionIndexResult
	if err := json.NewDecoder(f).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sessions-index.json: %w", err)
	}

	return &result, nil
}

// ParseSessionStatsWithFilter 带时间过滤解析会话统计
// 使用 ParseProjectsConcurrentOnce 一次遍历获取所有数据，然后从 aggregate 提取 session 统计
// 避免冗余的二次文件遍历
func ParseSessionStatsWithFilter(tf TimeFilter) (*SessionStats, error) {
	agg, err := ParseProjectsConcurrentOnce(tf)
	if err != nil {
		return nil, err
	}
	return extractSessionStatsFromAggregate(agg)
}
