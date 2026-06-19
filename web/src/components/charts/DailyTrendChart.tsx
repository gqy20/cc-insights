import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { DailyTrend } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

interface Props {
  trend?: DailyTrend
  loading?: boolean
}

export function DailyTrendChart({ trend, loading }: Props) {
  const hasData = !!trend && trend.counts.length > 0
  const rows = hasData ? trend.dates.map((d, i) => ({ date: d, count: trend.counts[i] })) : []
  const total = hasData ? trend.counts.reduce((a, b) => a + b, 0) : 0
  const avg = hasData ? Math.round(total / trend.counts.length) : 0
  const peakIdx = hasData ? trend.counts.indexOf(Math.max(...trend.counts)) : -1

  return (
    <ChartCard
      title="每日活动趋势"
      description="按日期汇总消息量，识别活跃峰值与近期走势。"
      insight={
        hasData ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> 条，日均{' '}
            <strong>{avg.toLocaleString()}</strong>，峰值在{' '}
            <strong>{trend.dates[peakIdx]}</strong>。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有活动消息。'
        )
      }
    >
      {hasData ? (
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={rows} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
            <XAxis
              dataKey="date"
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
              minTickGap={24}
            />
            <YAxis
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
              width={40}
            />
            <Tooltip
              contentStyle={{
                background: 'rgb(var(--card))',
                border: '1px solid rgb(var(--border))',
                borderRadius: 8,
                fontSize: 12,
              }}
              labelStyle={{ color: 'rgb(var(--foreground))' }}
            />
            <Line
              type="monotone"
              dataKey="count"
              name="消息数"
              stroke="rgb(var(--primary))"
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4 }}
            />
          </LineChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有活动消息。'} />
      )}
    </ChartCard>
  )
}
