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
	BashCommands   []BashCommandStat   `json:"bash_commands"`
	RiskyCommands  []BashCommandStat   `json:"risky_commands"`
	FileOperations []FileOperationStat `json:"file_operations"`
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
	ProjectStats        map[string]*ProjectStatItem        `json:"-"`                // 项目统计（map用于快速查找）
	Projects            []ProjectStatItem                  `json:"projects"`         // 项目列表（排序后）
	WeekdayData         [7]WeekdayItem                     `json:"-"`                // 星期数据
	WeekdayStats        *WeekdayStats                      `json:"weekday"`          // 星期统计（输出格式）
	DailyActivity       map[string]int                     `json:"-"`                // 每日消息数（map）
	DailyActivityList   []DailyActivity                    `json:"daily"`            // 每日活动（输出格式）
	DailySessions       map[string]map[string]bool         `json:"-"`                // 每日会话集 date→sessionID→true（用于提取SessionStats，避免重复解析）
	HourlyCounts        [24]int                            `json:"-"`                // 小时统计
	HourlyData          []HourlyItem                       `json:"-"`                // 小时数据
	ModelUsage          map[string]*ModelUsageItem         `json:"-"`                // 模型使用（map）
	ModelUsageList      []ModelUsageItem                   `json:"models"`           // 模型使用（输出格式）
	CostModelStats      map[string]*CostModelStat          `json:"-"`                // 模型 token 统计
	CostProjectStats    map[string]*CostProjectStat        `json:"-"`                // 项目 token 统计
	CostSessionStats    map[string]*CostSessionStat        `json:"-"`                // 会话 token 统计
	CostAgentStats      map[string]*CostAgentStat          `json:"-"`                // agent token 统计
	BudgetTimeline      []BudgetTimelineItem               `json:"-"`                // 预算事件时间线
	CostAnalysis        *CostAnalysisData                  `json:"costs"`            // 成本/token 分析（输出格式）
	ToolStats           map[string]*ToolStatItem           `json:"-"`                // 工具调用统计
	ToolModelStats      map[string]*ToolModelStatItem      `json:"-"`                // 模型+工具调用统计
	ToolAnalysis        *ToolAnalysisData                  `json:"tools"`            // 工具分析（输出格式）
	FailureReasons      map[string]*FailureReasonStat      `json:"-"`                // 失败原因统计
	FailureToolReasons  map[string]*FailureToolReasonStat  `json:"-"`                // 工具+失败原因统计
	FailureModelReasons map[string]*FailureModelReasonStat `json:"-"`                // 模型+失败原因统计
	FailureSamples      []ToolFailureSample                `json:"-"`                // 失败样例
	FailureAnalysis     *FailureAnalysisData               `json:"failures"`         // 失败原因细分（输出格式）
	SessionStatsMap     map[string]*SessionAnalysisItem    `json:"-"`                // session 生命周期统计
	SessionQueueOps     map[string]int                     `json:"-"`                // queue operation 聚合
	SessionAnalysis     *SessionAnalysisData               `json:"session_analysis"` // session 生命周期分析（输出格式）
	EventTypes          map[string]int                     `json:"-"`                // 运行事件类型
	HookStats           map[string]*HookStatItem           `json:"-"`                // hook 统计
	SkillStats          map[string]*SkillStatItem          `json:"-"`                // skill 统计
	PermissionModes     map[string]int                     `json:"-"`                // 权限模式统计
	OpenedFiles         map[string]*FileAccessStat         `json:"-"`                // IDE 打开文件统计
	BudgetSummary       *BudgetSummary                     `json:"-"`                // 预算事件摘要
	EventSamples        []EventSample                      `json:"-"`                // 事件样例
	EventAnalysis       *EventAnalysisData                 `json:"events"`           // 事件分析（输出格式）
	AgentStats          map[string]*AgentStatItem          `json:"-"`                // agent 统计
	AgentSessions       map[string]map[string]bool         `json:"-"`                // agent 会话去重
	AgentAnalysis       *AgentAnalysisData                 `json:"agents"`           // agent 分析（输出格式）
	BashCommandStats    map[string]*BashCommandStat        `json:"-"`                // Bash 命令统计
	FileOperationStats  map[string]*FileOperationStat      `json:"-"`                // 文件操作统计
	CommandAnalysis     *CommandAnalysisData               `json:"commands"`         // 命令/文件分析（输出格式）
	WorkHoursStats      *WorkHoursStats                    `json:"work_hours"`       // 工作时段统计
	mu                  sync.RWMutex                       `json:"-"`                // 保护并发写入
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

// MCPToolStats MCP工具统计
type MCPToolStats struct {
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
	ID          string
	Tool        string
	Model       string
	Project     string
	SessionID   string
	AgentID     string
	IsSidechain bool
	Timestamp   time.Time
	Input       json.RawMessage
	CommandName string
	FileOpKey   string
}
