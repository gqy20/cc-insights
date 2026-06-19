import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { CommandAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function CommandFileChart({
  data,
  loading,
}: {
  data?: CommandAnalysisData
  loading?: boolean
}) {
  const rows = (data?.bash_families ?? []).slice().sort((a, b) => b.call_count - a.call_count)
  const has = rows.length > 0
  const total = rows.reduce((s, x) => s + x.call_count, 0)
  const top = rows[0]
  const riskiest = rows.slice().sort((a, b) => (b.failure_rate ?? 0) - (a.failure_rate ?? 0))[0]

  return (
    <ChartCard
      title="命令族"
      description="Bash 命令族调用与失败率分布。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> 次调用，最高失败率族{' '}
            <strong className="font-mono">{riskiest.family}</strong>（
            {(riskiest.failure_rate ?? 0).toFixed(1)}%）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有命令族数据。'
        )
      }
      height={380}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={rows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="family"
              width={110}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              formatter={(v: number, n: string) =>
                n === 'failure_rate' ? [`${(v as number).toFixed(1)}%`, '失败率'] : [v.toLocaleString(), '调用']
              }
            />
            <Bar dataKey="call_count" name="call_count" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有命令族数据。'} />
      )}
    </ChartCard>
  )
}
