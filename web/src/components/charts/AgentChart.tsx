import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { AgentAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function AgentChart({
  data,
  loading,
}: {
  data?: AgentAnalysisData
  loading?: boolean
}) {
  const rows = (data?.agents ?? [])
    .slice()
    .sort((a, b) => (b.message_count ?? 0) - (a.message_count ?? 0))
    .slice(0, 12)
  const has = rows.length > 0
  const top = rows[0]
  const main = data?.main_tool_calls ?? 0
  const side = data?.sidechain_tool_calls ?? 0

  return (
    <ChartCard
      title="Agent 活跃度"
      description="主链与子链 Agent 的消息量分布。"
      insight={
        has ? (
          <>
            主链工具调用 <strong>{main.toLocaleString()}</strong>，子链{' '}
            <strong>{side.toLocaleString()}</strong>；最活跃{' '}
            <strong className="font-mono">{top.agent_id}</strong>（{top.message_count?.toLocaleString()} 消息）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Agent 数据。'
        )
      }
      height={340}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={rows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" horizontal={false} />
            <XAxis type="number" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
            <YAxis
              type="category"
              dataKey="agent_id"
              width={110}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar dataKey="message_count" name="消息" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Agent 数据。'} />
      )}
    </ChartCard>
  )
}
