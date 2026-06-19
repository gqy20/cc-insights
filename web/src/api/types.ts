// API 响应类型（对应 cmd/insights 的 Go 结构）。
// Phase 1 先覆盖 overview 与 daily_trend；其余子结构 Phase 2+ 按需补全。

export interface TimeRange {
  preset: string
  start?: string
  end?: string
}

export interface ApiMeta {
  source?: string
  cache_version?: string
  generated_at?: string
  runtime_ms?: number
  time_range?: TimeRange
  filters?: Record<string, unknown>
}

// 交互式 API（/api/overview、/api/diagnostics、/api/detail/*）统一外层
export interface InteractiveResponse<T> {
  success: boolean
  meta?: ApiMeta
  data?: T
  error?: string
}

// /api/data 外层（APIResponse）
export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: string
}

// /api/overview
export interface OverviewSummary {
  messages: number
  sessions: number
  commands: number
  tool_calls: number
  failures: number
  failure_rate: number
  tokens: number
  projects: number
  model_count: number
  slowest_call_ms?: number
}

export interface OverviewTrend {
  dates: string[]
  messages: number[]
  sessions: number[]
  tokens: number[]
  failures: number[]
}

export interface OverviewData {
  summary: OverviewSummary
  trend: OverviewTrend
  top: Record<string, unknown>
  diagnostics: { total: number; top?: unknown[] }
}

// /api/data
export interface DailyTrend {
  dates: string[]
  counts: number[]
}

export interface CommandStat {
  command: string
  count: number
}
export interface ProjectStatItem {
  project: string
  session_count: number
  message_count: number
}
export interface ProjectStatsData {
  projects: ProjectStatItem[]
  [key: string]: unknown
}
export interface ModelUsageItem {
  model: string
  count: number
  tokens: number
}
export interface WeekdayItem {
  weekday: number
  weekday_name: string
  message_count: number
}
export interface WeekdayStats {
  weekday_data: WeekdayItem[]
  [key: string]: unknown
}
export interface WorkHourItem {
  hour: number
  hour_label: string
  count: number
  is_work_hour: boolean
}
export interface WorkHoursStats {
  hourly_data: WorkHourItem[]
  [key: string]: unknown
}

// quality 组子结构
export interface ToolCallStat {
  tool?: string
  model?: string
  call_count: number
  success_count: number
  failure_count: number
  missing_result_count?: number
  failure_rate?: number
}
export interface ToolAnalysisData {
  total_calls?: number
  total_failures?: number
  missing_results?: number
  tools?: ToolCallStat[]
  by_model?: ToolCallStat[]
  [key: string]: unknown
}
export interface FailureReasonStat {
  category: string
  reason: string
  count: number
}
export interface FailureAnalysisData {
  total_failures?: number
  by_reason?: FailureReasonStat[]
  [key: string]: unknown
}
export interface CommandFamilyStat {
  family?: string
  command_name?: string
  call_count: number
  success_count: number
  failure_count: number
  failure_rate?: number
}
export interface CommandAnalysisData {
  bash_commands?: CommandFamilyStat[]
  bash_families?: CommandFamilyStat[]
  risky_commands?: CommandFamilyStat[]
  [key: string]: unknown
}
export interface FileAnalysisData {
  totals?: {
    unique_files?: number
    total_reads?: number
    total_edits?: number
    total_writes?: number
  }
  hot_files?: {
    path: string
    read_count: number
    edit_count: number
    write_count?: number
  }[]
  [key: string]: unknown
}
export interface ToolPerfCategoryItem {
  category: string
  base_tool?: string
  call_count: number
  success_count: number
  error_count: number
  missing_count?: number
  avg_duration_ms?: number
  error_rate?: number
}
export interface ToolPerformanceData {
  total_paired_calls?: number
  total_errors?: number
  overall_error_rate?: number
  overall_avg_duration_ms?: number
  by_category?: ToolPerfCategoryItem[]
  [key: string]: unknown
}

// cost + runtime 组子结构（Phase 3c）
export interface CostByModel {
  model: string
  request_count: number
  total_tokens?: number
  input_tokens?: number
  output_tokens?: number
}
export interface CostAnalysisData {
  totals?: Record<string, number>
  by_model?: CostByModel[]
  by_project?: CostByModel[]
  [key: string]: unknown
}
export interface SessionStats {
  total_sessions?: number
  peak_date?: string
  peak_count?: number
  valley_date?: string
  valley_count?: number
  daily_session_map?: Record<string, number>
  [key: string]: unknown
}
export interface SessionOutcome {
  outcome: string
  count: number
}
export interface SessionAnalysisData {
  sessions?: unknown[]
  top_failures?: unknown[]
  long_running?: unknown[]
  outcomes?: SessionOutcome[]
  queue_operations?: unknown[]
  [key: string]: unknown
}
export interface AgentStat {
  agent_id: string
  is_sidechain?: boolean
  session_count?: number
  message_count?: number
  tool_call_count?: number
  tool_failure_count?: number
  [key: string]: unknown
}
export interface AgentAnalysisData {
  main_tool_calls?: number
  sidechain_tool_calls?: number
  agents?: AgentStat[]
  [key: string]: unknown
}
export interface PlanFileStat {
  file_path: string
  file_name: string
  char_count?: number
  [key: string]: unknown
}
export interface TaskPlanAnalysisData {
  plan_lifecycle?: Record<string, number>
  plan_files?: PlanFileStat[]
  [key: string]: unknown
}
export interface EventTypeStat {
  type: string
  count: number
}
export interface EventAnalysisData {
  total_events?: number
  by_type?: EventTypeStat[]
  hooks?: unknown[]
  [key: string]: unknown
}
export interface SkillStat {
  name: string
  invocation_count?: number
  tool_use_count?: number
  attachment_count?: number
  failure_count?: number
  installed?: boolean
  [key: string]: unknown
}
export interface SkillAnalysisData {
  total_installed?: number
  total_invocations?: number
  tool_use_invocations?: number
  attachment_invocations?: number
  failure_count?: number
  skills?: SkillStat[]
  installed?: SkillStat[]
  by_project?: unknown[]
  [key: string]: unknown
}
export interface RuntimeToolSignal {
  tool?: string
  count?: number
  [key: string]: unknown
}

export interface DashboardData {
  timestamp?: string
  time_range?: TimeRange
  daily_trend: DailyTrend
  commands?: CommandStat[]
  project_stats?: ProjectStatsData
  model_usage?: ModelUsageItem[]
  weekday_stats?: WeekdayStats
  work_hours_stats?: WorkHoursStats
  tool_analysis?: ToolAnalysisData
  failure_analysis?: FailureAnalysisData
  command_analysis?: CommandAnalysisData
  file_analysis?: FileAnalysisData
  tool_performance?: ToolPerformanceData
  cost_analysis?: CostAnalysisData
  sessions?: SessionStats
  session_analysis?: SessionAnalysisData
  agent_analysis?: AgentAnalysisData
  task_plan_analysis?: TaskPlanAnalysisData
  event_analysis?: EventAnalysisData
  skill_analysis?: SkillAnalysisData
  runtime_tools?: RuntimeToolSignal[]
  [key: string]: unknown
}

// /api/timeline
export interface TimelineDay {
  date: string
  messages: number
  sessions: number
  tool_calls: number
  tokens: number
  failures: number
}

export interface TimelineData {
  start?: string
  end?: string
  days: TimelineDay[]
}

// 前端 filter（驱动所有 Query，见 hooks/useFilters.ts）
export interface Filters {
  preset: string
  start?: string
  end?: string
  project?: string
  tool?: string
  model?: string
  reason?: string
}

export const PRESETS = ['24h', '7d', '30d', '90d', 'all'] as const
export type Preset = (typeof PRESETS)[number]

// /api/diagnostics —— 诊断建议（对应 cli.go diagnosticFinding）
export interface DiagnosticEvidence {
  label: string
  value: string
}
export interface DiagnosticTrigger {
  metric: string
  value: string
  threshold: string
  source: string
  rationale?: string
}
export interface DiagnosticRootCause {
  type: string
  confidence: string
  summary: string
  evidence?: string[]
  recommendation_target: string
}
export interface DiagnosticExample {
  tool: string
  category?: string
  reason?: string
  project?: string
  session_id?: string
  timestamp?: string
  content_preview?: string
}
export interface DiagnosticAction {
  target: string
  action: string
  why: string
}
export interface DiagnosticFinding {
  id: string
  category: string
  severity: string
  title: string
  summary: string
  evidence: DiagnosticEvidence[]
  trigger?: DiagnosticTrigger
  root_causes?: DiagnosticRootCause[]
  targets?: string[]
  examples?: DiagnosticExample[]
  actions?: DiagnosticAction[]
  interpretation: string
  next_steps: string[]
  drilldown_commands?: unknown[]
  confidence: string
}
export interface NameCount {
  name: string
  count: number
}
export interface DiagnosticsReport {
  time_range?: TimeRange
  total_findings: number
  by_category: NameCount[]
  recommendations: DiagnosticFinding[]
  insights?: string[]
  detail?: unknown
}

// /api/detail/* —— 下钻（各 detail 结构不同，先宽松，Phase 4 下钻 UI 阶段细化）
export type DetailKind = 'failures' | 'commands' | 'tokens' | 'sessions' | 'tools'
export interface DetailReport {
  time_range?: TimeRange
  insights?: string[]
  [key: string]: unknown
}
