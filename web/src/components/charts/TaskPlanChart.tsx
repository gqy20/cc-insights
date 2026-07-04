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
  const fileRows = (data?.plan_files ?? [])
    .slice()
    .sort((a, b) => (b.char_count ?? 0) - (a.char_count ?? 0))
  const stepRows = (data?.plan_steps?.by_status ?? [])
    .filter((s) => (s.count ?? 0) > 0)
    .slice()
    .sort((a, b) => (b.count ?? 0) - (a.count ?? 0))
  const topSteps = (data?.plan_steps?.top_steps ?? [])
    .filter((s) => (s.count ?? 0) > 0)
    .slice(0, 5)
  const hasFiles = fileRows.length > 0
  const hasSteps = stepRows.length > 0
  const lifecycle = data?.plan_lifecycle ?? {}
  const entries = lifecycle.entry_count ?? 0
  const exits = lifecycle.exit_count ?? 0
  const stepSummary = data?.plan_steps
  const title = hasFiles ? 'Plan 文件' : hasSteps ? 'Plan 步骤' : 'Plan'
  const description = hasFiles
    ? '活跃 Plan 文件规模，观察计划沉淀与轮换。'
    : hasSteps
      ? 'Codex update_plan 步骤状态，观察计划推进和重入。'
      : '计划使用情况。'

  return (
    <ChartCard
      title={title}
      description={description}
      insight={
        hasFiles ? (
          <>
            <strong>{fileRows.length}</strong> 个 Plan 文件，进入 {entries} / 退出 {exits}
            ；最大 <strong className="font-mono">{fileRows[0].file_name}</strong>（{(fileRows[0].char_count ?? 0).toLocaleString()} 字符）。
          </>
        ) : hasSteps ? (
          <>
            <strong>{stepSummary?.update_events?.toLocaleString() ?? 0}</strong> 次 update_plan，
            共 <strong>{stepSummary?.total_steps?.toLocaleString() ?? 0}</strong> 个步骤；
            进入 {entries} / 退出 {exits}，涉及 {lifecycle.sessions_with_plan ?? 0} 个 session。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Plan 数据。'
        )
      }
      height={340}
    >
      {hasFiles ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart layout="vertical" data={fileRows} margin={{ top: 4, right: 16, bottom: 4, left: 8 }}>
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
      ) : hasSteps ? (
        <div className="grid h-full grid-rows-[minmax(0,1fr)_auto] gap-3">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={stepRows} margin={{ top: 8, right: 12, bottom: 8, left: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
              <XAxis
                dataKey="status"
                stroke="rgb(var(--muted-foreground))"
                fontSize={11}
                tickLine={false}
                axisLine={false}
              />
              <YAxis stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} />
              <Tooltip
                contentStyle={tooltipStyle}
                cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
                formatter={(v: number) => [v.toLocaleString(), '步骤']}
              />
              <Bar dataKey="count" name="步骤" fill="rgb(var(--primary))" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
          {topSteps.length > 0 && (
            <div className="grid grid-cols-1 gap-1.5 text-xs text-muted-foreground">
              {topSteps.map((step) => (
                <div key={`${step.step}-${step.last_status}`} className="flex min-w-0 items-center justify-between gap-3 rounded-md bg-secondary/50 px-2 py-1.5">
                  <span className="min-w-0 truncate font-mono text-foreground">{step.step}</span>
                  <span className="shrink-0">
                    {(step.count ?? 0).toLocaleString()} · {step.last_status ?? 'unknown'}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Plan 数据。'} />
      )}
    </ChartCard>
  )
}
