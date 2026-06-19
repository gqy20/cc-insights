import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { CommandStat } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function CommandsChart({ data, loading }: { data?: CommandStat[]; loading?: boolean }) {
  const has = !!data && data.length > 0
  const top = has ? [...data].sort((a, b) => b.count - a.count).slice(0, 15) : []
  const total = has ? data!.reduce((s, x) => s + x.count, 0) : 0
  const topCmd = top[0]

  return (
    <ChartCard
      title="Slash Commands"
      description="常用命令与调用占比，判断日常工作入口。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> 次调用，最频繁{' '}
            <strong className="font-mono">{topCmd.command}</strong>（{topCmd.count} 次）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Slash Command 调用。'
        )
      }
      height={420}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={top} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="command"
              width={92}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar dataKey="count" name="次数" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Slash Command 调用。'} />
      )}
    </ChartCard>
  )
}
