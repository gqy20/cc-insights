// Phase 0 占位页：验证 Vite→embed→单二进制闭环，并展示设计 token 是否生效。
// Phase 1 起替换为真实布局（SidebarRail + TopFilterBar + KPI + 图表）。
function App() {
  return (
    <div className="min-h-screen bg-canvas text-ink">
      <div className="mx-auto max-w-5xl px-6 py-16">
        <p className="text-sm font-semibold uppercase tracking-wider text-accent">
          Claude Code 使用诊断
        </p>
        <h1 className="mt-2 text-3xl font-bold">cc-insights</h1>
        <p className="mt-3 max-w-2xl text-muted">
          前端框架已就绪（React + Vite + Tailwind，单二进制 embed）。后续将接入 KPI 卡、
          分组图表与诊断下钻，视觉对标 PostHog 结构 + claude.ai 暖色。
        </p>

        <div className="mt-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Swatch name="canvas" className="border border-border text-ink" style="bg-canvas" />
          <Swatch name="accent" className="text-white" style="bg-accent" />
          <Swatch name="accent-soft" className="text-accent" style="bg-accent-soft" />
          <Swatch name="success" className="text-white" style="bg-success" />
          <Swatch name="warning" className="text-white" style="bg-warning" />
          <Swatch name="danger" className="text-white" style="bg-danger" />
          <Swatch name="surface" className="border border-border text-ink" style="bg-surface" />
          <Swatch name="ink" className="text-canvas" style="bg-ink" />
        </div>

        <p className="mt-8 font-mono text-sm text-muted tabular-nums">
          tabular-nums：128,493 ↑ 12.4% &nbsp;·&nbsp; cmd: /project/cc-insights
        </p>
      </div>
    </div>
  )
}

function Swatch({
  name,
  className,
  style,
}: {
  name: string
  className: string
  style: string
}) {
  return (
    <div className={`rounded-lg p-3 text-xs ${style} ${className}`}>
      <div className="font-semibold">{name}</div>
    </div>
  )
}

export default App
