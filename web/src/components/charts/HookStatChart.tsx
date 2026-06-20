import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { EventAnalysisData, HookStat } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

function fmtMs(ms?: number): string {
  if (!ms || ms <= 0) return '-'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.round(ms)}ms`
}

function fmtRate(rate?: number): string {
  if (rate == null) return '-'
  return `${rate.toFixed(1)}%`
}

function HookTooltip({ active, payload }: {
  active?: boolean
  payload?: Array<{ payload: HookStat }>
}) {
  if (!active || !payload?.length) return null
  const h = payload[0].payload
  return (
    <div style={tooltipStyle} className="space-y-0.5 p-2">
      <div className="font-mono text-[11px] text-muted-foreground">
        {h.hook_name}
        {h.hook_event ? ` · ${h.hook_event}` : ''}
      </div>
      <div className="text-foreground">
        总 {h.total_count ?? 0}：成功 {h.success_count ?? 0} / 取消{' '}
        {h.cancelled_count ?? 0} / 失败 {h.error_count ?? 0}
      </div>
      <div className="text-foreground">
        失败率 <strong>{fmtRate(h.failure_rate)}</strong> · 平均耗时{' '}
        <strong>{fmtMs(h.avg_duration_ms)}</strong> · 涉及{' '}
        <strong>{h.session_count ?? 0}</strong> session
      </div>
      {h.last_error ? (
        <div className="max-w-[280px] truncate text-destructive">{h.last_error}</div>
      ) : null}
    </div>
  )
}

export function HookStatChart({
  data,
  loading,
}: {
  data?: EventAnalysisData
  loading?: boolean
}) {
  // 按失败率降序，失败率相同则按总次数，取 Top 12
  const rows = (data?.hooks ?? [])
    .filter((h) => (h.total_count ?? 0) > 0)
    .slice()
    .sort((a, b) => {
      const dr = (b.failure_rate ?? 0) - (a.failure_rate ?? 0)
      if (Math.abs(dr) > 0.05) return dr
      return (b.total_count ?? 0) - (a.total_count ?? 0)
    })
    .slice(0, 12)
  const has = rows.length > 0

  const totalCalls = rows.reduce((s, h) => s + (h.total_count ?? 0), 0)
  const unstable = rows.reduce<HookStat | undefined>((top, h) => {
    if (!top) return h
    return (h.failure_rate ?? 0) > (top.failure_rate ?? 0) ? h : top
  }, undefined)
  const slowest = rows.reduce<HookStat | undefined>((top, h) => {
    if (!top) return h
    return (h.avg_duration_ms ?? 0) > (top.avg_duration_ms ?? 0) ? h : top
  }, undefined)

  return (
    <ChartCard
      title="Hook 运行时间与效果"
      description="每个 hook 的成功/取消/失败分布与平均耗时。"
      insight={
        has ? (
          <>
            共 <strong>{totalCalls.toLocaleString()}</strong> 次调用；最不稳定{' '}
            <strong className="font-mono">{unstable?.hook_name}</strong>（失败率{' '}
            {fmtRate(unstable?.failure_rate)}，涉及 {unstable?.session_count ?? 0} session）
            {slowest && slowest !== unstable ? (
              <>
                ，最慢 <strong className="font-mono">{slowest.hook_name}</strong>（
                {fmtMs(slowest.avg_duration_ms)}）
              </>
            ) : null}
            。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 hook 执行数据。'
        )
      }
      height={340}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={rows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="hook_name"
              width={120}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip content={<HookTooltip />} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar dataKey="success_count" stackId="h" name="成功" fill="rgb(var(--primary))" />
            <Bar dataKey="cancelled_count" stackId="h" name="取消" fill="rgb(var(--muted-foreground))" radius={[0, 0, 0, 0]} />
            <Bar dataKey="error_count" stackId="h" name="失败" fill="rgb(var(--destructive))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 hook 执行数据。'} />
      )}
    </ChartCard>
  )
}
