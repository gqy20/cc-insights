import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { SessionStats } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

export function SessionChart({ stats, loading }: { stats?: SessionStats; loading?: boolean }) {
  const map = stats?.daily_session_map ?? {}
  const rows = Object.entries(map)
    .map(([date, count]) => ({ date, count }))
    .sort((a, b) => a.date.localeCompare(b.date))
  const has = rows.length > 0
  const total = stats?.total_sessions ?? rows.reduce((s, x) => s + x.count, 0)
  const peak = stats?.peak_date
  const peakCount = stats?.peak_count

  return (
    <ChartCard
      title="Session 趋势"
      description="每日 Session 数，识别活跃日与空档。"
      insight={
        has ? (
          <>
            共 <strong>{total.toLocaleString()}</strong> Session
            {peak && (
              <>
                ，峰值 <strong>{peak}</strong>（{peakCount ?? 0}）
              </>
            )}
            。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有 Session 数据。'
        )
      }
      height={300}
    >
      {has ? (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={rows} margin={{ top: 8, right: 16, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--border))" vertical={false} />
            <XAxis dataKey="date" stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} minTickGap={24} />
            <YAxis stroke="rgb(var(--muted-foreground))" fontSize={11} tickLine={false} axisLine={false} width={36} />
            <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgb(var(--accent) / 0.3)' }} />
            <Bar dataKey="count" name="Session" fill="rgb(var(--primary))" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有 Session 数据。'} />
      )}
    </ChartCard>
  )
}
