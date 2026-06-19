import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { ModelUsageItem } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

function shortModel(m: string): string {
  const tail = m.split('/').pop() ?? m
  return tail.length > 16 ? tail.slice(0, 15) + '…' : tail
}

export function ModelChart({ data, loading }: { data?: ModelUsageItem[]; loading?: boolean }) {
  const has = !!data && data.length > 0
  const rows = has ? [...data].sort((a, b) => b.count - a.count) : []
  const total = rows.reduce((s, x) => s + x.count, 0)

  return (
    <ChartCard
      title="模型使用"
      description="对比不同模型的请求量与 Token 占比。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> 次请求，主力模型{' '}
            <strong className="font-mono">{shortModel(rows[0].model)}</strong>。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有模型使用数据。'
        )
      }
      height={360}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={rows} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
            <XAxis
              dataKey="model"
              tickFormatter={shortModel}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
              interval={0}
            />
            <YAxis stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} width={40} />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              formatter={(v: number, n: string) =>
                n === 'tokens' ? [v.toLocaleString(), 'Token'] : [v.toLocaleString(), '请求']
              }
            />
            <Bar dataKey="count" name="count" fill="rgb(var(--primary))" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有模型使用数据。'} />
      )}
    </ChartCard>
  )
}
