import { useFilters } from '@/hooks/useFilters'
import type { Filters } from '@/api/types'
import { Button } from '@/components/ui/button'

const FILTERS: { key: keyof Filters; placeholder: string }[] = [
  { key: 'project', placeholder: '项目过滤' },
  { key: 'tool', placeholder: '工具过滤' },
  { key: 'model', placeholder: '模型过滤' },
  { key: 'reason', placeholder: '失败原因' },
]

// §3.4 顶部 filter 条：项目/工具/模型/原因，全部双向同步到 URL search params。
export function TopFilterBar() {
  const [filters, setFilters] = useFilters()
  const hasFilter = FILTERS.some((f) => filters[f.key])

  return (
    <div className="flex flex-wrap items-center gap-2">
      {FILTERS.map((f) => (
        <input
          key={f.key}
          type="search"
          value={filters[f.key] ?? ''}
          onChange={(e) =>
            setFilters({ [f.key]: e.target.value || undefined } as Partial<Filters>)
          }
          placeholder={f.placeholder}
          className="h-9 min-w-[140px] flex-1 rounded-md border border-input bg-background px-3 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      ))}
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
