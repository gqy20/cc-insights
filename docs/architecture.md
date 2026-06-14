# 架构说明

cc-insights 是面向 Claude Code 的本地使用诊断工具。它读取 `~/.claude` 中的历史命令、项目会话、任务、工具调用和运行事件，经过解析、聚合、缓存后，为 CLI 和 Web Dashboard 提供统一数据。

## 核心目标

项目的主线不是展示更多图表，而是把 Claude Code 的行为问题解释到可行动：

- 失败集中在哪里，原因更像什么；
- 哪些命令、工具、Session 和项目消耗最多时间与 Token；
- 应该优化 `CLAUDE.md`、hook、MCP 配置、工作流，还是封装新的工具。

## 数据流

```text
~/.claude 数据
  -> parser / project parser / task parser
  -> ProjectAggregate
  -> CacheFile / diagnostics cache
  -> DashboardData
  -> CLI reports / Web API
```

主要数据源：

- `history.jsonl`：slash command 和历史命令概览。
- `projects/**/*.jsonl`：核心会话、工具调用、模型、Token、运行事件。
- `tasks/*/*.json`：Task 状态、Session 任务分布。
- `debug/`：runtime 事件日志路径，用于 hook、skill、budget、opened file 等运行时信号。

## 主要模块

- `cmd/insights/cli.go`、`cli_*.go`：CLI 命令、报告构建和输出。
- `cmd/insights/project_parser.go`、`parser.go`：项目 JSONL 解析和事件识别。
- `cmd/insights/aggregate.go`、`aggregate_finalize.go`：聚合与最终分析结果生成。
- `cmd/insights/cache.go`、`cache_builder.go`：缓存结构、文件级复用和诊断缓存。
- `cmd/insights/static/`：Web Dashboard 静态资源，ECharts 本地化。
- `cmd/insights/rules/bash.yml`：Bash 命令族分类规则。
- `cmd/insights/rules/diagnostics.yml`：诊断规则的指标、阈值、来源和触发解释。

## CLI 职责

命令保持收敛：

- `sum`：全局概览。
- `rec`：诊断、解释、触发条件、根因候选、样例、建议动作和下钻命令。
- `why`：失败样例下钻。
- `cmd`：Bash 命令族、具体命令和高风险命令。
- `tok`：Token、模型、项目和会话消耗。
- `ses`：Session 生命周期、长会话、高失败会话和 Plan/Task 信号。
- `web`：本地 Dashboard。

新增分析能力优先增强 `rec`，不要为每一种解释新增命令。

## 缓存策略

默认缓存位于 `~/.cc-insights/cache/`。

- `cache.db`：完整预聚合缓存，服务 Web 和完整数据构建。
- `diagnostics.db`：轻量诊断缓存，去掉项目文件级缓存，服务 `rec` 和下钻命令。

CLI 下钻命令优先复用诊断缓存，避免因为当前 Claude Code 会话正在写 JSONL 而频繁触发完整重建。

## Web Dashboard

Web Dashboard 负责可视化趋势、运行时统计和分析结果。当前主线是让 Web 后续承载 `rec` 的结构化诊断，而不是继续堆孤立图表。

## 交互式 API

为大屏联动新增的后端接口按“概览、诊断、详情、时间轴”分层：

- `/api/overview`：轻量汇总，服务顶部指标、趋势和 Top 列表。
- `/api/diagnostics`：结构化 `rec` 诊断，支持 `detail`、`id`、`severity`、`target` 等过滤。
- `/api/detail/failures`：失败样例和原因下钻。
- `/api/detail/commands`：Bash 命令和命令族下钻。
- `/api/detail/tokens`：Token、模型、项目和 Session 成本下钻。
- `/api/detail/sessions`：Session 生命周期下钻。
- `/api/detail/tools`：工具性能和慢调用下钻。
- `/api/timeline`：全局时间轴数据，服务 slider / brush。

这些接口和 `/api/data` 复用同一套 filter。后端会为响应附带 `coverage`，标记每个图表在当前筛选下是 `exact`、`sample` 还是 `unavailable`。前端只展示可解释的数据：无法精确重算的图表显示空态原因，不展示全局数据冒充联动结果。

日级趋势目前有三层索引：

- `DailyRuntime`：date 级运行时聚合，支持工具、失败原因、模型的单维或部分模型组合趋势。
- `DailyProjectRuntime`：date × project 运行时聚合，支持 `project + tool/reason/model` 等组合趋势。
- `DailySessionRuntime`：date × session 运行时聚合，支持 `session + tool/reason/model` 等组合趋势。

仍然需要谨慎的部分包括成本归因、文件编辑质量、运行事件、Task/Plan、Skill/Agent 等模块的复杂组合筛选。它们没有对应交叉索引时会被标记为 `unavailable` 或 `sample`。
