# 路线图

本文档记录 cc-insights 的当前能力、近期重点和暂缓方向。项目主线是 Claude Code 使用诊断，而不是单纯 Dashboard 图表扩展。

## 已完成

### 基础能力

- 单二进制 Go CLI / Web Dashboard。
- 时间范围筛选：`24h`、`7d`、`30d`、`90d`、`all` 和自定义日期。
- 文件级缓存和轻量诊断缓存。
- 静态资源嵌入，ECharts 本地化，支持局域网访问。

### 核心分析

- 失败原因分析：按 reason、tool+reason、model+reason 聚合，保留样例摘要。
- Bash 命令分析：命令族、具体命令、高风险命令、可配置分类规则。
- Token 与成本分析：按模型、项目、Session、Agent 聚合。
- Session 生命周期：长会话、高失败会话、状态、队列、权限和计划信号。
- Task / Plan 分析：Plan 生命周期、计划文件引用、Task 状态、提醒频率。
- Tool performance：工具类别耗时、最慢调用、质量分布、错误率。
- `rec` 结构化诊断：trigger、root cause、examples、actions、targets、drilldown commands。

### CLI 命令体系

- `sum`：全局概览。
- `rec`：诊断与解释。
- `why`：失败样例。
- `cmd`：Bash 命令。
- `tok`：Token 与成本。
- `ses`：Session 生命周期。
- `web`：Dashboard。

命令体系已经收敛，后续不优先新增命令。

## 当前重点

### P0: 提升诊断质量

- 用更多真实样例细化 root cause。
- 让 `examples` 与 finding 的匹配更精准。
- 引入“恢复是否成功”的信号，区分一次性错误和反复失败。
- 增强 `actions`，使其更明确地指向 `CLAUDE.md`、hook、MCP、tool 或 workflow。

### P1: Web 展示结构化诊断

- 在 Dashboard 中展示 `rec` finding。
- 支持按 category、severity、target 过滤。
- 为每条 finding 展示 trigger、root causes、examples、actions 和 drilldown command。
- 使用 `/api/overview`、`/api/diagnostics`、`/api/detail/*` 和 `/api/timeline` 支撑大屏联动。

### P1: 规则配置沉淀

- 保持 `rules/bash.yml` 管理 Bash 命令族。
- 使用 `rules/diagnostics.yml` 管理诊断规则说明。
- 等 root cause 与 action 稳定后，再考虑把更多阈值和分类映射配置化。

### P2: 诊断闭环

- 关联失败样例与后续重试是否成功。
- 识别“重复失败 -> 修正 -> 成功”的模式。
- 为 `CLAUDE.md`、hook、MCP、tool 给出更具体的改进模板。
- 逐步把恢复结果、重复失败和建议动作优先级纳入 `rec`。

## 暂缓方向

- 新增大量 Dashboard 图表：除非能服务诊断主线，否则暂缓。
- 新增 CLI 命令：优先增强 `rec` 和现有下钻命令。
- 自动修改 `CLAUDE.md`、hooks 或 MCP：当前只做建议，不自动改。
- Codex 数据源支持：保留调研，当前不纳入 Claude Code 诊断主线。

## 未来方向

- 多数据源 provider：Claude Code 之外的 Codex、其他 agent 日志。
- 项目级诊断基线：不同项目使用不同阈值。
- 诊断报告导出：面向周报、复盘或团队共享。
- 与 Web UI 联动：从图表直接跳到 finding 和下钻证据。
