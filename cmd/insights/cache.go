package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const CacheVersion = "3.0"

// CacheFile 缓存文件结构
type CacheFile struct {
	Version       string           // 缓存格式版本
	LastUpdate    time.Time        // 最后缓存时间戳
	TimeRange     TimeRange        // 缓存覆盖的时间范围
	BashRulesHash string           `json:"bash_rules_hash,omitempty"`
	BuildStats    *CacheBuildStats `json:"build_stats,omitempty"`

	// 预聚合数据
	DailyStats  map[string]*DayAggregate // "2026-01-08" -> 当天所有统计
	HourlyStats [24]*HourAggregate       // 每小时统计

	// 全局统计
	TotalMessages    int // 总消息数
	TotalSessions    int // 总会话数
	ProjectStats     map[string]*ProjectStatItem
	ModelUsage       map[string]*ModelUsageItem
	WeekdayStats     [7]*WeekdayItem
	MCPToolStats     map[string]int
	ToolStats        map[string]*ToolStatItem
	ToolAnalysis     *ToolAnalysisData
	SkillAnalysis    *SkillAnalysisData
	FailureAnalysis  *FailureAnalysisData
	SessionAnalysis  *SessionAnalysisData
	EventAnalysis    *EventAnalysisData
	AgentAnalysis    *AgentAnalysisData
	CommandAnalysis  *CommandAnalysisData
	CostAnalysis     *CostAnalysisData
	FileAnalysis     *FileAnalysisData
	TaskPlanAnalysis *TaskPlanAnalysisData           `json:"task_plan_analysis,omitempty"`
	ToolPerformance  *ToolPerformanceData            `json:"tool_performance,omitempty"`
	ProjectFiles     map[string]*ProjectFileCache    `json:"project_file_caches,omitempty"`
	DailyRuntime     map[string]ProjectFileAggregate `json:"daily_runtime,omitempty"`
}

// CacheBuildStats 记录最近一次缓存构建的结构化元数据
type CacheBuildStats struct {
	BuiltAt         string `json:"built_at"`
	BuildDurationMs int64  `json:"build_duration_ms"`
	TotalFiles      int    `json:"total_files"`
	ReusedFiles     int    `json:"reused_files"`
	ParsedFiles     int    `json:"parsed_files"`
	BashRulesHash   string `json:"bash_rules_hash,omitempty"`
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
	ProjectStats         map[string]ProjectStatItem        `json:"project_stats,omitempty"`
	WeekdayData          [7]WeekdayItem                    `json:"weekday_data"`
	DailyActivity        map[string]int                    `json:"daily_activity,omitempty"`
	DailySessions        map[string][]string               `json:"daily_sessions,omitempty"`
	DailyProjectCounts   map[string]map[string]int         `json:"daily_project_counts,omitempty"`
	DailyModelCounts     map[string]map[string]int         `json:"daily_model_counts,omitempty"`
	DailyModelTokens     map[string]map[string]int         `json:"daily_model_tokens,omitempty"`
	DailyHourlyCounts    map[string][24]int                `json:"daily_hourly_counts,omitempty"`
	DailyRuntime         map[string]ProjectFileAggregate   `json:"daily_runtime,omitempty"`
	HourlyCounts         [24]int                           `json:"hourly_counts"`
	ModelUsage           map[string]ModelUsageItem         `json:"model_usage,omitempty"`
	CostModelStats       map[string]CostModelStat          `json:"cost_model_stats,omitempty"`
	CostProjectStats     map[string]CostProjectStat        `json:"cost_project_stats,omitempty"`
	CostSessionStats     map[string]CostSessionStat        `json:"cost_session_stats,omitempty"`
	CostAgentStats       map[string]CostAgentStat          `json:"cost_agent_stats,omitempty"`
	BudgetTimeline       []BudgetTimelineItem              `json:"budget_timeline,omitempty"`
	ToolStats            map[string]ToolStatItem           `json:"tool_stats,omitempty"`
	ToolModelStats       map[string]ToolModelStatItem      `json:"tool_model_stats,omitempty"`
	FailureReasons       map[string]FailureReasonStat      `json:"failure_reasons,omitempty"`
	FailureToolReasons   map[string]FailureToolReasonStat  `json:"failure_tool_reasons,omitempty"`
	FailureModelReasons  map[string]FailureModelReasonStat `json:"failure_model_reasons,omitempty"`
	FailureSamples       []ToolFailureSample               `json:"failure_samples,omitempty"`
	SessionStats         map[string]SessionAnalysisItem    `json:"session_stats,omitempty"`
	SessionQueueOps      map[string]int                    `json:"session_queue_ops,omitempty"`
	EventTypes           map[string]int                    `json:"event_types,omitempty"`
	HookStats            map[string]HookStatItem           `json:"hook_stats,omitempty"`
	SkillStats           map[string]SkillStatItem          `json:"skill_stats,omitempty"`
	InstalledSkills      map[string]InstalledSkillItem     `json:"installed_skills,omitempty"`
	SkillUsageStats      map[string]SkillUsageStat         `json:"skill_usage_stats,omitempty"`
	SkillListingStats    map[string]int                    `json:"skill_listing_stats,omitempty"`
	SkillProjectStats    map[string]SkillProjectStat       `json:"skill_project_stats,omitempty"`
	SkillModelStats      map[string]SkillModelStat         `json:"skill_model_stats,omitempty"`
	SkillAgentStats      map[string]SkillAgentStat         `json:"skill_agent_stats,omitempty"`
	SkillToolChainStats  map[string]SkillToolChainStat     `json:"skill_tool_chain_stats,omitempty"`
	SkillListingEvents   int                               `json:"skill_listing_events,omitempty"`
	SkillInitialListings int                               `json:"skill_initial_listings,omitempty"`
	DynamicSkillEvents   int                               `json:"dynamic_skill_events,omitempty"`
	PermissionModes      map[string]int                    `json:"permission_modes,omitempty"`
	OpenedFiles          map[string]FileAccessStat         `json:"opened_files,omitempty"`
	BudgetSummary        *BudgetSummary                    `json:"budget_summary,omitempty"`
	EventSamples         []EventSample                     `json:"event_samples,omitempty"`
	AgentStats           map[string]AgentStatItem          `json:"agent_stats,omitempty"`
	AgentSessions        map[string][]string               `json:"agent_sessions,omitempty"`
	BashCommandStats     map[string]BashCommandStat        `json:"bash_command_stats,omitempty"`
	FileOperationStats   map[string]FileOperationStat      `json:"file_operation_stats,omitempty"`
	FileHotStats         map[string]FileHotStat            `json:"file_hot_stats,omitempty"`
	FileEditFailures     map[string]FileEditFailureAgg     `json:"file_edit_failures,omitempty"`
	FileSnapshotStats    map[string]FileSnapshotAgg        `json:"file_snapshot_stats,omitempty"`
	FileEditedStats      map[string]FileEditedAgg          `json:"file_edited_stats,omitempty"`
	PlanModeAgg          *SerializedPlanModeAgg            `json:"plan_mode_agg,omitempty"`
	GoalStatusAgg        *GoalStatusAgg                    `json:"goal_status_agg,omitempty"`
	ReminderAgg          *ReminderAgg                      `json:"reminder_agg,omitempty"`
	ToolPerfStats        map[string]ToolPerfAgg            `json:"tool_perf_stats,omitempty"`
	SlowestCalls         []ToolSlowCallItem                `json:"slowest_calls,omitempty"`
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
	ModelTokens   map[string]int // 模型 -> token 数
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

// SaveDiagnostics 保存诊断命令需要的轻量缓存，避免 CLI rec 读取完整文件级缓存。
func (cf *CacheFile) SaveDiagnostics(path string) error {
	copyValue := *cf
	copyValue.ProjectFiles = nil
	return copyValue.Save(path)
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
		BashRulesHash: cf.BashRulesHash,
		BuildStats:    cloneCacheBuildStats(cf.BuildStats),
		DailyStats:    make(map[string]*DayAggregate),
		HourlyStats:   [24]*HourAggregate{},
		ProjectStats:  make(map[string]*ProjectStatItem),
		ModelUsage:    make(map[string]*ModelUsageItem),
		MCPToolStats:  make(map[string]int),
		ToolStats:     make(map[string]*ToolStatItem),
	}

	queryRange := TimeRange{Start: start, End: end}
	runtimeAggregate := newProjectAggregate()
	hasRuntimeAggregate := false

	// 遍历每日统计，过滤时间范围
	for date, dayStats := range cf.DailyStats {
		dateParsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}

		if queryRange.Contains(dateParsed) {
			dayCopy := *dayStats
			dayCopy.ProjectCounts = copyIntMap(dayStats.ProjectCounts)
			dayCopy.ModelCounts = copyIntMap(dayStats.ModelCounts)
			dayCopy.ModelTokens = copyIntMap(dayStats.ModelTokens)
			result.DailyStats[date] = &dayCopy

			result.TotalMessages += dayStats.MessageCount
			result.TotalSessions += dayStats.SessionCount

			for hour, count := range dayStats.HourlyCounts {
				if count == 0 {
					continue
				}
				if result.HourlyStats[hour] == nil {
					result.HourlyStats[hour] = &HourAggregate{Hour: hour}
				}
				result.HourlyStats[hour].MessageCount += count
			}
			weekday := (int(dateParsed.Weekday()) + 6) % 7
			if result.WeekdayStats[weekday] == nil {
				result.WeekdayStats[weekday] = &WeekdayItem{
					Weekday:     weekday,
					WeekdayName: weekdayName(weekday),
				}
			}
			result.WeekdayStats[weekday].MessageCount += dayStats.MessageCount

			for project, count := range dayStats.ProjectCounts {
				if result.ProjectStats[project] == nil {
					result.ProjectStats[project] = &ProjectStatItem{Project: project}
				}
				result.ProjectStats[project].MessageCount += count
			}
			for model, count := range dayStats.ModelCounts {
				if result.ModelUsage[model] == nil {
					result.ModelUsage[model] = &ModelUsageItem{Model: model}
				}
				result.ModelUsage[model].Count += count
			}
			for model, tokens := range dayStats.ModelTokens {
				if result.ModelUsage[model] == nil {
					result.ModelUsage[model] = &ModelUsageItem{Model: model}
				}
				result.ModelUsage[model].Tokens += tokens
			}
			if runtimeSnapshot, ok := cf.DailyRuntime[date]; ok {
				mergeProjectAggregate(runtimeAggregate, projectFileAggregateToAggregate(runtimeSnapshot))
				hasRuntimeAggregate = true
			}
		}
	}

	if hasRuntimeAggregate {
		runtimeAggregate.finalize()
		result.ToolAnalysis = runtimeAggregate.ToolAnalysis
		result.SkillAnalysis = cloneSkillAnalysis(runtimeAggregate.SkillAnalysis)
		result.FailureAnalysis = runtimeAggregate.FailureAnalysis
		result.EventAnalysis = runtimeAggregate.EventAnalysis
		result.AgentAnalysis = runtimeAggregate.AgentAnalysis
		result.CommandAnalysis = runtimeAggregate.CommandAnalysis
		result.CostAnalysis = runtimeAggregate.CostAnalysis
		result.SessionAnalysis = runtimeAggregate.SessionAnalysis
		result.FileAnalysis = runtimeAggregate.FileAnalysis
		result.TaskPlanAnalysis = runtimeAggregate.TaskPlanAnalysis
		result.ToolPerformance = runtimeAggregate.ToolPerformance
		for _, tool := range runtimeAggregate.ToolAnalysis.Tools {
			toolCopy := tool
			result.ToolStats[tool.Tool] = &toolCopy
			if server, name, ok := splitMCPToolName(tool.Tool); ok {
				result.MCPToolStats[server+"::"+name] = tool.CallCount
			}
		}
	}

	return result
}

func cloneCacheBuildStats(source *CacheBuildStats) *CacheBuildStats {
	if source == nil {
		return nil
	}
	copyValue := *source
	return &copyValue
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

func cloneToolAnalysis(source *ToolAnalysisData) *ToolAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.Tools = append([]ToolStatItem(nil), source.Tools...)
	copyValue.ByModel = append([]ToolModelStatItem(nil), source.ByModel...)
	return &copyValue
}

func cloneToolPerformance(source *ToolPerformanceData) *ToolPerformanceData {
	if source == nil {
		return nil
	}
	copyVal := *source
	copyVal.ByCategory = append([]ToolPerfCategoryItem(nil), source.ByCategory...)
	copyVal.CategoryGroups = make(map[string][]ToolPerfCategoryItem, len(source.CategoryGroups))
	for k, v := range source.CategoryGroups {
		copyVal.CategoryGroups[k] = append([]ToolPerfCategoryItem(nil), v...)
	}
	copyVal.SlowestCalls = append([]ToolSlowCallItem(nil), source.SlowestCalls...)
	copyVal.QualityDistribution = append([]QualityBucketItem(nil), source.QualityDistribution...)
	return &copyVal
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

func cloneSkillAnalysis(source *SkillAnalysisData) *SkillAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.Installed = append([]InstalledSkillItem(nil), source.Installed...)
	copyValue.Skills = append([]SkillUsageStat(nil), source.Skills...)
	copyValue.ListingSkills = append([]SkillListingStat(nil), source.ListingSkills...)
	copyValue.ByProject = append([]SkillProjectStat(nil), source.ByProject...)
	copyValue.ByModel = append([]SkillModelStat(nil), source.ByModel...)
	copyValue.ByAgent = append([]SkillAgentStat(nil), source.ByAgent...)
	copyValue.ToolChains = append([]SkillToolChainStat(nil), source.ToolChains...)
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
	copyValue.BashFamilies = append([]BashCommandFamilyStat(nil), source.BashFamilies...)
	copyValue.RiskyCommands = append([]BashCommandStat(nil), source.RiskyCommands...)
	copyValue.FileOperations = append([]FileOperationStat(nil), source.FileOperations...)
	return &copyValue
}

func cloneFileAnalysis(source *FileAnalysisData) *FileAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.HotFiles = append([]FileHotItem(nil), source.HotFiles...)
	copyValue.EditFailures = append([]FileEditFailureItem(nil), source.EditFailures...)
	copyValue.Snapshots = append([]FileSnapshotItem(nil), source.Snapshots...)
	copyValue.EditedFiles = append([]FileEditedItem(nil), source.EditedFiles...)
	return &copyValue
}

func cloneTaskPlanAnalysis(source *TaskPlanAnalysisData) *TaskPlanAnalysisData {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.PlanFiles = append([]PlanFileItem(nil), source.PlanFiles...)
	copyValue.GoalStatus = append([]GoalStatusItem(nil), source.GoalStatus...)
	if source.ReminderSummary.TopTaskSessions != nil {
		copyValue.ReminderSummary.TopTaskSessions = append([]ReminderSessionItem(nil), source.ReminderSummary.TopTaskSessions...)
	}
	if source.ReminderSummary.TopTodoSessions != nil {
		copyValue.ReminderSummary.TopTodoSessions = append([]ReminderSessionItem(nil), source.ReminderSummary.TopTodoSessions...)
	}
	copyValue.Tasks.StatusDistribution = append([]TaskStatusItem(nil), source.Tasks.StatusDistribution...)
	copyValue.Tasks.SessionTaskCounts = append([]SessionTaskItem(nil), source.Tasks.SessionTaskCounts...)
	return &copyValue
}

func weekdayName(weekday int) string {
	names := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	if weekday < 0 || weekday >= len(names) {
		return ""
	}
	return names[weekday]
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
