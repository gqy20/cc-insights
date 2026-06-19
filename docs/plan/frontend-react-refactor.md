# Web Dashboard 前端重构计划：迁移到 React + shadcn/ui

> 状态：✅ 已完成实施（2026-06-19，Phase 0–6 全部落地；19 图表 + 诊断 + 下钻 + filter + 暗色 + Anthropic 双字体，单二进制 2.7MB）
> 日期：2026-06-19
> 关联模块：`cmd/insights/static/`、`cmd/insights/main.go`（web 服务）
> 变更：v0.2 新增 §3 设计方向与视觉规范（PostHog 结构 + claude.ai 暖色 + Tremor 组件）；细化 §9 文件级代码量预估

## 一句话总结

把 Web Dashboard 从「纯原生 JS + ECharts + HTML 硬编码进 Go 字符串」迁移到「React + TypeScript + shadcn/ui + Recharts + TanStack Query」组件化栈，前端源码独立到 `web/`，经 Vite 构建为 `static/dist/` 后由 Go `embed` 进单二进制，**保持 `make build` 仍是单文件、零外部运行时依赖**。视觉上对标 **PostHog 的分析 dashboard 结构 + claude.ai 的暖色气质 + Tremor 的组件范式**。

---

## 1. 背景与决策依据

### 1.1 现状

- 前端位于 `cmd/insights/static/`，约 **4200 行手写 JS + 980 行手写 CSS**，零框架、零构建。
- HTML 页面**硬编码在 `main.go` 的 `dashboardPageHandler` 字符串里**，改一个 class 都要重编 Go、重 embed。
- 渲染全靠 `innerHTML` 字符串拼接 + 手写 `escapeHtml`，状态是裸全局变量 `dashboardState`，脚本串行 `await loadScript`。
- 图表用 ECharts（本地化 `echarts.min.js` 1.02 MB），套内置 `wonderland` 主题。
- 视觉是经典 admin 模板审美（深色侧边栏 + 浅色卡片 + 蓝灰 `#2f80d1`），信息密度低，KPI 卡只有数字 + 标签。

### 1.2 为什么「显得低级」（分两层）

- **视觉层**：admin 默认配色、ECharts `wonderland` 土主题、数字无 `tabular-nums`、卡片无层次、无暗色模式、图表标题被 ECharts 与页面各渲染一遍。
- **工程层**：HTML 耦合 Go、`innerHTML` 拼接无组件复用与 XSS 保证、图表配置命令式散落 4 个文件（`charts_runtime.js` 单文件 57K）。

### 1.3 为什么选「彻底重构（C 档）」而非渐进换皮

前置调研排除了两个常见顾虑：

1. **二进制体积不是障碍**。实测当前 `make build` 产物 **2.6 MB**（UPX 前 8.6 MB，UPX `--best --lzma` 压缩比 30%）。`echarts.min.js` 占前端 87.5%、占 UPX 后二进制约 300 KB。React + shadcn + Recharts 替换 ECharts 后，静态资源净变化 −100KB ~ +400KB（原始），UPX 后约 **−30KB ~ +120KB**，二进制维持 2.6 ~ 2.75 MB，分发无感。
2. **图表迁移零能力损失**。经核查，19 个图表全部为 `bar`/`line`/`pie` 及其组合（双 Y 轴柱+线、柱状条件着色）。唯一的 `visualMap` 仅用于给「工作时段柱状图」按小时区间上色（工作/非工作时间），并非热力图。无 heatmap / 关系图 / 桑基 / 3D 等高级图，**Recharts 全覆盖，无迁移难点**。

选择 C 档的核心原因：**UI 壳（shadcn）、图表层（Recharts 声明式）、数据层（TanStack Query）三大块都有成熟轮子**，迁移后净代码量约 **4200 → 1500–2000 行**，同时换来产品级设计一致性与可维护性。

### 1.4 为什么不是 A 档（零构建换皮）/ B 档（Alpine）

- A 档（重写 CSS token + 换 ECharts 主题 + 抽 HTML）：1–2 天、零风险、能消除约 90% 的「低级感」。**可作为 C 档落地前的过渡，或 C 档被否决时的兜底**。
- B 档（Alpine.js）：仍单二进制、无构建，但生态小、需自建组件，设计天花板低于 C。
- 本计划选择 C 的前提是：接受「单二进制仍保留，但开发期引入 Node 构建链」这一权衡，以换取组件化与设计上限。若评审结论是不接受构建链，则回退 A 档。

---

## 2. 目标与非目标

### 目标

- 视觉达到 **PostHog / claude.ai 级别**的克制与一致性，含暗色模式（见 §3）。
- 用组件树取代 `innerHTML` 拼接；图表配置从命令式 `setOption` 改为声明式 JSX。
- **功能 100% 对齐现状**：所有 API 端点不变，全部 19 个图表、诊断卡、下钻、时间轴、filter 联动行为不丢。
- 保持 `make build` 产出**单二进制**；前端构建产物经 Go `embed` 内联，用户无需 Node 即可运行。
- `make build` 文件大小不显著膨胀（控制在 +150KB / UPX 后以内）。

### 非目标

- 不引入后端路由框架或 SSR（SPA 单页 + tab 切换，本地工具不需要 React Router）。
- 不引入 Redux/Zustand 等全局状态库（filter 走 URL search params，局部状态用 `useState`）。
- 不做账号、多用户、鉴权、云端同步。
- 不改 `/api/*` 的请求/响应契约（仅前端消费层重写）。

---

## 3. 设计方向与视觉规范

> 本章是 Phase 3–5（图表迁移、交互、打磨）的设计依据，所有视觉决策以本章为准。

### 3.1 整体风格与核心参照

**定位**：claude.ai 的暖色气质（cream 米白 + 赤陶橙）× PostHog 式分析 dashboard 结构（图表分组 + 全局筛选 + 下钻）。属于 Linear / Vercel / PostHog 血统的「克制冷峻 + 高密度读数」工具风，**不是 awwwards 式炫技品牌站**。

awwwards 调研结论（dashboard / data visualization / Stats 三切入点）：其 dashboard 结果 90% 是 admin 模板厂 + 品牌营销落地页，**无密集型开发者分析 dashboard 标杆**，仅能提炼共性趋势（浅色克制、单色强调、大留白、数据优先）。真正的对标在业界产品：

| 参照 | 看什么 | 优先级 |
|---|---|---|
| **PostHog**（posthog.com / app.posthog.com） | **结构骨架**：顶栏全局筛选 + 左导航 + 分组图表卡网格 + 点击下钻，与 cc-insights 同构；且为暖橙调，方向一致 | ★ 核心 |
| **claude.ai** | **配色气质**：cream 米白背景 + 赤陶橙强调 + 暖近黑文字（品牌一致性，cc-insights 诊断的就是 Claude Code） | ★ 核心 |
| **Tremor**（tremor.so） | **组件范式**：KPI 卡（数字 + delta + sparkline）、图表卡、布局网格，React 现成 | ★ 核心 |
| **Grafana**（play.grafana.com，免登录） | **结构骨架补充**：多面板网格 + 时间筛选 + 变量下钻（仅参考结构，其暗色科技风配色**不参考**） | 补充 |

### 3.2 配色系统（暖色，claude.ai 风，默认亮色）

所有颜色定义为 CSS 变量 / Tailwind token，亮暗双套。

**亮色（默认）**

| Token | 色值 | 用途 |
|---|---|---|
| `--canvas` | `#FAF9F5` | 页面背景（暖米白） |
| `--surface` | `#FFFFFF` | 卡片 |
| `--surface-subtle` | `#F5F3EE` | 次级面板 / 表头 / hover 底 |
| `--border` | `#E8E4DA` | 发丝级分隔（比现状 `#dbe3ec` 更暖更细） |
| `--ink` | `#1F1E1D` | 主文字（暖近黑） |
| `--muted` | `#6B6862` | 次文字 |
| `--subtle` | `#9C9890` | 弱文字 / 占位 |
| `--accent` | `#C15F3C` | **赤陶橙**（Claude 标志色），主强调 |
| `--accent-soft` | `#F3E4DC` | 强调色浅底（badge / hover） |
| `--success` | `#4A7C59` | 暖绿（健康 / 通过） |
| `--warning` | `#B8792B` | 暖琥珀 |
| `--danger` | `#B23A3A` | 暖红（失败 / 高风险） |

**暗色（备选，开发者偏好，跟随 `prefers-color-scheme`）**

| Token | 色值 |
|---|---|
| `--canvas` | `#1A1917` |
| `--surface` | `#26241F` |
| `--surface-subtle` | `#2F2D27` |
| `--border` | `#3A3833` |
| `--ink` | `#F5F3EE` |
| `--muted` | `#A8A39A` |
| `--accent` | `#E08560`（提亮的赤陶） |

**图表分类色序列**（暖冷混排，与 accent 协调，Recharts 共享）：
`#C15F3C`（赤陶）· `#2E7D8F`（青）· `#6B8E5A`（橄榄）· `#B8792B`（琥珀）· `#8B5A8C`（紫）· `#4A7C59`（绿）

### 3.3 字体系统

| 用途 | 字体 | 说明 |
|---|---|---|
| UI / 正文 | **Inter** | 开发者工具标配，可读性最佳 |
| 数字 / 命令 / 模型名 / 路径 | **JetBrains Mono**（`tabular-nums`） | cc-insights 全是命令、模型 ID、文件路径，等宽对齐是刚需，顺带解决现状 `shortModelName`/`shortPath` 靠 `...` 截断的狼狈 |

字号阶梯（收敛现状）：`12 / 13 / 14 / 17 / 24 / 32`。所有数字加 `font-variant-numeric: tabular-nums`。

### 3.4 布局架构（改动最大处）

**现状问题**：固定 240px 深色 sidebar 吃横向空间；巨型 `dashboard-header`（status strip + control panel + filter row + section-nav）占满首屏，图表被挤到折叠线以下。

**新架构**：

```text
┌─────────────────────────────────────────────────────┐
│ ┌─┐  cc-insights      [24h 7d 30d 90d All]  ⏱时间轴  │ ← 顶栏: logo + preset + 时间轴 (sticky)
│ │█│ ─────────────────────────────────────────────── │
│ │█│ [项目▾ 工具▾ 模型▾ 原因▾]  最后更新 12:30 · 1.2s │ ← 第二行: filters + status
│ └─┘                                                  │
│   ┌──KPI──┐┌──KPI──┐┌──KPI──┐┌──KPI──┐   概览        │ ← KPI 卡行
│   └────────┘└────────┘└────────┘└────────┘            │
│   ┌─ 概览 使用 质量 成本 运行 ──────────────────┐     │ ← 分组 tab (取代锚点跳转)
│   │  [图表卡] [图表卡]                          │     │
│   │  [图表卡 wide]                              │     │ ← 图表网格
│   └────────────────────────────────────────────┘     │
│   诊断建议  →  下钻详情                               │
└─────────────────────────────────────────────────────┘
```

- **左侧 56px 图标导航 rail**（可折叠成纯图标），取代 240px 深色 sidebar —— 释放内容宽度。
- **顶部 sticky 控制条**：preset + 时间轴 + filters 收进两行，不再独占一个大面板。
- **分组用 tab 切换**（概览/使用/质量/成本/运行）取代现状的 section-nav 锚点跳转，减少首屏压力。

### 3.5 KPI 卡规范（现状最大短板）

现状 KPI 只有「数字 + 标签」。升级为业界标配（参照 Tremor）：

```text
┌─────────────────────────────┐
│ 总消息数            ↑ 12.4%  │  ← 标签(muted/uppercase) + delta(绿/红 + 箭头 + %，vs 上期)
│ 128,493                     │  ← 大号粗体 tabular-nums
│ ╱╲╱╲___╱╲╱╲╱  sparkline     │  ← Recharts mini line（无轴无网格）
└─────────────────────────────┘
```

- delta 来自与上一同等窗口对比（7d vs 上 7d）。
- sparkline 复用该指标的时序数据。
- 失败率类指标：delta 红色 + sparkline 红色调。

### 3.6 图表卡与图表样式规范

所有 19 个图表套同一外壳（`<ChartCard>` 组件）：

```text
┌─ 每日活动趋势        [7d] ●完整覆盖 ──┐
│ 按日期汇总消息量 · 来源 stats-cache    │  ← title + 时间窗 + coverage badge + 一句话说明
│                                        │
│        📈 统一风格图表区                │  ← 克制: 1px 浅网格线、无图框、hover 统一 tooltip
│                                        │
│ 💡 共 128k 条，峰值在 6/12 达 9.2k     │  ← 现状"数据洞察"保留，样式统一为左侧 accent 竖条
└────────────────────────────────────────┘
```

- **抛弃 ECharts `wonderland` 主题 + 内置 title**（现状双重渲染），图表标题完全由卡片 header 承担，图表区只画数据。
- 统一：网格线用 `--border`、轴标签 `--muted`、tooltip 暖色卡片、空态/healthy 态（绿色 insight 条）语义对齐现状。

### 3.7 暗色模式与动效

- **暗色**：`next-themes` + CSS 变量，整套 token 自动切换；图表色通过变量驱动（Recharts 接收变量值），不写死两套 option；默认跟随系统，可手动切。
- **动效（克制，服务于状态）**：filter 切换 → 图表淡入 120ms；loading → skeleton shimmer（取代旋转 spinner，可保留趣味文案）；全程 `prefers-reduced-motion` 兜底（现状已有，保留）。

### 3.8 反低级清单（现状问题 → 设计决策）

| 现状问题 | 决策 |
|---|---|
| ECharts `wonderland` 土主题 + 内置 title 双重渲染 | 自定义主题对齐 token，title 归卡片 header |
| KPI 只有裸数字 | + delta + sparkline（§3.5） |
| 数字不对齐（`shortModelName` 靠 `...` 截断） | JetBrains Mono + `tabular-nums`（§3.3） |
| 配色 `#2f80d1` 冷蓝无辨识度 | Claude 暖色 token + 赤陶强调（§3.2） |
| 240px 深色 sidebar 吃宽度 | 56px 图标 rail（§3.4） |
| 首屏被巨型 control panel 占满 | sticky 两行紧凑控制条（§3.4） |
| 卡片同款无层次 | surface / subtle 分层 + 间距节奏 |
| 无暗色模式 | next-themes + 变量化图表色（§3.7） |
| 图表配置 4 文件各写各的 | 统一 `<ChartCard>` + 共享样式（§3.6） |

---

## 4. 技术栈选型

| 层 | 选型 | 作用 | 替代方案与取舍 |
|---|---|---|---|
| 构建 | **Vite** | 开发热更新 + 生产打包 | 构建快、生态默认；CRA 已过时 |
| 框架 | **React 18 + TypeScript** | 组件化 + 类型安全 | Vue 3 + naive-ui 亦可，选 React 因 shadcn/TanStack 生态最成熟 |
| 样式 | **Tailwind CSS** | 原子化 + 设计 token（承载 §3.2 配色） | 与 shadcn 强绑定 |
| 组件库 | **shadcn/ui** | 卡片/按钮/Badge/Tabs/Table/Select/Dialog 等复制即用 | 非 npm 依赖，代码进仓库可改；设计上限高 |
| 图表 | **Recharts** | 声明式 bar/line/pie/双轴组合 | 现状无高级图，Recharts 够用；nivo/visx 为备选 |
| 数据层 | **TanStack Query** | fetch/loading/error/缓存/重取/abort | 吃掉 `app_core.js` 大半粘合代码，关键选型 |
| 主题 | **next-themes** | 暗色模式切换（§3.7） | 与 shadcn 配套 |
| 状态 | URL search params + `useState` | filter 同步、可分享、可刷新 | 不引入全局 store |
| 字体 | **Inter + JetBrains Mono** | UI 正文 + 等宽数字（§3.3） | 开发者工具金标准组合 |

---

## 5. 目标目录结构

```text
cc-insights/
├── cmd/insights/
│   ├── main.go                 # embed 指向 static/dist，新增 SPA fallback
│   └── static/
│       └── dist/               # ← Vite 构建产物（embed 目标，gitignore 源产物）
│           ├── index.html
│           └── assets/
├── web/                        # ← 新增：前端源码（构建链所在）
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── tailwind.config.ts      # 承载 §3.2 配色 token + §3.3 字体
│   ├── components.json         # shadcn 配置
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── api/                # 类型 + Query hooks（数据层地基）
│   │   │   ├── types.ts
│   │   │   └── hooks.ts
│   │   ├── components/
│   │   │   ├── ui/             # shadcn 生成
│   │   │   ├── layout/         # SidebarRail / TopFilterBar / GroupTabs
│   │   │   ├── charts/         # ChartCard 外壳 + 每个图表一个组件（19 个）
│   │   │   └── dashboard/      # KpiCard / DiagnosticCard / DrilldownPanel
│   │   ├── lib/                # queryClient、formatters（迁移 app_formatters.js）
│   │   └── styles/globals.css  # CSS 变量（亮/暗 token）
│   └── README.md               # 前端开发与构建说明
└── Makefile                    # build/run/release 增加 web 构建步骤
```

> 约束：`cmd/insights/static/dist/` 是构建产物，**不进版本库**（加入 `.gitignore`），`make build` 负责生成；`web/` 源码进版本库。

---

## 6. 分阶段路线（最小闭环优先，风险前置）

### Phase 0｜脚手架与 embed 集成（1 天）

- 初始化 `web/`：Vite + React + TS + Tailwind + shadcn CLI。
- `tailwind.config.ts` 落入 §3.2 配色 token；`globals.css` 定义亮/暗 CSS 变量。
- `vite.config.ts`：`build.outDir = '../cmd/insights/static/dist'`，`base = '/static/'`。
- 改 `main.go`：`//go:embed static/dist`，`/dashboard` 返回 `dist/index.html` 并对未知路径做 SPA fallback。
- dev 模式：`vite dev` 代理 `/api`、`/static` 到 Go 服务。
- **验证**：`vite build` + `make build` 后，`./cc-insights web` 能打开空白 React 页。

### Phase 1｜最小数据闭环（2 天）—— 风险闸口

- 建立 `api/types.ts`（先覆盖 `overview` 与 `daily_trend`）+ `useOverview` / `useData` hook。
- 渲染：1 个 KPI 区（`KpiCard`，§3.5）+ 1 个 `DailyTrendChart`（`ChartCard` + Recharts，§3.6）。
- 接入 preset：URL search params 驱动 Query 重取。
- **验证**：preset 切换 → `/api/data` 重取 → 图表更新；loading / error 态正确。
- **决策点**：跑通即证明「Vite→embed→Go API→React」全链路可行，方可进入全量迁移。否则回退 A 档。

### Phase 2｜数据层地基（2 天）

- 为全部 `/api/*` 端点定义 TS 类型（见 §8.1）。
- 实现所有 Query hooks，设计 Query key 规范（`['overview', filters]` 等），确保 filter 变化精确失效。
- 抽离 formatters（迁移 `app_formatters.js`）。

### Phase 3｜图表批量迁移（3–4 天）

- 按 group 顺序迁移 19 个图表（见 §8.2）：usage → quality → cost → runtime。
- 每图一个 `components/charts/<Name>Chart.tsx`，统一套 `<ChartCard>`（§3.6）。
- 复刻现状的「空态 / healthy 态 / 数据洞察文案」语义。

### Phase 4｜交互层（2–3 天）

- 布局落地：SidebarRail + TopFilterBar + GroupTabs（§3.4）。
- Filters：preset / 自定义日期 / timeline slider + windowSize / 四个过滤项，全部双向同步到 URL。
- 诊断卡点击 → 同步下钻条件；下钻面板 5 个（failures/commands/tokens/sessions/tools）。

### Phase 5｜打磨（2 天）

- 暗色模式（next-themes + §3.2 暗色 token + 变量化图表色，§3.7）。
- loading 阶段提示、empty / error 统一态；delta + sparkline 在所有 KPI 卡铺开。
- 响应式、`prefers-reduced-motion`、可访问性。

### Phase 6｜构建链与发布适配（1 天）

- `Makefile`：`build` 前置 `web` 构建步骤；`release` 多平台流程同步。
- `.gitignore`：忽略 `static/dist/`、`web/node_modules/`。
- README 补充前端开发说明。

> 合计约 **13–15 个工作日**。

---

## 7. 关键技术决策

### 7.1 embed 与 SPA 路由

- `//go:embed static/dist` 嵌入整个产物目录。
- `/dashboard` 返回 `dist/index.html`；对 `/dashboard/*` 未命中静态资源的路径回退到 `index.html`（SPA history fallback）。
- 静态资源走 `/static/assets/*`（`base = '/static/'`），保留现有 `http.FileServer` 逻辑微调。

### 7.2 dev 双进程

- Go：`./cc-insights web` 提供 `/api/*`。
- Vite：`vite dev`（5173），`server.proxy` 把 `/api` 与 `/static` 转发到 Go（默认 `:8080`）。
- 开发时访问 Vite 端口享受热更新；生产访问 Go 端口用 embed 产物。

### 7.3 状态管理：URL search params 优先

- 所有 filter（`preset`/`start`/`end`/`project`/`tool`/`model`/`reason`）序列化进 URL，`useSearchParams` 驱动。
- 收益：可刷新、可分享链接、TanStack Query key 天然稳定。

### 7.4 数据层：TanStack Query key 规范

```ts
// 命名空间 + filter 快照，保证筛选变化精确失效
['overview', filters]
['data',     filters]
['diagnostics', filters]
['detail', 'failures', filters]
['timeline', {}]
```

- `staleTime` 设为较短（数据每次 filter 变化都来自 `/api/reload` 刷新后的缓存），配合现有 `/api/reload`。

### 7.5 图表条件着色（复刻 visualMap）

- 工作时段柱状图按小时区间上色：Recharts `<Bar>` 内用 `<Cell fill={isWorkHour ? ... : ...} />` 即可，无需任何热力图能力。

### 7.6 设计令牌单源

- 配色（§3.2）、字体（§3.3）只在 `globals.css`（CSS 变量）与 `tailwind.config.ts` 定义一处；Recharts 组件接收 CSS 变量值，保证亮暗切换与图表统一。

---

## 8. 迁移清单

### 8.1 API 端点 → TS 类型

| 端点 | 用途 | 备注 |
|---|---|---|
| `/api/data` | 主数据：daily_trend、commands、project_stats、model_usage、weekday_stats、work_hours_stats、tool_analysis、failure_analysis | 类型最大，拆分子类型 |
| `/api/overview` | KPI 概览卡片 |  |
| `/api/diagnostics` | 诊断建议卡片 | 含 severity、证据、触发条件、下钻条件 |
| `/api/detail/failures` | 失败下钻 | filters：reason/tool/model |
| `/api/detail/commands` | 命令族下钻 |  |
| `/api/detail/tokens` | Token 下钻 |  |
| `/api/detail/sessions` | Session 下钻 |  |
| `/api/detail/tools` | 工具下钻 |  |
| `/api/timeline` | 时间轴索引 |  |
| `/api/reload` | 刷新缓存 | 不取数，触发后端重算 |

### 8.2 图表 → Recharts 组件映射（19 个，按 group）

| Group | 图表（init 函数） | Recharts | 备注 |
|---|---|---|---|
| usage | DailyTrend | `LineChart`（smooth area） |  |
| usage | Commands | `BarChart`（横排 Top15） |  |
| usage | Project | `BarChart` |  |
| usage | Weekday | `BarChart` |  |
| usage | Model | `BarChart` + `PieChart` |  |
| usage | WorkHours | `BarChart` + `Cell` 条件着色 | 复刻 visualMap（§7.5） |
| quality | ToolModelFailure | `ComposedChart`（双 Y 轴 bar+line） |  |
| quality | FailureReason | `BarChart` |  |
| quality | ToolPerformance | `BarChart`/`LineChart` |  |
| quality | FileAnalysis | `BarChart` |  |
| quality | CommandFile | `BarChart` |  |
| cost | CostAnalysis | `ComposedChart`/`BarChart` |  |
| runtime | SessionAnalysis | `BarChart` |  |
| runtime | Session | `BarChart`/`LineChart` |  |
| runtime | Agent | `BarChart` |  |
| runtime | TaskPlan | `BarChart` |  |
| runtime | EventHook | `BarChart` |  |
| runtime | SkillAnalysis | `BarChart` |  |
| runtime | RuntimeTools | `BarChart` | debug 日志补充口径 |

---

## 9. 代码量与工时

### 9.1 文件级代码量预估

| 文件 / 模块 | 预估行数 | 工作量性质 |
|---|---|---|
| 配置层（vite / tsconfig / tailwind token / postcss / package / index.html） | ~225 | 脚手架·一次性 |
| 入口 + 全局（main / App + globals.css 亮暗 token） | ~215 | 手写 |
| `api/types.ts`（10 端点，主 data 接口最重） | ~500 | 手写·机械 |
| `api/hooks.ts` + `lib/formatters.ts` | ~270 | 手写 |
| shadcn ui（~13 个组件复制即用） | ~600 | **`add` 秒生成·免设计** |
| layout（SidebarRail / TopFilterBar / GroupTabs） | ~270 | 手写（TopFilterBar 最重） |
| dashboard 业务（KpiCard/Grid、诊断、下钻 5 表、useUrlFilters） | ~520 | 手写 |
| charts（ChartCard 外壳 + 共享 primitives + 19 图） | ~900 | 手写·**主体工作量** |
| `web/README.md` | ~60 | 文档 |
| **合计** | **≈ 3500 行**（区间 3000–4000） | |

### 9.2 按工作量性质归类（行数 ≠ 工时）

| 性质 | 行数 | 占比 | 说明 |
|---|---|---|---|
| 手写业务逻辑（动脑） | ~2500 | 70% | 含图表 ~900、TS 类型 ~500 |
| shadcn 复制粘贴（免设计） | ~600 | 17% | `npx shadcn add` 秒生成 |
| 配置 / 脚手架 | ~225 | 6% | 一次性 |
| CSS 变量 / 全局样式 | ~150 | 4% | token 落地 |

### 9.3 与现状对比

| 维度 | 现状 | 新方案 | 变化 |
|---|---|---|---|
| 进仓库总行数（JS+CSS） | ~4200 | ~3500 | ↓ 17% |
| **真正手写的业务逻辑** | ~4000 | ~2500 | **↓ 37%** |
| 图表代码 | `charts_*.js` ~2000（命令式） | ~900（声明式 JSX） | **↓ 55%** |
| 数据/状态粘合 | `app_core.js` ~1100（手写 fetch/loading/abort） | hooks + filters ~220（TanStack Query） | **↓ 80%** |
| CSS | 980 行手写 | ~150（Tailwind + 变量） | **↓ 85%** |

> 注：原 v0.1「4200 → 1500–2000」为「纯逻辑」口径（剔除 TS 类型 / shadcn 复制 / 配置），偏乐观；本节为含全部进仓库代码的完整口径。

### 9.4 工时驱动因素（不与行数线性相关）

真正花时间的是：① **19 个图表逐一迁移**（~900 行，~3–4 天）；② **TopFilterBar 交互**（preset / 日期 / timeline / 4 过滤 + URL 同步，~2 天）；③ **`types.ts`** 机械活（~1.5 天）；④ **视觉打磨**（暗色 / 间距 / sparkline / 对照参照，~2 天）；⑤ 脚手架 + embed 联通（~1 天）。

工时：约 **13–15 个工作日**（Phase 0–6）。

---

## 10. 风险与回滚

| 风险 | 影响 | 缓解 |
|---|---|---|
| embed 路径 / SPA fallback 配错 | 页面白屏、资源 404 | Phase 1 作为闸口，跑通才继续 |
| 图表视觉/交互与现状有差异 | 体验回退 | 每图对照现状截图验收（§11） |
| 视觉与参照偏差（PostHog/claude.ai） | 设计走样 | §3 规范为准绳；Phase 5 逐屏对照参照截图 |
| 构建链污染 `make build` | 单二进制流程断裂 | Phase 6 统一适配；`dist/` 不入库 |
| 前端依赖长期维护税 | 升级/安全补丁 | 锁定版本；依赖最小化 |
| 工期超预期 | 阻塞功能迭代 | 阶段独立可交付；Phase 1 后可随时叫停回退 A 档 |

**回滚策略**：全程在独立分支进行；旧 `static/*.js` 与 `dashboardPageHandler` 模板保留至 Phase 6 验收通过再删除。任意阶段可 `git checkout` 回退，Go 后端与 API 契约零改动，回滚无副作用。

---

## 11. 验收标准

- [ ] `make build` 仍是单二进制，体积 ≤ 2.75 MB（UPX 后）。
- [ ] `./cc-insights web` 打开 Dashboard，19 个图表、KPI、诊断卡、5 个下钻面板、时间轴全部渲染且数据正确。
- [ ] preset / 自定义日期 / timeline / 四个过滤项联动行为与现状一致；URL 可刷新复现。
- [ ] **视觉**：配色/字体/布局/卡片符合 §3 规范；KPI 卡含 delta + sparkline；暗色模式可用。
- [ ] **参照一致性**：整体观感对标 PostHog 结构 + claude.ai 暖色（逐屏对照）。
- [ ] loading / empty / error 态完整；`prefers-reduced-motion` 生效。
- [ ] 逐图对照现状截图，信息与洞察文案语义一致。
- [ ] `go test ./...` 与 `make test` 通过（前端迁移不动后端测试，作为回归基线）。

---

## 12. 对 `make build` / `release` 的影响

- `build` 目标前置 web 构建：

  ```makefile
  web-build:
      @cd web && npm ci && npm run build   # 产物落到 cmd/insights/static/dist
  build: web-build
      @CGO_ENABLED=0 go build -trimpath -tags=prod $(LDFLAGS) -o $(BINARY) ./cmd/insights
      @# UPX 压缩 …（不变）
  ```

- `release` 多平台流程：在打包前统一执行一次 `web-build`（前端产物跨平台一致，无需每平台重复）。
- 开发者文档：补充「改前端需 Node + `cd web && npm i`」，构建/发布仍只需 Node 在 CI/打包机上。

---

## 附：决策记录（ADR 摘要）

- **选 C 而非 A/B**：图表迁移零损失 + 三大层均有成熟轮子，代码量↓一半，换设计上限与可维护性；代价是引入 Node 构建链，权衡可接受。
- **设计方向：PostHog 结构 + claude.ai 暖色 + Tremor 组件**：PostHog 的「分组图表 + 全局筛选 + 下钻」结构与 cc-insights 同构且同为暖橙调；claude.ai 暖色提供品牌一致性与辨识度（区别于通用冷蓝 dashboard）；Tremor 提供可直接借鉴的 KPI/图表卡范式。放弃 awwwards 式炫技风与现状的冷蓝 admin 风。
- **配色单源**：CSS 变量 + Tailwind token 一处定义，Recharts 接收变量值，保证亮暗切换与图表统一（§7.6）。
- **保留 ECharts 的混合栈被否决**：现状未用 ECharts 高级能力，保留它纯属 YAGNI 包袱，徒增 1 MB 体积与双图表库维护成本。
- **不引入 React Router**：单页 tab 切换即满足本地工具需求，避免 SPA 路由过度工程。
- **若评审否决构建链**：回退 A 档（抽 HTML 出 Go + 现代主题 + 重写 token + KPI sparkline），1–2 天可消除约 90% 低级感；本文档 §3 设计规范（配色/字体/布局/卡片）仍可在 A 档下直接套用。
