import type { DiagnosticFinding } from '@/api/types'
import { Badge } from '@/components/ui/badge'

type Variant = 'destructive' | 'warning' | 'secondary'

const SEVERITY_VARIANT: Record<string, Variant> = {
  critical: 'destructive',
  high: 'destructive',
  medium: 'warning',
  low: 'secondary',
  info: 'secondary',
}

export function DiagnosticCard({ finding }: { finding: DiagnosticFinding }) {
  const sev = (finding.severity ?? 'info').toLowerCase()

  return (
    <div className="flex flex-col rounded-lg border bg-card p-4">
      <div className="flex items-center gap-2">
        <Badge variant={SEVERITY_VARIANT[sev] ?? 'secondary'}>{finding.severity}</Badge>
        <span className="text-xs text-muted-foreground">{finding.category}</span>
        {finding.confidence && (
          <span className="ml-auto text-xs text-muted-foreground">
            置信度 {finding.confidence}
          </span>
        )}
      </div>

      <h3 className="mt-2 font-semibold leading-snug">{finding.title}</h3>
      <p className="mt-1.5 text-sm text-muted-foreground">{finding.summary}</p>

      {finding.evidence?.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {finding.evidence.slice(0, 6).map((e, i) => (
            <span key={i} className="rounded bg-secondary px-2 py-0.5 text-xs">
              <span className="text-muted-foreground">{e.label}:</span>{' '}
              <span className="font-medium text-foreground">{e.value}</span>
            </span>
          ))}
        </div>
      )}

      {!!finding.actions && finding.actions.length > 0 && (
        <ul className="mt-3 space-y-1 text-sm">
          {finding.actions.slice(0, 3).map((a, i) => (
            <li key={i} className="text-muted-foreground">
              <span className="font-medium text-foreground">{a.target}</span>：{a.action}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
