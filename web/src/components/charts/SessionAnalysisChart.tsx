import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'
import type { SessionAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

const COLORS = [
  'rgb(var(--success))',
  'rgb(var(--destructive))',
  'rgb(var(--warning))',
  'rgb(var(--muted-foreground))',
]

export function SessionAnalysisChart({
  data,
  loading,
}: {
  data?: SessionAnalysisData
  loading?: boolean
}) {
  const rows = (data?.outcomes ?? []).slice()
  const has = rows.length > 0
  const total = rows.reduce((s, x) => s + x.count, 0)
  const completed = rows.find((x) => x.outcome === 'completed')?.count ?? 0
  const rate = total > 0 ? Math.round((completed / total) * 100) : 0

  return (
    <ChartCard
      title="Session 结局"
      description="completed / failed / cancelled 分布。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> Session，完成率{' '}
            <strong>{rate}%</strong>。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Session 结局数据。'
        )
      }
      height={300}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={rows}
              dataKey="count"
              nameKey="outcome"
              cx="50%"
              cy="50%"
              innerRadius={45}
              outerRadius={80}
              paddingAngle={2}
              label={(e: { outcome: string; percent?: number }) =>
                `${e.outcome} ${((e.percent ?? 0) * 100).toFixed(0)}%`
              }
              labelLine={false}
            >
              {rows.map((_, i) => (
                <Cell key={i} fill={COLORS[i % COLORS.length]} />
              ))}
            </Pie>
            <Tooltip contentStyle={tooltipStyle} />
          </PieChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Session 结局数据。'} />
      )}
    </ChartCard>
  )
}
