package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type cliPromptReport struct {
	TimeRange         TimeRangeInfo          `json:"time_range"`
	RawPrompts        int                    `json:"raw_prompts"`
	CleanPrompts      int                    `json:"clean_prompts"`
	NoisePrompts      int                    `json:"noise_prompts"`
	ToolResultRecords int                    `json:"tool_result_records"`
	ProjectCount      int                    `json:"project_count"`
	SessionCount      int                    `json:"session_count"`
	ByCategory        []promptCategoryStat   `json:"by_category"`
	TopShortPrompts   []nameCount            `json:"top_short_prompts"`
	TopProjects       []nameCount            `json:"top_projects"`
	Preferences       []promptPreferenceItem `json:"preferences"`
	Samples           []promptSample         `json:"samples,omitempty"`
	Runtime           *cliRuntimeInfo        `json:"runtime,omitempty"`
	Filter            cliPromptReportFilter  `json:"filter"`
	Insights          []string               `json:"insights"`
}

type cliPromptReportFilter struct {
	Project string `json:"project,omitempty"`
	Session string `json:"session,omitempty"`
}

type promptCategoryStat struct {
	Name     string         `json:"name"`
	Count    int            `json:"count"`
	Examples []promptSample `json:"examples,omitempty"`
}

type promptPreferenceItem struct {
	Title      string         `json:"title"`
	Confidence string         `json:"confidence"`
	Count      int            `json:"count"`
	Summary    string         `json:"summary"`
	Suggestion string         `json:"suggestion"`
	Evidence   []promptSample `json:"evidence,omitempty"`
}

type promptSample struct {
	Timestamp string   `json:"timestamp,omitempty"`
	Project   string   `json:"project,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
	Preview   string   `json:"preview"`
	Labels    []string `json:"labels,omitempty"`
}

type promptAggregate struct {
	rawPrompts        int
	cleanPrompts      int
	noisePrompts      int
	toolResultRecords int
	projects          map[string]int
	sessions          map[string]bool
	categories        map[string]int
	categoryExamples  map[string][]promptSample
	shortPrompts      map[string]int
	samples           []promptSample
}

type promptRecord struct {
	timestamp time.Time
	project   string
	sessionID string
	text      string
}

type promptRule struct {
	name     string
	keywords []string
}

var promptCategoryRules = []promptRule{
	{name: "先说明/规划", keywords: []string{"先说明", "先分析", "先介绍", "先思考", "先评估", "思考一下", "方案", "如何", "怎么做", "是否应该", "下一步", "预估"}},
	{name: "开始实现/推进", keywords: []string{"开始实现", "开始优化", "开始做", "开始改", "开始修复", "按照这个", "继续做", "实现", "优化", "改造"}},
	{name: "继续", keywords: []string{"继续"}},
	{name: "提交发布", keywords: []string{"提交", "push", "tag", "release", "发布"}},
	{name: "验证核对", keywords: []string{"看一下", "核对", "测试", "截图", "浏览器", "运行", "效果", "失败", "报错", "日志"}},
	{name: "批评/纠偏", keywords: []string{"不对", "错了", "有问题", "垃圾", "太粗糙", "奇怪", "不好", "丑", "没必要", "怎么还有问题"}},
	{name: "文档", keywords: []string{"文档", "README", "CHANGELOG", "CLAUDE.md", "AGENTS.md", "docs", "整理"}},
	{name: "产品/架构方向", keywords: []string{"核心", "需求", "理念", "架构", "方向", "机制", "体系", "命令", "前端", "后端", "CLI", "web", "大屏"}},
	{name: "偏好/约束", keywords: []string{"不要", "默认", "必须", "应该", "保持", "统一", "收敛", "简化", "优雅", "激进", "兼容", "局域网"}},
	{name: "数据/分析", keywords: []string{"数据", "统计", "分析", "结果", "原始", "提取", "分类", "耗时", "token", "成本", "失败原因"}},
}

func buildCLIPromptReport(tf TimeFilter, preset string, opts cliOptions) (cliPromptReport, error) {
	agg := newPromptAggregate()
	if err := scanPromptRecords(cfg.DataDir, tf, opts, agg); err != nil {
		return cliPromptReport{}, err
	}
	report := agg.toReport(TimeRangeInfo{Preset: preset}, opts)
	if tf.Start != nil {
		report.TimeRange.Start = tf.Start.Format("2006-01-02")
	}
	if tf.End != nil {
		report.TimeRange.End = tf.End.Format("2006-01-02")
	}
	if report.TimeRange.Start != "" || report.TimeRange.End != "" {
		report.TimeRange.Preset = preset
	}
	return report, nil
}

func newPromptAggregate() *promptAggregate {
	return &promptAggregate{
		projects:         make(map[string]int),
		sessions:         make(map[string]bool),
		categories:       make(map[string]int),
		categoryExamples: make(map[string][]promptSample),
		shortPrompts:     make(map[string]int),
	}
}

func scanPromptRecords(dataDir string, tf TimeFilter, opts cliOptions, agg *promptAggregate) error {
	projectsDir := filepath.Join(dataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("读取 projects 目录失败: %w", err)
	}
	files := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectFiles, err := projectJSONLFiles(filepath.Join(projectsDir, entry.Name()))
		if err != nil {
			continue
		}
		for _, path := range projectFiles {
			if shouldSkipPromptFile(path, tf) {
				continue
			}
			files = append(files, path)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil
	}

	maxWorkers := runtime.NumCPU()
	if len(files) < maxWorkers {
		maxWorkers = len(files)
	}
	jobs := make(chan string, maxWorkers*2)
	results := make(chan *promptAggregate, maxWorkers)
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := newPromptAggregate()
			for path := range jobs {
				scanPromptFile(path, tf, opts, local)
			}
			results <- local
		}()
	}
	for _, path := range files {
		jobs <- path
	}
	close(jobs)
	go func() {
		wg.Wait()
		close(results)
	}()
	for local := range results {
		mergePromptAggregate(agg, local)
	}
	return nil
}

func shouldSkipPromptFile(path string, tf TimeFilter) bool {
	if tf.Start == nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.ModTime().Before(*tf.Start)
}

func mergePromptAggregate(dst, src *promptAggregate) {
	dst.rawPrompts += src.rawPrompts
	dst.cleanPrompts += src.cleanPrompts
	dst.noisePrompts += src.noisePrompts
	dst.toolResultRecords += src.toolResultRecords
	for project, count := range src.projects {
		dst.projects[project] += count
	}
	for sessionID := range src.sessions {
		dst.sessions[sessionID] = true
	}
	for category, count := range src.categories {
		dst.categories[category] += count
	}
	for category, examples := range src.categoryExamples {
		for _, example := range examples {
			dst.categoryExamples[category] = addPromptExample(dst.categoryExamples[category], example, 3)
		}
	}
	for prompt, count := range src.shortPrompts {
		dst.shortPrompts[prompt] += count
	}
	dst.samples = appendPromptSamples(dst.samples, src.samples, 30)
}

func appendPromptSamples(dst, src []promptSample, limit int) []promptSample {
	if len(dst) >= limit {
		return dst
	}
	remaining := limit - len(dst)
	if len(src) > remaining {
		src = src[:remaining]
	}
	return append(dst, src...)
}

func addPromptExample(items []promptSample, sample promptSample, limit int) []promptSample {
	if limit <= 0 {
		return items
	}
	for _, item := range items {
		if item.Preview == sample.Preview {
			return items
		}
	}
	if len(items) < limit {
		items = append(items, sample)
		sort.SliceStable(items, func(i, j int) bool {
			return promptExampleScore(items[i]) < promptExampleScore(items[j])
		})
		return items
	}
	worstIndex := -1
	worstScore := -1
	for i, item := range items {
		score := promptExampleScore(item)
		if score > worstScore {
			worstScore = score
			worstIndex = i
		}
	}
	if promptExampleScore(sample) < worstScore && worstIndex >= 0 {
		items[worstIndex] = sample
		sort.SliceStable(items, func(i, j int) bool {
			return promptExampleScore(items[i]) < promptExampleScore(items[j])
		})
	}
	return items
}

func promptExampleScore(sample promptSample) int {
	length := utf8.RuneCountInString(sample.Preview)
	if length == 0 {
		return 10000
	}
	score := length
	if length < 4 {
		score += 180
	} else if length < 8 {
		score += 90
	} else if length < 12 {
		score += 35
	}
	if strings.Contains(sample.Preview, "://") || strings.Contains(sample.Preview, "/home/") {
		score += 120
	}
	if strings.Contains(sample.Preview, "#") || strings.Contains(sample.Preview, "<") {
		score += 60
	}
	return score
}

func scanPromptFile(path string, tf TimeFilter, opts cliOptions, agg *promptAggregate) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		if record.Type != "user" {
			continue
		}
		timestamp, hasTimestamp := parseProjectRecordTimestamp(record.Timestamp)
		if hasTimestamp && !tf.Contains(timestamp) {
			continue
		}
		if !hasTimestamp && hasTimeFilter(tf) {
			continue
		}
		project := nonEmpty(record.Cwd, "Unknown")
		if opts.Project != "" && !strings.Contains(project, opts.Project) {
			continue
		}
		if opts.Session != "" && record.SessionID != opts.Session {
			continue
		}
		text, ok, toolResult := extractUserPromptText(record.Message)
		if toolResult {
			agg.toolResultRecords++
			continue
		}
		if !ok {
			continue
		}
		agg.rawPrompts++
		item := promptRecord{timestamp: timestamp, project: project, sessionID: record.SessionID, text: text}
		if isPromptNoise(text) {
			agg.noisePrompts++
			continue
		}
		agg.addCleanPrompt(item)
	}
}

func extractUserPromptText(raw json.RawMessage) (string, bool, bool) {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return "", false, false
	}
	content := bytes.TrimSpace(msg.Content)
	if len(content) == 0 {
		return "", false, false
	}
	if content[0] == '"' {
		var text string
		if err := json.Unmarshal(content, &text); err != nil {
			return "", false, false
		}
		return strings.TrimSpace(text), strings.TrimSpace(text) != "", false
	}
	if content[0] == '[' {
		var parts []AssistantContent
		if err := json.Unmarshal(content, &parts); err != nil {
			return "", false, false
		}
		for _, part := range parts {
			if part.Type == "tool_result" {
				return "", false, true
			}
		}
		texts := make([]string, 0, len(parts))
		for _, part := range parts {
			if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
				texts = append(texts, strings.TrimSpace(part.Text))
			}
		}
		text := strings.Join(texts, "\n")
		return text, text != "", false
	}
	return "", false, false
}

func isPromptNoise(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	if utf8.RuneCountInString(trimmed) > 3000 {
		return true
	}
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(trimmed, "This session is being continued from a previous conversation"):
		return true
	case strings.HasPrefix(trimmed, "[Request interrupted"):
		return true
	case strings.Contains(trimmed, "__probe_"):
		return true
	case strings.Contains(trimmed, "请立即用 TaskCreate 创建一个主题"):
		return true
	case strings.Contains(trimmed, "轮到你") && strings.Contains(trimmed, "speak_log.md"):
		return true
	case strings.Contains(trimmed, "身份:") && strings.Contains(trimmed, "阵营:"):
		return true
	case startsWithAny(trimmed, []string{"💬", "☠️", "🌅", "🎉", "🗳️", "🔪", "🧙"}):
		return true
	case strings.HasPrefix(trimmed, "$ "):
		return true
	case strings.Contains(lower, "exit code") && utf8.RuneCountInString(trimmed) > 120:
		return true
	default:
		return false
	}
}

func (agg *promptAggregate) addCleanPrompt(item promptRecord) {
	agg.cleanPrompts++
	agg.projects[item.project]++
	if item.sessionID != "" {
		agg.sessions[item.sessionID] = true
	}
	labels := classifyPrompt(item.text)
	sample := promptSample{
		Timestamp: formatPromptTimestamp(item.timestamp),
		Project:   item.project,
		SessionID: item.sessionID,
		Preview:   promptPreview(item.text, 160),
		Labels:    labels,
	}
	if len(agg.samples) < 30 {
		agg.samples = append(agg.samples, sample)
	}
	for _, label := range labels {
		agg.categories[label]++
		agg.categoryExamples[label] = addPromptExample(agg.categoryExamples[label], sample, 3)
	}
	normalized := normalizePromptText(item.text)
	if utf8.RuneCountInString(normalized) <= 80 {
		agg.shortPrompts[normalized]++
	}
}

func classifyPrompt(text string) []string {
	normalized := strings.ToLower(normalizePromptText(text))
	labels := make([]string, 0, 4)
	for _, rule := range promptCategoryRules {
		for _, keyword := range rule.keywords {
			if strings.Contains(normalized, strings.ToLower(keyword)) {
				labels = append(labels, rule.name)
				break
			}
		}
	}
	return uniqueNonEmptyStrings(labels)
}

func (agg *promptAggregate) toReport(timeRange TimeRangeInfo, opts cliOptions) cliPromptReport {
	report := cliPromptReport{
		TimeRange:         timeRange,
		RawPrompts:        agg.rawPrompts,
		CleanPrompts:      agg.cleanPrompts,
		NoisePrompts:      agg.noisePrompts,
		ToolResultRecords: agg.toolResultRecords,
		ProjectCount:      len(agg.projects),
		SessionCount:      len(agg.sessions),
		ByCategory:        buildPromptCategoryStats(agg, opts.Limit),
		TopShortPrompts:   topNameCounts(agg.shortPrompts, opts.Limit),
		TopProjects:       topNameCounts(agg.projects, opts.Limit),
		Preferences:       buildPromptPreferences(agg, opts.Limit),
		Filter: cliPromptReportFilter{
			Project: opts.Project,
			Session: opts.Session,
		},
	}
	if opts.Detail {
		report.Samples = append([]promptSample(nil), agg.samples...)
	}
	report.Insights = buildPromptInsights(report)
	return report
}

func buildPromptCategoryStats(agg *promptAggregate, limit int) []promptCategoryStat {
	items := make([]promptCategoryStat, 0, len(agg.categories))
	for name, count := range agg.categories {
		items = append(items, promptCategoryStat{
			Name:     name,
			Count:    count,
			Examples: append([]promptSample(nil), agg.categoryExamples[name]...),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func buildPromptPreferences(agg *promptAggregate, limit int) []promptPreferenceItem {
	candidates := []promptPreferenceItem{
		promptPreferenceFromCategory(agg, "先说明/规划", "偏好先判断再动手", "涉及架构、命令体系、前端大改或文档整合时，先说明方案、取舍和风险，再实施。"),
		promptPreferenceFromCategory(agg, "验证核对", "重视可验证结果", "前端、CLI 输出和数据口径改动后，默认运行验证命令；Web 改动优先补截图或浏览器检查。"),
		promptPreferenceFromCategory(agg, "批评/纠偏", "对粗糙和发散敏感", "发现重复命令、弱兼容、界面粗糙或统计口径不清时，应主动收敛并解释修正。"),
		promptPreferenceFromCategory(agg, "产品/架构方向", "关注工具的核心定位", "新增能力需要落到诊断、解释、证据和改进建议，不只增加展示项。"),
		promptPreferenceFromCategory(agg, "偏好/约束", "明确约束需要沉淀", "把反复出现的默认行为、命令收敛、局域网可用等要求整理为 CLAUDE.md/AGENTS.md 候选规则。"),
	}
	items := make([]promptPreferenceItem, 0, len(candidates))
	for _, item := range candidates {
		if item.Count == 0 {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Title < items[j].Title
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func promptPreferenceFromCategory(agg *promptAggregate, category, title, suggestion string) promptPreferenceItem {
	count := agg.categories[category]
	return promptPreferenceItem{
		Title:      title,
		Confidence: promptConfidence(count, agg.cleanPrompts),
		Count:      count,
		Summary:    fmt.Sprintf("命中 %s 类输入 %s 次。", category, formatInt(count)),
		Suggestion: suggestion,
		Evidence:   append([]promptSample(nil), agg.categoryExamples[category]...),
	}
}

func promptConfidence(count, total int) string {
	if total == 0 || count == 0 {
		return "low"
	}
	ratio := float64(count) / float64(total)
	switch {
	case count >= 50 || ratio >= 0.20:
		return "high"
	case count >= 10 || ratio >= 0.08:
		return "medium"
	default:
		return "low"
	}
}

func buildPromptInsights(report cliPromptReport) []string {
	insights := []string{}
	if report.CleanPrompts == 0 {
		return append(insights, "当前范围没有可用于画像的真实用户输入。")
	}
	insights = append(insights, fmt.Sprintf("清洗后保留 %s 条真实输入，过滤噪声 %s 条，工具结果回传 %s 条。", formatInt(report.CleanPrompts), formatInt(report.NoisePrompts), formatInt(report.ToolResultRecords)))
	if len(report.ByCategory) > 0 {
		insights = append(insights, fmt.Sprintf("最常见输入类型是 %s，共 %s 次。", report.ByCategory[0].Name, formatInt(report.ByCategory[0].Count)))
	}
	if len(report.TopShortPrompts) > 0 {
		insights = append(insights, fmt.Sprintf("最高频短指令是「%s」，出现 %s 次。", report.TopShortPrompts[0].Name, formatInt(report.TopShortPrompts[0].Count)))
	}
	if len(report.Preferences) > 0 {
		insights = append(insights, fmt.Sprintf("最高置信偏好是「%s」，可沉淀为协作规则。", report.Preferences[0].Title))
	}
	return insights
}

func topNameCounts(values map[string]int, limit int) []nameCount {
	items := make([]nameCount, 0, len(values))
	for name, count := range values {
		if strings.TrimSpace(name) == "" || count <= 0 {
			continue
		}
		items = append(items, nameCount{Name: name, Count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func normalizePromptText(text string) string {
	if args := extractCommandArgsText(text); args != "" {
		text = args
	}
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func extractCommandArgsText(text string) string {
	const startTag = "<command-args>"
	const endTag = "</command-args>"
	start := strings.Index(text, startTag)
	if start < 0 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(text[start:], endTag)
	if end < 0 {
		return strings.TrimSpace(text[start:])
	}
	return strings.TrimSpace(text[start : start+end])
}

func promptPreview(text string, limit int) string {
	text = normalizePromptText(text)
	text = strings.ReplaceAll(text, `"`, "'")
	text = strings.ReplaceAll(text, "\\", "/")
	if limit <= 0 {
		return text
	}
	var sb strings.Builder
	count := 0
	for _, r := range text {
		if r < 32 || r == 127 {
			continue
		}
		if count >= limit {
			break
		}
		sb.WriteRune(r)
		count++
	}
	return sb.String()
}

func formatPromptTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func startsWithAny(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
