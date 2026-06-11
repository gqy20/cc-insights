package main

import (
	"flag"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	DataDir    string
	CacheDir   string
	ListenAddr string
	BaseURL    string
}

var cfg Config

func init() {
	// 家目录：~/.cc-insights/（缓存、配置、日志统一管理）
	homeDir, _ := os.UserHomeDir()
	insightsHome := filepath.Join(homeDir, ".cc-insights")

	// 默认数据目录：~/.claude（Claude Code 数据根目录，只读引用）
	defaultDataDir := filepath.Join(homeDir, ".claude")
	// 默认缓存目录：~/.cc-insights/cache/
	defaultCacheDir := filepath.Join(insightsHome, "cache")

	flag.StringVar(&cfg.DataDir, "data", defaultDataDir, "数据目录路径 (默认: ~/.claude)")
	flag.StringVar(&cfg.CacheDir, "cache", defaultCacheDir, "缓存目录路径 (默认: ~/.cc-insights/cache/)")
	flag.StringVar(&cfg.ListenAddr, "addr", ":8932", "监听地址")
	flag.StringVar(&cfg.BaseURL, "base", "", "基础URL（用于反向代理）")
}

// GetDataPath 获取数据文件路径
func GetDataPath(relPath ...string) string {
	paths := append([]string{cfg.DataDir}, relPath...)
	return filepath.Join(paths...)
}
