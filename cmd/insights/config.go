package main

import (
	"flag"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	DataDir     string
	CacheDir    string
	ListenAddr  string
	BaseURL     string
	RulesPath   string
	PricingPath string
}

var cfg Config

func defaultConfig() Config {
	// 家目录：~/.cc-insights/（缓存、配置、日志统一管理）
	homeDir, _ := os.UserHomeDir()
	insightsHome := filepath.Join(homeDir, ".cc-insights")

	// 默认数据目录：~/.claude（Claude Code 数据根目录，只读引用）
	defaultDataDir := filepath.Join(homeDir, ".claude")
	// 默认缓存目录：~/.cc-insights/cache/
	defaultCacheDir := filepath.Join(insightsHome, "cache")

	return Config{
		DataDir:     defaultDataDir,
		CacheDir:    defaultCacheDir,
		ListenAddr:  ":8932",
		BaseURL:     "",
		RulesPath:   "",
		PricingPath: "",
	}
}

func init() {
	cfg = defaultConfig()
}

// registerConfigFlags 注册所有命令通用的配置 flag（数据/缓存/规则路径）。
func registerConfigFlags(fs *flag.FlagSet, target *Config) {
	fs.StringVar(&target.DataDir, "data", target.DataDir, "数据目录路径 (默认: ~/.claude)")
	fs.StringVar(&target.CacheDir, "cache", target.CacheDir, "缓存目录路径 (默认: ~/.cc-insights/cache/)")
	fs.StringVar(&target.RulesPath, "rules", target.RulesPath, "Bash 命令分类规则 YAML 路径")
	fs.StringVar(&target.PricingPath, "pricing", target.PricingPath, "模型定价规则 YAML 路径（默认内置 rules/pricing.yml，可用 ~/.cc-insights/pricing.yml 覆盖）")
}

// registerServerFlags 注册仅 web 命令使用的服务 flag（监听地址/反向代理）。
func registerServerFlags(fs *flag.FlagSet, target *Config) {
	fs.StringVar(&target.ListenAddr, "addr", target.ListenAddr, "监听地址 (默认: :8932)")
	fs.StringVar(&target.BaseURL, "base", target.BaseURL, "基础 URL（用于反向代理）")
}

// GetDataPath 获取数据文件路径
func GetDataPath(relPath ...string) string {
	paths := append([]string{cfg.DataDir}, relPath...)
	return filepath.Join(paths...)
}
