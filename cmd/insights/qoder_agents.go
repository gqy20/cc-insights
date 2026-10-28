package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Qoder 的子代理 transcript 是与父会话分开的独立文件：
//
//	projects/<slug>/<parentSessionID>/subagents/agent-<agentID>.jsonl
//	projects/<slug>/<parentSessionID>/subagents/agent-<agentID>.meta.json
//
// 记录自带 agentId（ProjectRecord.AgentID 的 tag 就是 camelCase，归因不需要补）与父
// sessionId，所以 agent 桶和会话归属都是对的。缺的是**可读名字**：AgentID 是
// `aExplore-30c9817e20fe643f` 这种每次调用一个的哈希，诊断时想知道的是
// "Explore 子代理跑了多少次、失败率多少"。Claude 靠 `agent-name` 记录填 AgentName，
// Qoder 不写这种记录，同级 meta.json 的 agentType 是唯一来源。
// 模型也不缺：子代理记录自带的 message.model 就是 task json 里的 resolvedModel。
const qoderSubagentsDir = "subagents"

// parseQoderSessionFile 是 qoder provider 的解析入口：transcript 是 Claude 兼容
// schema，唯一需要额外处理的是把子代理的可读名字补上。
func parseQoderSessionFile(filePath string, tf TimeFilter, agg *ProjectAggregate) {
	name := qoderSubagentName(filePath)
	if name == "" {
		parseProjectFileAggregate(filePath, tf, agg)
		return
	}
	parseProjectFileAggregateWithAgentName(filePath, tf, agg, name)
}

// qoderSubagentName 返回子代理 transcript 的可读名字；非子代理文件返回空串。
func qoderSubagentName(filePath string) string {
	base := filepath.Base(filePath)
	if filepath.Base(filepath.Dir(filePath)) != qoderSubagentsDir ||
		!strings.HasSuffix(base, ".jsonl") {
		return ""
	}
	agentID := strings.TrimSuffix(strings.TrimPrefix(base, "agent-"), ".jsonl")
	if agentID == "" || agentID == base {
		return ""
	}
	if agentType := readQoderAgentType(filePath); agentType != "" {
		return agentType
	}
	return qoderAgentTypeFromID(agentID)
}

// readQoderAgentType 读同级 agent-<id>.meta.json 的 agentType。
// 文件可能不存在（旧版本或写入中断），返回空串让调用方退回文件名推导。
func readQoderAgentType(jsonlPath string) string {
	raw, err := os.ReadFile(strings.TrimSuffix(jsonlPath, ".jsonl") + ".meta.json")
	if err != nil {
		return ""
	}
	var meta struct {
		AgentType string `json:"agentType"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return ""
	}
	return strings.TrimSpace(meta.AgentType)
}

// qoderAgentTypeFromID 从 aExplore-30c9817e20fe643f 这类 agentId 反推 Explore。
// 类型名自身可含连字符（general-purpose），所以只切最后一段 hash，再去掉前缀 "a"。
func qoderAgentTypeFromID(agentID string) string {
	trimmed := strings.TrimSpace(agentID)
	if idx := strings.LastIndex(trimmed, "-"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimPrefix(trimmed, "a")
}

// applyQoderAgentName 把可读名字补到该文件触达的所有聚合层：顶层之外还包括三个
// 按天快照——时间过滤查询会从按天快照重建 agent 分析，只写顶层会让 -p 7d 与 all
// 给出不同的 agent 展示。
func applyQoderAgentName(agg *ProjectAggregate, agentName string, agentIDs map[string]bool) {
	if agg == nil || agentName == "" || len(agentIDs) == 0 {
		return
	}
	targets := []*ProjectAggregate{agg}
	for _, day := range agg.DailyRuntime {
		targets = append(targets, day)
	}
	for _, byProject := range agg.DailyProjectRuntime {
		for _, day := range byProject {
			targets = append(targets, day)
		}
	}
	for _, bySession := range agg.DailySessionRuntime {
		for _, day := range bySession {
			targets = append(targets, day)
		}
	}
	for _, target := range targets {
		for agentID := range agentIDs {
			if stat, ok := target.AgentStats[agentID]; ok && stat.AgentName == "" {
				stat.AgentName = agentName
			}
		}
	}
}
