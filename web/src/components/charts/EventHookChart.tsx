import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { EventAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function EventHookChart({
  data,
  loading,
}: {
  data?: EventAnalysisData
  loading?: boolean
}) {
  const rows = (data?.by_type ?? [])
    .slice()
    .sort((a, b) => b.count - a.count)
    .slice(0, 15)
  const has = rows.length > 0
  const total = data?.total_events ?? rows.reduce((s, x) => s + x.count, 0)
  const top = rows[0]

  return (
    <ChartCard
      title="事件类型"
      description="按类型聚合的运行事件量，含 hook/skill 信号。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> 事件，最频繁{' '}
            <strong className="font-mono">{top.type}</strong>（{top.count.toLocaleString()}）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有事件数据。'
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
              dataKey="type"
              width={120}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar dataKey="count" name="次数" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有事件数据。'} />
      )}
    </ChartCard>
  )
}
