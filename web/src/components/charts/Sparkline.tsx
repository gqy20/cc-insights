import { Line, LineChart, ResponsiveContainer } from 'recharts'
import { cn } from '@/lib/utils'

interface SparklineProps {
  data: number[]
  className?: string
}

// KPI 卡用的迷你趋势线（无轴无网格），颜色由父元素 currentColor 决定。
export function Sparkline({ data, className }: SparklineProps) {
  const points = data.map((v, i) => ({ i, v }))
  return (
    <div className={cn('h-full w-full', className)}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={points} margin={{ top: 2, right: 0, bottom: 2, left: 0 }}>
          <Line
            type="monotone"
            dataKey="v"
            stroke="currentColor"
            strokeWidth={1.5}
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
