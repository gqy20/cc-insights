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

export interface DashboardData {
  timestamp?: string
  time_range?: TimeRange
  daily_trend: DailyTrend
  // 其余子结构 Phase 2+ 按需补充
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
