# Claude Code Log Analysis Roadmap

本文档整理 `cc-insights` 针对 `~/.claude` 日志的已实现能力、尚未完善的统计维度，以及后续迭代优先级。

## 当前目标

核心目标是分析 Claude Code 运行日志，回答这些问题：

- 经常调用哪些工具？
- 哪些工具经常失败？
- 失败和模型、agent、skill、文件、命令之间有什么关系？
- 哪些会话成本高、失败多、自动化事件异常多？
- 如何据此优化 Claude Code 使用习惯和本项目的分析能力？

## 已实现能力

### 核心数据源

| 数据源 | 当前用途 | 状态 |
| --- | --- | --- |
| `projects/**/*.jsonl` | 核心日志，包含对话、工具、模型、attachment、agent/subagent | 已接入 |
| `history.jsonl` | slash command 统计 | 已接入，但不参与 project cache 重建 |
| `debug/` | 旧版 MCP debug 扫描 | 保留兼容，缓存构建已不依赖 |
| `subagents/agent-*.jsonl` | subagent 工具调用与失败分析 | 已递归接入 |

### 已发现但尚未充分利用的数据源

基于当前 `~/.claude` 盘点，除已接入的核心数据外，还有多类可用于增强分析的本地数据。实现时应坚持“默认聚合、不展示正文”的原则，避免把 prompt、文件快照、粘贴内容、工具完整输出直接暴露到 API 或 Dashboard。

| 数据源 | 当前观察 | 可分析价值 | 建议优先级 |
| --- | --- | --- | --- |
| `projects/**/*.jsonl` 新事件类型 | 样本中除 `assistant/user/attachment` 外，还有 `system`、`queue-operation`、`last-prompt`、`file-history-snapshot`、`permission-mode`、`mode`、`custom-title`、`ai-title`、`agent-name` | session 生命周期、标题、首尾 prompt、队列状态、停止原因、运行耗时、文件快照索引 | P0 |
| `projects/**/*.jsonl` 新 attachment 类型 | `task_reminder`、`queued_command`、`hook_success`、`budget_usd`、`skill_listing`、`file`、`structured_output`、`edited_text_file`、`compact_file_reference`、`plan_mode`、`plan_file_reference`、`command_permissions`、`goal_status` 等 | 任务/计划/skill/预算/权限/编辑行为分析 | P0-P1 |
| `stats-cache.json` | 含 `firstSessionDate`、`lastComputedDate`、`longestSession`、`totalMessages`、`totalSessions`、`totalSpeculationTimeSavedMs`、`dailyActivity`、`modelUsage` | 快速概览、最长 session、缓存健康检查、与实时解析结果一致性校验 | P0 |
| `file-history/` | 当前约 3232 个版本快照文件，文件名形如 `<hash>@vN`，内容是真实文件快照 | 热点编辑文件、版本次数、编辑密度、快照膨胀、与 `file-history-snapshot` 关联 | P0-P1 |
| `sessions/*.json` | 少量运行态 session 状态，含 `sessionId`、`cwd`、`status`、`startedAt`、`updatedAt`、`pid`、`entrypoint`、`version` | 活跃/空闲 session、运行态状态、daemon/session 对齐 | P1 |
| `tasks/*/*.json` | 当前约 1312 个任务 JSON，字段含 `subject`、`description`、`status`、`blocks`、`blockedBy` | 任务状态分布、阻塞关系图、任务完成率、任务与 session 关联 | P1 |
| `plans/*.md` | 当前约 28 个计划文件 | plan mode 产物统计、计划文件生命周期、计划与执行结果对齐 | P1 |
| `jobs/*/state.json` | 后台 job 状态，含 `state`、`detail`、`intent`、`sessionId`、`resumeSessionId`、`cwd`、`createdAt`、`updatedAt`、`backend` | 后台作业生命周期、失败/卡住 job、resume 关系 | P2 |
| `jobs/*/timeline.jsonl` | 每行含 `at`、`state`、`detail`、`text` | job 状态迁移、耗时、异常节点 | P2 |
| `daemon.status.json` / `daemon.log` | daemon worker/状态/日志 | daemon 稳定性、worker 数、后台故障 | P2 |
| `git_status_cache_*.txt` / `project_stats_cache_*.txt` | 项目级 Git/status cache 文件 | 项目脏状态、分支状态、与高失败 session 关联 | P2 |
| `paste-cache/` | 粘贴文本缓存，当前有少量 txt | 仅适合数量/大小统计，不建议内容分析 | P3 |
| `telemetry/1p_failed_events*.json` | 失败遥测缓存 | 可能用于故障补充，但隐私和格式风险较高 | P3 |
| `shell-snapshots/` | shell 环境快照脚本 | 环境变化诊断，默认不纳入主缓存 | P3 |

当前机器粗略规模：

- `projects/**/*.jsonl`: 约 `3414` 个文件
- `file-history/`: 约 `3232` 个快照文件
- `tasks/*/*.json`: 约 `1312` 个任务文件
- `plans/*.md`: 约 `28` 个计划文件
- `sessions/*.json`: `6` 个运行态 session 文件
- `debug/`: 当前未观察到普通 debug 文件，不能假定总是存在
- `~/.claude` 总文件数：约 `15729`

### 工具分析

已实现：

- 工具调用总数、成功数、失败数、missing result
- 工具失败率
- 失败类型聚合
- 失败样例
- 模型与工具的成功/失败/missing result 关联
- MCP 工具统计从 `mcp__server__tool` 派生

主要输出：

- `tool_analysis.tools`
- `tool_analysis.by_model`
- `tool_analysis.failure_kinds`
- `tool_analysis.failure_samples`

### 运行事件分析

已实现：

- attachment 类型统计
- hook 成功、取消、非阻塞错误统计
- `invoked_skills`
- `permission-mode`
- `plan_mode` / `plan_mode_exit`
- `queued_command`
- `opened_file_in_ide`
- `budget_usd` latest/max 摘要

主要输出：

- `event_analysis.by_type`
- `event_analysis.hooks`
- `event_analysis.skills`
- `event_analysis.permission_modes`
- `event_analysis.opened_files`
- `event_analysis.budget`

### Agent / Subagent 分析

已实现：

- 主会话 vs subagent 工具调用数
- agentId 维度统计
- agent message/session/tool call/failure/missing result
- agent 失败率

主要输出：

- `agent_analysis.main_tool_calls`
- `agent_analysis.sidechain_tool_calls`
- `agent_analysis.agents`

### Bash / 文件操作分析

已实现：

- Bash 命令 Top
- Bash 成功/失败/missing result
- 高风险命令识别：
  - `rm -rf`
  - `git reset --hard`
  - `git clean -fd`
  - `sudo`
  - `curl | sh`
  - `wget | sh`
- Read/Edit/Write/MultiEdit 文件路径级统计
- 文件操作成功/失败/missing result

主要输出：

- `command_analysis.bash_commands`
- `command_analysis.risky_commands`
- `command_analysis.file_operations`

### Dashboard 展示

已实现：

- 模型 × 工具失败分析
- 运行事件与 Hook 状态
- Agent / Subagent 工具调用
- Bash 与文件操作失败
- 原有趋势、项目、模型、工作时段、MCP 工具等图表

### 缓存与性能

已实现：

- cache version `2.3`
- project JSONL 文件级聚合缓存
- 每个文件记录：
  - path
  - size
  - mtime
  - aggregate snapshot
- 未变化文件复用 aggregate
- cache 使用紧凑 JSON
- cache rebuild 只检查 `projects/`
- `cost_analysis` token/预算分析随文件级 aggregate 缓存
- `failure_analysis` 失败原因细分随文件级 aggregate 缓存

真实数据验证：

- 项目日志文件数：约 `3431`
- 首次 2.1 构建：约 `10.9s`
- 全文件复用构建：约 `0.7s`
- cache 文件体积：约 `17MB`

## 待完善能力

## P0: 高优先级

### 1. Token / 成本分析

现状：

- 已实现 `cost_analysis` 初版
- 已拆分 `input_tokens`、`output_tokens`、`cache_read_input_tokens`、`cache_creation_input_tokens`
- 已统计 `server_tool_use` 请求数并归入 server tool use 计数
- 已按 model/project/session/agent 聚合 token
- 已输出高 token session/project/agent/model Top 列表
- 已输出 `budget_usd` 时间线

待完善：

- 工具调用前后 token 消耗
- cache 命中与成本节省估算
- 基于真实模型单价估算 USD 成本
- Dashboard 增加成本 Top 图表

建议输出：

- `cost_analysis.by_model`
- `cost_analysis.by_project`
- `cost_analysis.by_session`
- `cost_analysis.by_agent`
- `cost_analysis.totals`
- `cost_analysis.budget_timeline`

价值：

- 直接回答“哪些模型/项目/会话最花钱”
- 可以指导模型选择和上下文压缩策略

### 2. 失败原因细分

现状：

- 已实现 `failure_analysis` 初版
- 兼容保留 `tool_analysis.failure_kinds`
- 已新增细分字段：
  - `category`
  - `reason`
- 已按 reason/tool+reason/model+reason 聚合
- 已在 Dashboard 增加失败原因细分图

已覆盖：

- Bash 失败：
  - exit code N
  - command not found
  - test failure
  - permission denied
  - timeout
- Edit 失败：
  - old_string mismatch
  - file not found
  - permission
  - encoding / binary file
- MCP 失败：
  - auth error
  - schema error
  - network error
  - rate limit
  - HTTP status
  - model not found
- Web/网络失败：
  - DNS
  - TLS
  - timeout
  - 403/404/429/5xx

待完善：

- 从 Bash 输出中提取更多测试框架失败模式
- MCP 错误结构化字段优先于文本匹配
- 为失败样例增加可跳转 session/file 定位
- 失败原因与后续重试/恢复是否成功关联

建议输出：

- `failure_analysis.by_reason`
- `failure_analysis.by_tool_reason`
- `failure_analysis.by_model_reason`
- `failure_analysis.samples`

价值：

- 从“某工具失败多”升级到“为什么失败”
- 能生成可操作优化建议

### 3. 会话生命周期摘要

现状：

- 已统计 top-level event 类型
- 还没有形成 session 级生命周期画像
- `projects/**/*.jsonl` 中已经存在可利用的生命周期事件：
  - `system`：包含 `subtype`、`stopReason`、`durationMs`、`messageCount`、`hookCount`、`hookErrors`、`preventedContinuation`
  - `last-prompt`：包含 `lastPrompt`、`leafUuid`、`sessionId`
  - `custom-title` / `ai-title`：包含 session 标题
  - `queue-operation`：包含 `operation`、`content`、`timestamp`
  - `permission-mode` / `mode`：包含权限或模式变化
  - `agent-name`：包含 agent 命名信息

待实现：

- 每个 session 的：
  - started/result
  - first prompt / last prompt
  - ai-title/custom-title
  - started time / ended time / duration
  - result status
  - message count
  - tool count
  - failure count
  - token/cost
  - project
  - permission mode changes
  - plan mode usage
- system stop reason / prevented continuation
- queue operation timeline
- hook count / hook error count
- title 来源：
  - ai-title
  - custom-title
- Session 成功/失败/中断分类
- 高失败会话 Top N
- 长耗时会话 Top N
- 与 `stats-cache.longestSession` 做一致性校验

建议输出：

- `session_analysis.sessions`
- `session_analysis.top_failures`
- `session_analysis.long_running`
- `session_analysis.outcomes`
- `session_analysis.queue_operations`
- `session_analysis.titles`

价值：

- 把底层事件整理成可读任务摘要
- 后续可做“问题会话复盘页”

### 4. 文件编辑深度分析

现状：

- 已有 Read/Edit/Write/MultiEdit 的路径级统计
- `projects/**/*.jsonl` 中存在 `file-history-snapshot` 事件，字段为 `messageId`、`snapshot`、`isSnapshotUpdate`
- attachment 中存在 `edited_text_file`、`file`、`compact_file_reference`、`plan_file_reference`
- `file-history/` 保存真实文件快照，可与 `file-history-snapshot` 做关联，但不应直接展示内容

待实现：

- 接入：
  - `file-history-snapshot`
  - `edited_text_file`
  - `file-history/`
  - `opened_file_in_ide`
- 每个 session 修改文件数
- 每个 project 修改文件数
- 每个文件快照版本数
- 每个 session 快照数量
- 快照文件总体积与 Top N
- 文件编辑失败原因
- 文件修改规模：
  - lines added
  - lines removed
  - edit attempts
  - retry count
- 同一文件反复编辑/回滚/重试模式
- 高频失败文件 Top N

建议输出：

- `file_analysis.touched_files`
- `file_analysis.edited_files`
- `file_analysis.failed_edits`
- `file_analysis.by_session`
- `file_analysis.snapshots`
- `file_analysis.hot_files`

价值：

- 找出最容易出问题的文件和编辑模式
- 支持代码质量与工具使用诊断
- 发现大文件快照导致的本地存储膨胀

## P1: 中优先级

### 5. 多模态输入分析

现状：

- 日志中存在 image/document content type
- 尚未独立统计

待实现：

- image/document 输入次数
- 多模态输入所属 project/session
- 多模态任务触发的工具链
- 多模态任务失败率
- 多模态任务 token/cost

建议输出：

- `multimodal_analysis.inputs`
- `multimodal_analysis.by_project`
- `multimodal_analysis.tool_chain`

### 6. Skill 效果分析

现状：

- 已统计 `invoked_skills` 次数

待实现：

- skill 触发后的工具链
- skill 关联失败率
- skill 与 project 的关系
- skill 与 agent/tool/model 的关系
- skill 是否降低失败率或减少工具重试

建议输出：

- `skill_analysis.skills`
- `skill_analysis.by_project`
- `skill_analysis.tool_chains`
- `skill_analysis.failure_rate`

### 7. Agent 链路分析

现状：

- 已有 agent/subagent 工具调用和失败率

待实现：

- 主会话中的 Agent 工具调用与 subagent 文件关联
- subagent 任务输入类型
- subagent 平均执行深度
- subagent 平均耗时
- agent 常见失败工具链
- agent 输出质量/结果状态

建议输出：

- `agent_flow_analysis.tasks`
- `agent_flow_analysis.tool_chains`
- `agent_flow_analysis.failures`

### 8. Bash 命令分类增强

现状：

- 已按首个命令统计
- 已有基础风险规则

待实现：

- 命令分类：
  - git
  - package manager
  - test
  - build
  - python
  - go
  - docker
  - network
  - file operation
  - process
- exit code 分布
- 常见失败命令 Top N
- 命令失败后的重试模式
- 高风险命令上下文

建议输出：

- `command_analysis.categories`
- `command_analysis.exit_codes`
- `command_analysis.retry_patterns`

### 9. Task / Plan 分析

现状：

- `tasks/*/*.json` 中存在结构化任务数据，字段包括 `subject`、`description`、`status`、`blocks`、`blockedBy`
- `plans/*.md` 中存在 plan mode 产物
- attachment 中存在 `task_reminder`、`todo_reminder`、`plan_mode`、`plan_mode_exit`、`plan_mode_reentry`、`plan_file_reference`

待实现：

- task 状态分布：
  - pending
  - in_progress
  - completed
  - blocked
- task 阻塞关系图
- 每个 session / project 的 task 数
- plan mode 进入/退出/重入次数
- plan 文件创建与引用次数
- plan 与后续工具链、失败率、完成状态的关系

建议输出：

- `task_analysis.statuses`
- `task_analysis.block_graph`
- `task_analysis.by_project`
- `plan_analysis.files`
- `plan_analysis.mode_events`
- `plan_analysis.outcomes`

价值：

- 判断复杂任务是否被拆解清楚
- 发现长期 blocked task 与高失败 session
- 评估 plan mode 是否提升完成率

## P2: 低优先级

### 10. Debug 深层日志分析

现状：

- `debug/` 不参与缓存构建
- MCP 工具统计已从 `projects` 派生
- 当前机器上 `debug/` 未观察到普通 debug 文件，不能假定该目录总是存在或有数据
- 项目 JSONL 中的 tool/event/attachment 已经覆盖许多原先依赖 debug 文本扫描的场景

待实现：

- MCP transport 错误
- 原始请求/响应耗时
- SDK/连接错误
- debug 与 tool_result 关联
- debug 数据可用性检测与降级提示

使用场景：

- 需要排查 MCP server 或网络层问题时再启用
- 不建议默认纳入主缓存热路径

### 11. 后台作业 / Daemon 分析

现状：

- `sessions/*.json` 中存在运行态 session 状态，含 `sessionId`、`cwd`、`status`、`startedAt`、`updatedAt`、`pid`、`entrypoint`、`version`
- `jobs/*/state.json` 中存在后台 job 状态，含 `state`、`detail`、`intent`、`sessionId`、`resumeSessionId`、`cwd`、`createdAt`、`updatedAt`、`backend`
- `jobs/*/timeline.jsonl` 每行含 `at`、`state`、`detail`、`text`
- `daemon.status.json` 包含 supervisor 与 workers 状态

待实现：

- 活跃/空闲 session 统计
- job 状态分布与耗时
- job resume 链路
- job timeline 状态迁移
- daemon worker 数与异常状态
- 后台 job 与 project/session 的关联

建议输出：

- `runtime_analysis.sessions`
- `runtime_analysis.jobs`
- `runtime_analysis.job_timelines`
- `runtime_analysis.daemon`

使用场景：

- 排查后台任务卡住、重复 resume、daemon 状态异常
- 不建议作为主 dashboard 首屏，适合诊断页或高级页

### 12. 诊断页 / 建议页

现状：

- Dashboard 已展示多个图表
- 缺少面向行动的诊断视图

待实现：

- 最失败工具
- 最失败模型组合
- 最失败文件
- 最失败 agent
- Stop hook 错误详情
- 高成本 session
- 权限模式变化异常
- 长期 blocked task
- 卡住或失败的后台 job
- file-history 快照异常膨胀
- 自动生成优化建议

建议输出：

- `diagnostics.issues`
- `diagnostics.recommendations`

## 建议迭代顺序

### Milestone 1: 成本与失败诊断

目标：

- 知道哪里最贵
- 知道失败为什么发生

任务：

1. 实现 `cost_analysis`（已完成初版）
2. 实现 `failure_analysis`（已完成初版）
3. Dashboard 增加成本 Top 与失败原因图（已完成初版）

### Milestone 2: Session 级复盘

目标：

- 从零散事件变成任务级摘要

任务：

1. 实现 `session_analysis`
2. 接入 started/result/title/last-prompt
3. 输出高失败、高成本、长耗时 session

### Milestone 3: 文件与编辑质量

目标：

- 分析哪些文件最容易被反复读写或编辑失败

任务：

1. 实现 `file_analysis`
2. 细分 Edit 失败原因
3. 接入 file-history/edited_text_file

### Milestone 4: Task / Plan 结构分析

目标：

- 判断任务拆解、阻塞关系和 plan mode 是否真正帮助完成复杂任务

任务：

1. 实现 `task_analysis`
2. 实现 `plan_analysis`
3. 接入 `tasks/`、`plans/` 与 plan/task attachment 事件

### Milestone 5: Skill / Agent 效率分析

目标：

- 判断 skill 与 subagent 是否真的提高效率

任务：

1. 实现 `skill_analysis`
2. 实现 `agent_flow_analysis`
3. 输出 tool chain 与失败链路

### Milestone 6: Runtime / Job 诊断

目标：

- 排查后台 job、daemon、运行态 session 的异常状态

任务：

1. 实现 `runtime_analysis`
2. 接入 `sessions/*.json`、`jobs/*/state.json`、`jobs/*/timeline.jsonl`
3. 在诊断页展示卡住或失败的后台 job

### Milestone 7: 高级诊断与建议

目标：

- 从展示数据升级为给出行动建议

任务：

1. 实现 diagnostics API
2. Dashboard 增加问题列表
3. 输出优化建议和优先级
4. 增加 `debug/`、`telemetry/`、`paste-cache/` 等可选数据源的可用性检测与隐私提示

## 性能路线

已完成：

- 文件级 project aggregate cache
- 紧凑 JSON cache
- `projects/` 专属 rebuild check

后续可选：

1. 全局 aggregate add/subtract
   - 变化文件只 subtract 旧 aggregate，再 add 新 aggregate
   - 避免 merge 全部 3431 个文件快照

2. JSONL offset 级增量
   - 对追加型文件只解析新增行
   - 对大 session 文件收益最大

3. 二进制 cache 格式
   - gob/msgpack/zstd
   - 进一步降低体积和 encode/decode 时间

## 当前推荐下一步

优先做 Milestone 2：

1. `session_analysis`
2. 接入 started/result/title/last-prompt
3. 输出高失败、高成本、长耗时 session

原因：

- 与核心问题最相关
- Milestone 1 已有初版能力
- 下一步需要把底层工具/成本/失败事件整理成 session 级复盘
- 能回答“哪次任务出了问题、为什么、耗时和成本如何”
