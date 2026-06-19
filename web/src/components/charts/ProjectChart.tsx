import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { ProjectStatItem } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

function basename(p: string): string {
  const segs = p.replace(/\/+$/, '').split('/')
  const name = segs[segs.length - 1] || p
  return name.length > 22 ? name.slice(0, 21) + '…' : name
}

export function ProjectChart({
  data,
  loading,
}: {
  data?: ProjectStatItem[]
  loading?: boolean
}) {
  const has = !!data && data.length > 0
  const top = has ? [...data].sort((a, b) => b.message_count - a.message_count).slice(0, 12) : []
  const rows = top.map((p) => ({ ...p, name: basename(p.project) }))

  return (
    <ChartCard
      title="项目活跃度"
      description="项目维度的消息量与会话活跃排名。"
      insight={
        has ? (
          <>
            最活跃 <strong className="font-mono">{basename(top[0].project)}</strong>：
            <strong>{top[0].message_count.toLocaleString()}</strong> 条消息。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有项目活跃数据。'
        )
      }
      height={420}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={rows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="name"
              width={110}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              formatter={(v: number) => [v.toLocaleString(), '消息']}
              labelFormatter={(_, p) => p?.[0]?.payload?.project ?? ''}
            />
            <Bar dataKey="message_count" name="消息" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有项目活跃数据。'} />
      )}
    </ChartCard>
  )
}
