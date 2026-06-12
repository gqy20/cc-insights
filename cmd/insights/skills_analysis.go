package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var skillListingNameExpr = regexp.MustCompile(`(?m)^-\s*([A-Za-z0-9_.:+/-]+)`)

func ensureSkillUsageStat(agg *ProjectAggregate, name string) *SkillUsageStat {
	if name == "" {
		name = "unknown"
	}
	if agg.SkillUsageStats == nil {
		agg.SkillUsageStats = make(map[string]*SkillUsageStat)
	}
	if agg.SkillUsageStats[name] == nil {
		agg.SkillUsageStats[name] = &SkillUsageStat{Name: name}
	}
	return agg.SkillUsageStats[name]
}

func ensureSkillProjectStat(agg *ProjectAggregate, skillName, project string) *SkillProjectStat {
	key := skillName + "\x00" + project
	if agg.SkillProjectStats == nil {
		agg.SkillProjectStats = make(map[string]*SkillProjectStat)
	}
	if agg.SkillProjectStats[key] == nil {
		agg.SkillProjectStats[key] = &SkillProjectStat{SkillName: skillName, Project: project}
	}
	return agg.SkillProjectStats[key]
}

func ensureSkillModelStat(agg *ProjectAggregate, skillName, model string) *SkillModelStat {
	key := skillName + "\x00" + model
	if agg.SkillModelStats == nil {
		agg.SkillModelStats = make(map[string]*SkillModelStat)
	}
	if agg.SkillModelStats[key] == nil {
		agg.SkillModelStats[key] = &SkillModelStat{SkillName: skillName, Model: model}
	}
	return agg.SkillModelStats[key]
}

func ensureSkillAgentStat(agg *ProjectAggregate, skillName, agentID string, isSidechain bool) *SkillAgentStat {
	key := skillName + "\x00" + agentID + "\x00" + strconvBool(isSidechain)
	if agg.SkillAgentStats == nil {
		agg.SkillAgentStats = make(map[string]*SkillAgentStat)
	}
	if agg.SkillAgentStats[key] == nil {
		agg.SkillAgentStats[key] = &SkillAgentStat{SkillName: skillName, AgentID: agentID, IsSidechain: isSidechain}
	}
	return agg.SkillAgentStats[key]
}

func ensureSkillToolChainStat(agg *ProjectAggregate, skillName, tool string) *SkillToolChainStat {
	key := skillName + "\x00" + tool
	if agg.SkillToolChainStats == nil {
		agg.SkillToolChainStats = make(map[string]*SkillToolChainStat)
	}
	if agg.SkillToolChainStats[key] == nil {
		agg.SkillToolChainStats[key] = &SkillToolChainStat{SkillName: skillName, Tool: tool}
	}
	return agg.SkillToolChainStats[key]
}

func mergeSkillUsageStat(dst, src *SkillUsageStat) {
	if dst == nil || src == nil {
		return
	}
	dst.InvocationCount += src.InvocationCount
	dst.ToolUseCount += src.ToolUseCount
	dst.AttachmentCount += src.AttachmentCount
	dst.SuccessCount += src.SuccessCount
	dst.FailureCount += src.FailureCount
	dst.MissingResultCount += src.MissingResultCount
	dst.ArgsTotalLength += src.ArgsTotalLength
	if src.ArgsMaxLength > dst.ArgsMaxLength {
		dst.ArgsMaxLength = src.ArgsMaxLength
	}
	dst.SeenInListingCount += src.SeenInListingCount
	dst.DynamicCount += src.DynamicCount
	dst.Installed = dst.Installed || src.Installed
	if dst.Path == "" {
		dst.Path = src.Path
	}
}

func mergeSkillProjectStat(dst, src *SkillProjectStat) {
	if dst == nil || src == nil {
		return
	}
	dst.InvocationCount += src.InvocationCount
	dst.FailureCount += src.FailureCount
	dst.MissingResults += src.MissingResults
}

func mergeSkillModelStat(dst, src *SkillModelStat) {
	if dst == nil || src == nil {
		return
	}
	dst.InvocationCount += src.InvocationCount
	dst.FailureCount += src.FailureCount
	dst.MissingResults += src.MissingResults
}

func recordInstalledSkillsLocked(agg *ProjectAggregate, dataDir string) {
	if agg == nil {
		return
	}
	items := scanInstalledSkillsFromDir(dataDir)
	for _, item := range items {
		if agg.InstalledSkills == nil {
			agg.InstalledSkills = make(map[string]*InstalledSkillItem)
		}
		copyItem := item
		agg.InstalledSkills[item.Name] = &copyItem
		stat := ensureSkillUsageStat(agg, item.Name)
		stat.Path = item.Path
		stat.Installed = true
	}
}

func recordSkillListingLocked(agg *ProjectAggregate, attachment json.RawMessage) {
	if agg == nil || len(attachment) == 0 {
		return
	}
	var payload struct {
		Type       string   `json:"type"`
		Content    string   `json:"content"`
		Names      []string `json:"names"`
		SkillCount int      `json:"skillCount"`
		IsInitial  bool     `json:"isInitial"`
	}
	if err := json.Unmarshal(attachment, &payload); err != nil {
		return
	}
	agg.SkillListingEvents++
	if payload.IsInitial {
		agg.SkillInitialListings++
	}
	names := payload.Names
	if len(names) == 0 && payload.Content != "" {
		names = parseSkillListingNames(payload.Content)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		agg.SkillListingStats[name]++
		stat := ensureSkillUsageStat(agg, name)
		stat.SeenInListingCount++
	}
}

func recordDynamicSkillLocked(agg *ProjectAggregate, attachment json.RawMessage) {
	if agg == nil || len(attachment) == 0 {
		return
	}
	var payload struct {
		Name  string `json:"name"`
		Skill string `json:"skill"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(attachment, &payload); err != nil {
		return
	}
	name := payload.Name
	if name == "" {
		name = payload.Skill
	}
	if name == "" {
		return
	}
	agg.DynamicSkillEvents++
	stat := ensureSkillUsageStat(agg, name)
	stat.DynamicCount++
	if stat.Path == "" {
		stat.Path = payload.Path
	}
}

func recordSkillToolUseLocked(agg *ProjectAggregate, call pendingToolCall, skillName string, argsLen int) {
	stat := ensureSkillUsageStat(agg, skillName)
	stat.InvocationCount++
	stat.ToolUseCount++
	stat.ArgsTotalLength += argsLen
	if argsLen > stat.ArgsMaxLength {
		stat.ArgsMaxLength = argsLen
	}
	if call.Project != "" {
		projectStat := ensureSkillProjectStat(agg, skillName, call.Project)
		projectStat.InvocationCount++
	}
	if call.Model != "" {
		modelStat := ensureSkillModelStat(agg, skillName, call.Model)
		modelStat.InvocationCount++
	}
	agentStat := ensureSkillAgentStat(agg, skillName, call.AgentID, call.IsSidechain)
	agentStat.InvocationCount++
}

func recordSkillAttachmentInvocationLocked(agg *ProjectAggregate, skillName, path, project, model, agentID string, isSidechain bool) {
	stat := ensureSkillUsageStat(agg, skillName)
	stat.InvocationCount++
	stat.AttachmentCount++
	if path != "" && stat.Path == "" {
		stat.Path = path
	}
	if project != "" {
		projectStat := ensureSkillProjectStat(agg, skillName, project)
		projectStat.InvocationCount++
	}
	if model != "" {
		modelStat := ensureSkillModelStat(agg, skillName, model)
		modelStat.InvocationCount++
	}
	agentStat := ensureSkillAgentStat(agg, skillName, agentID, isSidechain)
	agentStat.InvocationCount++
}

func recordSkillToolResultLocked(agg *ProjectAggregate, call pendingToolCall, failed, missing bool) {
	if call.SkillName == "" {
		return
	}
	stat := ensureSkillUsageStat(agg, call.SkillName)
	if failed {
		stat.FailureCount++
	}
	if missing {
		stat.MissingResultCount++
	}
	if !failed && !missing {
		stat.SuccessCount++
	}
	if call.Project != "" {
		projectStat := ensureSkillProjectStat(agg, call.SkillName, call.Project)
		if failed {
			projectStat.FailureCount++
		}
		if missing {
			projectStat.MissingResults++
		}
	}
	if call.Model != "" {
		modelStat := ensureSkillModelStat(agg, call.SkillName, call.Model)
		if failed {
			modelStat.FailureCount++
		}
		if missing {
			modelStat.MissingResults++
		}
	}
}

func recordSkillToolChainLocked(agg *ProjectAggregate, skillName, tool string, failed, missing bool) {
	if skillName == "" || tool == "" || tool == "Skill" {
		return
	}
	stat := ensureSkillToolChainStat(agg, skillName, tool)
	stat.CallCount++
	if failed {
		stat.FailureCount++
	}
	if missing {
		stat.MissingResults++
	}
}

func recordSkillToolChainResultLocked(agg *ProjectAggregate, call pendingToolCall, failed, missing bool) {
	if len(call.ChainSkills) == 0 || call.Tool == "" || call.Tool == "Skill" {
		return
	}
	for _, skillName := range call.ChainSkills {
		if skillName == "" {
			continue
		}
		stat := ensureSkillToolChainStat(agg, skillName, call.Tool)
		if failed {
			stat.FailureCount++
		}
		if missing {
			stat.MissingResults++
		}
	}
}

func finalizeSkillAnalysisLocked(agg *ProjectAggregate) {
	if agg == nil {
		return
	}
	analysis := &SkillAnalysisData{
		TotalInstalled:        len(agg.InstalledSkills),
		TotalInvocations:      0,
		ToolUseInvocations:    0,
		AttachmentInvocations: 0,
		ListingEvents:         agg.SkillListingEvents,
		InitialListingEvents:  agg.SkillInitialListings,
		DynamicSkillEvents:    agg.DynamicSkillEvents,
		Installed:             make([]InstalledSkillItem, 0, len(agg.InstalledSkills)),
		Skills:                make([]SkillUsageStat, 0, len(agg.SkillUsageStats)),
		ListingSkills:         make([]SkillListingStat, 0, len(agg.SkillListingStats)),
		ByProject:             make([]SkillProjectStat, 0, len(agg.SkillProjectStats)),
		ByModel:               make([]SkillModelStat, 0, len(agg.SkillModelStats)),
		ByAgent:               make([]SkillAgentStat, 0, len(agg.SkillAgentStats)),
		ToolChains:            make([]SkillToolChainStat, 0, len(agg.SkillToolChainStats)),
	}
	for _, item := range agg.InstalledSkills {
		if item == nil {
			continue
		}
		analysis.Installed = append(analysis.Installed, *item)
	}
	sort.Slice(analysis.Installed, func(i, j int) bool {
		return analysis.Installed[i].Name < analysis.Installed[j].Name
	})
	for _, stat := range agg.SkillUsageStats {
		if stat == nil {
			continue
		}
		copyStat := *stat
		copyStat.InvocationCount = copyStat.ToolUseCount + copyStat.AttachmentCount
		copyStat.ArgsAvgLength = 0
		if copyStat.ToolUseCount > 0 {
			copyStat.ArgsAvgLength = float64(copyStat.ArgsTotalLength) / float64(copyStat.ToolUseCount)
		}
		if copyStat.InvocationCount > 0 {
			copyStat.FailureRate = float64(copyStat.FailureCount) / float64(copyStat.InvocationCount) * 100
		}
		analysis.TotalInvocations += copyStat.InvocationCount
		analysis.ToolUseInvocations += copyStat.ToolUseCount
		analysis.AttachmentInvocations += copyStat.AttachmentCount
		analysis.FailureCount += copyStat.FailureCount
		analysis.MissingResults += copyStat.MissingResultCount
		analysis.Skills = append(analysis.Skills, copyStat)
	}
	sort.Slice(analysis.Skills, func(i, j int) bool {
		if analysis.Skills[i].InvocationCount == analysis.Skills[j].InvocationCount {
			return analysis.Skills[i].Name < analysis.Skills[j].Name
		}
		return analysis.Skills[i].InvocationCount > analysis.Skills[j].InvocationCount
	})
	for name, count := range agg.SkillListingStats {
		analysis.ListingSkills = append(analysis.ListingSkills, SkillListingStat{Name: name, Count: count})
	}
	sort.Slice(analysis.ListingSkills, func(i, j int) bool {
		if analysis.ListingSkills[i].Count == analysis.ListingSkills[j].Count {
			return analysis.ListingSkills[i].Name < analysis.ListingSkills[j].Name
		}
		return analysis.ListingSkills[i].Count > analysis.ListingSkills[j].Count
	})
	for _, stat := range agg.SkillProjectStats {
		if stat == nil {
			continue
		}
		copyStat := *stat
		if copyStat.InvocationCount > 0 {
			copyStat.FailureRate = float64(copyStat.FailureCount+copyStat.MissingResults) / float64(copyStat.InvocationCount) * 100
		}
		analysis.ByProject = append(analysis.ByProject, copyStat)
	}
	sort.Slice(analysis.ByProject, func(i, j int) bool {
		if analysis.ByProject[i].InvocationCount == analysis.ByProject[j].InvocationCount {
			if analysis.ByProject[i].SkillName == analysis.ByProject[j].SkillName {
				return analysis.ByProject[i].Project < analysis.ByProject[j].Project
			}
			return analysis.ByProject[i].SkillName < analysis.ByProject[j].SkillName
		}
		return analysis.ByProject[i].InvocationCount > analysis.ByProject[j].InvocationCount
	})
	for _, stat := range agg.SkillModelStats {
		if stat == nil {
			continue
		}
		copyStat := *stat
		if copyStat.InvocationCount > 0 {
			copyStat.FailureRate = float64(copyStat.FailureCount+copyStat.MissingResults) / float64(copyStat.InvocationCount) * 100
		}
		analysis.ByModel = append(analysis.ByModel, copyStat)
	}
	sort.Slice(analysis.ByModel, func(i, j int) bool {
		if analysis.ByModel[i].InvocationCount == analysis.ByModel[j].InvocationCount {
			if analysis.ByModel[i].SkillName == analysis.ByModel[j].SkillName {
				return analysis.ByModel[i].Model < analysis.ByModel[j].Model
			}
			return analysis.ByModel[i].SkillName < analysis.ByModel[j].SkillName
		}
		return analysis.ByModel[i].InvocationCount > analysis.ByModel[j].InvocationCount
	})
	for _, stat := range agg.SkillAgentStats {
		if stat == nil {
			continue
		}
		analysis.ByAgent = append(analysis.ByAgent, *stat)
	}
	sort.Slice(analysis.ByAgent, func(i, j int) bool {
		if analysis.ByAgent[i].InvocationCount == analysis.ByAgent[j].InvocationCount {
			if analysis.ByAgent[i].SkillName == analysis.ByAgent[j].SkillName {
				return analysis.ByAgent[i].AgentID < analysis.ByAgent[j].AgentID
			}
			return analysis.ByAgent[i].SkillName < analysis.ByAgent[j].SkillName
		}
		return analysis.ByAgent[i].InvocationCount > analysis.ByAgent[j].InvocationCount
	})
	for _, stat := range agg.SkillToolChainStats {
		if stat == nil {
			continue
		}
		copyStat := *stat
		if copyStat.CallCount > 0 {
			copyStat.FailureRate = float64(copyStat.FailureCount+copyStat.MissingResults) / float64(copyStat.CallCount) * 100
		}
		analysis.ToolChains = append(analysis.ToolChains, copyStat)
	}
	sort.Slice(analysis.ToolChains, func(i, j int) bool {
		if analysis.ToolChains[i].CallCount == analysis.ToolChains[j].CallCount {
			if analysis.ToolChains[i].SkillName == analysis.ToolChains[j].SkillName {
				return analysis.ToolChains[i].Tool < analysis.ToolChains[j].Tool
			}
			return analysis.ToolChains[i].SkillName < analysis.ToolChains[j].SkillName
		}
		return analysis.ToolChains[i].CallCount > analysis.ToolChains[j].CallCount
	})
	agg.SkillAnalysis = analysis
}

func parseSkillListingNames(content string) []string {
	matches := skillListingNameExpr.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		name := strings.TrimSpace(match[1])
		if name == "" || name == "note" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func extractSkillInvocation(input json.RawMessage) (string, int) {
	if len(input) == 0 {
		return "", 0
	}
	var payload struct {
		Skill string `json:"skill"`
		Name  string `json:"name"`
		Args  string `json:"args"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return "", 0
	}
	name := strings.TrimSpace(payload.Skill)
	if name == "" {
		name = strings.TrimSpace(payload.Name)
	}
	return name, len(payload.Args)
}

func extractAttachmentSkillSignals(attachment json.RawMessage) ([]string, string) {
	if len(attachment) == 0 {
		return nil, ""
	}
	var payload struct {
		Type    string   `json:"type"`
		Content string   `json:"content"`
		Names   []string `json:"names"`
		Skills  []struct {
			Name string `json:"name"`
		} `json:"skills"`
		Name  string `json:"name"`
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal(attachment, &payload); err != nil {
		return nil, ""
	}
	switch payload.Type {
	case "invoked_skills":
		names := make([]string, 0, len(payload.Skills))
		for _, skill := range payload.Skills {
			if skill.Name != "" {
				names = append(names, skill.Name)
			}
		}
		return names, payload.Type
	case "dynamic_skill":
		name := payload.Name
		if name == "" {
			name = payload.Skill
		}
		if name == "" {
			return nil, payload.Type
		}
		return []string{name}, payload.Type
	case "skill_listing":
		if len(payload.Names) > 0 {
			return payload.Names, payload.Type
		}
		return parseSkillListingNames(payload.Content), payload.Type
	default:
		return nil, payload.Type
	}
}

func appendUniqueStrings(values []string, more ...string) []string {
	seen := make(map[string]bool, len(values)+len(more))
	out := make([]string, 0, len(values)+len(more))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, value := range more {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func scanInstalledSkillsFromDir(dataDir string) []InstalledSkillItem {
	skillsDir := filepath.Join(dataDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	items := make([]InstalledSkillItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		item := InstalledSkillItem{Name: entry.Name(), Path: filepath.ToSlash(filepath.Join("skills", entry.Name()))}
		skillDir := filepath.Join(skillsDir, entry.Name())
		files, err := os.ReadDir(skillDir)
		if err == nil {
			item.FileCount = len(files)
			for _, file := range files {
				if strings.EqualFold(file.Name(), "SKILL.md") {
					item.HasSkillMD = true
				}
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func strconvBool(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
