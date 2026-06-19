import type { SkillAnalysisData } from '@/api/types'
import { ChartCard } from './ChartCard'

// skill_analysis 为标量统计（无序列），以 KPI 网格呈现。
export function SkillAnalysisChart({
  data,
  loading,
}: {
  data?: SkillAnalysisData
  loading?: boolean
}) {
  const items: { label: string; value?: number }[] = [
    { label: '已安装', value: data?.total_installed },
    { label: '总调用', value: data?.total_invocations },
    { label: '工具调用', value: data?.tool_use_invocations },
    { label: '附件调用', value: data?.attachment_invocations },
    { label: '失败', value: data?.failure_count },
  ]
  const has = items.some((x) => typeof x.value === 'number')

  return (
    <ChartCard
      title="Skill 使用"
      description="已安装与调用统计（标量指标）。"
      insight={has ? undefined : loading ? '加载中…' : '该时间范围内没有 Skill 数据。'}
      height={260}
    >
      {has ? (
        <div className="grid h-full grid-cols-2 content-center gap-3 sm:grid-cols-3">
          {items.map((it) => (
            <div key={it.label} className="rounded-lg bg-secondary p-3">
              <div className="text-xs text-muted-foreground">{it.label}</div>
              <div className="mt-1 font-mono text-xl font-bold tabular-nums">
                {(it.value ?? 0).toLocaleString()}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
          {loading ? '加载中…' : '该时间范围内没有 Skill 数据。'}
        </div>
      )}
    </ChartCard>
  )
}
