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

export interface DashboardData {
  timestamp?: string
  time_range?: TimeRange
  daily_trend: DailyTrend
  commands?: CommandStat[]
  project_stats?: ProjectStatsData
  model_usage?: ModelUsageItem[]
  weekday_stats?: WeekdayStats
  work_hours_stats?: WorkHoursStats
  // 其余子结构（tool_analysis/failure_analysis/cost 等）Phase 3b+ 补充
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
