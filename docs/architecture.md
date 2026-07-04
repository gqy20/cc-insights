# 架构说明

cc-insights 是面向 code agent 的本地使用诊断工具。默认 provider 是 Claude Code，读取 `~/.claude`；也支持 Codex provider，读取 `~/.codex/sessions/**/*.jsonl` 和 Codex SQLite runtime 日志。不同 provider 的日志会经过各自 parser 映射到统一 `ProjectAggregate`，再为 CLI 和 Web Dashboard 提供统一数据。

## 核心目标

项目的主线不是展示更多图表，而是把 code agent 的行为问题解释到可行动：

- 失败集中在哪里，原因更像什么；
- 哪些命令、工具、Session 和项目消耗最多时间与 Token；
- 应该优化 `CLAUDE.md`、hook、MCP 配置、工作流，还是封装新的工具。

## 数据流

```text
~/.claude 或 ~/.codex 数据
  -> provider file discovery
  -> claude parser / codex parser
  -> ProjectAggregate
  -> CacheFile / diagnostics cache
  -> DashboardData
  -> CLI reports / Web API
```

Claude provider 主要数据源：

- `history.jsonl`：slash command 和历史命令概览。
- `projects/**/*.jsonl`：核心会话、工具调用、模型、Token、运行事件；真实 MCP 工具调用也属于这里的 `ToolAnalysis` 口径。
- `tasks/*/*.json`：Task 状态、Session 任务分布。
- `debug/`：runtime 事件日志路径，用于 hook、skill、budget、opened file 和 `runtime_tools` 等补充信号。

Codex provider 主要数据源：

- `sessions/**/*.jsonl`：核心 session、cwd/project、turn context、模型、token、工具调用、结构化工具结果和运行事件。
- `logs_2.sqlite`：Codex runtime trace，当前用于 hook trace、app-server hook 事件和部分运行上下文核对。
- `history.jsonl`、`session_index.jsonl`：暂未作为主分析来源，后续可补充 prompt/title 类信息。

Codex provider 不读取 `auth.json`、attachments 或 shell snapshots，避免扩大隐私和实现边界。SQLite 只读取聚合所需的 runtime 事件，不展示完整 prompt、assistant 回复、命令输出或 instructions。

## 主要模块

- `cmd/insights/cli.go`、`cli_*.go`：CLI 命令、报告构建和输出。
- `cmd/insights/project_parser.go`、`parser.go`：Claude 项目 JSONL 解析和事件识别。
- `cmd/insights/codex_parser.go`：Codex session JSONL 解析和 provider 映射。
- `cmd/insights/codex_hooks.go`：Codex `logs_2.sqlite` hook trace 解析、去重和日级 runtime 写入。
- `cmd/insights/aggregate.go`、`aggregate_finalize.go`：聚合与最终分析结果生成。
- `cmd/insights/cache.go`、`cache_builder.go`：缓存结构、文件级复用和诊断缓存。
- `web/`：React + Vite 前端源码；构建产物进入 `cmd/insights/static/dist/` 并由 Go embed 进单二进制。
- `cmd/insights/rules/bash.yml`：Bash 命令族分类规则。
- `cmd/insights/rules/diagnostics.yml`：诊断规则的指标、阈值、来源和触发解释。
- `cmd/insights/command_file_analysis.go`：Bash 命令解析核心，支持多段 `&&`/`;` 链式命令独立统计（引号感知分割、同链去重、结果分发、逐段风险检测）。

## 聚合与数据构建边界

`ProjectAggregate` 是解析后的唯一核心聚合模型。正常解析、空降级结果、文件级缓存还原和日级 runtime 索引都应通过 `newProjectAggregate()` 初始化，避免不同路径维护多份 map 字段清单。

日志文件发现统一由 provider-aware `listLogFileInfos` 负责。Claude 使用 `projects/**/*.jsonl`，Codex 使用 `sessions/**/*.jsonl`。实时解析和缓存构建共享同一套文件列表，cache builder 在此基础上额外使用 size / mtime 做文件级复用。

`DashboardData` 是 CLI 和 Web API 的对外数据面。缓存路径和实时解析路径可以有不同的数据来源，但时间范围、每日趋势、小时统计、项目总计等公共组装逻辑应放在 `api.go` 的共享 helper 中，避免新增分析模块时只更新其中一条路径。

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

默认缓存位于 `~/.cc-insights/cache/`，内部按 provider 隔离：

- `claude/cache.db`、`codex/cache.db`：完整预聚合缓存，服务 Web 和完整数据构建。
- `claude/diagnostics.db`、`codex/diagnostics.db`：轻量诊断缓存，去掉项目文件级缓存，服务 `rec` 和下钻命令。

CLI 下钻命令优先复用诊断缓存，避免因为当前 agent 会话正在写 JSONL 而频繁触发完整重建。缓存文件包含 provider 元数据，provider 不一致会触发重建，避免 Claude/Codex 混用缓存。

## Web Dashboard

Web Dashboard 负责可视化趋势、运行时统计和分析结果。当前主线是让 Web 后续承载 `rec` 的结构化诊断，而不是继续堆孤立图表。

Web 的 provider 选择是显式的：默认 Claude Code，用户可在顶部筛选栏切换 Codex。前端按 provider 调整展示语义，例如 Codex 下 Plan 卡展示 `update_plan` 步骤而不是 Claude 的 Plan 文件；Codex 没有可靠 skill invocation 或非 main agent 数据时隐藏对应空卡片，避免把“无等价数据源”误读为“解析失败”。

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

`/api/data` 仍是主图表的全量兼容数据面；`/api/overview`、`/api/diagnostics`、`/api/detail/*` 是面向交互和下钻的分层数据面。两者共享 `AnalysisFilter` 和 `loadDashboardDataWithFilter`，不要为新图表再增加平行过滤入口。

日级趋势目前有三层索引：

- `DailyRuntime`：date 级运行时聚合，支持工具、失败原因、模型的单维或部分模型组合趋势。
- `DailyProjectRuntime`：date × project 运行时聚合，支持 `project + tool/reason/model` 等组合趋势。
- `DailySessionRuntime`：date × session 运行时聚合，支持 `session + tool/reason/model` 等组合趋势。

仍然需要谨慎的部分包括成本归因、文件编辑质量、运行事件、Task/Plan、Skill/Agent 等模块的复杂组合筛选。它们没有对应交叉索引时会被标记为 `unavailable` 或 `sample`。
