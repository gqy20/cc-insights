import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { WeekdayStats } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function WeekdayChart({ stats, loading }: { stats?: WeekdayStats; loading?: boolean }) {
  const rows = stats?.weekday_data ?? []
  const has = rows.length > 0
  const total = rows.reduce((s, x) => s + x.message_count, 0)
  const peak = has ? rows.reduce((a, b) => (b.message_count > a.message_count ? b : a)) : null

  return (
    <ChartCard
      title="星期活动分布"
      description="按星期聚合消息量，观察固定工作节奏。"
      insight={
        has ? (
          <>
            周均 <strong>{Math.round(total / 7).toLocaleString()}</strong> 条，峰值在{' '}
            <strong>{peak!.weekday_name}</strong>（{peak!.message_count.toLocaleString()} 条）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有可展示的星期分布。'
        )
      }
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={rows} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
            <XAxis dataKey="weekday_name" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} width={40} />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar dataKey="message_count" name="消息" fill="rgb(var(--primary))" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有可展示的星期分布。'} />
      )}
    </ChartCard>
  )
}
