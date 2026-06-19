import * as React from 'react'
import { useState } from 'react'
import { useDetail } from '@/api/hooks'
import { useFilters } from '@/hooks/useFilters'
import type { DetailKind } from '@/api/types'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

type Row = Record<string, unknown>
interface Col {
  header: string
  cell: (row: Row) => React.ReactNode
}
interface KindCfg {
  label: string
  listKey: string
  cols: Col[]
}

function basename(p: string): string {
  const segs = p.replace(/\/+$/, '').split('/')
  return segs[segs.length - 1] || p
}
function compact(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return n.toLocaleString()
}

const KINDS: Record<DetailKind, KindCfg> = {
  failures: {
    label: '失败样例',
    listKey: 'samples',
    cols: [
      { header: '工具', cell: (r) => <span className="font-mono">{String(r.tool ?? '-')}</span> },
      { header: '原因', cell: (r) => String(r.reason ?? r.kind ?? '-') },
      { header: '模型', cell: (r) => String(r.model ?? '-') },
      { header: '项目', cell: (r) => <span className="text-muted-foreground">{basename(String(r.project ?? '-'))}</span> },
    ],
  },
  commands: {
    label: '命令',
    listKey: 'by_command',
    cols: [
      { header: '命令', cell: (r) => <span className="font-mono">{String(r.command_name ?? '-')}</span> },
      { header: '调用', cell: (r) => Number(r.call_count ?? 0).toLocaleString() },
      { header: '失败', cell: (r) => Number(r.failure_count ?? 0).toLocaleString() },
      { header: '失败率', cell: (r) => `${Number(r.failure_rate ?? 0).toFixed(2)}%` },
    ],
  },
  tokens: {
    label: 'Token',
    listKey: 'by_model',
    cols: [
      { header: '模型', cell: (r) => <span className="font-mono">{String(r.model ?? '-')}</span> },
      { header: '请求', cell: (r) => Number(r.request_count ?? 0).toLocaleString() },
      {
        header: 'Token',
        cell: (r) =>
          compact(
            Number(r.total_tokens ?? 0) ||
              Number(r.input_tokens ?? 0) + Number(r.output_tokens ?? 0),
          ),
      },
    ],
  },
  sessions: {
    label: 'Session',
    listKey: 'long_running',
    cols: [
      { header: 'Session', cell: (r) => <span className="font-mono">{String(r.session_id ?? '-').slice(0, 8)}</span> },
      { header: '项目', cell: (r) => <span className="text-muted-foreground">{basename(String(r.project ?? '-'))}</span> },
      { header: '状态', cell: (r) => String(r.status ?? r.outcome ?? '-') },
    ],
  },
  tools: {
    label: '工具',
    listKey: 'by_category',
    cols: [
      { header: '类别', cell: (r) => <span className="font-mono">{String(r.category ?? '-')}</span> },
      { header: '调用', cell: (r) => Number(r.call_count ?? 0).toLocaleString() },
      { header: '错误', cell: (r) => Number(r.error_count ?? 0).toLocaleString() },
      { header: '错误率', cell: (r) => `${Number(r.error_rate ?? 0).toFixed(2)}%` },
    ],
  },
}

export function DrilldownPanel() {
  const [filters] = useFilters()
  const [kind, setKind] = useState<DetailKind>('failures')
  const cfg = KINDS[kind]
  const q = useDetail(kind, filters)
  const list = (q.data?.[cfg.listKey] as Row[] | undefined) ?? []
  const insights = (q.data?.insights as string[] | undefined) ?? []

  return (
    <div>
      <div className="flex flex-wrap gap-1 rounded-lg bg-secondary p-1">
        {(Object.keys(KINDS) as DetailKind[]).map((k) => (
          <button
            key={k}
            onClick={() => setKind(k)}
            className={cn(
              'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
              kind === k
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {KINDS[k].label}
          </button>
        ))}
      </div>

      {insights.length > 0 && <p className="mt-3 text-sm text-muted-foreground">{insights[0]}</p>}

      <div className="mt-3 overflow-x-auto rounded-lg border">
        <table className="w-full text-sm">
          <thead className="bg-secondary text-xs text-muted-foreground">
            <tr>
              {cfg.cols.map((c) => (
                <th key={c.header} className="px-3 py-2 text-left font-medium">
                  {c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {q.isLoading ? (
              <tr>
                <td colSpan={cfg.cols.length} className="px-3 py-4">
                  <Skeleton className="h-6 w-full" />
                </td>
              </tr>
            ) : list.length === 0 ? (
              <tr>
                <td colSpan={cfg.cols.length} className="px-3 py-6 text-center text-muted-foreground">
                  无数据
                </td>
              </tr>
            ) : (
              list.slice(0, 20).map((row, i) => (
                <tr key={i} className="border-t hover:bg-secondary/50">
                  {cfg.cols.map((c, j) => (
                    <td key={j} className="max-w-[280px] truncate px-3 py-2">
                      {c.cell(row)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
