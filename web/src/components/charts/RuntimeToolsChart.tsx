import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { RuntimeToolSignal } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

// runtime_tools 来自 debug 日志，无 debug 数据时为空。
export function RuntimeToolsChart({
  data,
  loading,
}: {
  data?: RuntimeToolSignal[]
  loading?: boolean
}) {
  const rows = (data ?? [])
    .slice()
    .sort((a, b) => (b.count ?? 0) - (a.count ?? 0))
    .slice(0, 15)
  const has = rows.length > 0

  return (
    <ChartCard
      title="Runtime 工具信号"
      description="从 debug 日志补充的 MCP runtime 工具信号（不代表完整工具调用口径）。"
      coverage={has ? undefined : '无 debug 日志'}
      coverageTone={has ? 'default' : 'warning'}
      insight={
        has
          ? `共 ${rows.length} 个 runtime 工具信号。`
          : loading
            ? '加载中…'
            : '该时间范围内没有 Runtime 工具信号（debug 日志缺失）。'
      }
      height={has ? 340 : 220}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={rows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="tool"
              width={150}
              stroke="rgb(var(--muted-foreground))"
              fontSize={10}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar dataKey="count" name="信号数" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : 'debug 日志缺失，无 runtime 工具信号。'} />
      )}
    </ChartCard>
  )
}
