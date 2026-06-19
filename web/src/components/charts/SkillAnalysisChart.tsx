import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { SkillAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function SkillAnalysisChart({
  data,
  loading,
}: {
  data?: SkillAnalysisData
  loading?: boolean
}) {
  // 只展示有实际调用的 skill，按 invocation_count 排行
  const rows = (data?.skills ?? [])
    .filter((s) => (s.invocation_count ?? 0) > 0)
    .slice()
    .sort((a, b) => (b.invocation_count ?? 0) - (a.invocation_count ?? 0))
    .slice(0, 12)
  const has = rows.length > 0
  const installed = data?.total_installed ?? 0
  const total = data?.total_invocations ?? 0

  return (
    <ChartCard
      title="Skill 使用"
      description="按 invocation_count 排序的 Skill 调用排行。"
      insight={
        has ? (
          <>
            已安装 <strong>{installed}</strong> 个 Skill，共调用{' '}
            <strong>{total.toLocaleString()}</strong> 次；最频繁{' '}
            <strong className="font-mono">{rows[0].name}</strong>（{rows[0].invocation_count}）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Skill 调用。'
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
              dataKey="name"
              width={150}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              formatter={(v: number) => [v.toLocaleString(), '调用']}
            />
            <Bar dataKey="invocation_count" name="调用" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Skill 调用。'} />
      )}
    </ChartCard>
  )
}
