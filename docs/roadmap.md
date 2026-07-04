# 路线图

本文档记录 cc-insights 的当前能力、近期重点和暂缓方向。项目主线是 code agent 使用诊断，而不是单纯 Dashboard 图表扩展。Claude Code 是默认 provider，Codex 已进入可用分析阶段。

## 已完成

### 基础能力

- 单二进制 Go CLI / Web Dashboard。
- 时间范围筛选：`24h`、`7d`、`30d`、`90d`、`all` 和自定义日期。
- 文件级缓存和轻量诊断缓存。
- 多数据源 provider 基础设施：`claude` / `codex`，CLI 保留 `auto` 检测，Web 使用显式 provider 切换，缓存按 provider 隔离。
- 静态资源嵌入，React Dashboard 支持局域网访问。

### 核心分析

- 失败原因分析：按 reason、tool+reason、model+reason 聚合，保留样例摘要。
- Bash 命令分析：命令族、具体命令、高风险命令、可配置分类规则。
- Token 与成本分析：按模型、项目、Session、Agent 聚合，支持 Claude 与 GPT/Codex 价格估算。
- Session 生命周期：长会话、高失败会话、状态、队列、权限和计划信号。
- Task / Plan 分析：Claude Plan 文件、Task 状态、提醒频率；Codex `update_plan` 步骤状态和 Top steps。
- Tool performance：工具类别耗时、最慢调用、质量分布、错误率。
- `rec` 结构化诊断：trigger、root cause、examples、actions、targets、drilldown commands。
- Codex provider：读取 `~/.codex/sessions/**/*.jsonl` 与 `logs_2.sqlite`，统计 session、项目、模型 token、工具调用、命令、失败、Plan 步骤、hook trace、patch diff、上下文和预算信号。
- Provider-aware Web：Codex 下隐藏没有可靠来源的 Skill / Agent 空卡片，Plan 自动切换为步骤视图。

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

### P2: Codex provider 增强

- 更细致地区分 Codex `turn_aborted`、sandbox、approval policy 等运行上下文。
- 继续核对 `state_5.sqlite` 中 agent job / thread spawn 等信号，只有存在真实数据时才接入 Agent 分析。
- 为 Codex hook trace 提供更细的阶段命名和耗时估计；当前 SQLite 日志只能稳定识别运行阶段和 session 关联。
- 评估是否引入 Codex history/title/prompt 轻量分析，同时避免展示完整对话或 instructions。

## 暂缓方向

- 新增大量 Dashboard 图表：除非能服务诊断主线，否则暂缓。
- 新增 CLI 命令：优先增强 `rec` 和现有下钻命令。
- 自动修改 `CLAUDE.md`、hooks 或 MCP：当前只做建议，不自动改。

## 未来方向

- 更多数据源 provider：Codex 之外的其他 agent 日志。
- 项目级诊断基线：不同项目使用不同阈值。
- 诊断报告导出：面向周报、复盘或团队共享。
- 与 Web UI 联动：从图表直接跳到 finding 和下钻证据。
