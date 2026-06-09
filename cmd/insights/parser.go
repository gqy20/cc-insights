package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
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
	FailureKinds   []ToolFailureKind   `json:"failure_kinds"`
	FailureSamples []ToolFailureSample `json:"failure_samples"`
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

// EventSample 运行事件样例
type EventSample struct {
	Type           string `json:"type"`
	Project        string `json:"project"`
	SessionID      string `json:"session_id"`
	Timestamp      string `json:"timestamp"`
	ContentPreview string `json:"content_preview"`
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

// ToolFailureKind 工具失败类型聚合
type ToolFailureKind struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// ToolFailureSample 工具失败样例（只保存短摘要，不保存完整对话）
type ToolFailureSample struct {
	Tool           string `json:"tool"`
	Model          string `json:"model,omitempty"`
	Kind           string `json:"kind"`
	Project        string `json:"project"`
	SessionID      string `json:"session_id"`
	Timestamp      string `json:"timestamp"`
	ContentPreview string `json:"content_preview"`
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
	ProjectStats       map[string]*ProjectStatItem   `json:"-"`          // 项目统计（map用于快速查找）
	Projects           []ProjectStatItem             `json:"projects"`   // 项目列表（排序后）
	WeekdayData        [7]WeekdayItem                `json:"-"`          // 星期数据
	WeekdayStats       *WeekdayStats                 `json:"weekday"`    // 星期统计（输出格式）
	DailyActivity      map[string]int                `json:"-"`          // 每日消息数（map）
	DailyActivityList  []DailyActivity               `json:"daily"`      // 每日活动（输出格式）
	DailySessions      map[string]map[string]bool    `json:"-"`          // 每日会话集 date→sessionID→true（用于提取SessionStats，避免重复解析）
	HourlyCounts       [24]int                       `json:"-"`          // 小时统计
	HourlyData         []HourlyItem                  `json:"-"`          // 小时数据
	ModelUsage         map[string]*ModelUsageItem    `json:"-"`          // 模型使用（map）
	ModelUsageList     []ModelUsageItem              `json:"models"`     // 模型使用（输出格式）
	ToolStats          map[string]*ToolStatItem      `json:"-"`          // 工具调用统计
	ToolModelStats     map[string]*ToolModelStatItem `json:"-"`          // 模型+工具调用统计
	ToolFailureKinds   map[string]int                `json:"-"`          // 工具失败类型
	ToolAnalysis       *ToolAnalysisData             `json:"tools"`      // 工具分析（输出格式）
	EventTypes         map[string]int                `json:"-"`          // 运行事件类型
	HookStats          map[string]*HookStatItem      `json:"-"`          // hook 统计
	SkillStats         map[string]*SkillStatItem     `json:"-"`          // skill 统计
	PermissionModes    map[string]int                `json:"-"`          // 权限模式统计
	OpenedFiles        map[string]*FileAccessStat    `json:"-"`          // IDE 打开文件统计
	BudgetSummary      *BudgetSummary                `json:"-"`          // 预算事件摘要
	EventSamples       []EventSample                 `json:"-"`          // 事件样例
	EventAnalysis      *EventAnalysisData            `json:"events"`     // 事件分析（输出格式）
	AgentStats         map[string]*AgentStatItem     `json:"-"`          // agent 统计
	AgentSessions      map[string]map[string]bool    `json:"-"`          // agent 会话去重
	AgentAnalysis      *AgentAnalysisData            `json:"agents"`     // agent 分析（输出格式）
	BashCommandStats   map[string]*BashCommandStat   `json:"-"`          // Bash 命令统计
	FileOperationStats map[string]*FileOperationStat `json:"-"`          // 文件操作统计
	CommandAnalysis    *CommandAnalysisData          `json:"commands"`   // 命令/文件分析（输出格式）
	WorkHoursStats     *WorkHoursStats               `json:"work_hours"` // 工作时段统计
	mu                 sync.RWMutex                  `json:"-"`          // 保护并发写入
}

// ProjectRecord projects/*.jsonl 记录
type ProjectRecord struct {
	ParentUUID     string          `json:"parentUuid"`
	IsSidechain    bool            `json:"isSidechain"`
	UserType       string          `json:"userType"`
	Cwd            string          `json:"cwd"`
	SessionID      string          `json:"sessionId"`
	Version        string          `json:"version"`
	GitBranch      string          `json:"gitBranch"`
	AgentID        string          `json:"agentId"`
	Type           string          `json:"type"`    // "user" | "assistant"
	Message        json.RawMessage `json:"message"` // 可以是 user 或 assistant 消息
	Attachment     json.RawMessage `json:"attachment"`
	Content        json.RawMessage `json:"content"`
	Name           string          `json:"name"`
	PermissionMode string          `json:"permissionMode"`
	Timestamp      string          `json:"timestamp"`
}

// AssistantMessage assistant 消息详情
type AssistantMessage struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Model   string             `json:"model"`
	Content []AssistantContent `json:"content"`
	Usage   struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
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

var (
	mcpPattern      = regexp.MustCompile(`mcp__(\w+)__(\w+)`)
	toolFailureExpr = regexp.MustCompile(`(?i)(error|failed|exception|traceback|timed out|timeout|permission denied|no such file|command failed|exit code|is_error|\"success\"\s*:\s*false|失败|错误|异常|超时|权限|不存在)`)
)

// ParseHistoryWithFilter 带时间过滤解析 history.jsonl
func ParseHistoryWithFilter(tf TimeFilter) ([]CommandStats, map[string]int, error) {
	path := GetDataPath("history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 history.jsonl 失败: %w", err)
	}
	defer f.Close()

	cmdCounts := make(map[string]int)
	hourlyCounts := make(map[string]int)

	decoder := json.NewDecoder(f)
	for {
		var record HistoryRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// 时间过滤
		recordTime := time.Unix(record.Timestamp/1000, 0)
		if !tf.Contains(recordTime) {
			continue
		}

		// 统计 slash commands
		if strings.HasPrefix(record.Display, "/") {
			parts := strings.Fields(record.Display)
			if len(parts) > 0 {
				cmdCounts[parts[0]]++
			}
		}

		// 统计小时分布
		hour := fmt.Sprintf("%02d", recordTime.Hour())
		hourlyCounts[hour]++
	}

	// 转换为切片并排序
	var cmdStats []CommandStats
	for cmd, count := range cmdCounts {
		cmdStats = append(cmdStats, CommandStats{Command: cmd, Count: count})
	}
	sort.Slice(cmdStats, func(i, j int) bool {
		return cmdStats[i].Count > cmdStats[j].Count
	})

	return cmdStats, hourlyCounts, nil
}

// ParseHistory 解析 history.jsonl（全部数据）
func ParseHistory() ([]CommandStats, map[string]int, error) {
	return ParseHistoryWithFilter(TimeFilter{Start: nil, End: nil})
}

// ParseStatsCacheWithFilter 带时间过滤解析 stats-cache.json
func ParseStatsCacheWithFilter(tf TimeFilter) (*StatsCache, error) {
	cache, err := ParseStatsCache()
	if err != nil {
		return nil, err
	}

	// 过滤每日活动
	cache.DailyActivity = FilterDailyActivity(cache.DailyActivity, tf)

	return cache, nil
}

// ParseStatsCache 解析 stats-cache.json
func ParseStatsCache() (*StatsCache, error) {
	path := GetDataPath("stats-cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 stats-cache.json 失败: %w", err)
	}

	var cache StatsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("解析 stats-cache.json 失败: %w", err)
	}

	return &cache, nil
}

// ParseDebugLogs 解析 debug 日志目录
func ParseDebugLogs() ([]MCPToolStats, error) {
	debugDir := GetDataPath("debug")

	entries, err := os.ReadDir(debugDir)
	if err != nil {
		return nil, fmt.Errorf("读取 debug 目录失败: %w", err)
	}

	// 并发解析
	var wg sync.WaitGroup
	results := make(chan map[string]int, len(entries))
	workers := 8

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			files = append(files, filepath.Join(debugDir, entry.Name()))
		}
	}

	// 分批处理
	batchSize := (len(files) + workers - 1) / workers
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		wg.Add(1)
		go func(files []string) {
			defer wg.Done()
			toolCounts := make(map[string]int)

			for _, file := range files {
				parseDebugFile(file, toolCounts)
			}
			results <- toolCounts
		}(files[i:end])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 汇总结果
	aggregateCounts := make(map[string]int)
	for counts := range results {
		for tool, count := range counts {
			aggregateCounts[tool] += count
		}
	}

	// 转换为切片
	var toolStats []MCPToolStats
	for fullTool, count := range aggregateCounts {
		parts := strings.Split(fullTool, "::")
		if len(parts) == 2 {
			toolStats = append(toolStats, MCPToolStats{
				Tool:   parts[1],
				Server: parts[0],
				Count:  count,
			})
		}
	}
	sort.Slice(toolStats, func(i, j int) bool {
		return toolStats[i].Count > toolStats[j].Count
	})

	return toolStats, nil
}

// ParseDebugLogsWithFilter 带时间过滤解析 debug 日志目录
func ParseDebugLogsWithFilter(tf TimeFilter) ([]MCPToolStats, error) {
	debugDir := GetDataPath("debug")

	entries, err := os.ReadDir(debugDir)
	if err != nil {
		return nil, fmt.Errorf("读取 debug 目录失败: %w", err)
	}

	// 获取文件信息用于时间过滤
	var fileInfos []DebugFileInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			info, _ := entry.Info()
			fileInfos = append(fileInfos, DebugFileInfo{
				Path:    filepath.Join(debugDir, entry.Name()),
				ModTime: info.ModTime(),
			})
		}
	}

	// 时间过滤
	filteredFiles := FilterDebugFiles(fileInfos, tf)

	// 并发解析
	var wg sync.WaitGroup
	results := make(chan map[string]int, len(filteredFiles))
	workers := 8

	files := make([]string, 0, len(filteredFiles))
	for _, info := range filteredFiles {
		files = append(files, info.Path)
	}

	// 分批处理
	batchSize := (len(files) + workers - 1) / workers
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		wg.Add(1)
		go func(files []string) {
			defer wg.Done()
			toolCounts := make(map[string]int)

			for _, file := range files {
				parseDebugFile(file, toolCounts)
			}
			results <- toolCounts
		}(files[i:end])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 汇总结果
	aggregateCounts := make(map[string]int)
	for counts := range results {
		for tool, count := range counts {
			aggregateCounts[tool] += count
		}
	}

	// 转换为切片
	var toolStats []MCPToolStats
	for fullTool, count := range aggregateCounts {
		parts := strings.Split(fullTool, "::")
		if len(parts) == 2 {
			toolStats = append(toolStats, MCPToolStats{
				Tool:   parts[1],
				Server: parts[0],
				Count:  count,
			})
		}
	}
	sort.Slice(toolStats, func(i, j int) bool {
		return toolStats[i].Count > toolStats[j].Count
	})

	return toolStats, nil
}

func parseDebugFile(path string, counts map[string]int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		matches := mcpPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				key := match[1] + "::" + match[2]
				counts[key]++
			}
		}
	}
}

// GetDailyTrend 获取每日趋势（最近7天）
func GetDailyTrend() ([]string, []int, error) {
	cache, err := ParseStatsCache()
	if err != nil {
		return nil, nil, err
	}

	// 取最近7天
	n := len(cache.DailyActivity)
	start := 0
	if n > 7 {
		start = n - 7
	}

	var dates []string
	var counts []int
	for i := start; i < n; i++ {
		dates = append(dates, cache.DailyActivity[i].Date)
		counts = append(counts, cache.DailyActivity[i].MessageCount)
	}

	return dates, counts, nil
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

// ParseSessionIndex 解析 sessions-index.json 文件
// 返回会话索引数据，可用于更准确的会话统计
func ParseSessionIndex(projectPath string) (*SessionIndexResult, error) {
	if projectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}

	indexPath := filepath.Join(projectPath, "sessions-index.json")
	f, err := os.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("open sessions-index.json: %w", err)
	}
	defer f.Close()

	var result SessionIndexResult
	if err := json.NewDecoder(f).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sessions-index.json: %w", err)
	}

	return &result, nil
}

// ParseSessionStatsWithFilter 带时间过滤解析会话统计
// 使用 ParseProjectsConcurrentOnce 一次遍历获取所有数据，然后从 aggregate 提取 session 统计
// 避免冗余的二次文件遍历
func ParseSessionStatsWithFilter(tf TimeFilter) (*SessionStats, error) {
	agg, err := ParseProjectsConcurrentOnce(tf)
	if err != nil {
		return nil, err
	}
	return extractSessionStatsFromAggregate(agg)
}

// ParseProjectsConcurrentOnce 一次遍历并发解析所有项目统计
// 这个函数将所有统计合并到一次遍历中，大幅提升性能
func ParseProjectsConcurrentOnce(tf TimeFilter) (*ProjectAggregate, error) {
	return ParseProjectsConcurrentOnceFromDir(tf, cfg.DataDir)
}

// ParseProjectsConcurrentOnceFromDir 一次遍历并发解析指定数据目录下的项目统计
func ParseProjectsConcurrentOnceFromDir(tf TimeFilter, dataDir string) (*ProjectAggregate, error) {
	projectsDir := filepath.Join(dataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 projects 目录失败: %w", err)
	}

	aggregate := newProjectAggregate()

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		projectFiles, err := projectJSONLFiles(projectDir)
		if err != nil {
			continue
		}
		files = append(files, projectFiles...)
	}
	sort.Strings(files)

	maxWorkers := runtime.NumCPU()
	if len(files) < maxWorkers {
		maxWorkers = len(files)
	}
	if maxWorkers == 0 {
		aggregate.finalize()
		return aggregate, nil
	}

	jobs := make(chan string, maxWorkers*2)
	results := make(chan *ProjectAggregate, maxWorkers)
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerAggregate := newProjectAggregate()
			for filePath := range jobs {
				parseProjectFileAggregate(filePath, tf, workerAggregate)
			}
			results <- workerAggregate
		}()
	}

	for _, filePath := range files {
		jobs <- filePath
	}
	close(jobs)
	go func() {
		wg.Wait()
		close(results)
	}()

	for fileAggregate := range results {
		mergeProjectAggregate(aggregate, fileAggregate)
	}

	// 后处理：生成输出格式数据
	aggregate.finalize()

	return aggregate, nil
}

func newProjectAggregate() *ProjectAggregate {
	aggregate := &ProjectAggregate{
		ProjectStats:       make(map[string]*ProjectStatItem),
		DailyActivity:      make(map[string]int),
		DailySessions:      make(map[string]map[string]bool),
		ModelUsage:         make(map[string]*ModelUsageItem),
		ToolStats:          make(map[string]*ToolStatItem),
		ToolModelStats:     make(map[string]*ToolModelStatItem),
		ToolFailureKinds:   make(map[string]int),
		EventTypes:         make(map[string]int),
		HookStats:          make(map[string]*HookStatItem),
		SkillStats:         make(map[string]*SkillStatItem),
		PermissionModes:    make(map[string]int),
		OpenedFiles:        make(map[string]*FileAccessStat),
		AgentStats:         make(map[string]*AgentStatItem),
		AgentSessions:      make(map[string]map[string]bool),
		BashCommandStats:   make(map[string]*BashCommandStat),
		FileOperationStats: make(map[string]*FileOperationStat),
		HourlyCounts:       [24]int{},
		mu:                 sync.RWMutex{},
	}

	// 初始化星期数据
	weekdayNames := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	for i := 0; i < 7; i++ {
		aggregate.WeekdayData[i] = WeekdayItem{
			Weekday:      i,
			WeekdayName:  weekdayNames[i],
			MessageCount: 0,
		}
	}
	return aggregate
}

func mergeProjectAggregate(dst, src *ProjectAggregate) {
	for project, stat := range src.ProjectStats {
		if dst.ProjectStats[project] == nil {
			dst.ProjectStats[project] = &ProjectStatItem{Project: project}
		}
		dst.ProjectStats[project].MessageCount += stat.MessageCount
		dst.ProjectStats[project].SessionCount += stat.SessionCount
	}
	for i := range src.WeekdayData {
		dst.WeekdayData[i].MessageCount += src.WeekdayData[i].MessageCount
	}
	for date, count := range src.DailyActivity {
		dst.DailyActivity[date] += count
	}
	for date, sessions := range src.DailySessions {
		if dst.DailySessions[date] == nil {
			dst.DailySessions[date] = make(map[string]bool)
		}
		for sessionID := range sessions {
			dst.DailySessions[date][sessionID] = true
		}
	}
	for hour, count := range src.HourlyCounts {
		dst.HourlyCounts[hour] += count
	}
	for model, stat := range src.ModelUsage {
		if dst.ModelUsage[model] == nil {
			dst.ModelUsage[model] = &ModelUsageItem{Model: model}
		}
		dst.ModelUsage[model].Count += stat.Count
		dst.ModelUsage[model].Tokens += stat.Tokens
	}
	for tool, stat := range src.ToolStats {
		dstStat := ensureToolStat(dst, tool)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
	for _, stat := range src.ToolModelStats {
		dstStat := ensureToolModelStat(dst, stat.Tool, stat.Model)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
	for kind, count := range src.ToolFailureKinds {
		dst.ToolFailureKinds[kind] += count
	}
	if src.ToolAnalysis != nil {
		if dst.ToolAnalysis == nil {
			dst.ToolAnalysis = &ToolAnalysisData{}
		}
		remaining := 30 - len(dst.ToolAnalysis.FailureSamples)
		if remaining > 0 {
			samples := src.ToolAnalysis.FailureSamples
			if len(samples) > remaining {
				samples = samples[:remaining]
			}
			dst.ToolAnalysis.FailureSamples = append(dst.ToolAnalysis.FailureSamples, samples...)
		}
	}
	for eventType, count := range src.EventTypes {
		dst.EventTypes[eventType] += count
	}
	for key, stat := range src.HookStats {
		if dst.HookStats[key] == nil {
			statCopy := *stat
			dst.HookStats[key] = &statCopy
			continue
		}
		dstStat := dst.HookStats[key]
		oldTotal := dstStat.TotalCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.CancelledCount += stat.CancelledCount
		dstStat.ErrorCount += stat.ErrorCount
		dstStat.TotalCount += stat.TotalCount
		if dstStat.TotalCount > 0 {
			dstStat.AvgDurationMs = (dstStat.AvgDurationMs*float64(oldTotal) + stat.AvgDurationMs*float64(stat.TotalCount)) / float64(dstStat.TotalCount)
		}
		if stat.LastError != "" {
			dstStat.LastError = stat.LastError
		}
		if stat.LastCommand != "" {
			dstStat.LastCommand = stat.LastCommand
		}
	}
	for name, stat := range src.SkillStats {
		if dst.SkillStats[name] == nil {
			dst.SkillStats[name] = &SkillStatItem{Name: stat.Name, Path: stat.Path}
		}
		dst.SkillStats[name].Count += stat.Count
		if dst.SkillStats[name].Path == "" {
			dst.SkillStats[name].Path = stat.Path
		}
	}
	for mode, count := range src.PermissionModes {
		dst.PermissionModes[mode] += count
	}
	for path, stat := range src.OpenedFiles {
		if dst.OpenedFiles[path] == nil {
			dst.OpenedFiles[path] = &FileAccessStat{Path: path}
		}
		dst.OpenedFiles[path].Count += stat.Count
	}
	if src.BudgetSummary != nil {
		if dst.BudgetSummary == nil {
			budgetCopy := *src.BudgetSummary
			dst.BudgetSummary = &budgetCopy
		} else {
			dst.BudgetSummary.LatestUsed = src.BudgetSummary.LatestUsed
			dst.BudgetSummary.LatestTotal = src.BudgetSummary.LatestTotal
			dst.BudgetSummary.LatestRemaining = src.BudgetSummary.LatestRemaining
			dst.BudgetSummary.EventCount += src.BudgetSummary.EventCount
			if src.BudgetSummary.MaxUsed > dst.BudgetSummary.MaxUsed {
				dst.BudgetSummary.MaxUsed = src.BudgetSummary.MaxUsed
			}
		}
	}
	remainingEvents := 40 - len(dst.EventSamples)
	if remainingEvents > 0 {
		samples := src.EventSamples
		if len(samples) > remainingEvents {
			samples = samples[:remainingEvents]
		}
		dst.EventSamples = append(dst.EventSamples, samples...)
	}
	for agentID, stat := range src.AgentStats {
		dstStat := ensureAgentStat(dst, agentID, stat.IsSidechain)
		if dstStat.AgentName == "" {
			dstStat.AgentName = stat.AgentName
		}
		dstStat.SessionCount += stat.SessionCount
		dstStat.MessageCount += stat.MessageCount
		dstStat.ToolCallCount += stat.ToolCallCount
		dstStat.ToolFailureCount += stat.ToolFailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
	for agentID, sessions := range src.AgentSessions {
		if dst.AgentSessions[agentID] == nil {
			dst.AgentSessions[agentID] = make(map[string]bool)
		}
		for sessionID := range sessions {
			dst.AgentSessions[agentID][sessionID] = true
		}
	}
	for commandName, stat := range src.BashCommandStats {
		dstStat := ensureBashCommandStat(dst, commandName)
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
		if dstStat.RiskLevel == "" || riskRank(stat.RiskLevel) > riskRank(dstStat.RiskLevel) {
			dstStat.RiskLevel = stat.RiskLevel
			dstStat.RiskReason = stat.RiskReason
		}
		if dstStat.SampleCommand == "" {
			dstStat.SampleCommand = stat.SampleCommand
		}
	}
	for key, stat := range src.FileOperationStats {
		if dst.FileOperationStats[key] == nil {
			dst.FileOperationStats[key] = &FileOperationStat{Operation: stat.Operation, Path: stat.Path}
		}
		dstStat := dst.FileOperationStats[key]
		dstStat.CallCount += stat.CallCount
		dstStat.SuccessCount += stat.SuccessCount
		dstStat.FailureCount += stat.FailureCount
		dstStat.MissingResultCount += stat.MissingResultCount
	}
}

func projectJSONLFiles(projectDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(projectDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// parseProjectFileAggregate 解析单个项目文件并更新聚合数据
func parseProjectFileAggregate(filePath string, tf TimeFilter, agg *ProjectAggregate) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	pendingTools := make(map[string]pendingToolCall)
	decoder := json.NewDecoder(f)
	for {
		var record ProjectRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		timestamp, hasTimestamp := parseProjectRecordTimestamp(record.Timestamp)
		if hasTimestamp && !tf.Contains(timestamp) {
			continue
		}
		if !hasTimestamp && hasTimeFilter(tf) {
			continue
		}

		projectName := record.Cwd
		if projectName == "" {
			projectName = "Unknown"
		}

		recordRuntimeEventLocked(agg, record, timestamp, projectName)

		if record.Type == "user" {
			parseToolResults(record, timestamp, projectName, pendingTools, agg)
			continue
		}

		// 只统计 assistant 消息
		if record.Type != "assistant" {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(record.Message, &msg); err != nil {
			continue
		}

		// 1. 项目统计
		if agg.ProjectStats[projectName] == nil {
			agg.ProjectStats[projectName] = &ProjectStatItem{
				Project: projectName,
			}
		}
		agg.ProjectStats[projectName].MessageCount++

		// 2. 星期统计
		weekday := int(timestamp.Weekday())  // 0=周日, 1=周一...
		adjustedWeekday := (weekday + 6) % 7 // 转换为0=周一
		agg.WeekdayData[adjustedWeekday].MessageCount++

		// 3. 每日活动
		dateKey := timestamp.Format("2006-01-02")
		agg.DailyActivity[dateKey]++

		// 3.5 每日会话去重（同一 sessionID 同天只计一次）
		if record.SessionID != "" {
			if agg.DailySessions[dateKey] == nil {
				agg.DailySessions[dateKey] = make(map[string]bool)
			}
			if !agg.DailySessions[dateKey][record.SessionID] {
				agg.DailySessions[dateKey][record.SessionID] = true
			}
		}

		// 4. 小时统计
		hour := timestamp.Hour()
		agg.HourlyCounts[hour]++

		// 5. 模型使用统计
		if msg.Model != "" {
			if agg.ModelUsage[msg.Model] == nil {
				agg.ModelUsage[msg.Model] = &ModelUsageItem{
					Model: msg.Model,
				}
			}
			agg.ModelUsage[msg.Model].Count++
			agg.ModelUsage[msg.Model].Tokens += msg.Usage.InputTokens + msg.Usage.OutputTokens
		}

		// 6. 工具调用统计
		for _, content := range msg.Content {
			if content.Type != "tool_use" || content.ID == "" || content.Name == "" {
				continue
			}
			pendingTools[content.ID] = pendingToolCall{
				ID:          content.ID,
				Tool:        content.Name,
				Model:       msg.Model,
				Project:     projectName,
				SessionID:   record.SessionID,
				AgentID:     record.AgentID,
				IsSidechain: record.IsSidechain,
				Timestamp:   timestamp,
				Input:       content.Input,
			}
			call := pendingTools[content.ID]
			addToolCallLocked(agg, content.Name, msg.Model)
			addAgentToolCallLocked(agg, call)
			recordStructuredToolInputLocked(agg, &call)
			pendingTools[content.ID] = call
		}
	}

	if len(pendingTools) > 0 {
		for _, call := range pendingTools {
			addMissingToolResultLocked(agg, call)
		}
	}
}

func parseProjectRecordTimestamp(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return timestamp, true
}

func hasTimeFilter(tf TimeFilter) bool {
	return tf.Start != nil || tf.End != nil
}

func parseToolResults(record ProjectRecord, timestamp time.Time, projectName string, pendingTools map[string]pendingToolCall, agg *ProjectAggregate) {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(record.Message, &msg); err != nil {
		return
	}

	contentRaw := bytes.TrimSpace(msg.Content)
	if len(contentRaw) == 0 || contentRaw[0] != '[' {
		return
	}

	var contents []AssistantContent
	if err := json.Unmarshal(contentRaw, &contents); err != nil {
		return
	}

	for _, content := range contents {
		if content.Type != "tool_result" || content.ToolUseID == "" {
			continue
		}

		call, ok := pendingTools[content.ToolUseID]
		if !ok {
			call = pendingToolCall{
				ID:          content.ToolUseID,
				Tool:        "unknown",
				Model:       "unknown",
				Project:     projectName,
				SessionID:   record.SessionID,
				AgentID:     record.AgentID,
				IsSidechain: record.IsSidechain,
				Timestamp:   timestamp,
			}
		}

		kind, failed := classifyToolResult(content)
		preview := toolResultPreview(content.Content)

		if !ok {
			addToolCallLocked(agg, call.Tool, call.Model)
			addAgentToolCallLocked(agg, call)
		}
		addToolResultLocked(agg, call, failed, kind, preview, timestamp)

		delete(pendingTools, content.ToolUseID)
	}
}

func addToolCallLocked(agg *ProjectAggregate, tool string, model string) {
	stat := ensureToolStat(agg, tool)
	stat.CallCount++
	modelStat := ensureToolModelStat(agg, tool, model)
	modelStat.CallCount++
}

func addToolResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, kind string, preview string, timestamp time.Time) {
	stat := ensureToolStat(agg, call.Tool)
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	if failed {
		stat.FailureCount++
		modelStat.FailureCount++
		addAgentToolResultLocked(agg, call, true, false)
		addCommandOrFileResultLocked(agg, call, true, false)
		agg.ToolFailureKinds[kind]++
		if len(agg.ToolAnalysisFailureSamples()) < 30 {
			agg.ToolAnalysisAddFailureSample(ToolFailureSample{
				Tool:           call.Tool,
				Model:          call.Model,
				Kind:           kind,
				Project:        call.Project,
				SessionID:      call.SessionID,
				Timestamp:      timestamp.Format(time.RFC3339),
				ContentPreview: preview,
			})
		}
		return
	}
	stat.SuccessCount++
	modelStat.SuccessCount++
	addAgentToolResultLocked(agg, call, false, false)
	addCommandOrFileResultLocked(agg, call, false, false)
}

func addMissingToolResultLocked(agg *ProjectAggregate, call pendingToolCall) {
	stat := ensureToolStat(agg, call.Tool)
	stat.MissingResultCount++
	modelStat := ensureToolModelStat(agg, call.Tool, call.Model)
	modelStat.MissingResultCount++
	addAgentToolResultLocked(agg, call, false, true)
	addCommandOrFileResultLocked(agg, call, false, true)
}

func recordRuntimeEventLocked(agg *ProjectAggregate, record ProjectRecord, timestamp time.Time, projectName string) {
	eventType := record.Type
	if eventType == "" {
		eventType = "unknown"
	}

	if eventType == "attachment" {
		var attachment struct {
			Type       string  `json:"type"`
			HookName   string  `json:"hookName"`
			HookEvent  string  `json:"hookEvent"`
			Command    string  `json:"command"`
			Stdout     string  `json:"stdout"`
			Stderr     string  `json:"stderr"`
			ExitCode   int     `json:"exitCode"`
			DurationMs int     `json:"durationMs"`
			Filename   string  `json:"filename"`
			Used       float64 `json:"used"`
			Total      float64 `json:"total"`
			Remaining  float64 `json:"remaining"`
			Skills     []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"skills"`
		}
		if err := json.Unmarshal(record.Attachment, &attachment); err == nil && attachment.Type != "" {
			eventType = "attachment:" + attachment.Type
			switch attachment.Type {
			case "hook_success", "hook_cancelled", "hook_non_blocking_error":
				recordHookEventLocked(agg, attachment.Type, attachment.HookName, attachment.HookEvent, attachment.Command, attachment.Stderr, attachment.DurationMs)
			case "invoked_skills":
				for _, skill := range attachment.Skills {
					recordSkillLocked(agg, skill.Name, skill.Path)
				}
			case "opened_file_in_ide":
				recordOpenedFileLocked(agg, attachment.Filename)
			case "budget_usd":
				recordBudgetLocked(agg, attachment.Used, attachment.Total, attachment.Remaining)
			case "queued_command":
				// Counted through event type below.
			}
			if len(agg.EventSamples) < 40 && (strings.HasPrefix(attachment.Type, "hook_") || attachment.Type == "invoked_skills" || attachment.Type == "queued_command") {
				addEventSampleLocked(agg, eventType, projectName, record.SessionID, timestamp, string(record.Attachment))
			}
		}
	}

	agg.EventTypes[eventType]++
	switch eventType {
	case "attachment:queued_command":
		// Derived count comes from EventTypes in finalize.
	case "attachment:plan_mode", "attachment:plan_mode_exit":
		// Derived count comes from EventTypes in finalize.
	case "permission-mode":
		mode := record.PermissionMode
		if mode == "" {
			mode = rawStringField(record.Content, "mode")
		}
		if mode == "" {
			mode = rawStringField(record.Message, "permissionMode")
		}
		if mode == "" {
			mode = "unknown"
		}
		agg.PermissionModes[mode]++
	case "agent-name":
		name := record.Name
		if name == "" {
			name = rawStringField(record.Content, "name")
		}
		if name == "" {
			name = rawStringField(record.Message, "name")
		}
		if name != "" {
			agent := ensureAgentStat(agg, record.AgentID, record.IsSidechain)
			agent.AgentName = name
		}
	}

	agent := ensureAgentStat(agg, record.AgentID, record.IsSidechain)
	if record.Type == "assistant" || record.Type == "user" {
		agent.MessageCount++
	}
	if record.SessionID != "" {
		key := agent.AgentID
		if agg.AgentSessions[key] == nil {
			agg.AgentSessions[key] = make(map[string]bool)
		}
		if !agg.AgentSessions[key][record.SessionID] {
			agg.AgentSessions[key][record.SessionID] = true
			agent.SessionCount++
		}
	}
}

func recordHookEventLocked(agg *ProjectAggregate, attachmentType, hookName, hookEvent, command, stderr string, durationMs int) {
	if hookName == "" {
		hookName = "unknown"
	}
	key := hookName + "\x00" + hookEvent
	if agg.HookStats[key] == nil {
		agg.HookStats[key] = &HookStatItem{HookName: hookName, HookEvent: hookEvent}
	}
	stat := agg.HookStats[key]
	stat.TotalCount++
	switch attachmentType {
	case "hook_success":
		stat.SuccessCount++
	case "hook_cancelled":
		stat.CancelledCount++
	default:
		stat.ErrorCount++
		if stderr != "" {
			stat.LastError = previewString(stderr, 240)
		}
	}
	if command != "" {
		stat.LastCommand = previewString(command, 160)
	}
	if durationMs > 0 {
		previous := float64(stat.TotalCount - 1)
		stat.AvgDurationMs = (stat.AvgDurationMs*previous + float64(durationMs)) / float64(stat.TotalCount)
	}
}

func recordSkillLocked(agg *ProjectAggregate, name, path string) {
	if name == "" {
		name = "unknown"
	}
	if agg.SkillStats[name] == nil {
		agg.SkillStats[name] = &SkillStatItem{Name: name, Path: path}
	}
	agg.SkillStats[name].Count++
	if agg.SkillStats[name].Path == "" {
		agg.SkillStats[name].Path = path
	}
}

func recordOpenedFileLocked(agg *ProjectAggregate, path string) {
	if path == "" {
		return
	}
	if agg.OpenedFiles[path] == nil {
		agg.OpenedFiles[path] = &FileAccessStat{Path: path}
	}
	agg.OpenedFiles[path].Count++
}

func recordBudgetLocked(agg *ProjectAggregate, used, total, remaining float64) {
	if agg.BudgetSummary == nil {
		agg.BudgetSummary = &BudgetSummary{}
	}
	agg.BudgetSummary.LatestUsed = used
	agg.BudgetSummary.LatestTotal = total
	agg.BudgetSummary.LatestRemaining = remaining
	agg.BudgetSummary.EventCount++
	if used > agg.BudgetSummary.MaxUsed {
		agg.BudgetSummary.MaxUsed = used
	}
}

func addEventSampleLocked(agg *ProjectAggregate, eventType, project, sessionID string, timestamp time.Time, raw string) {
	agg.EventSamples = append(agg.EventSamples, EventSample{
		Type:           eventType,
		Project:        project,
		SessionID:      sessionID,
		Timestamp:      timestamp.Format(time.RFC3339),
		ContentPreview: previewString(raw, 240),
	})
}

func addAgentToolCallLocked(agg *ProjectAggregate, call pendingToolCall) {
	agent := ensureAgentStat(agg, call.AgentID, call.IsSidechain)
	agent.ToolCallCount++
	if call.IsSidechain {
		return
	}
}

func addAgentToolResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, missing bool) {
	agent := ensureAgentStat(agg, call.AgentID, call.IsSidechain)
	if failed {
		agent.ToolFailureCount++
	}
	if missing {
		agent.MissingResultCount++
	}
}

func ensureAgentStat(agg *ProjectAggregate, agentID string, isSidechain bool) *AgentStatItem {
	if agg.AgentStats == nil {
		agg.AgentStats = make(map[string]*AgentStatItem)
	}
	if agentID == "" {
		if isSidechain {
			agentID = "sidechain:unknown"
		} else {
			agentID = "main"
		}
	}
	if agg.AgentStats[agentID] == nil {
		agg.AgentStats[agentID] = &AgentStatItem{AgentID: agentID, IsSidechain: isSidechain}
	}
	if isSidechain {
		agg.AgentStats[agentID].IsSidechain = true
	}
	return agg.AgentStats[agentID]
}

func recordStructuredToolInputLocked(agg *ProjectAggregate, call *pendingToolCall) {
	switch call.Tool {
	case "Bash":
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(call.Input, &input); err != nil || strings.TrimSpace(input.Command) == "" {
			return
		}
		commandName := bashCommandName(input.Command)
		riskLevel, riskReason := classifyBashRisk(input.Command)
		call.CommandName = commandName
		stat := ensureBashCommandStat(agg, commandName)
		stat.CallCount++
		if stat.SampleCommand == "" {
			stat.SampleCommand = previewString(input.Command, 180)
		}
		if riskLevel != "" {
			stat.RiskLevel = riskLevel
			stat.RiskReason = riskReason
		}
	case "Read", "Edit", "Write", "MultiEdit":
		path := filePathFromToolInput(call.Input)
		if path == "" {
			return
		}
		key := call.Tool + "\x00" + path
		call.FileOpKey = key
		if agg.FileOperationStats[key] == nil {
			agg.FileOperationStats[key] = &FileOperationStat{Operation: call.Tool, Path: path}
		}
		agg.FileOperationStats[key].CallCount++
	}
}

func addCommandOrFileResultLocked(agg *ProjectAggregate, call pendingToolCall, failed bool, missing bool) {
	if call.CommandName != "" {
		stat := ensureBashCommandStat(agg, call.CommandName)
		switch {
		case missing:
			stat.MissingResultCount++
		case failed:
			stat.FailureCount++
		default:
			stat.SuccessCount++
		}
	}
	if call.FileOpKey != "" && agg.FileOperationStats[call.FileOpKey] != nil {
		stat := agg.FileOperationStats[call.FileOpKey]
		switch {
		case missing:
			stat.MissingResultCount++
		case failed:
			stat.FailureCount++
		default:
			stat.SuccessCount++
		}
	}
}

func ensureBashCommandStat(agg *ProjectAggregate, commandName string) *BashCommandStat {
	if agg.BashCommandStats == nil {
		agg.BashCommandStats = make(map[string]*BashCommandStat)
	}
	if commandName == "" {
		commandName = "unknown"
	}
	if agg.BashCommandStats[commandName] == nil {
		agg.BashCommandStats[commandName] = &BashCommandStat{CommandName: commandName}
	}
	return agg.BashCommandStats[commandName]
}

func bashCommandName(command string) string {
	fields := strings.Fields(firstExecutableShellSegment(command))
	if len(fields) == 0 {
		return "unknown"
	}
	if fields[0] == "sudo" && len(fields) > 1 {
		return "sudo " + fields[1]
	}
	return fields[0]
}

func classifyBashRisk(command string) (string, string) {
	lower := strings.ToLower(firstExecutableShellSegment(command))
	switch {
	case strings.HasPrefix(lower, "rm -rf ") || strings.HasPrefix(lower, "rm -fr ") || lower == "rm -rf" || lower == "rm -fr":
		return "high", "recursive delete"
	case strings.HasPrefix(lower, "git reset --hard") || strings.HasPrefix(lower, "git clean -fd"):
		return "high", "destructive git cleanup"
	case strings.HasPrefix(lower, "sudo "):
		return "medium", "privileged command"
	case strings.HasPrefix(lower, "curl ") && strings.Contains(lower, "| sh"):
		return "high", "download pipe to shell"
	case strings.HasPrefix(lower, "wget ") && strings.Contains(lower, "| sh"):
		return "high", "download pipe to shell"
	default:
		return "", ""
	}
}

func firstExecutableShellSegment(command string) string {
	for _, segment := range strings.Split(command, "\n") {
		segment = strings.TrimSpace(segment)
		if segment == "" || strings.HasPrefix(segment, "#") {
			continue
		}
		for _, separator := range []string{"&&", ";"} {
			if idx := strings.Index(segment, separator); idx >= 0 {
				segment = strings.TrimSpace(segment[:idx])
			}
		}
		return segment
	}
	return ""
}

func filePathFromToolInput(raw json.RawMessage) string {
	var input struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ""
	}
	if input.FilePath != "" {
		return input.FilePath
	}
	return input.Path
}

func rawStringField(raw json.RawMessage, field string) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return ""
	}
	value, ok := data[field]
	if !ok {
		return ""
	}
	s, _ := value.(string)
	return s
}

func previewString(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func ensureToolStat(agg *ProjectAggregate, tool string) *ToolStatItem {
	if agg.ToolStats == nil {
		agg.ToolStats = make(map[string]*ToolStatItem)
	}
	if tool == "" {
		tool = "unknown"
	}
	if agg.ToolStats[tool] == nil {
		agg.ToolStats[tool] = &ToolStatItem{Tool: tool}
	}
	return agg.ToolStats[tool]
}

func ensureToolModelStat(agg *ProjectAggregate, tool string, model string) *ToolModelStatItem {
	if agg.ToolModelStats == nil {
		agg.ToolModelStats = make(map[string]*ToolModelStatItem)
	}
	if tool == "" {
		tool = "unknown"
	}
	if model == "" {
		model = "unknown"
	}
	key := model + "\x00" + tool
	if agg.ToolModelStats[key] == nil {
		agg.ToolModelStats[key] = &ToolModelStatItem{Model: model, Tool: tool}
	}
	return agg.ToolModelStats[key]
}

func classifyToolResult(content AssistantContent) (string, bool) {
	if content.IsError {
		return "explicit_error", true
	}

	text := toolResultText(content.Content)
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, `"success":false`) || strings.Contains(lower, `"success": false`):
		return "api_failure", true
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout") || strings.Contains(lower, "超时"):
		return "timeout", true
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "权限"):
		return "permission", true
	case strings.Contains(lower, "no such file") || strings.Contains(lower, "不存在"):
		return "not_found", true
	case strings.Contains(lower, "exit code") || strings.Contains(lower, "command failed"):
		return "command_failed", true
	case toolFailureExpr.MatchString(text):
		return "error_text", true
	default:
		return "", false
	}
}

func toolResultPreview(raw json.RawMessage) string {
	text := toolResultText(raw)
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		return text[:240]
	}
	return text
}

func toolResultText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func (agg *ProjectAggregate) ToolAnalysisFailureSamples() []ToolFailureSample {
	if agg.ToolAnalysis == nil {
		return nil
	}
	return agg.ToolAnalysis.FailureSamples
}

func (agg *ProjectAggregate) ToolAnalysisAddFailureSample(sample ToolFailureSample) {
	if agg.ToolAnalysis == nil {
		agg.ToolAnalysis = &ToolAnalysisData{}
	}
	agg.ToolAnalysis.FailureSamples = append(agg.ToolAnalysis.FailureSamples, sample)
}

// finalize 生成输出格式的数据
func (agg *ProjectAggregate) finalize() {
	// 1. 转换项目列表并排序
	agg.Projects = make([]ProjectStatItem, 0, len(agg.ProjectStats))
	for _, proj := range agg.ProjectStats {
		agg.Projects = append(agg.Projects, *proj)
	}
	sort.Slice(agg.Projects, func(i, j int) bool {
		return agg.Projects[i].MessageCount > agg.Projects[j].MessageCount
	})

	// 2. 转换星期统计
	weekdayData := make([]WeekdayItem, 7)
	copy(weekdayData, agg.WeekdayData[:])
	agg.WeekdayStats = &WeekdayStats{WeekdayData: weekdayData}

	// 3. 转换每日活动为列表
	dates := make([]string, 0, len(agg.DailyActivity))
	for date := range agg.DailyActivity {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	agg.DailyActivityList = make([]DailyActivity, len(dates))
	for i, date := range dates {
		agg.DailyActivityList[i] = DailyActivity{
			Date:         date,
			MessageCount: agg.DailyActivity[date],
		}
	}

	// 4. 转换小时数据
	agg.HourlyData = make([]HourlyItem, 24)
	for i := 0; i < 24; i++ {
		agg.HourlyData[i] = HourlyItem{
			Hour:       i,
			HourLabel:  fmt.Sprintf("%02d:00", i),
			Count:      agg.HourlyCounts[i],
			IsWorkHour: i >= 9 && i <= 18,
		}
	}

	// 5. 转换模型使用列表
	agg.ModelUsageList = make([]ModelUsageItem, 0, len(agg.ModelUsage))
	for _, model := range agg.ModelUsage {
		agg.ModelUsageList = append(agg.ModelUsageList, *model)
	}
	sort.Slice(agg.ModelUsageList, func(i, j int) bool {
		return agg.ModelUsageList[i].Count > agg.ModelUsageList[j].Count
	})

	// 6. 转换工具分析
	agg.finalizeToolAnalysis()

	// 7. 转换运行事件、agent、命令/文件分析
	agg.finalizeEventAnalysis()
	agg.finalizeAgentAnalysis()
	agg.finalizeCommandAnalysis()

	// 8. 生成工作时段统计
	var workHoursCount, offHoursCount int
	var peakHour, peakCount int

	for _, item := range agg.HourlyData {
		if item.IsWorkHour {
			workHoursCount += item.Count
		} else {
			offHoursCount += item.Count
		}

		if item.Count > peakCount {
			peakCount = item.Count
			peakHour = item.Hour
		}
	}

	total := workHoursCount + offHoursCount
	var workRatio float64
	if total > 0 {
		workRatio = float64(workHoursCount) / float64(total) * 100
	}

	agg.WorkHoursStats = &WorkHoursStats{
		HourlyData:     agg.HourlyData,
		WorkHoursCount: workHoursCount,
		OffHoursCount:  offHoursCount,
		WorkHoursRatio: workRatio,
		PeakHour:       peakHour,
		PeakHourCount:  peakCount,
	}
}

func (agg *ProjectAggregate) finalizeToolAnalysis() {
	analysis := &ToolAnalysisData{
		Tools:          make([]ToolStatItem, 0, len(agg.ToolStats)),
		ByModel:        make([]ToolModelStatItem, 0, len(agg.ToolModelStats)),
		FailureKinds:   make([]ToolFailureKind, 0, len(agg.ToolFailureKinds)),
		FailureSamples: nil,
	}
	if agg.ToolAnalysis != nil {
		analysis.FailureSamples = agg.ToolAnalysis.FailureSamples
	}

	for _, stat := range agg.ToolStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.TotalCalls += statCopy.CallCount
		analysis.TotalFailures += statCopy.FailureCount
		analysis.MissingResults += statCopy.MissingResultCount
		analysis.Tools = append(analysis.Tools, statCopy)
	}
	sort.Slice(analysis.Tools, func(i, j int) bool {
		if analysis.Tools[i].CallCount == analysis.Tools[j].CallCount {
			return analysis.Tools[i].Tool < analysis.Tools[j].Tool
		}
		return analysis.Tools[i].CallCount > analysis.Tools[j].CallCount
	})

	for _, stat := range agg.ToolModelStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.ByModel = append(analysis.ByModel, statCopy)
	}
	sort.Slice(analysis.ByModel, func(i, j int) bool {
		if analysis.ByModel[i].FailureCount == analysis.ByModel[j].FailureCount {
			if analysis.ByModel[i].CallCount == analysis.ByModel[j].CallCount {
				if analysis.ByModel[i].Model == analysis.ByModel[j].Model {
					return analysis.ByModel[i].Tool < analysis.ByModel[j].Tool
				}
				return analysis.ByModel[i].Model < analysis.ByModel[j].Model
			}
			return analysis.ByModel[i].CallCount > analysis.ByModel[j].CallCount
		}
		return analysis.ByModel[i].FailureCount > analysis.ByModel[j].FailureCount
	})

	for kind, count := range agg.ToolFailureKinds {
		analysis.FailureKinds = append(analysis.FailureKinds, ToolFailureKind{Kind: kind, Count: count})
	}
	sort.Slice(analysis.FailureKinds, func(i, j int) bool {
		if analysis.FailureKinds[i].Count == analysis.FailureKinds[j].Count {
			return analysis.FailureKinds[i].Kind < analysis.FailureKinds[j].Kind
		}
		return analysis.FailureKinds[i].Count > analysis.FailureKinds[j].Count
	})

	agg.ToolAnalysis = analysis
}

func (agg *ProjectAggregate) finalizeEventAnalysis() {
	analysis := &EventAnalysisData{
		ByType:          make([]EventTypeStat, 0, len(agg.EventTypes)),
		Hooks:           make([]HookStatItem, 0, len(agg.HookStats)),
		Skills:          make([]SkillStatItem, 0, len(agg.SkillStats)),
		PermissionModes: make([]PermissionModeStat, 0, len(agg.PermissionModes)),
		OpenedFiles:     make([]FileAccessStat, 0, len(agg.OpenedFiles)),
		Samples:         append([]EventSample(nil), agg.EventSamples...),
	}
	for eventType, count := range agg.EventTypes {
		analysis.TotalEvents += count
		analysis.ByType = append(analysis.ByType, EventTypeStat{Type: eventType, Count: count})
	}
	sort.Slice(analysis.ByType, func(i, j int) bool {
		if analysis.ByType[i].Count == analysis.ByType[j].Count {
			return analysis.ByType[i].Type < analysis.ByType[j].Type
		}
		return analysis.ByType[i].Count > analysis.ByType[j].Count
	})

	for _, stat := range agg.HookStats {
		statCopy := *stat
		if statCopy.TotalCount > 0 {
			statCopy.FailureRate = float64(statCopy.CancelledCount+statCopy.ErrorCount) / float64(statCopy.TotalCount) * 100
		}
		analysis.Hooks = append(analysis.Hooks, statCopy)
	}
	sort.Slice(analysis.Hooks, func(i, j int) bool {
		if analysis.Hooks[i].ErrorCount == analysis.Hooks[j].ErrorCount {
			return analysis.Hooks[i].TotalCount > analysis.Hooks[j].TotalCount
		}
		return analysis.Hooks[i].ErrorCount > analysis.Hooks[j].ErrorCount
	})

	for _, stat := range agg.SkillStats {
		analysis.Skills = append(analysis.Skills, *stat)
	}
	sort.Slice(analysis.Skills, func(i, j int) bool {
		if analysis.Skills[i].Count == analysis.Skills[j].Count {
			return analysis.Skills[i].Name < analysis.Skills[j].Name
		}
		return analysis.Skills[i].Count > analysis.Skills[j].Count
	})

	for mode, count := range agg.PermissionModes {
		analysis.PermissionModes = append(analysis.PermissionModes, PermissionModeStat{Mode: mode, Count: count})
	}
	sort.Slice(analysis.PermissionModes, func(i, j int) bool {
		if analysis.PermissionModes[i].Count == analysis.PermissionModes[j].Count {
			return analysis.PermissionModes[i].Mode < analysis.PermissionModes[j].Mode
		}
		return analysis.PermissionModes[i].Count > analysis.PermissionModes[j].Count
	})

	for _, stat := range agg.OpenedFiles {
		analysis.OpenedFiles = append(analysis.OpenedFiles, *stat)
	}
	sort.Slice(analysis.OpenedFiles, func(i, j int) bool {
		if analysis.OpenedFiles[i].Count == analysis.OpenedFiles[j].Count {
			return analysis.OpenedFiles[i].Path < analysis.OpenedFiles[j].Path
		}
		return analysis.OpenedFiles[i].Count > analysis.OpenedFiles[j].Count
	})
	if len(analysis.OpenedFiles) > 50 {
		analysis.OpenedFiles = analysis.OpenedFiles[:50]
	}

	analysis.QueuedCommands = agg.EventTypes["attachment:queued_command"]
	analysis.PlanModeCount = agg.EventTypes["attachment:plan_mode"]
	analysis.PlanModeExitCount = agg.EventTypes["attachment:plan_mode_exit"]
	if agg.BudgetSummary != nil {
		budgetCopy := *agg.BudgetSummary
		analysis.Budget = &budgetCopy
	}
	agg.EventAnalysis = analysis
}

func (agg *ProjectAggregate) finalizeAgentAnalysis() {
	analysis := &AgentAnalysisData{
		Agents: make([]AgentStatItem, 0, len(agg.AgentStats)),
	}
	for _, stat := range agg.AgentStats {
		statCopy := *stat
		if statCopy.ToolCallCount > 0 {
			statCopy.FailureRate = float64(statCopy.ToolFailureCount) / float64(statCopy.ToolCallCount) * 100
		}
		if statCopy.IsSidechain {
			analysis.SidechainToolCalls += statCopy.ToolCallCount
		} else {
			analysis.MainToolCalls += statCopy.ToolCallCount
		}
		analysis.Agents = append(analysis.Agents, statCopy)
	}
	sort.Slice(analysis.Agents, func(i, j int) bool {
		if analysis.Agents[i].ToolFailureCount == analysis.Agents[j].ToolFailureCount {
			if analysis.Agents[i].ToolCallCount == analysis.Agents[j].ToolCallCount {
				return analysis.Agents[i].AgentID < analysis.Agents[j].AgentID
			}
			return analysis.Agents[i].ToolCallCount > analysis.Agents[j].ToolCallCount
		}
		return analysis.Agents[i].ToolFailureCount > analysis.Agents[j].ToolFailureCount
	})
	agg.AgentAnalysis = analysis
}

func (agg *ProjectAggregate) finalizeCommandAnalysis() {
	analysis := &CommandAnalysisData{
		BashCommands:   make([]BashCommandStat, 0, len(agg.BashCommandStats)),
		RiskyCommands:  make([]BashCommandStat, 0),
		FileOperations: make([]FileOperationStat, 0, len(agg.FileOperationStats)),
	}
	for _, stat := range agg.BashCommandStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.BashCommands = append(analysis.BashCommands, statCopy)
		if statCopy.RiskLevel != "" {
			analysis.RiskyCommands = append(analysis.RiskyCommands, statCopy)
		}
	}
	sort.Slice(analysis.BashCommands, func(i, j int) bool {
		if analysis.BashCommands[i].CallCount == analysis.BashCommands[j].CallCount {
			return analysis.BashCommands[i].CommandName < analysis.BashCommands[j].CommandName
		}
		return analysis.BashCommands[i].CallCount > analysis.BashCommands[j].CallCount
	})
	sort.Slice(analysis.RiskyCommands, func(i, j int) bool {
		if analysis.RiskyCommands[i].RiskLevel == analysis.RiskyCommands[j].RiskLevel {
			return analysis.RiskyCommands[i].CallCount > analysis.RiskyCommands[j].CallCount
		}
		return riskRank(analysis.RiskyCommands[i].RiskLevel) > riskRank(analysis.RiskyCommands[j].RiskLevel)
	})

	for _, stat := range agg.FileOperationStats {
		statCopy := *stat
		if statCopy.CallCount > 0 {
			statCopy.FailureRate = float64(statCopy.FailureCount) / float64(statCopy.CallCount) * 100
		}
		analysis.FileOperations = append(analysis.FileOperations, statCopy)
	}
	sort.Slice(analysis.FileOperations, func(i, j int) bool {
		if analysis.FileOperations[i].FailureCount == analysis.FileOperations[j].FailureCount {
			if analysis.FileOperations[i].CallCount == analysis.FileOperations[j].CallCount {
				return analysis.FileOperations[i].Path < analysis.FileOperations[j].Path
			}
			return analysis.FileOperations[i].CallCount > analysis.FileOperations[j].CallCount
		}
		return analysis.FileOperations[i].FailureCount > analysis.FileOperations[j].FailureCount
	})
	if len(analysis.BashCommands) > 50 {
		analysis.BashCommands = analysis.BashCommands[:50]
	}
	if len(analysis.RiskyCommands) > 50 {
		analysis.RiskyCommands = analysis.RiskyCommands[:50]
	}
	if len(analysis.FileOperations) > 100 {
		analysis.FileOperations = analysis.FileOperations[:100]
	}
	agg.CommandAnalysis = analysis
}

func riskRank(level string) int {
	switch level {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
