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

- cache version `2.1`
- project JSONL 文件级聚合缓存
- 每个文件记录：
  - path
  - size
  - mtime
  - aggregate snapshot
- 未变化文件复用 aggregate
- cache 使用紧凑 JSON
- cache rebuild 只检查 `projects/`

真实数据验证：

- 项目日志文件数：约 `3431`
- 首次 2.1 构建：约 `10.9s`
- 全文件复用构建：约 `0.7s`
- cache 文件体积：约 `17MB`

## 待完善能力

## P0: 高优先级

### 1. Token / 成本分析

现状：

- 目前模型 token 只粗略统计 `input_tokens + output_tokens`
- `budget_usd` 只做 latest/max 摘要

待实现：

- 拆分 token 类型：
  - `input_tokens`
  - `output_tokens`
  - `cache_read_input_tokens`
  - `cache_creation_input_tokens`
  - `server_tool_use`
- 按维度统计 token：
  - model
  - session
  - project
  - tool 前后
  - agent/subagent
- 高成本 session Top N
- 高成本 project Top N
- 不同模型平均输出长度
- cache 命中与成本节省估算
- `budget_usd` 时间线

建议输出：

- `cost_analysis.by_model`
- `cost_analysis.by_project`
- `cost_analysis.by_session`
- `cost_analysis.cache_tokens`
- `cost_analysis.budget_timeline`

价值：

- 直接回答“哪些模型/项目/会话最花钱”
- 可以指导模型选择和上下文压缩策略

### 2. 失败原因细分

现状：

- 当前失败分类较粗：
  - `timeout`
  - `permission`
  - `not_found`
  - `command_failed`
  - `error_text`

待实现：

- Bash 失败：
  - exit code 1/2/126/127/130
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
- Session 成功/失败/中断分类
- 高失败会话 Top N
- 长耗时会话 Top N

建议输出：

- `session_analysis.sessions`
- `session_analysis.top_failures`
- `session_analysis.long_running`
- `session_analysis.outcomes`

价值：

- 把底层事件整理成可读任务摘要
- 后续可做“问题会话复盘页”

### 4. 文件编辑深度分析

现状：

- 已有 Read/Edit/Write/MultiEdit 的路径级统计

待实现：

- 接入：
  - `file-history-snapshot`
  - `edited_text_file`
  - `file-history/`
  - `opened_file_in_ide`
- 每个 session 修改文件数
- 每个 project 修改文件数
- 文件编辑失败原因
- 文件修改规模：
  - lines added
  - lines removed
  - edit attempts
  - retry count
- 高频失败文件 Top N

建议输出：

- `file_analysis.touched_files`
- `file_analysis.edited_files`
- `file_analysis.failed_edits`
- `file_analysis.by_session`

价值：

- 找出最容易出问题的文件和编辑模式
- 支持代码质量与工具使用诊断

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

## P2: 低优先级

### 9. Debug 深层日志分析

现状：

- `debug/` 不参与缓存构建
- MCP 工具统计已从 `projects` 派生

待实现：

- MCP transport 错误
- 原始请求/响应耗时
- SDK/连接错误
- debug 与 tool_result 关联

使用场景：

- 需要排查 MCP server 或网络层问题时再启用
- 不建议默认纳入主缓存热路径

### 10. 诊断页 / 建议页

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

1. 实现 `cost_analysis`
2. 实现 `failure_analysis`
3. Dashboard 增加成本 Top 与失败原因图

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

### Milestone 4: Skill / Agent 效率分析

目标：

- 判断 skill 与 subagent 是否真的提高效率

任务：

1. 实现 `skill_analysis`
2. 实现 `agent_flow_analysis`
3. 输出 tool chain 与失败链路

### Milestone 5: 高级诊断与建议

目标：

- 从展示数据升级为给出行动建议

任务：

1. 实现 diagnostics API
2. Dashboard 增加问题列表
3. 输出优化建议和优先级

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

优先做 Milestone 1：

1. `cost_analysis`
2. `failure_analysis`

原因：

- 与核心问题最相关
- 数据源已经具备
- 能直接解释“哪些模型/工具/会话最贵、最容易失败、为什么失败”
