# 容错与降级策略

cc-insights 运行在本地数据目录上，必须容忍数据不完整、缓存过期、单条 JSONL 损坏、工具结果缺失和部分目录不存在。容错目标是：尽量返回可用的聚合结果，同时把缺失和异常作为质量信号暴露给诊断层。

## 总体策略

- 优先使用缓存，缓存不可用时实时解析。
- 单个数据源失败不阻断整体报告。
- 单条坏记录跳过，不中断整个文件。
- 缺失工具结果不视为程序错误，而是记录为 missing result。
- 前端和 CLI 对空数据展示空态或摘要，不 panic。

## 缓存降级

主要入口：

- `initializeCache`
- `buildDashboardData`
- `CacheBuilder.NeedsRebuild`
- `CacheBuilder.BuildFullCache`

行为：

- 缓存不存在、版本不匹配或 JSON 损坏时重建。
- 缓存读取失败时降级到实时解析。
- 文件级缓存命中时复用 `ProjectFileCache`。
- 文件大小或修改时间变化时重解析该文件。
- 诊断命令优先使用轻量 `diagnostics.db`，不可用时回退到完整 `cache.db` 或重建。

影响：

- 数据仍可返回，但首次响应可能变慢。
- 构建成本高时会触发 `performance.cache.build_cost` 诊断。

## 数据源降级

### history

`history.jsonl` 不存在或解析失败时，slash command 和部分小时统计为空；不影响 projects 主分析。

### projects

`projects/**/*.jsonl` 是核心数据源。目录缺失或解析入口失败时返回空聚合，项目、模型、失败、成本、Session、性能等分析为空。

### tasks

`tasks/` 不存在时返回空 Task 分析。单个 task JSON 损坏时跳过该文件，不影响 Plan 事件和其他聚合。

### debug

`debug/` 是旧版 MCP 统计兼容路径。当前主分析已经主要来自 `projects`，debug 缺失时只影响 legacy MCP debug 统计。

## 记录级容错

解析 JSONL 时应遵循：

- 坏 JSON 行跳过并继续。
- 缺失 timestamp 的运行时事件不应误判为核心消息质量问题。
- 缺失 cwd 的非 assistant/user 事件应尽量保留其可用信号。
- orphan `tool_result` 和 pending `tool_use` 计入质量信号。

这些信号会影响：

- `ToolAnalysis.MissingResults`
- `FailureAnalysis.Samples`
- `SessionAnalysis`
- `rec` 的 workflow / failure 诊断

## 前端降级

Web Dashboard 应对空数组、空对象和缺失字段保持稳定：

- 图表无数据时显示空态。
- 本地 ECharts 资源必须可用，避免局域网环境依赖 CDN。
- API 返回部分数据时，不应阻断其他图表渲染。

## 维护原则

- fallback 不能吞掉所有信息；能形成质量信号的异常应进入聚合。
- 不要把真实机器的一次性统计数字写成长期事实。
- 新增数据源时必须明确：缺失时返回什么、坏记录如何处理、是否影响诊断。
