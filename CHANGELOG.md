# 更新日志

这个文件记录项目中值得关注的版本变化。

项目使用带 `v` 前缀的语义化版本标签，例如 `v0.1.0`。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.1.1] - 2026-06-15

### 新增
- **诊断推荐系统 (`rec`)**：新增结构化诊断命令，支持按时间范围查询，输出证据摘要、触发条件、根因候选、建议动作和下钻命令。
- **快速诊断下钻**：`rec` 支持 `--detail` 参数展示详细诊断信息，包含指标值、阈值对比和数据来源引用。
- **Prompt 分析诊断**：新增 Prompt Profile 诊断维度，分析用户输入模式、命令分布和交互特征。
- **可配置 Bash 命令规则**：支持通过 `rules/bash.yml` 自定义 Bash 命令分类规则，新增风险等级标记和模式匹配。
- **Bash 链式命令解析**：完整解析 `&&` / `;` 链式命令的每个段，独立统计各段执行结果和风险检测。
- **Skill 分析 Dashboard**：新增 Skill 使用统计面板，展示 Slash Command 调用频率、成功率和分布趋势。
- **交互式 Dashboard API**：新增 `/api/interactive/*` 系列接口，支持下钻查询、动态筛选和实时数据联动。
- **Dashboard 高级筛选**：支持多维度组合筛选（项目、模型、时间范围、工具类型），前端筛选逻辑后移至 API 层提升精度。
- **运行时覆盖索引**：新增过滤后的运行时事件覆盖索引，加速特定工具和时间范围的查询性能。
- **性能上下文元数据**：缓存和解析结果携带性能元数据（解析耗时、记录数、时间范围），辅助调试和监控。

### 改进
- **诊断规则引擎**：引入 `rules/diagnostics.yml` 配置文件，支持可维护的阈值、指标定义和触发解释。
- **诊断性能优化**：预计算轻量级诊断缓存，避免重复聚合，大幅提升 `rec` 命令响应速度。
- **Session 下钻命令 (`ses`)**：新增会话生命周期分析 CLI，支持查看长会话、高失败会话和 Plan/Task 信号。
- **Bash 命令族分析 (`cmd`)**：新增 `cmd` 命令用于 Bash 命令族统计、具体命令查看和高风险命令识别。
- **缓存重载机制**：支持通过 API 和 CLI 触发缓存重建，规则文件变更时自动失效相关缓存。
- **文档体系重组**：拆分冗余文档，新增 `docs/diagnostics.md`、`docs/architecture.md` 和精简版 `docs/fallbacks.md`。

### 移除
- **旧版 stats API**：移除 `/api/stats` 接口，统一使用 `/api/data` 和新的交互式 API。

### 修复
- **运行时工具信号命名**：统一 runtime event 中的工具信号字段命名，消除歧义。
- **缓存行为澄清**：明确缓存命中、未命中和重建的条件判断逻辑，改进日志输出。

## [0.1.0] - 2026-06-12

### 新增
- **CLI-first 使用方式**：默认运行 `cc-insights` 输出最近 30 天摘要，`web` 命令启动本地 Web Dashboard，适合终端和 AI Agent 直接调用。
- **统一短命令体系**：提供唯一命令集 `sum`、`err`、`why`、`tok`、`web`，分别用于总览、失败来源、失败样例下钻、Token/成本和 Web Dashboard。
- **AI 友好输出**：CLI 支持 table、JSON、Markdown 输出，提供 `-p`、`-j`、`-m`、`-f`、`-n` 等短参数，stdout 保持结果输出，便于脚本和 AI 读取。
- **失败分析 CLI**：`err` 按失败原因、工具、模型组合拆解失败来源，`why` 支持按 reason、category、tool、model、project、session 过滤并输出失败样例。
- **Token 与成本分析 CLI**：`tok` 输出 Token 总量、模型消耗、项目消耗和会话消耗，数字使用紧凑单位展示。
- **Web Dashboard**：提供 Claude Code 使用数据仪表盘，支持默认局域网可访问的单机 Web 服务。
- **Dashboard 时间范围筛选**：支持 24h、7天、30天、90天、全部和自定义日期范围，核心图表随筛选范围联动。
- **基础使用趋势图表**：展示每日活动趋势、Slash Commands 使用排行、Runtime 工具信号、每日会话趋势、项目活跃度、星期活动分布、模型使用和工作时段分布。
- **失败来源图表**：按原因、工具和模型展示失败来源，辅助定位常见错误模式。
- **Session 生命周期分析**：展示会话数量、消息量、工具调用、生命周期事件、会话持续时间和异常信号。
- **Task / Plan 结构分析**：分析计划模式、任务状态、提醒信号、Plan 文件引用和目标状态，用于评估 Claude Code 工作流质量。
- **工具与文件操作分析**：统计工具调用成功率、缺失结果、慢调用、文件热点、文件编辑失败和 Bash 命令风险。
- **运行时分析面板**：解析 runtime 事件、agent 工具调用、hook、skill、budget、opened file 等 debug 日志信息。
- **成本与模型分析**：从 Claude Code project JSONL 中解析模型、Token、缓存读取、项目和会话维度的消耗。
- **数据解析能力**：支持解析 `history.jsonl`、`projects/*.jsonl`、`sessions-index.json` 和 `debug/` 日志目录。
- **AssistantMessage thinking 解析**：支持 Claude thinking 类型内容，兼容不同字段命名的 Token usage 结构。
- **并发解析引擎**：对 history、projects 和 debug 日志进行并行解析，`projects/*.jsonl` 使用 `ParseProjectsConcurrentOnce` 单次遍历聚合多类统计。
- **缓存系统**：使用 `CacheBuilder` 预聚合并持久化缓存，按时间范围查询，基于文件修改时间判断是否重建。
- **性能基准与大数据支持**：面向 GB 级 Claude Code 数据目录优化解析和缓存流程，README 记录 2.2GB 数据集下的性能测试结果。
- **HTTP API**：提供 `/api/data`、交互式下钻接口和 `/api/reload`，支持前端数据读取、大屏联动和手动刷新。
- **单文件部署**：静态资源嵌入 Go 二进制，支持静态链接构建和跨平台发布包。
- **测试覆盖**：覆盖解析器、缓存、API、会话分析、失败分析、成本分析、CLI 命令规范和一致性检查。
- **项目文档**：README 提供快速开始、CLI 用法、API 示例、性能测试和开发命令说明。
