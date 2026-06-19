import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { WorkHoursStats } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

// §7.5 用 Cell 条件着色复刻 ECharts visualMap：工作时段（9-18）赤陶，其余 muted。
export function WorkHoursChart({ stats, loading }: { stats?: WorkHoursStats; loading?: boolean }) {
  const rows = stats?.hourly_data ?? []
  const has = rows.length > 0
  const total = rows.reduce((s, x) => s + x.count, 0)
  const workTotal = rows.filter((x) => x.is_work_hour).reduce((s, x) => s + x.count, 0)
  const workPct = total > 0 ? Math.round((workTotal / total) * 100) : 0

  return (
    <ChartCard
      title="工作时段"
      description="按小时统计活动，定位高峰时段与离散工作模式。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> 条，其中{' '}
            <strong>{workPct}%</strong> 在工作时段（9-18 点）。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有可展示的工作时段分布。'
        )
      }
      height={340}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={rows} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
            <XAxis
              dataKey="hour_label"
              stroke="rgb(var(--muted-foreground))"
              fontSize={10}
              tickLine={false}
              axisLine={false}
              interval={2}
            />
            <YAxis stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} width={40} />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              formatter={(v: number) => [v.toLocaleString(), '活动']}
            />
            <Bar dataKey="count" name="count" radius={[3, 3, 0, 0]}>
              {rows.map((r, i) => (
                <Cell
                  key={i}
                  fill={r.is_work_hour ? 'rgb(var(--primary))' : 'rgb(var(--muted-foreground) / 0.35)'}
                />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有可展示的工作时段分布。'} />
      )}
    </ChartCard>
  )
}
