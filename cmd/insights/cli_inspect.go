package main

import (
	"fmt"
	"sort"
	"strings"
)

type nameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type cliInspectFailureFilter struct {
	Reason   string `json:"reason,omitempty"`
	Category string `json:"category,omitempty"`
	Tool     string `json:"tool,omitempty"`
	Model    string `json:"model,omitempty"`
	Project  string `json:"project,omitempty"`
	Session  string `json:"session,omitempty"`
}

type cliInspectFailureSummary struct {
	AvailableSamples int         `json:"available_samples"`
	MatchedSamples   int         `json:"matched_samples"`
	TopTools         []nameCount `json:"top_tools"`
	TopProjects      []nameCount `json:"top_projects"`
	TopModels        []nameCount `json:"top_models"`
	TopReasons       []nameCount `json:"top_reasons"`
}

type cliInspectFailuresReport struct {
	TimeRange TimeRangeInfo            `json:"time_range"`
	Filter    cliInspectFailureFilter  `json:"filter"`
	Summary   cliInspectFailureSummary `json:"summary"`
	Samples   []ToolFailureSample      `json:"samples"`
	Insights  []string                 `json:"insights"`
}

func buildCLIInspectFailuresReport(data *DashboardData, opts cliOptions) cliInspectFailuresReport {
	report := cliInspectFailuresReport{
		TimeRange: data.TimeRange,
		Filter: cliInspectFailureFilter{
			Reason:   opts.Reason,
			Category: opts.Category,
			Tool:     opts.Tool,
			Model:    opts.Model,
			Project:  opts.Project,
			Session:  opts.Session,
		},
	}
	if data.FailureAnalysis == nil {
		return report
	}

	samples := data.FailureAnalysis.Samples
	report.Summary.AvailableSamples = len(samples)

	matched := make([]ToolFailureSample, 0, len(samples))
	for _, sample := range samples {
		if matchFailureSample(sample, report.Filter) {
			matched = append(matched, sample)
		}
	}
	report.Summary.MatchedSamples = len(matched)
	report.Summary.TopTools = topSampleCounts(matched, opts.Limit, func(sample ToolFailureSample) string { return sample.Tool })
	report.Summary.TopProjects = topSampleCounts(matched, opts.Limit, func(sample ToolFailureSample) string { return sample.Project })
	report.Summary.TopModels = topSampleCounts(matched, opts.Limit, func(sample ToolFailureSample) string { return sample.Model })
	report.Summary.TopReasons = topSampleCounts(matched, opts.Limit, func(sample ToolFailureSample) string {
		if sample.Category == "" && sample.Reason == "" {
			return sample.Kind
		}
		return sample.Category + "/" + sample.Reason
	})

	sampleLimit := opts.Samples
	if sampleLimit > len(matched) {
		sampleLimit = len(matched)
	}
	report.Samples = sanitizeFailureSamples(matched[:sampleLimit])
	report.Insights = buildInspectFailureInsights(report)
	return report
}

func matchFailureSample(sample ToolFailureSample, filter cliInspectFailureFilter) bool {
	return matchEqual(filter.Reason, sample.Reason) &&
		matchEqual(filter.Category, sample.Category) &&
		matchEqual(filter.Tool, sample.Tool) &&
		matchEqual(filter.Model, sample.Model) &&
		matchContains(filter.Project, sample.Project) &&
		matchEqual(filter.Session, sample.SessionID)
}

func matchEqual(filter, value string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	return strings.EqualFold(filter, strings.TrimSpace(value))
}

func matchContains(filter, value string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(filter))
}

func topSampleCounts(samples []ToolFailureSample, limit int, keyFn func(ToolFailureSample) string) []nameCount {
	counts := make(map[string]int)
	for _, sample := range samples {
		key := strings.TrimSpace(keyFn(sample))
		if key == "" {
			key = "unknown"
		}
		counts[key]++
	}
	items := make([]nameCount, 0, len(counts))
	for name, count := range counts {
		items = append(items, nameCount{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func buildInspectFailureInsights(report cliInspectFailuresReport) []string {
	if report.Summary.MatchedSamples == 0 {
		return []string{"当前过滤条件没有匹配到缓存中的失败样例。"}
	}
	insights := []string{
		fmt.Sprintf("匹配到 %s/%s 个缓存失败样例。", formatInt(report.Summary.MatchedSamples), formatInt(report.Summary.AvailableSamples)),
	}
	if len(report.Summary.TopTools) > 0 {
		top := report.Summary.TopTools[0]
		insights = append(insights, fmt.Sprintf("样例中最常见工具是 %s，共 %s 次。", top.Name, formatInt(top.Count)))
	}
	if len(report.Summary.TopProjects) > 0 {
		top := report.Summary.TopProjects[0]
		insights = append(insights, fmt.Sprintf("样例中最集中项目是 %s，共 %s 次。", top.Name, formatInt(top.Count)))
	}
	return insights
}

func sanitizeFailureSamples(samples []ToolFailureSample) []ToolFailureSample {
	clean := make([]ToolFailureSample, 0, len(samples))
	for _, sample := range samples {
		preview := strings.ToValidUTF8(sample.ContentPreview, "")
		preview = strings.ReplaceAll(preview, "�", "")
		sample.ContentPreview = strings.Join(strings.Fields(preview), " ")
		clean = append(clean, sample)
	}
	return clean
}
