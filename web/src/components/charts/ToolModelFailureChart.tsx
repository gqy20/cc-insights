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
import type { ToolAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

function shortLabel(model?: string, tool?: string): string {
  const m = (model?.split('/').pop() ?? model ?? '?').slice(0, 10)
  const t = (tool ?? '?').replace(/^mcp__/, '').slice(0, 12)
  return `${m}/${t}`
}

export function ToolModelFailureChart({
  data,
  loading,
}: {
  data?: ToolAnalysisData
  loading?: boolean
}) {
  const rows = (data?.by_model ?? [])
    .slice()
    .sort((a, b) => b.failure_count - a.failure_count)
    .slice(0, 15)
    .map((r) => ({ ...r, label: shortLabel(r.model, r.tool) }))
  const has = rows.length > 0
  const totalFail = data?.total_failures ?? 0
  const worst = rows[0]

  return (
    <ChartCard
      title="模型 × 工具失败"
      description="交叉分析模型与工具的失败数（柱）与失败率（线）。"
      insight={
        has ? (
          <>
            共 <strong>{totalFail.toLocaleString()}</strong> 次失败，最差组合{' '}
            <strong className="font-mono">{worst.label}</strong>（{worst.failure_count} 次，{(worst.failure_rate ?? 0).toFixed(1)}%）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有工具失败或缺失结果关联。'
        )
      }
      height={420}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={rows} margin={{ top: 8, right: 16, bottom: 24, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
            <XAxis
              dataKey="label"
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
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--destructive) / 0.1)' }} />
            <Bar yAxisId="left" dataKey="failure_count" name="失败数" fill="rgb(var(--destructive))" radius={[4, 4, 0, 0]} />
            <Line
              yAxisId="right"
              type="monotone"
              dataKey="failure_rate"
              name="失败率%"
              stroke="rgb(var(--primary))"
              strokeWidth={2}
              dot={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有工具失败或缺失结果关联。'} />
      )}
    </ChartCard>
  )
}
