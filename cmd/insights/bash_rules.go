package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	Name                   string   `yaml:"name"`
	Priority               int      `yaml:"priority"`
	Commands               []string `yaml:"commands"`
	CommandContains        []string `yaml:"command_contains"`
	Contains               []string `yaml:"contains"`
	CommandRegex           []string `yaml:"command_regex"`
	Regex                  []string `yaml:"regex"`
	ExcludeCommands        []string `yaml:"exclude_commands"`
	ExcludeCommandContains []string `yaml:"exclude_command_contains"`
	ExcludeContains        []string `yaml:"exclude_contains"`
	ExcludeCommandRegex    []string `yaml:"exclude_command_regex"`
	ExcludeRegex           []string `yaml:"exclude_regex"`

	order                  int
	compiledCommandRegex   []*regexp.Regexp
	compiledRegex          []*regexp.Regexp
	compiledExcludeCommand []*regexp.Regexp
	compiledExcludeRegex   []*regexp.Regexp
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

	for idx, family := range file.Families {
		name := strings.TrimSpace(family.Name)
		if name == "" {
			continue
		}
		rule := bashFamilyRule{
			Name:                   name,
			Priority:               family.Priority,
			Commands:               normalizeRuleList(family.Commands),
			CommandContains:        normalizeRuleList(family.CommandContains),
			Contains:               normalizeRuleList(family.Contains),
			CommandRegex:           normalizeRuleList(family.CommandRegex),
			Regex:                  normalizeRuleList(family.Regex),
			ExcludeCommands:        normalizeRuleList(family.ExcludeCommands),
			ExcludeCommandContains: normalizeRuleList(family.ExcludeCommandContains),
			ExcludeContains:        normalizeRuleList(family.ExcludeContains),
			ExcludeCommandRegex:    normalizeRuleList(family.ExcludeCommandRegex),
			ExcludeRegex:           normalizeRuleList(family.ExcludeRegex),
			order:                  idx,
		}
		if err := compileBashRuleRegex(&rule); err != nil {
			return nil, fmt.Errorf("解析 Bash 规则失败 (%s): %w", source, err)
		}
		rules.Families = append(rules.Families, rule)
	}
	sort.SliceStable(rules.Families, func(i, j int) bool {
		if rules.Families[i].Priority != rules.Families[j].Priority {
			return rules.Families[i].Priority > rules.Families[j].Priority
		}
		return rules.Families[i].order < rules.Families[j].order
	})

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

func compileBashRuleRegex(rule *bashFamilyRule) error {
	var err error
	if rule.compiledCommandRegex, err = compileRegexList(rule.CommandRegex); err != nil {
		return fmt.Errorf("%s.command_regex: %w", rule.Name, err)
	}
	if rule.compiledRegex, err = compileRegexList(rule.Regex); err != nil {
		return fmt.Errorf("%s.regex: %w", rule.Name, err)
	}
	if rule.compiledExcludeCommand, err = compileRegexList(rule.ExcludeCommandRegex); err != nil {
		return fmt.Errorf("%s.exclude_command_regex: %w", rule.Name, err)
	}
	if rule.compiledExcludeRegex, err = compileRegexList(rule.ExcludeRegex); err != nil {
		return fmt.Errorf("%s.exclude_regex: %w", rule.Name, err)
	}
	return nil
}

func compileRegexList(items []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(items))
	for _, item := range items {
		expr, err := regexp.Compile(item)
		if err != nil {
			return nil, err
		}
		out = append(out, expr)
	}
	return out, nil
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
	if excludedByBashFamilyRule(rule, command, text) {
		return false
	}
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
	for _, expr := range rule.compiledCommandRegex {
		if expr.MatchString(command) {
			return true
		}
	}
	for _, expr := range rule.compiledRegex {
		if expr.MatchString(text) {
			return true
		}
	}
	return false
}

func excludedByBashFamilyRule(rule bashFamilyRule, command, text string) bool {
	for _, item := range rule.ExcludeCommands {
		if command == item {
			return true
		}
	}
	for _, item := range rule.ExcludeCommandContains {
		if strings.Contains(command, item) {
			return true
		}
	}
	for _, item := range rule.ExcludeContains {
		if strings.Contains(text, item) {
			return true
		}
	}
	for _, expr := range rule.compiledExcludeCommand {
		if expr.MatchString(command) {
			return true
		}
	}
	for _, expr := range rule.compiledExcludeRegex {
		if expr.MatchString(text) {
			return true
		}
	}
	return false
}
