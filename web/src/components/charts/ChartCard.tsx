import * as React from 'react'
import { cn } from '@/lib/utils'
import { Card, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface ChartCardProps {
  title: string
  description?: string
  coverage?: string
  coverageTone?: 'default' | 'warning'
  insight?: React.ReactNode
  height?: number | string
  className?: string
  children?: React.ReactNode
}

// §3.6 统一图表卡外壳：title + coverage badge + 图表区 + 洞察条。
export function ChartCard({
  title,
  description,
  coverage,
  coverageTone = 'default',
  insight,
  height = 320,
  className,
  children,
}: ChartCardProps) {
  return (
    <Card className={cn('flex flex-col', className)}>
      <CardHeader className="border-b pb-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle className="text-base">{title}</CardTitle>
            {description && <CardDescription className="mt-1">{description}</CardDescription>}
          </div>
          {coverage && (
            <Badge
              variant={coverageTone === 'warning' ? 'warning' : 'secondary'}
              className="shrink-0"
            >
              {coverage}
            </Badge>
          )}
        </div>
      </CardHeader>
      <div className="flex-1 p-4">
        <div style={{ height }} className="w-full">
          {children}
        </div>
      </div>
      {insight && (
        <div className="border-t border-l-4 border-l-primary bg-secondary/50 px-5 py-3 text-sm text-muted-foreground [&_strong]:text-foreground">
          {insight}
        </div>
      )}
    </Card>
  )
}

export function ChartEmpty({ text }: { text: string }) {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
      {text}
    </div>
  )
}
