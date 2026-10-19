import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { ContextAnalysisData } from '@/api/types'
import { ChartCard, ChartEmpty } from './ChartCard'

const tooltipStyle = {
  background: 'rgb(var(--card))',
  border: '1px solid rgb(var(--border))',
  borderRadius: 8,
  fontSize: 12,
}

function compact(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return n.toLocaleString()
}

// 上下文质量卡：缓存命中率 / 压缩次数 / 输入规模 / reasoning 占比。
// 数据来自 cost_analysis.context（codex/zcode 渠道按天快照重建，claude 无此信号）。
export function ContextStatsChart({
  data,
  loading,
}: {
  data?: ContextAnalysisData
  loading?: boolean
}) {
  const has =
    !!data &&
    ((data.token_events ?? 0) > 0 ||
      (data.context_compactions ?? 0) > 0 ||
      (data.max_input_tokens ?? 0) > 0)

  const rows = [
    { key: '缓存读', value: data?.total_cached_input_tokens ?? 0 },
    { key: '输入', value: Math.max(0, (data?.token_events ?? 0) * (data?.avg_input_tokens ?? 0) - (data?.total_cached_input_tokens ?? 0)) },
    { key: 'reasoning', value: data?.total_reasoning_tokens ?? 0 },
  ]
  const cacheHit = data?.cache_hit_rate ?? 0
  const reasoningRatio = data?.reasoning_output_ratio ?? 0

  return (
    <ChartCard
      title="上下文质量"
      description="缓存命中、压缩与输入规模（cost_analysis.context）"
      height={220}
    >
      {!has ? (
        <ChartEmpty text="当前渠道无上下文统计信号" />
      ) : (
        <div className="flex h-full flex-col gap-3">
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div className="rounded-md border p-3">
              <div className="text-muted-foreground">缓存命中率</div>
              <div className="mt-1 font-mono text-lg font-semibold">{cacheHit.toFixed(1)}%</div>
            </div>
            <div className="rounded-md border p-3">
              <div className="text-muted-foreground">上下文压缩</div>
              <div className="mt-1 font-mono text-lg font-semibold">
                {data?.context_compactions ?? 0} 次
              </div>
            </div>
            <div className="rounded-md border p-3">
              <div className="text-muted-foreground">平均输入 / 最大输入</div>
              <div className="mt-1 font-mono text-lg font-semibold">
                {compact(data?.avg_input_tokens ?? 0)} / {compact(data?.max_input_tokens ?? 0)}
              </div>
            </div>
            <div className="rounded-md border p-3">
              <div className="text-muted-foreground">reasoning 输出占比</div>
              <div className="mt-1 font-mono text-lg font-semibold">{reasoningRatio.toFixed(1)}%</div>
            </div>
          </div>
          <div className="min-h-0 flex-1">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={rows} margin={{ top: 4, right: 8, bottom: 0, left: 8 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="key" fontSize={11} tickLine={false} axisLine={false} />
                <YAxis fontSize={11} tickLine={false} axisLine={false} tickFormatter={compact} width={44} />
                <Tooltip
                  contentStyle={tooltipStyle}
                  formatter={(v: number) => [compact(v), 'tokens']}
                />
                <Bar dataKey="value" fill="rgb(22 163 74)" radius={[4, 4, 0, 0]} name="tokens" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}
    </ChartCard>
  )
}
