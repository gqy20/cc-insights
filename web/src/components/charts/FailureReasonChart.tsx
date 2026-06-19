import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { FailureAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function FailureReasonChart({
  data,
  loading,
}: {
  data?: FailureAnalysisData
  loading?: boolean
}) {
  const rows = (data?.by_reason ?? []).slice().sort((a, b) => b.count - a.count).slice(0, 15)
  const has = rows.length > 0
  const total = data?.total_failures ?? rows.reduce((s, x) => s + x.count, 0)
  const top = rows[0]

  return (
    <ChartCard
      title="失败原因"
      description="按原因、类别拆解失败来源。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> 次失败，主因{' '}
            <strong className="font-mono">{top.reason}</strong>（{top.count} 次）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有失败原因细分数据。'
        )
      }
      height={420}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={rows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="reason"
              width={120}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--destructive) / 0.15)' }} />
            <Bar dataKey="count" name="次数" fill="rgb(var(--destructive))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有失败原因细分数据。'} />
      )}
    </ChartCard>
  )
}
