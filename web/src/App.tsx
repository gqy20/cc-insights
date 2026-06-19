import { useFilters } from '@/hooks/useFilters'
import { useOverview, useDashboardData } from '@/api/hooks'
import { PRESETS } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { KpiCard } from '@/components/dashboard/KpiCard'
import { DailyTrendChart } from '@/components/charts/DailyTrendChart'
import { CommandsChart } from '@/components/charts/CommandsChart'
import { ProjectChart } from '@/components/charts/ProjectChart'
import { WeekdayChart } from '@/components/charts/WeekdayChart'
import { ModelChart } from '@/components/charts/ModelChart'
import { WorkHoursChart } from '@/components/charts/WorkHoursChart'
import { FailureReasonChart } from '@/components/charts/FailureReasonChart'
import { CommandFileChart } from '@/components/charts/CommandFileChart'
import { FileAnalysisChart } from '@/components/charts/FileAnalysisChart'
import { ToolModelFailureChart } from '@/components/charts/ToolModelFailureChart'
import { ToolPerformanceChart } from '@/components/charts/ToolPerformanceChart'

function App() {
  const [filters, setFilters] = useFilters()
  const overview = useOverview(filters)
  const dashboard = useDashboardData(filters)

  const s = overview.data?.summary
  const trend = overview.data?.trend

  return (
    <div className="min-h-screen bg-background">
      <div className="w-full px-6 py-6 lg:px-10">
        <header className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wider text-primary">
              Claude Code 使用诊断
            </p>
            <h1 className="mt-1 text-2xl font-bold tracking-tight">cc-insights</h1>
          </div>
          <div className="flex gap-1 rounded-lg bg-secondary p-1">
            {PRESETS.map((p) => (
              <Button
                key={p}
                size="sm"
                variant={filters.preset === p ? 'default' : 'ghost'}
                onClick={() => setFilters({ preset: p })}
                className="px-3 font-mono"
              >
                {p}
              </Button>
            ))}
          </div>
        </header>

        <section className="mt-6 grid grid-cols-2 gap-3 md:grid-cols-4">
          {overview.isLoading || !s || !trend
            ? Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-[112px] rounded-lg" />
              ))
            : (
              <>
                <KpiCard label="消息" value={s.messages.toLocaleString()} spark={trend.messages} />
                <KpiCard
                  label="会话"
                  value={s.sessions.toLocaleString()}
                  spark={trend.sessions}
                />
                <KpiCard
                  label="失败率"
                  value={s.failure_rate.toFixed(2)}
                  unit="%"
                  deltaGoodWhenUp={false}
                  spark={trend.failures}
                />
                <KpiCard label="Token" value={compact(s.tokens)} spark={trend.tokens} />
              </>
            )}
        </section>

        <h2 className="mt-8 text-lg font-semibold tracking-tight">使用</h2>
        <section className="mt-3 grid grid-cols-1 gap-4 xl:grid-cols-2">
          <div className="xl:col-span-2">
            <DailyTrendChart
              trend={dashboard.data?.daily_trend}
              loading={dashboard.isLoading}
            />
          </div>
          <CommandsChart data={dashboard.data?.commands} loading={dashboard.isLoading} />
          <ProjectChart
            data={dashboard.data?.project_stats?.projects}
            loading={dashboard.isLoading}
          />
          <WeekdayChart stats={dashboard.data?.weekday_stats} loading={dashboard.isLoading} />
          <ModelChart data={dashboard.data?.model_usage} loading={dashboard.isLoading} />
          <div className="xl:col-span-2">
            <WorkHoursChart
              stats={dashboard.data?.work_hours_stats}
              loading={dashboard.isLoading}
            />
          </div>
        </section>

        <h2 className="mt-8 text-lg font-semibold tracking-tight">质量</h2>
        <section className="mt-3 grid grid-cols-1 gap-4 xl:grid-cols-2">
          <ToolModelFailureChart
            data={dashboard.data?.tool_analysis}
            loading={dashboard.isLoading}
          />
          <FailureReasonChart
            data={dashboard.data?.failure_analysis}
            loading={dashboard.isLoading}
          />
          <CommandFileChart
            data={dashboard.data?.command_analysis}
            loading={dashboard.isLoading}
          />
          <FileAnalysisChart data={dashboard.data?.file_analysis} loading={dashboard.isLoading} />
          <div className="xl:col-span-2">
            <ToolPerformanceChart
              data={dashboard.data?.tool_performance}
              loading={dashboard.isLoading}
            />
          </div>
        </section>

        {overview.isError && (
          <p className="mt-4 text-sm text-destructive">
            概览加载失败，请确认后端 /api 可达。
          </p>
        )}
      </div>
    </div>
  )
}

function compact(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return n.toLocaleString()
}

export default App
