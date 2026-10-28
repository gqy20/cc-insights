# 容错与降级策略

cc-insights 读取的是本机 code agent 数据目录，不是受控数据库。默认 provider 是 Claude Code 的 `~/.claude`，也可读取 Codex 的 `~/.codex`。目录可能缺失，JSONL 可能被截断，SQLite runtime 日志可能正在写入，缓存可能过期，工具调用也可能没有对应结果。容错目标不是掩盖问题，而是在可用数据范围内继续产出诊断，并把缺失、样本不足和无法精确计算的部分显式暴露出来。

## 基本原则

- 单个数据源失败不阻断整体报告。
- 单条坏记录跳过，不中断整个文件。
- 缺失工具结果计入 `missing result`，它是质量信号，不是程序异常。
- 缓存只是加速层，不能成为唯一数据来源。
- 前端不能用全局数据冒充筛选后的结果；无法精确计算时必须标记空态或样本态。

## 缓存策略

主要入口：

- `refreshGlobalCache`
- `CacheBuilder.NeedsRebuild`
- `CacheBuilder.BuildFullCache`
- `CacheBuilder.RebuildIfChanged`
- `CacheFile.QueryByTimeRange`

当前缓存模型是文件级快照加时间范围查询，不宣称真正的增量解析。

- `NeedsRebuild` 负责判断缓存是否需要重建：缓存不存在、损坏、版本不匹配、provider 不一致、Bash 规则 hash 变化或数据文件更新时间超过缓存时间时返回 true。
- `BuildFullCache` 会重建完整缓存，同时复用未变化的 `ProjectFileCache`，避免重复解析未变化的 provider log file。
- `RebuildIfChanged` 只做变化检测后重建，不做局部增量合并。
- `QueryByTimeRange` 基于日级、项目级、session 级 runtime 索引重建指定时间范围内的聚合结果。
- `rec` 诊断优先尝试轻量 `diagnostics.db`，不可用时再回退到完整 `cache.db` 或触发重建。

缓存失败的影响是性能下降，不应该改变诊断语义。只有缓存构建本身失败时，Web 启动或 API 刷新才应返回明确错误。Web 按 provider 维护独立 cache loader 和 singleflight，避免 Claude/Codex 切换时互相等待或串用缓存。

## 实时解析降级

缓存不可用时，`buildDataFromParsing` 会按 provider 并行解析可用来源。Claude Code 主要来源：

- `history.jsonl`
- `projects/**/*.jsonl`
- `debug/`
- `tasks/`

Codex 主要来源：

- `sessions/**/*.jsonl`
- `logs_2.sqlite`

每个入口都有 safe wrapper：

- `safeParseHistoryConcurrent`
- `safeParseProjectsOnce`
- `safeParseDebugLogs`
- `safeParseTasksOnce`
- `safeParseSessionStats`

这些 wrapper 的契约是：解析失败时记录 warn，返回非 nil 的空结果，不把错误向上传播到整个 Dashboard。这样局部数据缺失时，CLI 和 Web 仍能展示其他来源的结果。

## 数据源契约

### history

`history.jsonl` 主要提供 slash command、命令历史和部分小时分布。缺失或损坏时，相关命令统计为空，不影响 projects 主分析。

### projects / sessions

Claude 的 `projects/**/*.jsonl` 和 Codex 的 `sessions/**/*.jsonl` 是各自 provider 的核心数据源，负责消息、模型、Token、工具调用、失败原因、Session 生命周期、文件编辑和性能统计。目录缺失或解析失败时返回空 `ProjectAggregate`，对应分析模块为空。

空 `ProjectAggregate` 必须通过 `newProjectAggregate()` 初始化并 finalize，不能手写一份字段 map 清单。这样新增聚合字段后，正常解析路径和降级路径会同时具备相同结构，避免 fallback 只在少数字段上可用。

### debug

`debug/` 是 runtime 事件日志来源，用于补充 MCP runtime 工具信号、hook、skill、budget、opened file 等事件。缺失时这些运行时信号为空，但不影响 projects 中解析出的工具调用和成本统计。

### Codex SQLite runtime

`logs_2.sqlite` 是 Codex runtime trace 来源，当前用于 hook trace 和 app-server hook 事件。文件缺失、被锁或查询失败时只记录 warn，不影响 session JSONL 中的核心工具、Token、失败和 Plan 分析。Codex hook trace 会做 turn/session 去重，避免 register/deregister poller 日志重复计数。

`state_5.sqlite` 目前只用于能力核对，不作为主分析来源。只有当 agent jobs、thread spawn 等表中存在稳定数据并能映射到现有语义时，才应进入 Agent 分析。

### tasks

`tasks/` 提供 Task 状态和提醒相关信号。目录缺失时返回空 Task 分析。单个 task JSON 损坏时跳过该文件。

### Qoder 状态库

AppData 下的 `main.sqlite` 是只读增强层，补 turn 级 `failed`/`interrupted` 结局、整轮耗时和上下文构成占比。文件缺失、被锁、表结构不匹配或路径探测不到（Linux 路径尚未核实）时整层静默跳过，transcript 已给出的会话、工具、失败与 credits 统计不受影响。上下文构成按天快照写入，所以时间筛选后仍然可用。

### Qoder 运行日志（模型真名）

transcript 里的 `message.model` 是内部别名（`qfmodel`、`smodel` 等），真实模型名只在 `logs/runs/*/qodercli.log` 的 `model_config` 与 `[Routing]` 行里。这一层是纯展示增强：

- 只在 provider 解析为 `qoder` 或 `all` 时扫描，其他 provider 完全不付这个成本。
- 按 run 目录从新到旧累计，超出扫描预算的文件跳过；日志缺失、超长或排版变化导致查不到时，模型只显示别名。
- 真名不进入统计 key，也不写进缓存。日志随时会被清掉，若以真名聚合，同一模型会在缓存里裂成两个桶。
- 报表标签保留别名（`Qwen3.8-Flash (qfmodel)`），因为 `--model` 过滤和聚合口径用的都是别名。

### Qoder 估算输入 token

官方不下发 token（transcript/日志全 0），但 `usage.context_usage_ratio` 每条都有。估算链路：

- 解析层按模型累加 `context_ratio_sum`（原料），渲染层乘以窗口得 `est_input_tokens`（`~` 前缀展示）。窗口变化或日志缺失只会让估算退化为不展示，不会污染缓存统计。
- 窗口来源优先日志 `model_config.max_input_tokens`（请求时实际配置，qfmodel=180k、dfmodel/ultimate=1M），日志没有该别名时用状态库 `chat_model_preferences.context_window` 兜底（实测清一色 400k，更像 UI 默认）。
- 口径是"输入侧上下文吞吐"：含每轮重复计入的缓存上下文，与 Claude 的 input+cache_read 量级可比；输出侧无任何锚点，不做估算。

## 记录级容错

解析 JSONL 时应遵循：

- 坏 JSON 行跳过并继续。
- 缺失 `timestamp` 的运行时事件不应误判为核心消息质量问题。
- 缺失 `cwd` 的非 assistant/user 事件应尽量保留可用信号。
- orphan `tool_result` 和 pending `tool_use` 应进入质量统计。
- 失败样例只保留短 preview，避免输出过大或泄露过多原始内容。

这些信号会影响：

- `ToolAnalysis.MissingResults`
- `FailureAnalysis.Samples`
- `SessionAnalysis`
- `ToolPerformance`
- `rec` 的 failure / workflow / performance 诊断

## Coverage 语义

交互式 Web 使用 `coverage` 标记每个图在当前筛选条件下的可信度：

- `exact`：可由缓存索引或聚合字段精确重建。
- `sample`：只能基于样例过滤，不能代表完整总体。
- `unavailable`：当前筛选缺少交叉索引，不展示冒充结果。

前端遇到 `unavailable` 必须显示空态原因；遇到 `sample` 必须标记“样本”。新增图表或筛选条件时，应先定义 coverage 规则，再实现展示。

## 前端行为

Web Dashboard 必须对空数组、空对象和缺失字段稳定：

- 图表无数据时显示空态。
- 本地静态资源必须可用，局域网访问不能依赖 CDN。
- 一个接口失败不应导致已成功返回的图表状态被误渲染。
- loading 和错误文案要说明是数据不可用、筛选不可计算，还是接口错误。
- 前端必须区分“无数据”和“该 provider 没有可靠等价信号”。例如 Codex 当前没有稳定 skill invocation 和非 main agent 数据时，应隐藏对应卡片，而不是显示误导性的空图。

## 维护要求

- 不为旧接口保留无意义兼容层；当前主线接口是 `/api/data` 和交互式下钻 API。
- fallback 不能吞掉所有信息；能形成诊断信号的异常应进入聚合。
- 不要把真实机器的一次性统计数字写成长期事实。
- 新增数据源时必须明确：缺失时返回什么、坏记录如何处理、是否进入 coverage、是否影响 `rec`。
- 如果某个函数只是变化检测后完整重建，命名中不要使用“增量”。
- provider 日志文件发现应复用 `listLogFileInfos`。需要路径列表时从 `AbsPath` 派生，不要新增第二套 `WalkDir`。
- cache 和实时解析最终都应走共享的 `DashboardData` 组装 helper；新增对外字段时要同时检查 cache、parsing、filter 和 coverage。
