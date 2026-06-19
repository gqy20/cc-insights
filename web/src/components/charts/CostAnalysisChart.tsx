import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { CostAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

function compact(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return n.toLocaleString()
}

export function CostAnalysisChart({
  data,
  loading,
}: {
  data?: CostAnalysisData
  loading?: boolean
}) {
  const rows = (data?.by_model ?? [])
    .slice()
    .sort((a, b) => (b.total_tokens ?? 0) - (a.total_tokens ?? 0))
  const has = rows.length > 0
  const total = rows.reduce((s, x) => s + (x.total_tokens ?? 0), 0)
  const top = rows[0]

  return (
    <ChartCard
      title="Token 成本"
      description="按模型拆解 Token 消耗，定位成本主力。"
      insight={
        has ? (
          <>
            共 <strong>{compact(total)}</strong> Token，主力{' '}
            <strong className="font-mono">{top.model}</strong>（{compact(top.total_tokens ?? 0)}）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Token 成本数据。'
        )
      }
      height={340}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={rows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" tickFormatter={compact} stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="model"
              width={120}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              formatter={(v: number) => [v.toLocaleString(), 'Token']}
            />
            <Bar dataKey="total_tokens" name="Token" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Token 成本数据。'} />
      )}
    </ChartCard>
  )
}
