import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { TaskPlanAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function TaskPlanChart({
  data,
  loading,
}: {
  data?: TaskPlanAnalysisData
  loading?: boolean
}) {
  const rows = (data?.plan_files ?? [])
    .slice()
    .sort((a, b) => (b.char_count ?? 0) - (a.char_count ?? 0))
  const has = rows.length > 0
  const lifecycle = data?.plan_lifecycle ?? {}
  const entries = lifecycle.entry_count ?? 0
  const exits = lifecycle.exit_count ?? 0

  return (
    <ChartCard
      title="Plan 文件"
      description="活跃 Plan 文件规模，观察计划沉淀与轮换。"
      insight={
        has ? (
          <>
            <strong>{rows.length}</strong> 个 Plan 文件，进入 {entries} / 退出 {exits}
            ；最大 <strong className="font-mono">{rows[0].file_name}</strong>（{(rows[0].char_count ?? 0).toLocaleString()} 字符）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Plan 数据。'
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
              dataKey="file_name"
              width={150}
              stroke="rgb(var(--muted-foreground))"
              fontSize={10}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              labelFormatter={(_, p) => p?.[0]?.payload?.file_path ?? ''}
            />
            <Bar dataKey="char_count" name="字符" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Plan 数据。'} />
      )}
    </ChartCard>
  )
}
