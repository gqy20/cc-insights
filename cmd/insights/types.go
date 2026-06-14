package main

import (
	"encoding/json"
	"sync"
	"time"
)

// HistoryRecord history.jsonl 记录
type HistoryRecord struct {
	Display        string            `json:"display"`
	PastedContents map[string]string `json:"pastedContents"`
	Timestamp      int64             `json:"timestamp"`
	Project        string            `json:"project"`
}

// DailyActivity 每日活动统计
type DailyActivity struct {
	Date          string `json:"date"`
	MessageCount  int    `json:"messageCount"`
	SessionCount  int    `json:"sessionCount"`
	ToolCallCount int    `json:"toolCallCount"`
}

// StatsCache stats-cache.json 结构
type StatsCache struct {
	DailyActivity    []DailyActivity          `json:"dailyActivity"`
	DailyModelTokens []map[string]interface{} `json:"dailyModelTokens"`
	ModelUsage       map[string]struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	} `json:"modelUsage"`
	HourCounts map[string]int `json:"hourCounts"`
}

// SessionStats 会话统计数据
type SessionStats struct {
	TotalSessions   int            `json:"total_sessions"`
	PeakDate        string         `json:"peak_date"`
	PeakCount       int            `json:"peak_count"`
	ValleyDate      string         `json:"valley_date"`
	ValleyCount     int            `json:"valley_count"`
	DailySessionMap map[string]int `json:"daily_session_map"`
}

// CommandStats 命令统计
type CommandStats struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
}

// ProjectStats 项目统计
type ProjectStats struct {
	Project string
	Count   int
}

// ProjectStatsData 项目统计数据（扩展版）
type ProjectStatsData struct {
	Projects      []ProjectStatItem `json:"projects"`
	TotalMessages int               `json:"total_messages"`
	TotalSessions int               `json:"total_sessions"`
}

// ProjectStatItem 单个项目统计
type ProjectStatItem struct {
	Project      string `json:"project"`
	SessionCount int    `json:"session_count"`
	MessageCount int    `json:"message_count"`
}

// WeekdayStats 星期统计
type WeekdayStats struct {
	WeekdayData []WeekdayItem `json:"weekday_data"`
}

// WeekdayItem 单个星期数据
type WeekdayItem struct {
	Weekday      int    `json:"weekday"`      // 0=周一, 6=周日
	WeekdayName  string `json:"weekday_name"` // "周一"..."周日"
	MessageCount int    `json:"message_count"`
}

// ModelUsageItem 单个模型使用统计
type ModelUsageItem struct {
	Model  string `json:"model"`
	Count  int    `json:"count"`
	Tokens int    `json:"tokens"`
}

// ToolAnalysisData 工具调用分析结果
type ToolAnalysisData struct {
	TotalCalls     int                 `json:"total_calls"`
	TotalFailures  int                 `json:"total_failures"`
	MissingResults int                 `json:"missing_results"`
	Tools          []ToolStatItem      `json:"tools"`
	ByModel        []ToolModelStatItem `json:"by_model"`
}

// SkillAnalysisData skill 可见性、调用和效果分析结果
type SkillAnalysisData struct {
	TotalInstalled         int                    `json:"total_installed"`
	TotalInvocations       int                    `json:"total_invocations"`
	ToolUseInvocations     int                    `json:"tool_use_invocations"`
	AttachmentInvocations  int                    `json:"attachment_invocations"`
	ListingEvents          int                    `json:"listing_events"`
	InitialListingEvents   int                    `json:"initial_listing_events"`
	DynamicSkillEvents     int                    `json:"dynamic_skill_events"`
	FailureCount           int                    `json:"failure_count"`
	MissingResults         int                    `json:"missing_results"`
	Installed              []InstalledSkillItem   `json:"installed"`
	Skills                 []SkillUsageStat       `json:"skills"`
	ListingSkills          []SkillListingStat     `json:"listing_skills"`
	ByProject              []SkillProjectStat     `json:"by_project"`
	ByModel                []SkillModelStat       `json:"by_model"`
	ByAgent                []SkillAgentStat       `json:"by_agent"`
	SessionAssociatedTools []SkillSessionToolStat `json:"session_associated_tools"`
}

// InstalledSkillItem 本地安装 skill 目录摘要
type InstalledSkillItem struct {
	Name       string `json:"name"`
	Path       string `json:"path,omitempty"`
	HasSkillMD bool   `json:"has_skill_md"`
	FileCount  int    `json:"file_count"`
}

// SkillUsageStat skill 调用统计
type SkillUsageStat struct {
	Name               string  `json:"name"`
	Path               string  `json:"path,omitempty"`
	InvocationCount    int     `json:"invocation_count"`
	ToolUseCount       int     `json:"tool_use_count"`
	AttachmentCount    int     `json:"attachment_count"`
	SuccessCount       int     `json:"success_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResultCount int     `json:"missing_result_count"`
	ToolUseFailureRate float64 `json:"tool_use_failure_rate"`
	ArgsTotalLength    int     `json:"args_total_length,omitempty"`
	ArgsMaxLength      int     `json:"args_max_length,omitempty"`
	ArgsAvgLength      float64 `json:"args_avg_length"`
	SeenInListingCount int     `json:"seen_in_listing_count"`
	DynamicCount       int     `json:"dynamic_count"`
	Installed          bool    `json:"installed"`
}

// SkillListingStat skill_listing 中出现的 skill
type SkillListingStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SkillProjectStat skill 与项目关系
type SkillProjectStat struct {
	SkillName          string  `json:"skill_name"`
	Project            string  `json:"project"`
	InvocationCount    int     `json:"invocation_count"`
	ToolUseCount       int     `json:"tool_use_count"`
	AttachmentCount    int     `json:"attachment_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResults     int     `json:"missing_results"`
	ToolUseFailureRate float64 `json:"tool_use_failure_rate"`
}

// SkillModelStat skill 与模型关系
type SkillModelStat struct {
	SkillName          string  `json:"skill_name"`
	Model              string  `json:"model"`
	InvocationCount    int     `json:"invocation_count"`
	ToolUseCount       int     `json:"tool_use_count"`
	AttachmentCount    int     `json:"attachment_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResults     int     `json:"missing_results"`
	ToolUseFailureRate float64 `json:"tool_use_failure_rate"`
}

// SkillAgentStat skill 与 agent/subagent 关系
type SkillAgentStat struct {
	SkillName       string `json:"skill_name"`
	AgentID         string `json:"agent_id"`
	IsSidechain     bool   `json:"is_sidechain"`
	InvocationCount int    `json:"invocation_count"`
}

// SkillSessionToolStat skill 激活后同 session 关联工具统计，不代表 skill 内部直接调用。
type SkillSessionToolStat struct {
	SkillName      string  `json:"skill_name"`
	Tool           string  `json:"tool"`
	CallCount      int     `json:"call_count"`
	FailureCount   int     `json:"failure_count"`
	MissingResults int     `json:"missing_results"`
	FailureRate    float64 `json:"failure_rate"`
}

// EventAnalysisData Claude Code 运行事件分析结果
type EventAnalysisData struct {
	TotalEvents       int                  `json:"total_events"`
	ByType            []EventTypeStat      `json:"by_type"`
	Hooks             []HookStatItem       `json:"hooks"`
	Skills            []SkillStatItem      `json:"skills"`
	PermissionModes   []PermissionModeStat `json:"permission_modes"`
	QueuedCommands    int                  `json:"queued_commands"`
	PlanModeCount     int                  `json:"plan_mode_count"`
	PlanModeExitCount int                  `json:"plan_mode_exit_count"`
	OpenedFiles       []FileAccessStat     `json:"opened_files"`
	Budget            *BudgetSummary       `json:"budget,omitempty"`
	Samples           []EventSample        `json:"samples"`
}

// EventTypeStat 顶层/附件事件类型统计
type EventTypeStat struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// HookStatItem hook 执行状态统计
type HookStatItem struct {
	HookName       string  `json:"hook_name"`
	HookEvent      string  `json:"hook_event"`
	SuccessCount   int     `json:"success_count"`
	CancelledCount int     `json:"cancelled_count"`
	ErrorCount     int     `json:"error_count"`
	TotalCount     int     `json:"total_count"`
	FailureRate    float64 `json:"failure_rate"`
	AvgDurationMs  float64 `json:"avg_duration_ms"`
	LastError      string  `json:"last_error,omitempty"`
	LastCommand    string  `json:"last_command,omitempty"`
}

// SkillStatItem skill 调用统计
type SkillStatItem struct {
	Name  string `json:"name"`
	Path  string `json:"path,omitempty"`
	Count int    `json:"count"`
}

// PermissionModeStat 权限模式统计
type PermissionModeStat struct {
	Mode  string `json:"mode"`
	Count int    `json:"count"`
}

// FileAccessStat 文件访问统计
type FileAccessStat struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// BudgetSummary 预算事件摘要
type BudgetSummary struct {
	LatestUsed      float64 `json:"latest_used"`
	LatestTotal     float64 `json:"latest_total"`
	LatestRemaining float64 `json:"latest_remaining"`
	MaxUsed         float64 `json:"max_used"`
	EventCount      int     `json:"event_count"`
}

// BudgetTimelineItem 单个预算事件
type BudgetTimelineItem struct {
	Timestamp string  `json:"timestamp"`
	Project   string  `json:"project"`
	SessionID string  `json:"session_id"`
	Used      float64 `json:"used"`
	Total     float64 `json:"total"`
	Remaining float64 `json:"remaining"`
}

// EventSample 运行事件样例
type EventSample struct {
	Type           string `json:"type"`
	Project        string `json:"project"`
	SessionID      string `json:"session_id"`
	Timestamp      string `json:"timestamp"`
	ContentPreview string `json:"content_preview"`
}

// SessionAnalysisData session 生命周期分析结果
type SessionAnalysisData struct {
	Sessions        []SessionAnalysisItem `json:"sessions"`
	TopFailures     []SessionAnalysisItem `json:"top_failures"`
	LongRunning     []SessionAnalysisItem `json:"long_running"`
	Outcomes        []SessionOutcomeStat  `json:"outcomes"`
	QueueOperations []QueueOperationStat  `json:"queue_operations"`
	Titles          []SessionTitleStat    `json:"titles"`
}

// SessionAnalysisItem 单个 session 摘要
type SessionAnalysisItem struct {
	SessionID             string `json:"session_id"`
	Project               string `json:"project"`
	Title                 string `json:"title,omitempty"`
	TitleSource           string `json:"title_source,omitempty"`
	FirstPromptPreview    string `json:"first_prompt_preview,omitempty"`
	LastPromptPreview     string `json:"last_prompt_preview,omitempty"`
	StartedAt             string `json:"started_at,omitempty"`
	EndedAt               string `json:"ended_at,omitempty"`
	DurationMs            int64  `json:"duration_ms"`
	SystemDurationMs      int64  `json:"system_duration_ms"`
	Outcome               string `json:"outcome"`
	StopReason            string `json:"stop_reason,omitempty"`
	PreventedContinuation bool   `json:"prevented_continuation"`
	MessageCount          int    `json:"message_count"`
	AssistantMessageCount int    `json:"assistant_message_count"`
	UserMessageCount      int    `json:"user_message_count"`
	SystemEventCount      int    `json:"system_event_count"`
	ToolCallCount         int    `json:"tool_call_count"`
	ToolFailureCount      int    `json:"tool_failure_count"`
	MissingResultCount    int    `json:"missing_result_count"`
	TotalTokens           int    `json:"total_tokens"`
	InputTokens           int    `json:"input_tokens"`
	OutputTokens          int    `json:"output_tokens"`
	PermissionModeChanges int    `json:"permission_mode_changes"`
	ModeChanges           int    `json:"mode_changes"`
	PlanModeCount         int    `json:"plan_mode_count"`
	PlanModeExitCount     int    `json:"plan_mode_exit_count"`
	QueueOperationCount   int    `json:"queue_operation_count"`
	HookCount             int    `json:"hook_count"`
	HookErrorCount        int    `json:"hook_error_count"`
	LastPermissionMode    string `json:"last_permission_mode,omitempty"`
	LastMode              string `json:"last_mode,omitempty"`
	QueueOperationsSample string `json:"queue_operations_sample,omitempty"`
}

// SessionOutcomeStat session outcome 聚合
type SessionOutcomeStat struct {
	Outcome string `json:"outcome"`
	Count   int    `json:"count"`
}

// QueueOperationStat queue-operation 聚合
type QueueOperationStat struct {
	Operation string `json:"operation"`
	Count     int    `json:"count"`
}

// SessionTitleStat title 来源聚合
type SessionTitleStat struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

// AgentAnalysisData agent/subagent 分析结果
type AgentAnalysisData struct {
	MainToolCalls      int             `json:"main_tool_calls"`
	SidechainToolCalls int             `json:"sidechain_tool_calls"`
	Agents             []AgentStatItem `json:"agents"`
}

// AgentStatItem 单个 agent 统计
type AgentStatItem struct {
	AgentID            string  `json:"agent_id"`
	AgentName          string  `json:"agent_name,omitempty"`
	IsSidechain        bool    `json:"is_sidechain"`
	SessionCount       int     `json:"session_count"`
	MessageCount       int     `json:"message_count"`
	ToolCallCount      int     `json:"tool_call_count"`
	ToolFailureCount   int     `json:"tool_failure_count"`
	MissingResultCount int     `json:"missing_result_count"`
	FailureRate        float64 `json:"failure_rate"`
}

// CommandAnalysisData Bash 与文件操作分析结果
type CommandAnalysisData struct {
	BashCommands   []BashCommandStat       `json:"bash_commands"`
	BashFamilies   []BashCommandFamilyStat `json:"bash_families"`
	RiskyCommands  []BashCommandStat       `json:"risky_commands"`
	FileOperations []FileOperationStat     `json:"file_operations"`
}

// BashCommandFamilyStat Bash 命令族统计
type BashCommandFamilyStat struct {
	Family             string  `json:"family"`
	CallCount          int     `json:"call_count"`
	SuccessCount       int     `json:"success_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResultCount int     `json:"missing_result_count"`
	FailureRate        float64 `json:"failure_rate"`
	TopCommand         string  `json:"top_command,omitempty"`
	TopCommandCalls    int     `json:"top_command_calls,omitempty"`
	SampleCommand      string  `json:"sample_command,omitempty"`
}

// BashCommandStat Bash 命令统计
type BashCommandStat struct {
	CommandName        string  `json:"command_name"`
	CallCount          int     `json:"call_count"`
	SuccessCount       int     `json:"success_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResultCount int     `json:"missing_result_count"`
	FailureRate        float64 `json:"failure_rate"`
	RiskLevel          string  `json:"risk_level,omitempty"`
	RiskReason         string  `json:"risk_reason,omitempty"`
	SampleCommand      string  `json:"sample_command,omitempty"`
}

// FileOperationStat 文件工具操作统计
type FileOperationStat struct {
	Operation          string  `json:"operation"`
	Path               string  `json:"path"`
	CallCount          int     `json:"call_count"`
	SuccessCount       int     `json:"success_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResultCount int     `json:"missing_result_count"`
	FailureRate        float64 `json:"failure_rate"`
}

// ToolStatItem 单个工具调用统计
type ToolStatItem struct {
	Tool               string  `json:"tool"`
	CallCount          int     `json:"call_count"`
	SuccessCount       int     `json:"success_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResultCount int     `json:"missing_result_count"`
	FailureRate        float64 `json:"failure_rate"`
}

// ToolModelStatItem 单个模型下的工具调用统计
type ToolModelStatItem struct {
	Model              string  `json:"model"`
	Tool               string  `json:"tool"`
	CallCount          int     `json:"call_count"`
	SuccessCount       int     `json:"success_count"`
	FailureCount       int     `json:"failure_count"`
	MissingResultCount int     `json:"missing_result_count"`
	FailureRate        float64 `json:"failure_rate"`
}

// ToolFailureSample 工具失败样例（只保存短摘要，不保存完整对话）
type ToolFailureSample struct {
	Tool           string `json:"tool"`
	Model          string `json:"model,omitempty"`
	Kind           string `json:"kind"`
	Category       string `json:"category,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Project        string `json:"project"`
	SessionID      string `json:"session_id"`
	Timestamp      string `json:"timestamp"`
	ContentPreview string `json:"content_preview"`
}

// FailureAnalysisData 失败原因细分分析结果
type FailureAnalysisData struct {
	TotalFailures int                      `json:"total_failures"`
	ByReason      []FailureReasonStat      `json:"by_reason"`
	ByToolReason  []FailureToolReasonStat  `json:"by_tool_reason"`
	ByModelReason []FailureModelReasonStat `json:"by_model_reason"`
	Samples       []ToolFailureSample      `json:"samples"`
}

// FailureReasonStat 按失败原因聚合
type FailureReasonStat struct {
	Category string `json:"category"`
	Reason   string `json:"reason"`
	Count    int    `json:"count"`
}

// FailureToolReasonStat 按工具和失败原因聚合
type FailureToolReasonStat struct {
	Tool     string  `json:"tool"`
	Category string  `json:"category"`
	Reason   string  `json:"reason"`
	Count    int     `json:"count"`
	Rate     float64 `json:"rate"`
}

// FailureModelReasonStat 按模型和失败原因聚合
type FailureModelReasonStat struct {
	Model    string  `json:"model"`
	Category string  `json:"category"`
	Reason   string  `json:"reason"`
	Count    int     `json:"count"`
	Rate     float64 `json:"rate"`
}

// TokenUsageBreakdown token 类型拆分
type TokenUsageBreakdown struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	ServerToolUseRequests    int `json:"server_tool_use_requests"`
	TotalTokens              int `json:"total_tokens"`
	BillableInputTokens      int `json:"billable_input_tokens"`
	RequestCount             int `json:"request_count"`
}

// CostModelStat 按模型统计 token
type CostModelStat struct {
	Model                 string  `json:"model"`
	RequestCount          int     `json:"request_count"`
	InputTokens           int     `json:"input_tokens"`
	OutputTokens          int     `json:"output_tokens"`
	CacheReadTokens       int     `json:"cache_read_input_tokens"`
	CacheCreationTokens   int     `json:"cache_creation_input_tokens"`
	ServerToolUseRequests int     `json:"server_tool_use_requests"`
	TotalTokens           int     `json:"total_tokens"`
	AvgOutputTokens       float64 `json:"avg_output_tokens"`
	CacheReadRatio        float64 `json:"cache_read_ratio"`
}

// CostProjectStat 按项目统计 token
type CostProjectStat struct {
	Project      string `json:"project"`
	RequestCount int    `json:"request_count"`
	TotalTokens  int    `json:"total_tokens"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// CostSessionStat 按会话统计 token
type CostSessionStat struct {
	SessionID    string `json:"session_id"`
	Project      string `json:"project"`
	Model        string `json:"model,omitempty"`
	RequestCount int    `json:"request_count"`
	TotalTokens  int    `json:"total_tokens"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// CostAgentStat 按 agent/subagent 统计 token
type CostAgentStat struct {
	AgentID      string `json:"agent_id"`
	IsSidechain  bool   `json:"is_sidechain"`
	RequestCount int    `json:"request_count"`
	TotalTokens  int    `json:"total_tokens"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// CostAnalysisData token/成本分析结果
type CostAnalysisData struct {
	Totals         TokenUsageBreakdown  `json:"totals"`
	ByModel        []CostModelStat      `json:"by_model"`
	ByProject      []CostProjectStat    `json:"by_project"`
	BySession      []CostSessionStat    `json:"by_session"`
	ByAgent        []CostAgentStat      `json:"by_agent"`
	BudgetTimeline []BudgetTimelineItem `json:"budget_timeline"`
}

// FileAnalysisData 文件与编辑质量分析结果
type FileAnalysisData struct {
	Totals       FileAnalysisTotals    `json:"totals"`
	HotFiles     []FileHotItem         `json:"hot_files"`     // 综合活跃度 Top N（跨操作类型按路径聚合）
	EditFailures []FileEditFailureItem `json:"edit_failures"` // 按文件的 Edit 失败原因分布
	Snapshots    []FileSnapshotItem    `json:"snapshots"`     // file-history-snapshot 统计
	EditedFiles  []FileEditedItem      `json:"edited_files"`  // edited_text_file 统计
}

// FileAnalysisTotals 文件分析汇总数字
type FileAnalysisTotals struct {
	UniqueFiles            int     `json:"unique_files"`              // 唯一文件数
	TotalReads             int     `json:"total_reads"`               // Read 总调用
	TotalEdits             int     `json:"total_edits"`               // Edit+MultiEdit 总调用
	TotalWrites            int     `json:"total_writes"`              // Write 总调用
	TotalEditFailures      int     `json:"total_edit_failures"`       // Edit 失败总数
	TotalWriteFailures     int     `json:"total_write_failures"`      // Write 失败总数
	OverallEditFailureRate float64 `json:"overall_edit_failure_rate"` // Edit 整体失败率 %
	SnapshotEventCount     int     `json:"snapshot_event_count"`      // snapshot 事件总数
	EditedFileCount        int     `json:"edited_file_count"`         // edited_text_file 数
}

// FileHotItem 最活跃文件（按路径聚合，跨操作类型）
type FileHotItem struct {
	Path            string  `json:"path"`
	ReadCount       int     `json:"read_count"`
	EditCount       int     `json:"edit_count"` // Edit + MultiEdit
	WriteCount      int     `json:"write_count"`
	TotalOps        int     `json:"total_ops"` // Read + Edit + Write
	SuccessCount    int     `json:"success_count"`
	FailureCount    int     `json:"failure_count"`
	MissingCount    int     `json:"missing_count"`
	FailureRate     float64 `json:"failure_rate"`      // 总失败率 %
	EditFailureRate float64 `json:"edit_failure_rate"` // 仅 Edit 操作的失败率 %
}

// FileEditFailureItem 单文件编辑失败细分
type FileEditFailureItem struct {
	Path           string                    `json:"path"`
	TotalFailures  int                       `json:"total_failures"`
	FailureReasons []FileFailureReasonDetail `json:"failure_reasons"` // 该文件的失败原因分布
}

// FileFailureReasonDetail 单文件内某原因的失败次数
type FileFailureReasonDetail struct {
	Reason string  `json:"reason"`
	Count  int     `json:"count"`
	Rate   float64 `json:"rate"` // 占该文件总失败的比例
}

// FileSnapshotItem file-history-snapshot 统计（按文件路径聚合）
type FileSnapshotItem struct {
	Path          string `json:"path"`
	SnapshotCount int    `json:"snapshot_count"`  // 出现在多少条 snapshot 事件中
	MaxVersion    int    `json:"max_version"`     // 最高版本号
	SessionCount  int    `json:"session_count"`   // 跨多少个不同 session
	IsUpdateCount int    `json:"is_update_count"` // 增量更新次数(isSnapshotUpdate=true)
}

// FileEditedItem edited_text_file attachment 统计
type FileEditedItem struct {
	Path       string `json:"path"`
	EditCount  int    `json:"edit_count"`
	AvgLines   int    `json:"avg_lines"`   // 平均行数
	AvgChars   int    `json:"avg_chars"`   // 平均字符数
	TotalChars int64  `json:"total_chars"` // 累计字符数(估算编辑规模)
}

// FileHotStat 中间聚合：按文件路径汇总所有操作
type FileHotStat struct {
	Path         string
	ReadCount    int
	EditCount    int // Edit + MultiEdit
	WriteCount   int
	SuccessCount int
	FailureCount int
	MissingCount int
}

// FileEditFailureAgg 中间聚合：按文件路径汇总 Edit 失败及原因
type FileEditFailureAgg struct {
	Path          string
	TotalFailures int
	ReasonCounts  map[string]int // reason -> count
}

// FileSnapshotAgg 中间聚合：file-history-snapshot 事件
type FileSnapshotAgg struct {
	Path          string
	SnapshotCount int
	MaxVersion    int
	SessionSet    map[string]bool // sessionID -> true (去重)
	IsUpdateCount int
}

// FileEditedAgg 中间聚合：edited_text_file attachment
type FileEditedAgg struct {
	Path       string
	EditCount  int
	TotalLines int
	TotalChars int64
}

// === Task / Plan 分析结构体（Milestone 4）===

// TaskPlanAnalysisData Task / Plan 结构分析结果（输出格式）
type TaskPlanAnalysisData struct {
	PlanLifecycle   PlanLifecycleData   `json:"plan_lifecycle"`   // Plan mode 生命周期
	PlanFiles       []PlanFileItem      `json:"plan_files"`       // 引用的计划文件
	GoalStatus      []GoalStatusItem    `json:"goal_status"`      // 目标达成状态
	ReminderSummary ReminderSummaryData `json:"reminder_summary"` // task/todo 提醒频率
	Tasks           TaskAnalysisData    `json:"tasks"`            // Task 分析（来自 tasks/ 目录）
}

// PlanLifecycleData Plan 模式生命周期统计
type PlanLifecycleData struct {
	EntryCount       int                  `json:"entry_count"`        // plan_mode 进入次数
	ExitCount        int                  `json:"exit_count"`         // plan_mode_exit 次数
	ReentryCount     int                  `json:"reentry_count"`      // plan_mode_reentry 次数
	UniquePlans      int                  `json:"unique_plans"`       // 涉及的唯一计划文件数
	SessionsWithPlan int                  `json:"sessions_with_plan"` // 使用过 plan mode 的 session 数
	ExitReasons      []PlanExitReasonItem `json:"exit_reasons"`       // 退出原因分布
}

// PlanExitReasonItem Plan mode 退出原因
type PlanExitReasonItem struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// PlanFileItem 单个计划文件引用记录
type PlanFileItem struct {
	FilePath  string `json:"file_path"`         // 完整路径
	FileName  string `json:"file_name"`         // 文件名(basename)
	CharCount int    `json:"char_count"`        // 内容字符数
	LineCount int    `json:"line_count"`        // 内容行数
	HasCode   bool   `json:"has_code"`          // 是否包含代码块
	Preview   string `json:"preview,omitempty"` // 内容前200字符预览
	RefCount  int    `json:"ref_count"`         // 被引用次数(去重session)
}

// GoalStatusItem 目标状态事件
type GoalStatusItem struct {
	SessionID string `json:"session_id"`
	Met       bool   `json:"met"`
	Sentinel  bool   `json:"sentinel"`
	Condition string `json:"condition,omitempty"`
	Timestamp string `json:"timestamp"`
}

// ReminderSummaryData 任务提醒汇总
type ReminderSummaryData struct {
	TaskReminderCount int                   `json:"task_reminder_count"` // task_reminder 总次数
	TodoReminderCount int                   `json:"todo_reminder_count"` // todo_reminder 总次数
	SessionsWithTask  int                   `json:"sessions_with_task"`  // 有task_reminder的session数
	SessionsWithTodo  int                   `json:"sessions_with_todo"`  // 有todo_reminder的session数
	TopTaskSessions   []ReminderSessionItem `json:"top_task_sessions"`   // task_reminder最多的Top N session
	TopTodoSessions   []ReminderSessionItem `json:"top_todo_sessions"`   // todo_reminder最多的Top N session
}

// ReminderSessionItem 单session的提醒次数
type ReminderSessionItem struct {
	SessionID string `json:"session_id"`
	Project   string `json:"project,omitempty"`
	Count     int    `json:"count"`
}

// TaskAnalysisData Task 分析结果（来自 tasks/ 目录扫描）
type TaskAnalysisData struct {
	TotalTasks         int               `json:"total_tasks"`
	TotalSessions      int               `json:"total_sessions"`
	StatusDistribution []TaskStatusItem  `json:"status_distribution"` // 状态分布
	SessionTaskCounts  []SessionTaskItem `json:"session_task_counts"` // per-session任务数Top N
	AvgTasksPerSession float64           `json:"avg_tasks_per_session"`
	CompletionRate     float64           `json:"completion_rate"` // 整体完成率%
}

// TaskStatusItem 任务状态分布项
type TaskStatusItem struct {
	Status string  `json:"status"`
	Count  int     `json:"count"`
	Rate   float64 `json:"rate"` // 占比%
}

// SessionTaskItem 单session任务概况
type SessionTaskItem struct {
	SessionID       string  `json:"session_id"`
	TotalTasks      int     `json:"total_tasks"`
	CompletedCount  int     `json:"completed_count"`
	PendingCount    int     `json:"pending_count"`
	InProgressCount int     `json:"in_progress_count"`
	CompletionRate  float64 `json:"completion_rate"`
}

// PlanModeAgg 中间聚合：plan_mode 事件
type PlanModeAgg struct {
	EntryCount    int                     // plan_mode 进入次数
	ExitCount     int                     // plan_mode_exit 次数
	ReentryCount  int                     // plan_mode_reentry 次数
	ExitReasons   map[string]int          // exitReason -> count
	PlanFilePaths map[string]*PlanFileAgg // filePath -> 详情
	SessionSet    map[string]bool         // sessionID去重
}

// PlanFileAgg 中间聚合：单个计划文件的引用详情
type PlanFileAgg struct {
	FilePath    string
	FileName    string
	PlanContent string // 完整markdown内容(仅首次)
	Preview     string // 前200字符
	CharCount   int
	LineCount   int
	HasCode     bool
	RefCount    int             // 跨session去重引用次数
	RefSessions map[string]bool // sessionID去重
}

// SerializedPlanFileAgg 可序列化版本（无 RefSessions map）
type SerializedPlanFileAgg struct {
	FilePath  string `json:"file_path"`
	FileName  string `json:"file_name"`
	Preview   string `json:"preview,omitempty"`
	CharCount int    `json:"char_count"`
	LineCount int    `json:"line_count"`
	HasCode   bool   `json:"has_code"`
	RefCount  int    `json:"ref_count"`
}

// SerializedPlanModeAgg 可序列化版本（用于缓存）
type SerializedPlanModeAgg struct {
	EntryCount    int                               `json:"entry_count"`
	ExitCount     int                               `json:"exit_count"`
	ReentryCount  int                               `json:"reentry_count"`
	ExitReasons   map[string]int                    `json:"exit_reasons"`
	PlanFilePaths map[string]*SerializedPlanFileAgg `json:"plan_file_paths"`
}

// GoalStatusAgg 中间聚合：goal_status 事件
type GoalStatusAgg struct {
	Items []GoalStatusItem
}

// ReminderAgg 中间聚合：task/todo reminder 频率
type ReminderAgg struct {
	TaskReminderCount   int
	TodoReminderCount   int
	TaskSessionCounts   map[string]int    // sessionID -> count
	TodoSessionCounts   map[string]int    // sessionID -> count
	TaskSessionProjects map[string]string // sessionID -> projectName
	TodoSessionProjects map[string]string // sessionID -> projectName
}

// TaskAgg 中间聚合：tasks/ 目录扫描结果
type TaskAgg struct {
	TotalTasks   int
	StatusCounts map[string]int             // status -> count
	SessionTasks map[string]*SessionTaskAgg // sessionUUID -> stats
}

// SessionTaskAgg 中间聚合：单session的任务统计
type SessionTaskAgg struct {
	SessionID         string
	TotalTasks        int
	CompletedCount    int
	PendingCount      int
	InProgressCount   int
	StatusCountsLocal map[string]int // 局部状态计数（用于合并到全局）
}

// TaskRaw tasks/*.json 原始 JSON 结构
type TaskRaw struct {
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	ActiveForm  string   `json:"activeForm"`
	Status      string   `json:"status"`
	Blocks      []string `json:"blocks"`
	BlockedBy   []string `json:"blockedBy"`
}

// === M5: Tool Performance & Quality Analysis ===

// ToolPerformanceData 工具性能与质量分析结果（输出格式）
type ToolPerformanceData struct {
	TotalPairedCalls    int                               `json:"total_paired_calls"`
	TotalErrors         int                               `json:"total_errors"`
	OverallErrorRate    float64                           `json:"overall_error_rate"`
	OverallAvgDuration  float64                           `json:"overall_avg_duration_ms"`
	ByCategory          []ToolPerfCategoryItem            `json:"by_category"`               // 按 BaseTool 分层截断后的类别列表
	CategoryGroups      map[string][]ToolPerfCategoryItem `json:"category_groups,omitempty"` // 按 BaseTool 分组（前端可选展示）
	SlowestCalls        []ToolSlowCallItem                `json:"slowest_calls"`
	QualityDistribution []QualityBucketItem               `json:"quality_distribution"`
}

// ToolPerfCategoryItem 单个细分类别的工具性能统计
type ToolPerfCategoryItem struct {
	Category        string  `json:"category"`          // "Bash:git", "Read:/path", "mcp__jina__web_search"
	BaseTool        string  `json:"base_tool"`         // "Bash", "Read", "mcp__jina__web_search"
	SubKey          string  `json:"sub_key,omitempty"` // "git", "/path/to/file", ""
	CallCount       int     `json:"call_count"`
	SuccessCount    int     `json:"success_count"`
	ErrorCount      int     `json:"error_count"`
	MissingCount    int     `json:"missing_count"`
	ErrorRate       float64 `json:"error_rate"`
	TotalDurationMs int64   `json:"total_duration_ms"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
	MinDurationMs   int64   `json:"min_duration_ms"`
	MaxDurationMs   int64   `json:"max_duration_ms"`
	TotalResultSize int64   `json:"total_result_size"`
	AvgResultSize   float64 `json:"avg_result_size"`
	EmptyResults    int     `json:"empty_results"`
	SampleInput     string  `json:"sample_input,omitempty"`
}

// ToolSlowCallItem 最慢的单次工具调用记录
type ToolSlowCallItem struct {
	Tool        string `json:"tool"`
	Category    string `json:"category"`
	Project     string `json:"project,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Model       string `json:"model,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`
	IsSidechain bool   `json:"is_sidechain,omitempty"`
	DurationMs  int64  `json:"duration_ms"`
	IsError     bool   `json:"is_error"`
	ResultSize  int    `json:"result_size"`
	Timestamp   string `json:"timestamp"`
	SampleInput string `json:"sample_input,omitempty"`
}

// QualityBucketItem 结果质量分桶
type QualityBucketItem struct {
	Bucket string  `json:"bucket"` // "空", "极小(≤50B)", ...
	Count  int     `json:"count"`
	Rate   float64 `json:"rate"`
}

// ToolPerfAgg 中间聚合：按细分类别的工具性能统计
// key = category, e.g. "Bash\x00git" or "Read\x00/path/to/file" or "mcp__jina__web_search\x00"
type ToolPerfAgg struct {
	CallCount       int
	SuccessCount    int
	ErrorCount      int
	MissingCount    int
	TotalDurationMs int64
	MinDurationMs   int64 // 初始化为 math.MaxInt64，首条有效数据时更新
	MaxDurationMs   int64
	TotalResultSize int64
	EmptyResults    int
	SampleInput     string
}

// WorkHoursStats 工作时段统计
type WorkHoursStats struct {
	HourlyData     []HourlyItem `json:"hourly_data"` // 每小时数据
	WorkHoursCount int          `json:"work_hours"`  // 工作时段(9-18点)总次数
	OffHoursCount  int          `json:"off_hours"`   // 非工作时段总次数
	WorkHoursRatio float64      `json:"work_ratio"`  // 工作时段占比
	PeakHour       int          `json:"peak_hour"`   // 峰值小时
	PeakHourCount  int          `json:"peak_count"`  // 峰值小时次数
}

// HourlyItem 单小时数据
type HourlyItem struct {
	Hour       int    `json:"hour"`         // 小时(0-23)
	HourLabel  string `json:"hour_label"`   // 标签 "09:00"
	Count      int    `json:"count"`        // 次数
	IsWorkHour bool   `json:"is_work_hour"` // 是否工作时段
}

// ProjectAggregate 一次遍历获取的所有统计数据
type ProjectAggregate struct {
	ProjectStats          map[string]*ProjectStatItem             `json:"-"`                // 项目统计（map用于快速查找）
	Projects              []ProjectStatItem                       `json:"projects"`         // 项目列表（排序后）
	WeekdayData           [7]WeekdayItem                          `json:"-"`                // 星期数据
	WeekdayStats          *WeekdayStats                           `json:"weekday"`          // 星期统计（输出格式）
	DailyActivity         map[string]int                          `json:"-"`                // 每日消息数（map）
	DailyActivityList     []DailyActivity                         `json:"daily"`            // 每日活动（输出格式）
	DailySessions         map[string]map[string]bool              `json:"-"`                // 每日会话集 date→sessionID→true（用于提取SessionStats，避免重复解析）
	DailyProjectCounts    map[string]map[string]int               `json:"-"`                // 每日项目消息数 date→project→count
	DailyModelCounts      map[string]map[string]int               `json:"-"`                // 每日模型请求数 date→model→count
	DailyModelTokens      map[string]map[string]int               `json:"-"`                // 每日模型 token 数 date→model→tokens
	DailyHourlyCounts     map[string][24]int                      `json:"-"`                // 每日小时消息数 date→hour→count
	DailyRuntime          map[string]*ProjectAggregate            `json:"-"`                // 每日运行时聚合（工具/成本/失败/性能等）
	DailyProjectRuntime   map[string]map[string]*ProjectAggregate `json:"-"`                // 每日项目运行时聚合 date→project→aggregate
	DailySessionRuntime   map[string]map[string]*ProjectAggregate `json:"-"`                // 每日 Session 运行时聚合 date→session→aggregate
	HourlyCounts          [24]int                                 `json:"-"`                // 小时统计
	HourlyData            []HourlyItem                            `json:"-"`                // 小时数据
	ModelUsage            map[string]*ModelUsageItem              `json:"-"`                // 模型使用（map）
	ModelUsageList        []ModelUsageItem                        `json:"models"`           // 模型使用（输出格式）
	CostModelStats        map[string]*CostModelStat               `json:"-"`                // 模型 token 统计
	CostProjectStats      map[string]*CostProjectStat             `json:"-"`                // 项目 token 统计
	CostSessionStats      map[string]*CostSessionStat             `json:"-"`                // 会话 token 统计
	CostAgentStats        map[string]*CostAgentStat               `json:"-"`                // agent token 统计
	BudgetTimeline        []BudgetTimelineItem                    `json:"-"`                // 预算事件时间线
	CostAnalysis          *CostAnalysisData                       `json:"costs"`            // 成本/token 分析（输出格式）
	ToolStats             map[string]*ToolStatItem                `json:"-"`                // 工具调用统计
	ToolModelStats        map[string]*ToolModelStatItem           `json:"-"`                // 模型+工具调用统计
	ToolAnalysis          *ToolAnalysisData                       `json:"tools"`            // 工具分析（输出格式）
	FailureReasons        map[string]*FailureReasonStat           `json:"-"`                // 失败原因统计
	FailureToolReasons    map[string]*FailureToolReasonStat       `json:"-"`                // 工具+失败原因统计
	FailureModelReasons   map[string]*FailureModelReasonStat      `json:"-"`                // 模型+失败原因统计
	FailureSamples        []ToolFailureSample                     `json:"-"`                // 失败样例
	FailureAnalysis       *FailureAnalysisData                    `json:"failures"`         // 失败原因细分（输出格式）
	SessionStatsMap       map[string]*SessionAnalysisItem         `json:"-"`                // session 生命周期统计
	SessionQueueOps       map[string]int                          `json:"-"`                // queue operation 聚合
	SessionAnalysis       *SessionAnalysisData                    `json:"session_analysis"` // session 生命周期分析（输出格式）
	EventTypes            map[string]int                          `json:"-"`                // 运行事件类型
	HookStats             map[string]*HookStatItem                `json:"-"`                // hook 统计
	SkillStats            map[string]*SkillStatItem               `json:"-"`                // skill 统计
	InstalledSkills       map[string]*InstalledSkillItem          `json:"-"`                // 本地安装 skill
	SkillUsageStats       map[string]*SkillUsageStat              `json:"-"`                // skill 调用统计
	SkillListingStats     map[string]int                          `json:"-"`                // skill_listing 中出现次数
	SkillProjectStats     map[string]*SkillProjectStat            `json:"-"`                // skill + project
	SkillModelStats       map[string]*SkillModelStat              `json:"-"`                // skill + model
	SkillAgentStats       map[string]*SkillAgentStat              `json:"-"`                // skill + agent
	SkillSessionToolStats map[string]*SkillSessionToolStat        `json:"-"`                // skill + session associated tool
	SkillListingEvents    int                                     `json:"-"`                // skill_listing attachment 数
	SkillInitialListings  int                                     `json:"-"`                // isInitial=true listing 数
	DynamicSkillEvents    int                                     `json:"-"`                // dynamic_skill attachment 数
	SkillAnalysis         *SkillAnalysisData                      `json:"skill_analysis"`   // skill 分析（输出格式）
	PermissionModes       map[string]int                          `json:"-"`                // 权限模式统计
	OpenedFiles           map[string]*FileAccessStat              `json:"-"`                // IDE 打开文件统计
	BudgetSummary         *BudgetSummary                          `json:"-"`                // 预算事件摘要
	EventSamples          []EventSample                           `json:"-"`                // 事件样例
	EventAnalysis         *EventAnalysisData                      `json:"events"`           // 事件分析（输出格式）
	AgentStats            map[string]*AgentStatItem               `json:"-"`                // agent 统计
	AgentSessions         map[string]map[string]bool              `json:"-"`                // agent 会话去重
	AgentAnalysis         *AgentAnalysisData                      `json:"agents"`           // agent 分析（输出格式）
	BashCommandStats      map[string]*BashCommandStat             `json:"-"`                // Bash 命令统计
	FileOperationStats    map[string]*FileOperationStat           `json:"-"`                // 文件操作统计
	CommandAnalysis       *CommandAnalysisData                    `json:"commands"`         // 命令/文件分析（输出格式）
	FileHotStats          map[string]*FileHotStat                 `json:"-"`                // 文件活跃度统计（按路径聚合）
	FileEditFailures      map[string]*FileEditFailureAgg          `json:"-"`                // 文件编辑失败（按路径+原因聚合）
	FileSnapshotStats     map[string]*FileSnapshotAgg             `json:"-"`                // file-history-snapshot 统计
	FileEditedStats       map[string]*FileEditedAgg               `json:"-"`                // edited_text_file 统计
	FileAnalysis          *FileAnalysisData                       `json:"file_analysis"`    // 文件与编辑质量分析（输出格式）
	// --- task_plan_analysis (Milestone 4) ---
	PlanModeAgg      *PlanModeAgg          `json:"-"`                  // plan_mode 事件聚合
	GoalStatusAgg    *GoalStatusAgg        `json:"-"`                  // goal_status 事件聚合
	ReminderAgg      *ReminderAgg          `json:"-"`                  // reminder 频率聚合
	TaskAgg          *TaskAgg              `json:"-"`                  // tasks/ 目录扫描结果
	TaskPlanAnalysis *TaskPlanAnalysisData `json:"task_plan_analysis"` // Task/Plan 分析（输出格式）
	// --- tool_performance (Milestone 5) ---
	ToolPerfStats   map[string]*ToolPerfAgg `json:"-"`                // M5: 工具性能中间聚合
	SlowestCalls    []ToolSlowCallItem      `json:"-"`                // M5: 全局最慢 Top-N 调用
	ToolPerformance *ToolPerformanceData    `json:"tool_performance"` // M5: 输出格式
	WorkHoursStats  *WorkHoursStats         `json:"work_hours"`       // 工作时段统计
	mu              sync.RWMutex            `json:"-"`                // 保护并发写入
}

// ProjectRecord projects/*.jsonl 记录
type ProjectRecord struct {
	ParentUUID            string          `json:"parentUuid"`
	IsSidechain           bool            `json:"isSidechain"`
	UserType              string          `json:"userType"`
	Cwd                   string          `json:"cwd"`
	SessionID             string          `json:"sessionId"`
	Version               string          `json:"version"`
	GitBranch             string          `json:"gitBranch"`
	AgentID               string          `json:"agentId"`
	AttributionSkill      string          `json:"attributionSkill"`
	Type                  string          `json:"type"`    // "user" | "assistant"
	Message               json.RawMessage `json:"message"` // 可以是 user 或 assistant 消息
	Attachment            json.RawMessage `json:"attachment"`
	Content               json.RawMessage `json:"content"`
	Name                  string          `json:"name"`
	PermissionMode        string          `json:"permissionMode"`
	Mode                  string          `json:"mode"`
	Subtype               string          `json:"subtype"`
	StopReason            string          `json:"stopReason"`
	Status                string          `json:"status"`
	LastPrompt            string          `json:"lastPrompt"`
	AITitle               string          `json:"aiTitle"`
	CustomTitle           string          `json:"customTitle"`
	Operation             string          `json:"operation"`
	DurationMs            int             `json:"durationMs"`
	MessageCount          int             `json:"messageCount"`
	HookCount             int             `json:"hookCount"`
	HookErrors            []interface{}   `json:"hookErrors"`
	PreventedContinuation bool            `json:"preventedContinuation"`
	Timestamp             string          `json:"timestamp"`
}

// AssistantMessage assistant 消息详情
type AssistantMessage struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Model   string             `json:"model"`
	Content []AssistantContent `json:"content"`
	Usage   struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		ServerToolUse            struct {
			WebSearchRequests int `json:"web_search_requests"`
			WebFetchRequests  int `json:"web_fetch_requests"`
		} `json:"server_tool_use"`
	} `json:"usage"`
}

// AssistantContent 支持多种内容类型（text, thinking）
type AssistantContent struct {
	Type      string          `json:"type"`               // "text" | "thinking" | "tool_use" | "tool_result"
	Text      string          `json:"text"`               // text 类型内容
	Thinking  string          `json:"thinking,omitempty"` // thinking 类型内容
	ID        string          `json:"id,omitempty"`       // tool_use ID
	Name      string          `json:"name,omitempty"`     // tool_use name
	Input     json.RawMessage `json:"input,omitempty"`    // tool_use input
	ToolUseID string          `json:"tool_use_id"`        // tool_result 关联 ID
	Content   json.RawMessage `json:"content,omitempty"`  // tool_result content
	IsError   bool            `json:"is_error,omitempty"` // tool_result 显式失败
}

// SessionIndexEntry sessions-index.json 单个条目
type SessionIndexEntry struct {
	SessionID    string `json:"sessionId"`
	FullPath     string `json:"fullPath"`
	FileMtime    int64  `json:"fileMtime"`
	FirstPrompt  string `json:"firstPrompt"`
	Summary      string `json:"summary"`
	MessageCount int    `json:"messageCount"`
	Created      string `json:"created"`
	Modified     string `json:"modified"`
	ProjectPath  string `json:"projectPath"`
	IsSidechain  bool   `json:"isSidechain"`
}

// SessionIndexResult sessions-index.json 解析结果
type SessionIndexResult struct {
	Version int                 `json:"version"`
	Entries []SessionIndexEntry `json:"entries"`
}

// RuntimeToolSignal runtime debug 日志中识别到的 MCP 工具信号。
type RuntimeToolSignal struct {
	Tool   string `json:"tool"`
	Server string `json:"server"`
	Count  int    `json:"count"`
}

// DebugFileInfo debug 文件信息
type DebugFileInfo struct {
	Path    string
	ModTime time.Time
}

type pendingToolCall struct {
	ID              string
	Tool            string
	Model           string
	Project         string
	SessionID       string
	AgentID         string
	IsSidechain     bool
	Timestamp       time.Time
	Input           json.RawMessage
	CommandName     string
	FileOpKey       string
	SkillName       string
	SkillArgsLength int
	ChainSkills     []string
}
