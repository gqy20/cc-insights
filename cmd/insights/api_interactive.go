package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type apiMeta struct {
	Source       string                 `json:"source"`
	CacheVersion string                 `json:"cache_version,omitempty"`
	GeneratedAt  string                 `json:"generated_at"`
	RuntimeMs    int64                  `json:"runtime_ms"`
	TimeRange    TimeRangeInfo          `json:"time_range"`
	Filters      map[string]interface{} `json:"filters,omitempty"`
}

type interactiveAPIResponse struct {
	Success bool        `json:"success"`
	Meta    apiMeta     `json:"meta"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type AnalysisFilter struct {
	TimeFilter TimeFilter
	Preset     string
	Start      string
	End        string
	Limit      int
	Samples    int
	Detail     bool
	ID         string
	Project    string
	Session    string
	Tool       string
	Model      string
	Category   string
	Reason     string
	Severity   string
	Target     string
	Family     string
}

type overviewData struct {
	Summary     overviewSummary `json:"summary"`
	Trend       overviewTrend   `json:"trend"`
	Top         overviewTop     `json:"top"`
	Diagnostics overviewDiag    `json:"diagnostics"`
}

type overviewSummary struct {
	Messages      int     `json:"messages"`
	Sessions      int     `json:"sessions"`
	Commands      int     `json:"commands"`
	ToolCalls     int     `json:"tool_calls"`
	Failures      int     `json:"failures"`
	FailureRate   float64 `json:"failure_rate"`
	Tokens        int     `json:"tokens"`
	Projects      int     `json:"projects"`
	ModelCount    int     `json:"model_count"`
	SlowestCallMs int64   `json:"slowest_call_ms,omitempty"`
}

type overviewTrend struct {
	Dates    []string `json:"dates"`
	Messages []int    `json:"messages"`
	Sessions []int    `json:"sessions"`
	Tokens   []int    `json:"tokens"`
	Failures []int    `json:"failures"`
}

type overviewTop struct {
	Projects       []ProjectStatItem     `json:"projects,omitempty"`
	Models         []ModelUsageItem      `json:"models,omitempty"`
	Tools          []ToolStatItem        `json:"tools,omitempty"`
	FailureReasons []FailureReasonStat   `json:"failure_reasons,omitempty"`
	Sessions       []SessionAnalysisItem `json:"sessions,omitempty"`
}

type overviewDiag struct {
	Total int                 `json:"total"`
	Top   []diagnosticFinding `json:"top,omitempty"`
}

type timelineData struct {
	Start string        `json:"start,omitempty"`
	End   string        `json:"end,omitempty"`
	Days  []timelineDay `json:"days"`
}

type timelineDay struct {
	Date      string `json:"date"`
	Messages  int    `json:"messages"`
	Sessions  int    `json:"sessions"`
	ToolCalls int    `json:"tool_calls"`
	Tokens    int    `json:"tokens"`
	Failures  int    `json:"failures"`
}

type toolsDetailReport struct {
	TimeRange       TimeRangeInfo          `json:"time_range"`
	ToolAnalysis    *ToolAnalysisData      `json:"tool_analysis,omitempty"`
	ToolPerformance *ToolPerformanceData   `json:"tool_performance,omitempty"`
	ByCategory      []ToolPerfCategoryItem `json:"by_category,omitempty"`
	SlowestCalls    []ToolSlowCallItem     `json:"slowest_calls,omitempty"`
	Insights        []string               `json:"insights"`
}

func handleOverviewAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildDashboardDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	diagnostics := buildCLIRecommendationReport(data, cliOptions{Limit: 3})
	payload := buildOverviewData(data, diagnostics, filter)
	sendInteractiveJSON(w, payload, source, data.TimeRange, filter, startedAt)
}

func handleDiagnosticsAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildRecommendationDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report := buildCLIRecommendationReport(data, filter.toCLIOptions())
	report.Recommendations = filterDiagnosticFindings(report.Recommendations, filter)
	report.TotalFindings = len(report.Recommendations)
	report.ByCategory = countFindingsByCategory(report.Recommendations)
	report.Insights = buildRecommendationInsights(report)
	sendInteractiveJSON(w, report, source, data.TimeRange, filter, startedAt)
}

func handleDetailFailuresAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildRecommendationDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report := buildCLIInspectFailuresReport(data, filter.toCLIOptions())
	sendInteractiveJSON(w, report, source, data.TimeRange, filter, startedAt)
}

func handleDetailCommandsAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildRecommendationDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report := buildCLICommandReport(data, filter.Limit)
	if filter.Family != "" {
		report.ByFamily = filterCommandFamilies(report.ByFamily, filter.Family)
	}
	sendInteractiveJSON(w, report, source, data.TimeRange, filter, startedAt)
}

func handleDetailTokensAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildRecommendationDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report := buildCLICostReport(data, filter.Limit)
	report.ByProject = filterCostProjects(report.ByProject, filter.Project)
	report.BySession = filterCostSessions(report.BySession, filter.Session, filter.Project)
	report.ByModel = filterCostModels(report.ByModel, filter.Model)
	sendInteractiveJSON(w, report, source, data.TimeRange, filter, startedAt)
}

func handleDetailSessionsAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildRecommendationDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report := buildCLISessionReport(data, filter.toCLIOptions())
	sendInteractiveJSON(w, report, source, data.TimeRange, filter, startedAt)
}

func handleDetailToolsAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildRecommendationDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report := buildToolsDetailReport(data, filter)
	sendInteractiveJSON(w, report, source, data.TimeRange, filter, startedAt)
}

func handleTimelineAPI(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalysisFilter(r)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusBadRequest)
		return
	}
	startedAt := time.Now()
	data, source, err := buildDashboardDataWithFilter(filter)
	if err != nil {
		sendInteractiveError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload := buildTimelineData(data)
	sendInteractiveJSON(w, payload, source, data.TimeRange, filter, startedAt)
}

func buildRecommendationDataWithFilter(filter AnalysisFilter) (*DashboardData, string, error) {
	data, source, err := buildRecommendationDashboardData(filter.TimeFilter, filter.Preset)
	if err != nil {
		return nil, source, err
	}
	applyDashboardFilter(data, filter)
	return data, source, nil
}

func parseAnalysisFilter(r *http.Request) (AnalysisFilter, error) {
	q := r.URL.Query()
	preset := strings.TrimSpace(q.Get("preset"))
	start := strings.TrimSpace(q.Get("start"))
	end := strings.TrimSpace(q.Get("end"))
	if preset == "" && start == "" && end == "" {
		preset = "30d"
	}
	opts := cliOptions{
		Config:   cfg,
		Preset:   preset,
		Start:    start,
		End:      end,
		Limit:    parsePositiveInt(q.Get("limit"), 10),
		Samples:  parsePositiveInt(q.Get("samples"), parsePositiveInt(q.Get("limit"), 10)),
		ID:       strings.TrimSpace(q.Get("id")),
		Detail:   parseBoolQuery(q.Get("detail")),
		Project:  strings.TrimSpace(q.Get("project")),
		Session:  strings.TrimSpace(q.Get("session")),
		Tool:     strings.TrimSpace(q.Get("tool")),
		Model:    strings.TrimSpace(q.Get("model")),
		Category: strings.TrimSpace(q.Get("category")),
		Reason:   strings.TrimSpace(q.Get("reason")),
	}
	tf, normalizedPreset, err := timeFilterFromCLIOptions(opts)
	if err != nil {
		return AnalysisFilter{}, err
	}
	return AnalysisFilter{
		TimeFilter: tf,
		Preset:     normalizedPreset,
		Start:      start,
		End:        end,
		Limit:      opts.Limit,
		Samples:    opts.Samples,
		ID:         opts.ID,
		Detail:     opts.Detail,
		Project:    opts.Project,
		Session:    opts.Session,
		Tool:       opts.Tool,
		Model:      opts.Model,
		Category:   opts.Category,
		Reason:     opts.Reason,
		Severity:   strings.TrimSpace(q.Get("severity")),
		Target:     strings.TrimSpace(q.Get("target")),
		Family:     strings.TrimSpace(q.Get("family")),
	}, nil
}

func (filter AnalysisFilter) toCLIOptions() cliOptions {
	return cliOptions{
		Config:   cfg,
		Preset:   filter.Preset,
		Start:    filter.Start,
		End:      filter.End,
		Limit:    filter.Limit,
		Samples:  filter.Samples,
		ID:       filter.ID,
		Detail:   filter.Detail,
		Project:  filter.Project,
		Session:  filter.Session,
		Tool:     filter.Tool,
		Model:    filter.Model,
		Category: filter.Category,
		Reason:   filter.Reason,
	}
}

func buildOverviewData(data *DashboardData, diagnostics cliRecommendationReport, filter AnalysisFilter) overviewData {
	summary := overviewSummary{}
	if data.ProjectStats != nil {
		summary.Messages = data.ProjectStats.TotalMessages
		summary.Sessions = data.ProjectStats.TotalSessions
		summary.Projects = len(data.ProjectStats.Projects)
	}
	if data.CostAnalysis != nil {
		summary.Tokens = data.CostAnalysis.Totals.TotalTokens
	}
	if data.ToolAnalysis != nil {
		summary.ToolCalls = data.ToolAnalysis.TotalCalls
		summary.Failures = data.ToolAnalysis.TotalFailures
		if data.ToolAnalysis.TotalCalls > 0 {
			summary.FailureRate = float64(data.ToolAnalysis.TotalFailures) / float64(data.ToolAnalysis.TotalCalls) * 100
		}
	}
	if data.CommandAnalysis != nil {
		for _, item := range data.CommandAnalysis.BashCommands {
			summary.Commands += item.CallCount
		}
	}
	if data.ToolPerformance != nil && len(data.ToolPerformance.SlowestCalls) > 0 {
		summary.SlowestCallMs = data.ToolPerformance.SlowestCalls[0].DurationMs
	}
	summary.ModelCount = len(data.ModelUsage)

	trend := overviewTrend{Dates: append([]string(nil), data.DailyTrend.Dates...), Messages: append([]int(nil), data.DailyTrend.Counts...)}
	if data.Sessions != nil {
		for _, date := range trend.Dates {
			trend.Sessions = append(trend.Sessions, data.Sessions.DailySessionMap[date])
		}
	}
	if globalCache != nil {
		for _, date := range trend.Dates {
			day := globalCache.DailyStats[date]
			if day == nil {
				trend.Tokens = append(trend.Tokens, 0)
				trend.Failures = append(trend.Failures, 0)
				continue
			}
			trend.Tokens = append(trend.Tokens, sumIntMap(day.ModelTokens))
			trend.Failures = append(trend.Failures, 0)
		}
	}

	top := overviewTop{}
	if data.ProjectStats != nil {
		top.Projects = limitProjectStats(data.ProjectStats.Projects, filter.Limit)
	}
	top.Models = limitModelUsage(data.ModelUsage, filter.Limit)
	if data.ToolAnalysis != nil {
		top.Tools = limitToolStatItems(data.ToolAnalysis.Tools, filter.Limit)
	}
	if data.FailureAnalysis != nil {
		top.FailureReasons = limitFailureReasons(data.FailureAnalysis.ByReason, filter.Limit)
	}
	if data.SessionAnalysis != nil {
		top.Sessions = limitSessionItems(data.SessionAnalysis.LongRunning, filter.Limit)
	}
	return overviewData{
		Summary: summary,
		Trend:   trend,
		Top:     top,
		Diagnostics: overviewDiag{
			Total: diagnostics.TotalFindings,
			Top:   diagnostics.Recommendations,
		},
	}
}

func buildTimelineData(data *DashboardData) timelineData {
	out := timelineData{Days: make([]timelineDay, 0, len(data.DailyTrend.Dates))}
	for _, date := range data.DailyTrend.Dates {
		day := timelineDay{Date: date}
		if globalCache != nil && globalCache.DailyStats[date] != nil {
			stats := globalCache.DailyStats[date]
			day.Messages = stats.MessageCount
			day.Sessions = stats.SessionCount
			day.ToolCalls = stats.ToolCallCount
			day.Tokens = sumIntMap(stats.ModelTokens)
		} else {
			day.Messages = dailyTrendCount(data.DailyTrend, date)
			if data.Sessions != nil {
				day.Sessions = data.Sessions.DailySessionMap[date]
			}
		}
		out.Days = append(out.Days, day)
	}
	if len(out.Days) > 0 {
		out.Start = out.Days[0].Date
		out.End = out.Days[len(out.Days)-1].Date
	}
	return out
}

func buildToolsDetailReport(data *DashboardData, filter AnalysisFilter) toolsDetailReport {
	report := toolsDetailReport{TimeRange: data.TimeRange, ToolAnalysis: cloneToolAnalysis(data.ToolAnalysis), ToolPerformance: cloneToolPerformance(data.ToolPerformance)}
	if data.ToolPerformance != nil {
		for _, item := range data.ToolPerformance.ByCategory {
			if filter.Tool == "" || strings.EqualFold(item.BaseTool, filter.Tool) || strings.Contains(strings.ToLower(item.Category), strings.ToLower(filter.Tool)) {
				report.ByCategory = append(report.ByCategory, item)
			}
			if len(report.ByCategory) >= filter.Limit {
				break
			}
		}
		for _, item := range data.ToolPerformance.SlowestCalls {
			if filter.Tool == "" || strings.EqualFold(item.Tool, filter.Tool) || strings.Contains(strings.ToLower(item.Category), strings.ToLower(filter.Tool)) {
				report.SlowestCalls = append(report.SlowestCalls, item)
			}
			if len(report.SlowestCalls) >= filter.Limit {
				break
			}
		}
	}
	if len(report.ByCategory) > 0 {
		report.Insights = append(report.Insights, "工具性能详情已按当前过滤条件返回。")
	}
	return report
}

func filterDiagnosticFindings(items []diagnosticFinding, filter AnalysisFilter) []diagnosticFinding {
	filtered := make([]diagnosticFinding, 0, len(items))
	for _, item := range items {
		if filter.Severity != "" && !strings.EqualFold(filter.Severity, item.Severity) {
			continue
		}
		if filter.Target != "" && !containsStringFold(item.Targets, filter.Target) {
			continue
		}
		if filter.Project != "" && !findingContains(item, filter.Project) {
			continue
		}
		if filter.Session != "" && !findingContains(item, filter.Session) {
			continue
		}
		if filter.Tool != "" && !findingContains(item, filter.Tool) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func sendInteractiveJSON(w http.ResponseWriter, data interface{}, source string, tr TimeRangeInfo, filter AnalysisFilter, startedAt time.Time) {
	sendJSON(w, interactiveAPIResponse{
		Success: true,
		Meta: apiMeta{
			Source:       source,
			CacheVersion: cacheVersionForMeta(),
			GeneratedAt:  time.Now().Format(time.RFC3339),
			RuntimeMs:    time.Since(startedAt).Milliseconds(),
			TimeRange:    tr,
			Filters:      filter.metaFilters(),
		},
		Data: data,
	})
}

func sendInteractiveError(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)
	sendJSON(w, interactiveAPIResponse{Success: false, Error: msg, Meta: apiMeta{GeneratedAt: time.Now().Format(time.RFC3339)}})
}

func (filter AnalysisFilter) metaFilters() map[string]interface{} {
	values := map[string]interface{}{}
	add := func(k, v string) {
		if v != "" {
			values[k] = v
		}
	}
	add("id", filter.ID)
	add("project", filter.Project)
	add("session", filter.Session)
	add("tool", filter.Tool)
	add("model", filter.Model)
	add("category", filter.Category)
	add("reason", filter.Reason)
	add("severity", filter.Severity)
	add("target", filter.Target)
	add("family", filter.Family)
	if filter.Detail {
		values["detail"] = true
	}
	values["limit"] = filter.Limit
	values["samples"] = filter.Samples
	return values
}

func cacheVersionForMeta() string {
	if globalCache != nil {
		return globalCache.Version
	}
	return CacheVersion
}

func parsePositiveInt(value string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

func parseBoolQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sumIntMap(items map[string]int) int {
	total := 0
	for _, value := range items {
		total += value
	}
	return total
}

func dailyTrendCount(trend DailyTrendData, date string) int {
	for i, item := range trend.Dates {
		if item == date && i < len(trend.Counts) {
			return trend.Counts[i]
		}
	}
	return 0
}

func limitProjectStats(items []ProjectStatItem, limit int) []ProjectStatItem {
	if len(items) > limit {
		return append([]ProjectStatItem(nil), items[:limit]...)
	}
	return append([]ProjectStatItem(nil), items...)
}

func limitModelUsage(items []ModelUsageItem, limit int) []ModelUsageItem {
	if len(items) > limit {
		return append([]ModelUsageItem(nil), items[:limit]...)
	}
	return append([]ModelUsageItem(nil), items...)
}

func filterCostProjects(items []CostProjectStat, project string) []CostProjectStat {
	if project == "" {
		return items
	}
	return filterSlice(items, func(item CostProjectStat) bool {
		return strings.Contains(strings.ToLower(item.Project), strings.ToLower(project))
	})
}

func filterCostSessions(items []CostSessionStat, session, project string) []CostSessionStat {
	return filterSlice(items, func(item CostSessionStat) bool {
		return matchContains(session, item.SessionID) && matchContains(project, item.Project)
	})
}

func filterCostModels(items []CostModelStat, model string) []CostModelStat {
	if model == "" {
		return items
	}
	return filterSlice(items, func(item CostModelStat) bool {
		return strings.Contains(strings.ToLower(item.Model), strings.ToLower(model))
	})
}

func filterCommandFamilies(items []BashCommandFamilyStat, family string) []BashCommandFamilyStat {
	return filterSlice(items, func(item BashCommandFamilyStat) bool { return strings.EqualFold(item.Family, family) })
}

func limitToolStatItems(items []ToolStatItem, limit int) []ToolStatItem {
	if len(items) > limit {
		return append([]ToolStatItem(nil), items[:limit]...)
	}
	return append([]ToolStatItem(nil), items...)
}

func filterSlice[T any](items []T, keep func(T) bool) []T {
	filtered := make([]T, 0, len(items))
	for _, item := range items {
		if keep(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func containsStringFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func findingContains(item diagnosticFinding, value string) bool {
	value = strings.ToLower(value)
	for _, ev := range item.Evidence {
		if strings.Contains(strings.ToLower(ev.Value), value) {
			return true
		}
	}
	for _, example := range item.Examples {
		if strings.Contains(strings.ToLower(example.Project), value) ||
			strings.Contains(strings.ToLower(example.SessionID), value) ||
			strings.Contains(strings.ToLower(example.Tool), value) {
			return true
		}
	}
	return false
}
