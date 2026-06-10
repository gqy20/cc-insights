package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const CacheVersion = "2.5"

// CacheFile 缓存文件结构
type CacheFile struct {
	Version    string    // 缓存格式版本
	LastUpdate time.Time // 最后缓存时间戳
	TimeRange  TimeRange // 缓存覆盖的时间范围

	// 预聚合数据
	DailyStats  map[string]*DayAggregate // "2026-01-08" -> 当天所有统计
	HourlyStats [24]*HourAggregate       // 每小时统计

	// 全局统计
	TotalMessages   int // 总消息数
	TotalSessions   int // 总会话数
	ProjectStats    map[string]*ProjectStatItem
	ModelUsage      map[string]*ModelUsageItem
	WeekdayStats    [7]*WeekdayItem
	MCPToolStats    map[string]int
	ToolStats       map[string]*ToolStatItem
	ToolAnalysis    *ToolAnalysisData
	FailureAnalysis *FailureAnalysisData
	SessionAnalysis *SessionAnalysisData
	EventAnalysis   *EventAnalysisData
	AgentAnalysis   *AgentAnalysisData
	CommandAnalysis *CommandAnalysisData
	CostAnalysis    *CostAnalysisData
	ProjectFiles    map[string]*ProjectFileCache `json:"project_file_caches,omitempty"`
}

// ProjectFileCache 单个 projects JSONL 文件的增量缓存
type ProjectFileCache struct {
	Path        string               `json:"path"`
	Size        int64                `json:"size"`
	ModTimeUnix int64                `json:"mod_time_unix"`
	Aggregate   ProjectFileAggregate `json:"aggregate"`
}

// ProjectFileAggregate 可序列化的文件级聚合快照
type ProjectFileAggregate struct {
	ProjectStats        map[string]ProjectStatItem        `json:"project_stats,omitempty"`
	WeekdayData         [7]WeekdayItem                    `json:"weekday_data"`
	DailyActivity       map[string]int                    `json:"daily_activity,omitempty"`
	DailySessions       map[string][]string               `json:"daily_sessions,omitempty"`
	HourlyCounts        [24]int                           `json:"hourly_counts"`
	ModelUsage          map[string]ModelUsageItem         `json:"model_usage,omitempty"`
	CostModelStats      map[string]CostModelStat          `json:"cost_model_stats,omitempty"`
	CostProjectStats    map[string]CostProjectStat        `json:"cost_project_stats,omitempty"`
	CostSessionStats    map[string]CostSessionStat        `json:"cost_session_stats,omitempty"`
	CostAgentStats      map[string]CostAgentStat          `json:"cost_agent_stats,omitempty"`
	BudgetTimeline      []BudgetTimelineItem              `json:"budget_timeline,omitempty"`
	ToolStats           map[string]ToolStatItem           `json:"tool_stats,omitempty"`
	ToolModelStats      map[string]ToolModelStatItem      `json:"tool_model_stats,omitempty"`
	FailureReasons      map[string]FailureReasonStat      `json:"failure_reasons,omitempty"`
	FailureToolReasons  map[string]FailureToolReasonStat  `json:"failure_tool_reasons,omitempty"`
	FailureModelReasons map[string]FailureModelReasonStat `json:"failure_model_reasons,omitempty"`
	FailureSamples      []ToolFailureSample               `json:"failure_samples,omitempty"`
	SessionStats        map[string]SessionAnalysisItem    `json:"session_stats,omitempty"`
	SessionQueueOps     map[string]int                    `json:"session_queue_ops,omitempty"`
	EventTypes          map[string]int                    `json:"event_types,omitempty"`
	HookStats           map[string]HookStatItem           `json:"hook_stats,omitempty"`
	SkillStats          map[string]SkillStatItem          `json:"skill_stats,omitempty"`
	PermissionModes     map[string]int                    `json:"permission_modes,omitempty"`
	OpenedFiles         map[string]FileAccessStat         `json:"opened_files,omitempty"`
	BudgetSummary       *BudgetSummary                    `json:"budget_summary,omitempty"`
	EventSamples        []EventSample                     `json:"event_samples,omitempty"`
	AgentStats          map[string]AgentStatItem          `json:"agent_stats,omitempty"`
	AgentSessions       map[string][]string               `json:"agent_sessions,omitempty"`
	BashCommandStats    map[string]BashCommandStat        `json:"bash_command_stats,omitempty"`
	FileOperationStats  map[string]FileOperationStat      `json:"file_operation_stats,omitempty"`
}

// DayAggregate 每日聚合数据
type DayAggregate struct {
	Date          string         // "2026-01-08"
	MessageCount  int            // 当天消息数
	SessionCount  int            // 当天会话数
	ToolCallCount int            // 当天工具调用数
	HourlyCounts  [24]int        // 每小时消息数
	ProjectCounts map[string]int // 项目 -> 消息数
	ModelCounts   map[string]int // 模型 -> 请求次数
}

// HourAggregate 每小时聚合数据
type HourAggregate struct {
	Hour         int // 0-23
	MessageCount int // 该小时消息数
	SessionCount int // 该小时会话数
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Contains 检查时间是否在范围内
func (tr TimeRange) Contains(t time.Time) bool {
	// 无限制情况
	if tr.Start.IsZero() && tr.End.IsZero() {
		return true
	}

	// 检查开始时间
	if !tr.Start.IsZero() && t.Before(tr.Start) {
		return false
	}

	// 检查结束时间
	if !tr.End.IsZero() && t.After(tr.End) {
		return false
	}

	return true
}

// Save 保存缓存到文件
func (cf *CacheFile) Save(path string) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 序列化为紧凑 JSON。缓存偏向机器读写，避免 pretty print 放大文件级聚合缓存体积。
	data, err := json.Marshal(cf)
	if err != nil {
		return fmt.Errorf("序列化缓存失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入缓存文件失败: %w", err)
	}

	return nil
}

// LoadCacheFile 从文件加载缓存
func LoadCacheFile(path string) (*CacheFile, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("缓存文件不存在: %s", path)
	}

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取缓存文件失败: %w", err)
	}

	// 反序列化
	var cache CacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("解析缓存文件失败: %w", err)
	}

	return &cache, nil
}

// IsExpired 检查缓存是否过期
func (cf *CacheFile) IsExpired(dataLastModified time.Time) bool {
	// 如果数据文件的修改时间晚于缓存更新时间，则缓存过期
	return dataLastModified.After(cf.LastUpdate)
}

// QueryByTimeRange 按时间范围查询缓存数据
func (cf *CacheFile) QueryByTimeRange(start, end time.Time) *CacheFile {
	result := &CacheFile{
		Version:    cf.Version,
		LastUpdate: cf.LastUpdate,
		TimeRange: TimeRange{
			Start: start,
			End:   end,
		},
		DailyStats:   make(map[string]*DayAggregate),
		HourlyStats:  [24]*HourAggregate{},
		ProjectStats: make(map[string]*ProjectStatItem),
		ModelUsage:   make(map[string]*ModelUsageItem),
		MCPToolStats: make(map[string]int),
		ToolStats:    make(map[string]*ToolStatItem),
	}

	queryRange := TimeRange{Start: start, End: end}

	// 遍历每日统计，过滤时间范围
	for date, dayStats := range cf.DailyStats {
		dateParsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}

		if queryRange.Contains(dateParsed) {
			dayCopy := *dayStats
			result.DailyStats[date] = &dayCopy

			result.TotalMessages += dayStats.MessageCount
			result.TotalSessions += dayStats.SessionCount
		}
	}

	// 从过滤后的 DailyStats 中收集有效项目名和模型名
	activeProjects := make(map[string]bool)
	activeModels := make(map[string]bool)
	for _, dayStats := range result.DailyStats {
		for proj := range dayStats.ProjectCounts {
			activeProjects[proj] = true
		}
		for model := range dayStats.ModelCounts {
			activeModels[model] = true
		}
	}

	// 复制 HourlyStats（全局分布模式，不过滤）
	for i, hs := range cf.HourlyStats {
		if hs != nil {
			hsCopy := *hs
			result.HourlyStats[i] = &hsCopy
		}
	}

	// 复制 WeekdayStats（全局分布模式，不过滤）
	for i, ws := range cf.WeekdayStats {
		if ws != nil {
			result.WeekdayStats[i] = ws
		}
	}

	// 过滤 ProjectStats：只保留时间范围内有活动的项目
	for proj, stats := range cf.ProjectStats {
		if activeProjects[proj] {
			projCopy := *stats
			result.ProjectStats[proj] = &projCopy
		}
	}

	// 过滤 ModelUsage：只保留时间范围内使用的模型
	for model, mu := range cf.ModelUsage {
		if activeModels[model] {
			muCopy := *mu
			result.ModelUsage[model] = &muCopy
		}
	}

	// MCPToolStats（全局统计，不过滤——debug日志无每日分解）
	for tool, count := range cf.MCPToolStats {
		result.MCPToolStats[tool] = count
	}

	// ToolStats（全局统计，v1 暂不按日分解）
	for tool, stats := range cf.ToolStats {
		if stats != nil {
			statsCopy := *stats
			result.ToolStats[tool] = &statsCopy
		}
	}
	if cf.ToolAnalysis != nil {
		analysisCopy := *cf.ToolAnalysis
		analysisCopy.Tools = append([]ToolStatItem(nil), cf.ToolAnalysis.Tools...)
		analysisCopy.ByModel = append([]ToolModelStatItem(nil), cf.ToolAnalysis.ByModel...)
		result.ToolAnalysis = &analysisCopy
	}
	result.EventAnalysis = cloneEventAnalysis(cf.EventAnalysis)
	result.AgentAnalysis = cloneAgentAnalysis(cf.AgentAnalysis)
	result.CommandAnalysis = cloneCommandAnalysis(cf.CommandAnalysis)
	result.CostAnalysis = cloneCostAnalysis(cf.CostAnalysis)
	result.FailureAnalysis = cloneFailureAnalysis(cf.FailureAnalysis)
	result.SessionAnalysis = cloneSessionAnalysis(cf.SessionAnalysis)

	return result
}

func cloneSessionAnalysis(source *SessionAnalysisData) *SessionAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.Sessions = append([]SessionAnalysisItem(nil), source.Sessions...)
	copyValue.TopFailures = append([]SessionAnalysisItem(nil), source.TopFailures...)
	copyValue.LongRunning = append([]SessionAnalysisItem(nil), source.LongRunning...)
	copyValue.Outcomes = append([]SessionOutcomeStat(nil), source.Outcomes...)
	copyValue.QueueOperations = append([]QueueOperationStat(nil), source.QueueOperations...)
	copyValue.Titles = append([]SessionTitleStat(nil), source.Titles...)
	return &copyValue
}

func cloneFailureAnalysis(source *FailureAnalysisData) *FailureAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.ByReason = append([]FailureReasonStat(nil), source.ByReason...)
	copyValue.ByToolReason = append([]FailureToolReasonStat(nil), source.ByToolReason...)
	copyValue.ByModelReason = append([]FailureModelReasonStat(nil), source.ByModelReason...)
	copyValue.Samples = append([]ToolFailureSample(nil), source.Samples...)
	return &copyValue
}

func cloneCostAnalysis(source *CostAnalysisData) *CostAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.ByModel = append([]CostModelStat(nil), source.ByModel...)
	copyValue.ByProject = append([]CostProjectStat(nil), source.ByProject...)
	copyValue.BySession = append([]CostSessionStat(nil), source.BySession...)
	copyValue.ByAgent = append([]CostAgentStat(nil), source.ByAgent...)
	copyValue.BudgetTimeline = append([]BudgetTimelineItem(nil), source.BudgetTimeline...)
	return &copyValue
}

func cloneEventAnalysis(source *EventAnalysisData) *EventAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.ByType = append([]EventTypeStat(nil), source.ByType...)
	copyValue.Hooks = append([]HookStatItem(nil), source.Hooks...)
	copyValue.Skills = append([]SkillStatItem(nil), source.Skills...)
	copyValue.PermissionModes = append([]PermissionModeStat(nil), source.PermissionModes...)
	copyValue.OpenedFiles = append([]FileAccessStat(nil), source.OpenedFiles...)
	copyValue.Samples = append([]EventSample(nil), source.Samples...)
	if source.Budget != nil {
		budgetCopy := *source.Budget
		copyValue.Budget = &budgetCopy
	}
	return &copyValue
}

func cloneAgentAnalysis(source *AgentAnalysisData) *AgentAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.Agents = append([]AgentStatItem(nil), source.Agents...)
	return &copyValue
}

func cloneCommandAnalysis(source *CommandAnalysisData) *CommandAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.BashCommands = append([]BashCommandStat(nil), source.BashCommands...)
	copyValue.RiskyCommands = append([]BashCommandStat(nil), source.RiskyCommands...)
	copyValue.FileOperations = append([]FileOperationStat(nil), source.FileOperations...)
	return &copyValue
}

// AddMessage 添加一条消息记录到每日聚合
func (da *DayAggregate) AddMessage(project string, hour int) {
	da.MessageCount++

	// 更新小时统计
	if hour >= 0 && hour < 24 {
		da.HourlyCounts[hour]++
	}

	// 更新项目统计
	if da.ProjectCounts == nil {
		da.ProjectCounts = make(map[string]int)
	}
	da.ProjectCounts[project]++
}
