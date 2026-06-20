import { useState } from 'react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { CostAnalysisData, CostByModel } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

// 成本四分类配色
const COST_COLORS = {
  input: 'rgb(217 119 6)', // amber-600 输入
  output: 'rgb(194 65 12)', // orange-700 输出
  cacheRead: 'rgb(22 163 74)', // green-600 缓存读
  cacheCreation: 'rgb(124 58 237)', // violet-600 缓存写
}

function compact(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return n.toLocaleString()
}

function formatCNY(n: number): string {
  if (n >= 1000) return `¥${n.toFixed(0)}`
  if (n >= 1) return `¥${n.toFixed(2)}`
  return `¥${n.toFixed(3)}`
}

type Metric = 'cost' | 'tokens'

export function CostAnalysisChart({
  data,
  loading,
}: {
  data?: CostAnalysisData
  loading?: boolean
}) {
  const [metric, setMetric] = useState<Metric>('cost')

  const rows = (data?.by_model ?? []).slice()
  const has = rows.length > 0
  const totalTokens = rows.reduce((s, x) => s + (x.total_tokens ?? 0), 0)
  const totalCost = rows.reduce((s, x) => s + (x.cost_cny ?? 0), 0)
  const top = rows[0]
  const isCost = metric === 'cost'

  const valueOf = (x: CostByModel): number =>
    isCost ? Number(x.cost_cny ?? 0) : Number(x.total_tokens ?? 0)
  const displayRows = rows
    .slice()
    .sort((a, b) => valueOf(b) - valueOf(a))
    .slice(0, 12)

  // 总成本按分类拆分（用于洞察文字）
  const breakdown = rows.reduce(
    (s, x) => ({
      input: s.input + Number(x.input_cost_cny ?? 0),
      output: s.output + Number(x.output_cost_cny ?? 0),
      cacheRead: s.cacheRead + Number(x.cache_read_cost_cny ?? 0),
      cacheCreation: s.cacheCreation + Number(x.cache_creation_cost_cny ?? 0),
    }),
    { input: 0, output: 0, cacheRead: 0, cacheCreation: 0 },
  )

  return (
    <ChartCard
      title="Token 成本"
      description="按模型拆解 Token 消耗与人民币成本（输入/输出/缓存读/缓存写），定位成本主力。"
      insight={
        has ? (
          <>
            共 <strong>{compact(totalTokens)}</strong> Token、成本{' '}
            <strong>{formatCNY(totalCost)}</strong>（输入 {formatCNY(breakdown.input)} / 输出{' '}
            {formatCNY(breakdown.output)} / 缓存读 {formatCNY(breakdown.cacheRead)}），主力{' '}
            <strong className="font-mono">{top.model}</strong>。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Token 成本数据。'
        )
      }
      height={360}
    >
      <div className="mb-2 flex gap-1">
        {(['cost', 'tokens'] as Metric[]).map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => setMetric(m)}
            className={`rounded px-2 py-0.5 text-xs transition-colors ${
              metric === m
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted text-muted-foreground hover:bg-muted/80'
            }`}
          >
            {m === 'cost' ? '成本 ¥' : 'Token'}
          </button>
        ))}
      </div>
      {has ? (
        <ResponsiveContainer width="100%" height="80%">
          <BarChart layout="vertical" data={displayRows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis
              type="number"
              tickFormatter={(v: number) => (isCost ? formatCNY(v) : compact(v))}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
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
              formatter={(value, name) => {
                const v = Number(value)
                return isCost ? [formatCNY(v), name] : [`${compact(v)} Token`, name]
              }}
            />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            {isCost ? (
              <>
                <Bar dataKey="input_cost_cny" name="输入" stackId="m" fill={COST_COLORS.input} />
                <Bar dataKey="output_cost_cny" name="输出" stackId="m" fill={COST_COLORS.output} />
                <Bar dataKey="cache_read_cost_cny" name="缓存读" stackId="m" fill={COST_COLORS.cacheRead} />
                <Bar dataKey="cache_creation_cost_cny" name="缓存写" stackId="m" fill={COST_COLORS.cacheCreation} />
              </>
            ) : (
              <>
                <Bar dataKey="input_tokens" name="输入" stackId="m" fill={COST_COLORS.input} />
                <Bar dataKey="output_tokens" name="输出" stackId="m" fill={COST_COLORS.output} />
                <Bar dataKey="cache_read_input_tokens" name="缓存读" stackId="m" fill={COST_COLORS.cacheRead} />
                <Bar dataKey="cache_creation_input_tokens" name="缓存写" stackId="m" fill={COST_COLORS.cacheCreation} />
              </>
            )}
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Token 成本数据。'} />
      )}
    </ChartCard>
  )
}
