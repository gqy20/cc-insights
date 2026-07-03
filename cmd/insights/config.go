package main

import (
	"flag"
	"fmt"
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
	Provider    Provider
}

var cfg Config

type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
	ProviderAuto   Provider = "auto"
)

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
		Provider:    ProviderClaude,
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
	fs.Var(&target.Provider, "provider", "数据源 provider: claude|codex|auto")
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

func (p *Provider) String() string {
	if p == nil || *p == "" {
		return string(ProviderClaude)
	}
	return string(*p)
}

func (p *Provider) Set(value string) error {
	provider := Provider(value)
	switch provider {
	case ProviderClaude, ProviderCodex, ProviderAuto:
		*p = provider
		return nil
	default:
		return fmt.Errorf("未知 provider %q，可选: claude|codex|auto", value)
	}
}

func resolveProvider(provider Provider, dataDir string) Provider {
	if provider != ProviderAuto && provider != "" {
		return provider
	}
	if looksLikeCodexDataDir(dataDir) && !looksLikeClaudeDataDir(dataDir) {
		return ProviderCodex
	}
	return ProviderClaude
}

func looksLikeClaudeDataDir(dataDir string) bool {
	if dataDir == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(dataDir, "projects")); err == nil && info.IsDir() {
		return true
	}
	return false
}

func looksLikeCodexDataDir(dataDir string) bool {
	if dataDir == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(dataDir, "sessions")); err == nil && info.IsDir() {
		return true
	}
	return false
}

func defaultDataDirForProvider(provider Provider) string {
	homeDir, _ := os.UserHomeDir()
	switch provider {
	case ProviderCodex:
		return filepath.Join(homeDir, ".codex")
	default:
		return filepath.Join(homeDir, ".claude")
	}
}

func providerCacheDir(baseCacheDir string, provider Provider) string {
	if provider == "" {
		provider = ProviderClaude
	}
	return filepath.Join(baseCacheDir, string(provider))
}

func cachePathForConfig(config Config) string {
	provider := resolveProvider(config.Provider, config.DataDir)
	return filepath.Join(providerCacheDir(config.CacheDir, provider), "cache.db")
}
