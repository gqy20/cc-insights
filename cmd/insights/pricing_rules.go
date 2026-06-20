package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed rules/pricing.yml
var embeddedPricingRules embed.FS

const defaultPricingRulesName = "pricing.yml"

// pricingRulesFile 对应 rules/pricing.yml 的磁盘结构。
type pricingRulesFile struct {
	Version  int                    `yaml:"version"`
	Currency string                 `yaml:"currency"`
	Default  *pricingTier           `yaml:"default"`
	Models   map[string]pricingTier `yaml:"models"`
	Prefixes map[string]pricingTier `yaml:"prefixes"`
}

// pricingTier 单个模型（或 default/prefix）的单价，单位为 currency / 百万 token。
// 字段用 *float64 以区分「未设置」（触发缺省派生）与「显式为 0」（如限时免费缓存读）。
type pricingTier struct {
	Input         *float64 `yaml:"input"`
	Output        *float64 `yaml:"output"`
	CacheRead     *float64 `yaml:"cache_read"`
	CacheCreation *float64 `yaml:"cache_creation"`
	Rationale     string   `yaml:"rationale"`
}

// resolvedPrice 匹配并应用缺省派生后的确定单价（currency / 百万 token）。
type resolvedPrice struct {
	Input         float64
	Output        float64
	CacheRead     float64
	CacheCreation float64
}

type prefixEntry struct {
	prefix string
	tier   pricingTier
}

// pricingRulesSet 解析后的定价规则集。
type pricingRulesSet struct {
	Source   string
	Hash     string
	Currency string
	Default  pricingTier
	Models   map[string]pricingTier
	Prefixes []prefixEntry // 按 prefix 长度降序，便于最长前缀匹配
}

var (
	pricingRulesMu          sync.Mutex
	cachedPricingRulesKey   string
	cachedPricingRulesValue *pricingRulesSet
)

// currentPricingRules 加载定价规则（三级：--pricing → ~/.cc-insights/pricing.yml → 内置），
// 带进程内缓存，按内容 sha256 失效。与 currentBashRules 同构。
func currentPricingRules() (*pricingRulesSet, error) {
	key, data, source, err := loadPricingRulesData()
	if err != nil {
		return nil, err
	}

	pricingRulesMu.Lock()
	defer pricingRulesMu.Unlock()
	if cachedPricingRulesValue != nil && cachedPricingRulesKey == key {
		return cachedPricingRulesValue, nil
	}

	rules, err := parsePricingRules(data, source)
	if err != nil {
		return nil, err
	}
	cachedPricingRulesKey = key
	cachedPricingRulesValue = rules
	return rules, nil
}

func loadPricingRulesData() (string, []byte, string, error) {
	if path := strings.TrimSpace(cfg.PricingPath); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil, "", fmt.Errorf("读取定价规则失败: %w", err)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		sum := sha256.Sum256(data)
		return abs + ":" + hex.EncodeToString(sum[:]), data, abs, nil
	}

	if defaultPath, ok := defaultExternalPricingRulesPath(); ok {
		data, err := os.ReadFile(defaultPath)
		if err != nil {
			return "", nil, "", fmt.Errorf("读取定价规则失败: %w", err)
		}
		abs, err := filepath.Abs(defaultPath)
		if err != nil {
			abs = defaultPath
		}
		sum := sha256.Sum256(data)
		return abs + ":" + hex.EncodeToString(sum[:]), data, abs, nil
	}

	data, err := embeddedPricingRules.ReadFile("rules/" + defaultPricingRulesName)
	if err != nil {
		return "", nil, "", fmt.Errorf("读取内置定价规则失败: %w", err)
	}
	sum := sha256.Sum256(data)
	return "embedded:" + hex.EncodeToString(sum[:]), data, "embedded", nil
}

func defaultExternalPricingRulesPath() (string, bool) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return "", false
	}
	path := filepath.Join(homeDir, ".cc-insights", defaultPricingRulesName)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

func parsePricingRules(data []byte, source string) (*pricingRulesSet, error) {
	var file pricingRulesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("解析定价规则失败 (%s): %w", source, err)
	}
	if file.Version <= 0 {
		return nil, fmt.Errorf("定价规则版本无效 (%s)", source)
	}

	rules := &pricingRulesSet{
		Source:   source,
		Currency: strings.TrimSpace(file.Currency),
		Models:   make(map[string]pricingTier, len(file.Models)),
	}
	if rules.Currency == "" {
		rules.Currency = "CNY"
	}
	if file.Default != nil {
		rules.Default = *file.Default
	}
	for name, tier := range file.Models {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		rules.Models[name] = tier
	}
	for prefix, tier := range file.Prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		rules.Prefixes = append(rules.Prefixes, prefixEntry{prefix: prefix, tier: tier})
	}
	// 按 prefix 长度降序，最长前缀优先命中。
	sort.Slice(rules.Prefixes, func(i, j int) bool {
		if len(rules.Prefixes[i].prefix) == len(rules.Prefixes[j].prefix) {
			return rules.Prefixes[i].prefix < rules.Prefixes[j].prefix
		}
		return len(rules.Prefixes[i].prefix) > len(rules.Prefixes[j].prefix)
	})

	sum := sha256.Sum256(data)
	rules.Hash = hex.EncodeToString(sum[:])
	return rules, nil
}

// matchPrice 按 精确(models) → 最长前缀(prefixes) → default 匹配，并应用缺省派生。
// 缺省派生：cache_read 未设 → input×0.1；cache_creation 未设 → input×1.25。
// input/output 在 tier/default 均缺时用硬编码兜底（5.0 / 20.0），保证总有价可算。
// 第二个返回值恒为 true（除非 rules 为 nil），便于调用方忽略 ok。
func (r *pricingRulesSet) matchPrice(model string) (resolvedPrice, bool) {
	if r == nil {
		return resolvedPrice{}, false
	}

	tier := r.Default
	if t, ok := r.Models[model]; ok {
		tier = t
	} else {
		for _, pe := range r.Prefixes {
			if strings.HasPrefix(model, pe.prefix) {
				tier = pe.tier
				break
			}
		}
	}

	in := valf(tier.Input, valf(r.Default.Input, 5.0))
	out := valf(tier.Output, valf(r.Default.Output, 20.0))
	cr := valf(tier.CacheRead, in*0.1)
	cc := valf(tier.CacheCreation, in*1.25)
	return resolvedPrice{Input: in, Output: out, CacheRead: cr, CacheCreation: cc}, true
}

// pricingCurrency 返回当前规则集的货币（默认 CNY）；加载失败时回退 CNY。
func pricingCurrency() string {
	rules, err := currentPricingRules()
	if err != nil || rules == nil || rules.Currency == "" {
		return "CNY"
	}
	return rules.Currency
}

// valf 解引用可选 float：p 非 nil 取 *p，否则取 fallback。
func valf(p *float64, fallback float64) float64 {
	if p != nil {
		return *p
	}
	return fallback
}
