import { useDashboardData } from '@/api/hooks'
import { useFilters } from '@/hooks/useFilters'
import type { Filters } from '@/api/types'
import { Button } from '@/components/ui/button'

const fieldClass =
  'h-9 min-w-[150px] flex-1 rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'

// §3.4 顶部 filter 条：项目搜索 + 工具/模型/原因下拉（候选来自当前 dashboard 数据）。
export function TopFilterBar() {
  const [filters, setFilters] = useFilters()
  // 候选用 preset-only 数据（不受当前 filter 影响），避免选中某项后候选缩水、无法直接切换。
  const cq = useDashboardData({ preset: filters.preset })

  const models = (cq.data?.model_usage ?? []).map((m) => m.model)
  const tools = (cq.data?.tool_analysis?.tools ?? [])
    .map((t) => t.tool)
    .filter((t): t is string => !!t)
  const reasons = (cq.data?.failure_analysis?.by_reason ?? []).map((r) => r.reason)

  const hasFilter = (['project', 'tool', 'model', 'reason'] as const).some((k) => filters[k])

  return (
    <div className="flex flex-wrap items-center gap-2">
      <input
        type="search"
        value={filters.project ?? ''}
        onChange={(e) => setFilters({ project: e.target.value || undefined } as Partial<Filters>)}
        placeholder="项目过滤"
        className={fieldClass}
      />

      <select
        value={filters.tool ?? ''}
        onChange={(e) => setFilters({ tool: e.target.value || undefined } as Partial<Filters>)}
        className={fieldClass}
      >
        <option value="">全部工具</option>
        {tools.map((t) => (
          <option key={t} value={t}>
            {t}
          </option>
        ))}
      </select>

      <select
        value={filters.model ?? ''}
        onChange={(e) => setFilters({ model: e.target.value || undefined } as Partial<Filters>)}
        className={fieldClass}
      >
        <option value="">全部模型</option>
        {models.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>

      <select
        value={filters.reason ?? ''}
        onChange={(e) => setFilters({ reason: e.target.value || undefined } as Partial<Filters>)}
        className={fieldClass}
      >
        <option value="">全部原因</option>
        {reasons.map((r) => (
          <option key={r} value={r}>
            {r}
          </option>
        ))}
      </select>

      {hasFilter && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() =>
            setFilters({ project: undefined, tool: undefined, model: undefined, reason: undefined })
          }
        >
          清空
        </Button>
      )}
    </div>
  )
}
