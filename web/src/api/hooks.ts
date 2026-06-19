import { useQuery } from '@tanstack/react-query'
import type {
  OverviewData,
  DashboardData,
  TimelineData,
  DiagnosticsReport,
  DetailKind,
  DetailReport,
  Filters,
  InteractiveResponse,
  ApiResponse,
} from './types'

const BASE = '/api'

// filter → query string（只带非空字段）
function toQuery(filters: Filters): string {
  const p = new URLSearchParams()
  p.set('preset', filters.preset)
  for (const [k, v] of Object.entries(filters)) {
    if (k === 'preset') continue
    if (v) p.set(k, v)
  }
  return p.toString()
}

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return (await res.json()) as T
}

// /api/overview —— KPI 概览（trend 用于 sparkline）
export function useOverview(filters: Filters) {
  return useQuery({
    queryKey: ['overview', filters],
    queryFn: async (): Promise<OverviewData> => {
      const r = await getJSON<InteractiveResponse<OverviewData>>(
        `${BASE}/overview?${toQuery(filters)}`,
      )
      if (!r.success || !r.data) throw new Error(r.error || 'overview failed')
      return r.data
    },
  })
}

// /api/data —— 主数据（Phase 1 仅消费 daily_trend）
export function useDashboardData(filters: Filters) {
  return useQuery({
    queryKey: ['data', filters],
    queryFn: async (): Promise<DashboardData> => {
      const r = await getJSON<ApiResponse<DashboardData>>(`${BASE}/data?${toQuery(filters)}`)
      if (!r.success || !r.data) throw new Error(r.error || 'data failed')
      return r.data
    },
  })
}

// /api/timeline —— 时间轴索引（较少变动，长缓存）
export function useTimeline() {
  return useQuery({
    queryKey: ['timeline'],
    queryFn: async (): Promise<TimelineData> => {
      const r = await getJSON<ApiResponse<TimelineData>>(`${BASE}/timeline`)
      if (!r.success || !r.data) throw new Error(r.error || 'timeline failed')
      return r.data
    },
    staleTime: 5 * 60 * 1000,
  })
}

// /api/diagnostics —— 诊断建议（Phase 4 诊断卡）
export function useDiagnostics(filters: Filters) {
  return useQuery({
    queryKey: ['diagnostics', filters],
    queryFn: async (): Promise<DiagnosticsReport> => {
      const r = await getJSON<InteractiveResponse<DiagnosticsReport>>(
        `${BASE}/diagnostics?${toQuery(filters)}`,
      )
      if (!r.success || !r.data) throw new Error(r.error || 'diagnostics failed')
      return r.data
    },
  })
}

// /api/detail/{kind} —— 下钻（failures/commands/tokens/sessions/tools）
export function useDetail(kind: DetailKind | null, filters: Filters) {
  return useQuery({
    queryKey: ['detail', kind, filters],
    queryFn: async (): Promise<DetailReport> => {
      const r = await getJSON<InteractiveResponse<DetailReport>>(
        `${BASE}/detail/${kind}?${toQuery(filters)}`,
      )
      if (!r.success || !r.data) throw new Error(r.error || 'detail failed')
      return r.data
    },
    enabled: !!kind,
  })
}
