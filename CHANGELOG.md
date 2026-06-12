# 更新日志

这个文件记录项目中值得关注的版本变化。

项目使用带 `v` 前缀的语义化版本标签，例如 `v0.1.0`。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.1.0] - 2026-06-12

### 新增
- **CLI-first 使用方式**：默认运行 `cc-insights` 输出最近 30 天摘要，`web` 命令启动本地 Web Dashboard，适合终端和 AI Agent 直接调用。
- **统一短命令体系**：提供唯一命令集 `sum`、`err`、`why`、`tok`、`web`，分别用于总览、失败来源、失败样例下钻、Token/成本和 Web Dashboard。
- **AI 友好输出**：CLI 支持 table、JSON、Markdown 输出，提供 `-p`、`-j`、`-m`、`-f`、`-n` 等短参数，stdout 保持结果输出，便于脚本和 AI 读取。
- **失败分析 CLI**：`err` 按失败原因、工具、模型组合拆解失败来源，`why` 支持按 reason、category、tool、model、project、session 过滤并输出失败样例。
- **Token 与成本分析 CLI**：`tok` 输出 Token 总量、模型消耗、项目消耗和会话消耗，数字使用紧凑单位展示。
- **Web Dashboard**：提供 Claude Code 使用数据仪表盘，支持默认局域网可访问的单机 Web 服务。
- **Dashboard 时间范围筛选**：支持 24h、7天、30天、90天、全部和自定义日期范围，核心图表随筛选范围联动。
- **基础使用趋势图表**：展示每日活动趋势、Slash Commands 使用排行、MCP 工具调用统计、每日会话趋势、项目活跃度、星期活动分布、模型使用和工作时段分布。
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
- **HTTP API**：提供 `/api/data`、`/api/stats` 和 `/api/reload`，支持前端数据读取、旧版统计接口兼容和手动刷新。
- **单文件部署**：静态资源嵌入 Go 二进制，支持静态链接构建和跨平台发布包。
- **测试覆盖**：覆盖解析器、缓存、API、会话分析、失败分析、成本分析、CLI 命令规范和一致性检查。
- **项目文档**：README 提供快速开始、CLI 用法、API 示例、性能测试和开发命令说明。
