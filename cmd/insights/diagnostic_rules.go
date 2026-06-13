package main

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed rules/diagnostics.yml
var embeddedDiagnosticRules embed.FS

type diagnosticRulesFile struct {
	Version int                    `yaml:"version"`
	Rules   []diagnosticRuleConfig `yaml:"rules"`
}

type diagnosticRuleConfig struct {
	ID        string `yaml:"id"`
	IDPrefix  string `yaml:"id_prefix"`
	Metric    string `yaml:"metric"`
	Threshold string `yaml:"threshold"`
	Source    string `yaml:"source"`
	Rationale string `yaml:"rationale"`
}

var (
	diagnosticRulesMu     sync.Mutex
	cachedDiagnosticRules *diagnosticRulesFile
)

func currentDiagnosticRules() (*diagnosticRulesFile, error) {
	diagnosticRulesMu.Lock()
	defer diagnosticRulesMu.Unlock()
	if cachedDiagnosticRules != nil {
		return cachedDiagnosticRules, nil
	}
	data, err := embeddedDiagnosticRules.ReadFile("rules/diagnostics.yml")
	if err != nil {
		return nil, fmt.Errorf("读取内置诊断规则失败: %w", err)
	}
	var file diagnosticRulesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("解析内置诊断规则失败: %w", err)
	}
	if file.Version <= 0 {
		return nil, fmt.Errorf("诊断规则版本无效")
	}
	cachedDiagnosticRules = &file
	return cachedDiagnosticRules, nil
}

func diagnosticRuleForID(id string) (diagnosticRuleConfig, bool) {
	rules, err := currentDiagnosticRules()
	if err != nil {
		return diagnosticRuleConfig{}, false
	}
	for _, rule := range rules.Rules {
		if rule.ID != "" && rule.ID == id {
			return rule, true
		}
	}
	for _, rule := range rules.Rules {
		if rule.IDPrefix != "" && strings.HasPrefix(id, rule.IDPrefix) {
			return rule, true
		}
	}
	return diagnosticRuleConfig{}, false
}
