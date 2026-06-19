import * as React from 'react'
import { ArrowDownRight, ArrowUpRight } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Card } from '@/components/ui/card'
import { Sparkline } from '@/components/charts/Sparkline'

interface KpiCardProps {
  label: string
  value: React.ReactNode
  delta?: number
  deltaGoodWhenUp?: boolean
  spark?: number[]
  unit?: string
  className?: string
}

// §3.5 KPI 卡：标签 + delta + 大号 tabular 数字 + sparkline。
export function KpiCard({
  label,
  value,
  delta,
  deltaGoodWhenUp = true,
  spark,
  unit,
  className,
}: KpiCardProps) {
  const hasDelta = typeof delta === 'number' && Number.isFinite(delta)
  const up = (delta ?? 0) >= 0
  const good = deltaGoodWhenUp ? up : !up
  const sparkColor = good ? 'text-success' : 'text-destructive'

  return (
    <Card className={cn('p-4', className)}>
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {label}
        </span>
        {hasDelta && (
          <span
            className={cn(
              'inline-flex shrink-0 items-center gap-0.5 text-xs font-semibold',
              good ? 'text-success' : 'text-destructive',
            )}
          >
            {up ? <ArrowUpRight className="h-3 w-3" /> : <ArrowDownRight className="h-3 w-3" />}
            {Math.abs(delta as number).toFixed(2)}%
          </span>
        )}
      </div>
      <div className="mt-2 font-mono text-2xl font-bold tracking-tight tabular-nums">
        {value}
        {unit && <span className="ml-1 text-sm font-normal text-muted-foreground">{unit}</span>}
      </div>
      {spark && spark.length > 1 && (
        <div className="mt-3 h-8">
          <Sparkline data={spark} className={sparkColor} />
        </div>
      )}
    </Card>
  )
}
