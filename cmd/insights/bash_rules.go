package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed rules/bash.yml
var embeddedBashRules embed.FS

const defaultBashRulesName = "bash.yml"

type bashRulesFile struct {
	Version  int              `yaml:"version"`
	Families []bashFamilyRule `yaml:"families"`
}

type bashFamilyRule struct {
	Name            string   `yaml:"name"`
	Commands        []string `yaml:"commands"`
	CommandContains []string `yaml:"command_contains"`
	Contains        []string `yaml:"contains"`
}

type bashRulesSet struct {
	Source   string
	Hash     string
	Families []bashFamilyRule
}

var (
	bashRulesMu          sync.Mutex
	cachedBashRulesKey   string
	cachedBashRulesValue *bashRulesSet
)

func classifyBashCommandFamily(stat BashCommandStat) string {
	rules, err := currentBashRules()
	if err != nil {
		return "other"
	}
	return rules.classify(stat)
}

func currentBashRules() (*bashRulesSet, error) {
	key, data, source, err := loadBashRulesData()
	if err != nil {
		return nil, err
	}

	bashRulesMu.Lock()
	defer bashRulesMu.Unlock()
	if cachedBashRulesValue != nil && cachedBashRulesKey == key {
		return cachedBashRulesValue, nil
	}

	rules, err := parseBashRules(data, source)
	if err != nil {
		return nil, err
	}
	cachedBashRulesKey = key
	cachedBashRulesValue = rules
	return rules, nil
}

func currentBashRulesHash() (string, error) {
	rules, err := currentBashRules()
	if err != nil {
		return "", err
	}
	return rules.Hash, nil
}

func loadBashRulesData() (string, []byte, string, error) {
	path := strings.TrimSpace(cfg.RulesPath)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil, "", fmt.Errorf("读取 Bash 规则失败: %w", err)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		sum := sha256.Sum256(data)
		return abs + ":" + hex.EncodeToString(sum[:]), data, abs, nil
	}

	if defaultPath, ok := defaultExternalBashRulesPath(); ok {
		data, err := os.ReadFile(defaultPath)
		if err != nil {
			return "", nil, "", fmt.Errorf("读取 Bash 规则失败: %w", err)
		}
		abs, err := filepath.Abs(defaultPath)
		if err != nil {
			abs = defaultPath
		}
		sum := sha256.Sum256(data)
		return abs + ":" + hex.EncodeToString(sum[:]), data, abs, nil
	}

	data, err := embeddedBashRules.ReadFile("rules/" + defaultBashRulesName)
	if err != nil {
		return "", nil, "", fmt.Errorf("读取内置 Bash 规则失败: %w", err)
	}
	sum := sha256.Sum256(data)
	return "embedded:" + hex.EncodeToString(sum[:]), data, "embedded", nil
}

func defaultExternalBashRulesPath() (string, bool) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return "", false
	}
	path := filepath.Join(homeDir, ".cc-insights", defaultBashRulesName)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

func parseBashRules(data []byte, source string) (*bashRulesSet, error) {
	var file bashRulesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("解析 Bash 规则失败 (%s): %w", source, err)
	}
	if file.Version <= 0 {
		return nil, fmt.Errorf("Bash 规则版本无效 (%s)", source)
	}

	rules := &bashRulesSet{
		Source:   source,
		Families: make([]bashFamilyRule, 0, len(file.Families)),
	}
	sum := sha256.Sum256(data)
	rules.Hash = hex.EncodeToString(sum[:])

	for _, family := range file.Families {
		name := strings.TrimSpace(family.Name)
		if name == "" {
			continue
		}
		rule := bashFamilyRule{
			Name:            name,
			Commands:        normalizeRuleList(family.Commands),
			CommandContains: normalizeRuleList(family.CommandContains),
			Contains:        normalizeRuleList(family.Contains),
		}
		rules.Families = append(rules.Families, rule)
	}

	return rules, nil
}

func normalizeRuleList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func (rules *bashRulesSet) classify(stat BashCommandStat) string {
	command := strings.ToLower(strings.TrimSpace(stat.CommandName))
	sample := strings.ToLower(strings.TrimSpace(stat.SampleCommand))
	text := command
	if sample != "" {
		text += " " + sample
	}

	for _, family := range rules.Families {
		if matchesBashFamilyRule(family, command, text) {
			return family.Name
		}
	}
	return "other"
}

func matchesBashFamilyRule(rule bashFamilyRule, command, text string) bool {
	for _, item := range rule.Commands {
		if command == item {
			return true
		}
	}
	for _, item := range rule.CommandContains {
		if strings.Contains(command, item) {
			return true
		}
	}
	for _, item := range rule.Contains {
		if strings.Contains(text, item) {
			return true
		}
	}
	return false
}
