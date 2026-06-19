import { useDiagnostics } from '@/api/hooks'
import { useFilters } from '@/hooks/useFilters'
import { Skeleton } from '@/components/ui/skeleton'
import { DiagnosticCard } from './DiagnosticCard'

export function DiagnosticList() {
  const [filters] = useFilters()
  const q = useDiagnostics(filters)

  if (q.isLoading) {
    return (
      <div className="grid gap-3 md:grid-cols-2">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-40 rounded-lg" />
        ))}
      </div>
    )
  }

  if (q.isError) {
    return <p className="text-sm text-destructive">诊断加载失败。</p>
  }

  const recs = q.data?.recommendations ?? []
  if (recs.length === 0) {
    return <p className="text-sm text-muted-foreground">该时间范围内没有诊断建议。</p>
  }

  return (
    <div className="grid gap-3 md:grid-cols-2">
      {recs.map((f) => (
        <DiagnosticCard key={f.id} finding={f} />
      ))}
    </div>
  )
}
