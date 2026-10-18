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
	// DataDirExplicit 表示用户通过 --data 显式指定了数据目录。
	// 为真时，Web API 不得在切换 provider 时用该 provider 的默认目录覆盖 DataDir。
	DataDirExplicit bool
}

var cfg Config

type Provider string

const (
	ProviderClaude   Provider = "claude"
	ProviderCodex    Provider = "codex"
	ProviderOpenCode Provider = "opencode"
	ProviderPi       Provider = "pi"
	ProviderAll      Provider = "all" // 合并所有可用渠道
	ProviderAuto     Provider = "auto"
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
	fs.Var(&target.Provider, "provider", "数据源 provider: claude|codex|opencode|pi|all|auto（all 为跨渠道合并视图）")
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
	case ProviderClaude, ProviderCodex, ProviderOpenCode, ProviderPi, ProviderAll, ProviderAuto:
		*p = provider
		return nil
	default:
		return fmt.Errorf("未知 provider %q，可选: claude|codex|opencode|pi|auto", value)
	}
}

func resolveProvider(provider Provider, dataDir string) Provider {
	if provider != ProviderAuto && provider != "" {
		return provider
	}
	if looksLikeClaudeDataDir(dataDir) {
		return ProviderClaude
	}
	if looksLikePiDataDir(dataDir) {
		return ProviderPi
	}
	if looksLikeCodexDataDir(dataDir) {
		return ProviderCodex
	}
	if looksLikeOpenCodeDataDir(dataDir) {
		return ProviderOpenCode
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

// looksLikePiDataDir 识别 pi agent 数据目录（~/.pi，含 agent/sessions）。
func looksLikePiDataDir(dataDir string) bool {
	if dataDir == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(dataDir, "agent", "sessions")); err == nil && info.IsDir() {
		return true
	}
	return false
}

// looksLikeOpenCodeDataDir 识别 opencode 数据目录（~/.local/share/opencode，含 storage）。
func looksLikeOpenCodeDataDir(dataDir string) bool {
	if dataDir == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(dataDir, "storage")); err == nil && info.IsDir() {
		return true
	}
	return false
}

func defaultDataDirForProvider(provider Provider) string {
	homeDir, _ := os.UserHomeDir()
	switch provider {
	case ProviderCodex:
		return filepath.Join(homeDir, ".codex")
	case ProviderOpenCode:
		return filepath.Join(homeDir, ".local", "share", "opencode")
	case ProviderPi:
		return filepath.Join(homeDir, ".pi")
	default:
		return filepath.Join(homeDir, ".claude")
	}
}

// configForRequestedProvider 根据请求的 provider 派生配置。
// 用户显式传入 --data 时保留该目录；否则显式 provider 使用各自默认目录。
func configForRequestedProvider(config Config, requested Provider) Config {
	if requested == "" {
		return config
	}
	if !config.DataDirExplicit && requested != ProviderAuto && requested != ProviderAll {
		config.DataDir = defaultDataDirForProvider(requested)
	}
	config.Provider = resolveProvider(requested, config.DataDir)
	return config
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
