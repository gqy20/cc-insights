# 容错与降级策略

cc-insights 读取的是本机 `~/.claude` 数据，不是受控数据库。目录可能缺失，JSONL 可能被截断，缓存可能过期，工具调用也可能没有对应结果。容错目标不是掩盖问题，而是在可用数据范围内继续产出诊断，并把缺失、样本不足和无法精确计算的部分显式暴露出来。

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

- `NeedsRebuild` 负责判断缓存是否需要重建：缓存不存在、损坏、版本不匹配、Bash 规则 hash 变化或数据文件更新时间超过缓存时间时返回 true。
- `BuildFullCache` 会重建完整缓存，同时复用未变化的 `ProjectFileCache`，避免重复解析未变化的 project JSONL。
- `RebuildIfChanged` 只做变化检测后重建，不做局部增量合并。
- `QueryByTimeRange` 基于日级、项目级、session 级 runtime 索引重建指定时间范围内的聚合结果。
- `rec` 诊断优先尝试轻量 `diagnostics.db`，不可用时再回退到完整 `cache.db` 或触发重建。

缓存失败的影响是性能下降，不应该改变诊断语义。只有缓存构建本身失败时，Web 启动或 API 刷新才应返回明确错误。

## 实时解析降级

缓存不可用时，`buildDataFromParsing` 会并行解析四类来源：

- `history.jsonl`
- `projects/**/*.jsonl`
- `debug/`
- `tasks/`

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

### projects

`projects/**/*.jsonl` 是核心数据源，负责消息、模型、Token、工具调用、失败原因、Session 生命周期、文件编辑、性能、Skill 和 Agent 统计。目录缺失或解析失败时返回空 `ProjectAggregate`，对应分析模块为空。

### debug

`debug/` 是 runtime 事件日志来源，用于补充 MCP runtime 工具信号、hook、skill、budget、opened file 等事件。缺失时这些运行时信号为空，但不影响 projects 中解析出的工具调用和成本统计。

### tasks

`tasks/` 提供 Task 状态和提醒相关信号。目录缺失时返回空 Task 分析。单个 task JSON 损坏时跳过该文件。

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
- 本地 ECharts 资源必须可用，局域网访问不能依赖 CDN。
- 一个接口失败不应导致已成功返回的图表状态被误渲染。
- loading 和错误文案要说明是数据不可用、筛选不可计算，还是接口错误。

## 维护要求

- 不为旧接口保留无意义兼容层；当前主线接口是 `/api/data` 和交互式下钻 API。
- fallback 不能吞掉所有信息；能形成诊断信号的异常应进入聚合。
- 不要把真实机器的一次性统计数字写成长期事实。
- 新增数据源时必须明确：缺失时返回什么、坏记录如何处理、是否进入 coverage、是否影响 `rec`。
- 如果某个函数只是变化检测后完整重建，命名中不要使用“增量”。
