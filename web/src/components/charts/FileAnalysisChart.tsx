import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { FileAnalysisData } from '@/api/types'
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
  return name.length > 26 ? name.slice(0, 25) + '…' : name
}

export function FileAnalysisChart({
  data,
  loading,
}: {
  data?: FileAnalysisData
  loading?: boolean
}) {
  const rows = (data?.hot_files ?? [])
    .slice()
    .sort((a, b) => b.read_count + b.edit_count - (a.read_count + a.edit_count))
    .slice(0, 12)
    .map((f) => ({ ...f, name: basename(f.path) }))
  const has = rows.length > 0
  const totals = data?.totals
  const hot = rows[0]

  return (
    <ChartCard
      title="文件编辑热度"
      description="高频读写文件，定位核心工作区与改动热点。"
      insight={
        has ? (
          <>
            {totals && (
              <>
                共 <strong>{(totals.unique_files ?? 0).toLocaleString()}</strong> 文件，读{' '}
                <strong>{(totals.total_reads ?? 0).toLocaleString()}</strong> / 改{' '}
                <strong>{(totals.total_edits ?? 0).toLocaleString()}</strong>。{' '}
              </>
            )}
            最热 <strong className="font-mono">{hot.name}</strong>。
          </>
        ) : loading ? (
          '加载中…'
        ) : (
          '该时间范围内没有文件编辑数据。'
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
              width={140}
              stroke="rgb(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              contentStyle={tooltipStyle}
              cursor={{ fill: 'rgb(var(--accent) / 0.3)' }}
              labelFormatter={(_, p) => p?.[0]?.payload?.path ?? ''}
            />
            <Bar dataKey="read_count" stackId="a" name="读" fill="rgb(var(--primary) / 0.55)" />
            <Bar dataKey="edit_count" stackId="a" name="改" fill="rgb(var(--primary))" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <ChartEmpty text={loading ? '加载中…' : '该时间范围内没有文件编辑数据。'} />
      )}
    </ChartCard>
  )
}
