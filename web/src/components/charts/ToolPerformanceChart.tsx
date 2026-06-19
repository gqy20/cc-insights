import {
  Bar,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { ToolPerformanceData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function ToolPerformanceChart({
  data,
  loading,
}: {
  data?: ToolPerformanceData
  loading?: boolean
}) {
  const rows = (data?.by_category ?? [])
    .slice()
    .sort((a, b) => b.call_count - a.call_count)
    .slice(0, 12)
  const has = rows.length > 0
  const overallRate = data?.overall_error_rate ?? 0
  const slowest = rows.slice().sort((a, b) => (b.avg_duration_ms ?? 0) - (a.avg_duration_ms ?? 0))[0]

  return (
    <ChartCard
      title="工具性能"
      description="按工具类别的调用数（柱）与错误率（线），定位慢与易错工具。"
      insight={
        has ? (
          <>
            整体错误率 <strong>{overallRate.toFixed(1)}%</strong>
            {slowest && (
              <>
                ，最慢 <strong className="font-mono">{slowest.category}</strong>（均值{' '}
                {((slowest.avg_duration_ms ?? 0) / 1000).toFixed(1)}s）
              </>
            )}
            。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有工具性能数据。'
        )
      }
      height={380}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={rows} margin={{ top: 8, right: 16, bottom: 24, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
            <XAxis
              dataKey="category"
              stroke="rgb(var(--muted-foreground))"
              fontSize={10}
              tickLine={false}
              axisLine={false}
              interval={0}
              angle={-35}
              textAnchor="end"
              height={56}
            />
            <YAxis
              yAxisId="left"
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
              width={44}
            />
            <YAxis
              yAxisId="right"
              orientation="right"
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
              width={40}
              unit="%"
            />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar yAxisId="left" dataKey="call_count" name="调用数" fill="rgb(var(--primary))" radius={[4, 4, 0, 0]} />
            <Line
              yAxisId="right"
              type="monotone"
              dataKey="error_rate"
              name="错误率%"
              stroke="rgb(var(--destructive))"
              strokeWidth={2}
              dot={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有工具性能数据。'} />
      )}
    </ChartCard>
  )
}
