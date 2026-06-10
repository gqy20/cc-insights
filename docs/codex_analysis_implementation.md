# Codex 本地日志分析实现方案

本文档基于当前机器的 `~/.codex` 实际目录与 `cc-insights` 现有 Claude Code 解析架构整理，目标是为后续实现 Codex 数据源支持提供可执行的设计依据。

结论：Codex 可以分析，且能复用当前大部分 dashboard、API、缓存、聚合与图表逻辑。主要工作不在前端，而在新增一个 Codex provider/parser，把 `~/.codex/sessions/**/*.jsonl` 映射到现有 `ProjectAggregate`。

## 当前 cc-insights 架构要点

当前项目是针对 Claude Code 本地数据目录实现的 Go 单体 CLI/Web dashboard。

主要入口与解析链路：

- `cmd/insights/config.go`: `-data`、`-cache`、`-addr` 等配置，目前默认数据目录为可执行文件旁的 `data`。
- `cmd/insights/history_parser.go`: 解析 Claude 的 `history.jsonl`，用于 slash command 和小时分布。
- `cmd/insights/project_parser.go`: 解析 Claude 的 `projects/**/*.jsonl`，生成核心 `ProjectAggregate`。
- `cmd/insights/debug_parser.go`: 扫描 Claude 的 `debug/*.txt`，用正则统计 MCP 工具。
- `cmd/insights/cache_builder.go`: 解析项目文件并增量缓存，每个项目 JSONL 文件转成 `ProjectFileCache`。
- `cmd/insights/api.go`: 从缓存或实时解析结果构建 `DashboardData`。
- `cmd/insights/types.go`: 定义对外 API 数据结构和内部聚合结构。

最值得复用的是 `ProjectAggregate`。它已经覆盖以下通用统计：

- 项目活跃度：`ProjectStats`
- 每日活动：`DailyActivity`
- 每日 session 去重：`DailySessions`
- 小时与工作时段：`HourlyCounts`、`WorkHoursStats`
- 模型使用：`ModelUsage`
- 工具调用、成功、失败、缺失结果：`ToolStats`、`ToolModelStats`、`ToolAnalysis`
- 运行事件：`EventTypes`、`EventAnalysis`
- 命令与文件操作：`BashCommandStats`、`FileOperationStats`

Claude parser 当前强绑定 Claude schema：

- 目录：`projects/<project>/*.jsonl`
- 顶层字段：`type=user|assistant|attachment|permission-mode`
- session 字段：`sessionId`
- 项目字段：`cwd`
- assistant 消息：`message.model`、`message.usage.input_tokens/output_tokens`
- 工具调用：`message.content[].type == "tool_use"`
- 工具结果：user message 里的 `message.content[].type == "tool_result"`

Codex 不能直接套这个 schema，需要独立 parser。

## ~/.codex 实际目录观察

`~/.codex` 在本机是符号链接：

```text
/home/qy113/.codex -> workspace/qy113/.codex
```

实现文件遍历时要注意：`find` 默认不跟随目录 symlink。Go 里如果用户传入 `~/.codex`，`filepath.WalkDir` 能进入 symlink 目标的前提是起始路径本身解析后作为目录打开；但如果中间路径包含 symlink，需要测试确认。建议在启动时用 `filepath.EvalSymlinks(dataDir)` 标准化根目录。

当前机器主要内容：

```text
~/.codex/
  auth.json
  config.toml
  history.jsonl
  session_index.jsonl
  sessions/
    2026/02/DD/rollout-*.jsonl
    2026/03/DD/rollout-*.jsonl
    2026/04/DD/rollout-*.jsonl
    2026/05/DD/rollout-*.jsonl
    2026/06/DD/rollout-*.jsonl
  attachments/
  shell_snapshots/
  cache/
  plugins/
  skills/
  *.sqlite
```

文件统计：

```text
total files: 5441
jsonl: 329
json: 428
toml: 2
log: 0
```

session 文件统计：

```text
~/.codex/sessions/**/*.jsonl: 326
2026-02: 32
2026-03: 155
2026-04: 17
2026-05: 78
2026-06: 44
```

除 `sessions/**/*.jsonl` 外，还有两个轻量索引/历史文件：

- `history.jsonl`: 每行字段为 `session_id`、`text`、`ts`。适合做用户输入历史、首条 prompt、命令前缀辅助统计，但缺少 `cwd`、model、tool、usage。
- `session_index.jsonl`: 每行字段为 `id`、`thread_name`、`updated_at`。适合 session 列表标题或索引，不足以做主分析。

主数据源应选 `sessions/**/*.jsonl`。

## Codex Session JSONL Schema

Codex session JSONL 的顶层结构与 Claude 不同。典型行：

```json
{
  "timestamp": "2026-02-08T18:37:55.926Z",
  "type": "session_meta",
  "payload": {}
}
```

全量 session 事件类型统计：

```text
185797 response_item
116890 event_msg
  8524 turn_context
   502 session_meta
   197 compacted
```

注意：session 文件数为 326，但 `session_meta` 有 502 条，说明部分文件可能出现多段元信息或重复元信息。session 计数不要简单累计 `session_meta` 行数，应以 `payload.id` 去重，缺失时以文件路径作为 fallback session id。

### session_meta

用于识别 session、项目目录、客户端来源和 Git 信息。

已观察字段：

```json
{
  "timestamp": "2026-02-08T18:37:55.926Z",
  "type": "session_meta",
  "payload": {
    "id": "019c3e8b-cf03-7f13-8c7c-0cfb4effb50e",
    "timestamp": "2026-02-08T18:37:55.843Z",
    "cwd": "/mnt/chestnut/chestnut/docs/01_thesis_proposal",
    "originator": "codex_cli_rs",
    "cli_version": "0.98.0",
    "source": "cli",
    "model_provider": "openai",
    "git": {
      "commit_hash": "...",
      "branch": "main",
      "repository_url": "..."
    }
  }
}
```

建议映射：

- `session_id`: `payload.id`
- `project`: `payload.cwd`
- `session_created_at`: 优先 `payload.timestamp`，否则顶层 `timestamp`
- `event_type`: `session_meta`
- 可选扩展维度：`originator`、`source`、`cli_version`、`git.branch`

### turn_context

用于识别每个 turn 的模型、cwd、权限和 sandbox 信息。

已观察字段：

```json
{
  "timestamp": "2026-02-08T18:38:19.389Z",
  "type": "turn_context",
  "payload": {
    "cwd": "/mnt/chestnut/chestnut/docs/01_thesis_proposal",
    "model": "gpt-5.3-codex",
    "effort": "medium",
    "approval_policy": "never",
    "sandbox_policy": {"type": "danger-full-access"},
    "collaboration_mode": {"mode": "default"},
    "personality": "pragmatic",
    "summary": "..."
  }
}
```

全量模型分布：

```text
3482 gpt-5.4
2179 gpt-5.5
1923 gpt-5.3-codex
 936 gpt-5.2-codex
   4 gpt-5.2
```

建议映射：

- 当前模型状态：读到 `turn_context.payload.model` 后保存为 `currentModel`
- 当前项目状态：如果 `session_meta.cwd` 缺失，可用 `turn_context.payload.cwd`
- 事件统计：`turn_context`
- 权限模式类统计可从 `approval_policy`、`sandbox_policy.type` 扩展
- 模型调用次数：可以按 `turn_context` 数量近似统计 turn 次数，也可以按 `token_count.info.last_token_usage` 统计实际模型响应次数

### response_item

这是 Codex 的响应项，包含 assistant/user/developer message、reasoning、工具调用、工具输出等。

全量 `response_item.payload.type` 分布：

```text
53689 function_call
53682 function_call_output
36045 message
24728 reasoning
 8408 custom_tool_call
 8407 custom_tool_call_output
  836 web_search_call
    1 tool_search_call
    1 tool_search_output
```

#### message

字段形态：

```json
{
  "type": "response_item",
  "payload": {
    "type": "message",
    "role": "assistant",
    "content": []
  }
}
```

建议映射：

- `role == "assistant"`: 计入 assistant message / daily activity
- `role == "user"`: 可选计入 user message，或只用于会话上下文，不建议把系统/developer 指令计入用户活跃度
- `role == "developer"`: 不计入用户活跃度，避免把启动指令/环境上下文当成真实工作

当前 dashboard 的 `MessageCount` 原来是 Claude assistant 消息数。Codex 里建议保持同义：只计 assistant message 或 agent message，不计 developer/system。

#### function_call

字段形态：

```json
{
  "type": "response_item",
  "payload": {
    "type": "function_call",
    "name": "exec_command",
    "arguments": "{\"cmd\":\"pwd && ls -la\"}",
    "call_id": "call_DG0WaWjGfS3N0oOA58T2Hi8k"
  }
}
```

全量普通 function call 工具排行：

```text
42157 exec_command
10135 write_stdin
  556 view_image
  494 update_plan
   68 mcp__article-mcp__get_article_details
   41 mcp__github__pull_request_read
   36 mcp__github__get_file_contents
   27 search_literature
   26 mcp__article_mcp__search_literature
   23 mcp__github__search_code
   20 mcp__article-mcp__search_literature
   18 mcp__article_mcp__get_article_details
   13 mcp__github__search_issues
   11 mcp__github__search_repositories
   10 search_text
   10 resolve_library_id
   10 mcp__crawl_mcp__search_text
```

建议映射：

- `Tool`: `payload.name`
- `Tool ID`: `payload.call_id`
- `Input`: `payload.arguments` 是 JSON 字符串，需要二次 `json.Unmarshal`
- `Model`: 当前 `currentModel`
- `Project`: 当前 `cwd`
- `SessionID`: 当前 session id
- `Timestamp`: 顶层 `timestamp`

#### function_call_output

字段形态：

```json
{
  "type": "response_item",
  "payload": {
    "type": "function_call_output",
    "call_id": "call_DG0WaWjGfS3N0oOA58T2Hi8k",
    "output": "Chunk ID: ... Process exited with code 0 ..."
  }
}
```

建议映射：

- 通过 `call_id` 关联前面的 function call
- 作为工具结果兜底来源
- 成功失败不要优先从文本 `output` 正则判断，Codex 有更结构化的 `event_msg` 结果事件

#### custom_tool_call

全量统计显示目前主要是：

```text
8410 apply_patch
```

建议映射：

- 和 `function_call` 同样计入工具调用
- `Tool`: `payload.name`，若缺失则使用 `custom:<call_id>` 或 `custom_tool_call`
- 结果用 `custom_tool_call_output` 或结构化 `patch_apply_end` 关联

#### web_search_call / tool_search_call

建议映射：

- `web_search_call`: 计为 `web_search`
- `tool_search_call`: 计为 `tool_search`
- 如果没有 call id 或输出不稳定，可只统计调用次数，不强行计算成功率

### event_msg

`event_msg` 是 Codex 比 Claude 更有价值的结构化运行事件来源。

全量 `event_msg.payload.type` 分布：

```text
61114 token_count
28554 agent_message
 6188 user_message
 5533 task_started
 5143 task_complete
 4187 patch_apply_end
 3089 exec_command_end
 2011 agent_reasoning
  397 turn_aborted
  356 web_search_end
  197 context_compacted
   83 mcp_tool_call_end
   16 view_image_tool_call
   16 error
    4 thread_name_updated
    2 thread_rolled_back
```

#### token_count

字段形态：

```json
{
  "type": "event_msg",
  "payload": {
    "type": "token_count",
    "info": {
      "total_token_usage": {
        "input_tokens": 33661,
        "cached_input_tokens": 19200,
        "output_tokens": 644,
        "reasoning_output_tokens": 229,
        "total_tokens": 34305
      },
      "last_token_usage": {
        "input_tokens": 17977,
        "cached_input_tokens": 15744,
        "output_tokens": 402,
        "reasoning_output_tokens": 156,
        "total_tokens": 18379
      },
      "model_context_window": 258400
    },
    "rate_limits": {}
  }
}
```

建议用法：

- 用 `last_token_usage` 累加模型 token，避免 `total_token_usage` 重复累计。
- 如果 `last_token_usage` 缺失，不要退回累计 `total_token_usage` 直接累加；最多记录最新累计值作为 session summary。
- 现有 `ModelUsageItem.Tokens` 只有总 token，可暂时写入 `last_token_usage.total_tokens`。
- 后续可扩展 `ModelUsageItem`，拆分 input/output/cached/reasoning。

#### exec_command_end

字段集合：

```text
aggregated_output,call_id,command,cwd,duration,exit_code,formatted_output,parsed_cmd,process_id,source,status,stderr,stdout,turn_id,type
```

全量状态：

```text
2929 completed
 160 failed
```

建议映射：

- 通过 `call_id` 关联 `exec_command`
- `status == "completed"` 且 `exit_code == 0`: success
- `status == "failed"` 或 `exit_code != 0`: failure
- `command` 或 `parsed_cmd` 可用于 `BashCommandStats`
- `cwd` 可补当前 project
- `duration` 可作为未来性能分析维度

这比从 `function_call_output.output` 文本中正则判断更可靠。

#### patch_apply_end

字段集合：

```text
call_id,changes,status,stderr,stdout,success,turn_id,type
```

全量成功状态：

```text
4189 true
```

建议映射：

- 通过 `call_id` 关联 `apply_patch`
- `success == true`: success
- `success == false` 或 `status` 非成功值: failure
- `changes` 可用于未来文件修改统计

#### mcp_tool_call_end

字段集合：

```text
call_id,duration,invocation,result,type
```

建议映射：

- 通过 `call_id` 关联 MCP 工具调用
- `invocation` 里通常能拿到工具名/服务名，实际实现时需为该字段加 RawMessage 解析
- `result` 里若有错误字段则判失败，否则判成功
- 如果无法确定工具名，回退到前序 `function_call.payload.name`

#### task_started / task_complete / turn_aborted

`task_complete` 字段集合：

```text
last_agent_message,turn_id,type
completed_at,duration_ms,last_agent_message,time_to_first_token_ms,turn_id,type
completed_at,duration_ms,last_agent_message,turn_id,type
```

`turn_aborted` 字段集合：

```text
completed_at,duration_ms,reason,turn_id,type
reason,turn_id,type
reason,type
```

建议映射：

- `task_started`: turn/session 活跃事件
- `task_complete`: 成功完成 turn，可用于完成率/时延分析
- `turn_aborted`: 失败/中断事件，`reason` 计入 `ToolFailureKinds` 或新增 turn outcome 分析

## 推荐数据源优先级

第一优先级：`~/.codex/sessions/**/*.jsonl`

- 拥有 session、cwd、模型、工具、token、事件、成功失败信号
- 能支撑绝大多数 dashboard

第二优先级：`~/.codex/history.jsonl`

- 拥有 `session_id`、`text`、`ts`
- 可辅助 slash-like command、用户 prompt 次数、首条 prompt
- 不能替代 session JSONL

第三优先级：`~/.codex/session_index.jsonl`

- 拥有 `id`、`thread_name`、`updated_at`
- 适合 session 标题与列表展示
- 不能做主统计

暂不建议第一版读取：

- `auth.json`: 敏感认证信息
- `*.sqlite`: schema 未确认且不是必需
- `attachments/`: 可能包含用户粘贴内容
- `shell_snapshots/`: 可后续用于 shell 环境分析，但第一版不需要
- `plugins/skills/cache`: 属于配置/能力库，不是使用日志主数据

## 与现有 ProjectAggregate 的映射表

| 现有统计 | Claude 来源 | Codex 来源 | 第一版建议 |
| --- | --- | --- | --- |
| session 数 | `sessionId` | `session_meta.payload.id` 或文件路径 | 支持 |
| 项目活跃度 | `cwd` | `session_meta.payload.cwd` / `turn_context.payload.cwd` | 支持 |
| 每日活动 | assistant message timestamp | `response_item.message(role=assistant)` 或 `event_msg.agent_message` | 支持 |
| 小时分布 | assistant timestamp | 同每日活动 timestamp | 支持 |
| 工作时段 | 小时分布 | 同小时分布 | 支持 |
| 模型使用次数 | `message.model` | `turn_context.payload.model` 或 token_count 关联模型 | 支持 |
| token 用量 | `message.usage` | `event_msg.token_count.info.last_token_usage` | 支持，先总 token |
| 工具调用 | `tool_use` | `function_call`、`custom_tool_call`、`web_search_call` | 支持 |
| 工具结果 | `tool_result` | `function_call_output`、`custom_tool_call_output`、结构化 end events | 支持 |
| 工具失败 | 正则/`is_error` | `exec_command_end.status/exit_code`、`patch_apply_end.success`、`turn_aborted` | 支持，更可靠 |
| Bash 命令 | Bash tool input | `exec_command.arguments.cmd` 或 `exec_command_end.command` | 支持 |
| 文件操作 | Read/Edit/Write 工具 | `apply_patch`、部分工具 arguments | 部分支持 |
| MCP 工具 | Claude debug 正则和 tool_use | `mcp__...` function name、`mcp_tool_call_end` | 支持 |
| hooks/skills/permission mode | Claude attachment | Codex 无直接等价，approval/sandbox 可部分替代 | 第一版隐藏或改名 |
| agent/subagent | `isSidechain`、`agentId` | 暂未见稳定 agent id，只有 agent_message/reasoning | 第一版弱化 |
| slash commands | Claude history display `/...` | Codex history text 可能包含 `/...`，但不保证 | 可选 |

## 实现设计

### 1. 增加 Provider 概念

新增配置：

```go
type Provider string

const (
    ProviderClaude Provider = "claude"
    ProviderCodex  Provider = "codex"
    ProviderAuto   Provider = "auto"
)
```

CLI 参数建议：

```text
-provider claude|codex|auto
-data <path>
```

默认行为建议：

- 如果用户显式传 `-provider codex`，默认 data dir 可为 `~/.codex`。
- 如果用户显式传 `-provider claude`，沿用当前行为。
- 如果 `-provider auto`：
  - 存在 `projects/` 且有 `history.jsonl` 或 `stats-cache.json`: Claude
  - 存在 `sessions/` 且有 `sessions/**/*.jsonl`: Codex
  - 都存在时优先用户参数，否则报错提示明确指定

注意 cache 路径应包含 provider，避免 Claude/Codex 缓存混用：

```text
cache/claude/cache.db
cache/codex/cache.db
```

当前缓存文件名虽然叫 `cache.db`，实际是 JSON cache。实现时不必改格式，但应避免 provider 之间共享同一个 cache。

### 2. 抽象文件发现与解析

当前 Claude 路径在 `listProjectJSONLFileInfos(dataDir)` 里固定为 `dataDir/projects`。

建议拆成 provider-specific file lister：

```go
type LogFileInfo struct {
    RelPath string
    AbsPath string
    Size    int64
    ModTime int64
}

func listLogFileInfos(provider Provider, dataDir string) ([]LogFileInfo, error)
```

Claude:

```text
<dataDir>/projects/**/*.jsonl
```

Codex:

```text
<dataDir>/sessions/**/*.jsonl
```

如果 `dataDir` 是 symlink，先尝试：

```go
resolved, err := filepath.EvalSymlinks(dataDir)
if err == nil {
    dataDir = resolved
}
```

### 3. 新增 Codex Parser

建议新增文件：

```text
cmd/insights/codex_types.go
cmd/insights/codex_parser.go
cmd/insights/codex_parser_test.go
```

核心入口：

```go
func ParseCodexSessionsConcurrentOnceFromDir(tf TimeFilter, dataDir string) (*ProjectAggregate, error)
func parseCodexSessionFileAggregate(filePath string, tf TimeFilter, agg *ProjectAggregate)
```

内部状态：

```go
type codexSessionState struct {
    SessionID    string
    Project      string
    CurrentModel string
    StartedAt    time.Time
    PendingTools map[string]pendingToolCall
}
```

处理顺序：

1. 顺序读取单个 JSONL 文件，保持 `currentModel`、`sessionID`、`project` 状态。
2. 顶层 timestamp 解析为 event time。
3. 若时间过滤不命中，跳过该事件；但 `session_meta` 和 `turn_context` 可先读取状态，再对统计行为过滤。
4. `session_meta`: 设置 session/project，记录 event type。
5. `turn_context`: 更新 model/project，记录 model turn count 或仅作为状态。
6. `response_item.message(role=assistant)`: 计入 daily/project/hourly/weekday message。
7. `response_item.function_call/custom_tool_call/web_search_call/tool_search_call`: 创建 pending tool，增加 call count。
8. `response_item.function_call_output/custom_tool_call_output`: 如果没有结构化结果，兜底判断 success/failure。
9. `event_msg.token_count`: 使用 `last_token_usage.total_tokens` 累加到当前 model。
10. `event_msg.exec_command_end/patch_apply_end/mcp_tool_call_end`: 结构化更新工具成功/失败。
11. 文件结束后仍在 pending 的工具记为 missing result。

### 4. 工具结果判定优先级

建议按可靠性排序：

1. `event_msg.exec_command_end`
   - `status == "completed"` 且 `exit_code == 0`: success
   - `status == "failed"` 或 `exit_code != 0`: failure
2. `event_msg.patch_apply_end`
   - `success == true`: success
   - `success == false`: failure
3. `event_msg.mcp_tool_call_end`
   - `result` 有明确 error 字段: failure
   - 否则 success
4. `response_item.function_call_output`
   - 文本中包含 `Process exited with code 0`: success
   - 文本中包含 `Process exited with code N` 且 N 非 0: failure
   - 文本中包含 error/failed/timeout 等关键词: failure
   - 否则 success 或 unknown
5. 文件结束仍未匹配结果: missing

需要避免重复计数：同一个 `call_id` 可能同时出现 `function_call_output` 和 `exec_command_end`。建议 pending tool 增加状态：

```go
type codexPendingTool struct {
    pendingToolCall
    ResultRecorded bool
    WeakResultSeen bool
}
```

如果先看到弱结果 `function_call_output`，可以暂存 preview，但不立刻最终计 success；等结构化 end event 出现后再最终计数。若文件结束仍没有结构化 end event，再用弱结果兜底。

### 5. 命令和文件操作分析

`exec_command` 的 arguments 是 JSON 字符串：

```json
{"cmd":"pwd && ls -la"}
```

处理建议：

- 对 `payload.arguments` 二次 unmarshal。
- 取 `cmd` 字段作为命令样例。
- 复用现有 `recordStructuredToolInputLocked` 或抽出公共函数。
- `exec_command_end.command` 可作为结果侧兜底。

`apply_patch` 可统计为文件操作：

- `Tool`: `apply_patch`
- `Operation`: `patch`
- `changes` 字段若可解析出文件路径，记录到 `FileOperationStats`
- 第一版可只计 call/success/failure，不解析 patch 详情

### 6. ModelUsage 的 token 语义

现有：

```go
type ModelUsageItem struct {
    Model  string `json:"model"`
    Count  int    `json:"count"`
    Tokens int    `json:"tokens"`
}
```

Codex 第一版建议：

- `Count`: `turn_context` 中该 model 出现次数，或有 `last_token_usage` 的 token_count 次数。
- `Tokens`: 累加 `last_token_usage.total_tokens`。

更准确的后续增强：

```go
type ModelUsageItem struct {
    Model                 string `json:"model"`
    Count                 int    `json:"count"`
    Tokens                int    `json:"tokens"`
    InputTokens           int    `json:"input_tokens,omitempty"`
    CachedInputTokens     int    `json:"cached_input_tokens,omitempty"`
    OutputTokens          int    `json:"output_tokens,omitempty"`
    ReasoningOutputTokens int    `json:"reasoning_output_tokens,omitempty"`
}
```

为保持 API 兼容，新增字段用 `omitempty`。

## MVP 范围建议

第一版应实现：

- `-provider codex`
- 读取 `~/.codex/sessions/**/*.jsonl`
- session 总数与每日 session
- 每日 activity、小时分布、工作时段
- project stats
- model usage，含 token 总量
- tool analysis，支持 `function_call`、`custom_tool_call`、`web_search_call`
- exec command 成功/失败
- apply_patch 成功/失败
- MCP 工具名统计
- cache 增量构建
- API 与 dashboard 复用现有字段

第一版暂缓：

- SQLite 数据库读取
- attachments 内容分析
- shell snapshot 分析
- 完整 agent/subagent 分析
- session 详情页
- 对话内容展示
- 成本估算
- 插件/skill 安装库统计

## 测试策略

不要依赖真实 `~/.codex` 测试。新增 fixture：

```text
testdata/codex/
  sessions/2026/06/10/rollout-2026-06-10T10-00-00-test.jsonl
  history.jsonl
  session_index.jsonl
```

最小 fixture 应覆盖：

1. `session_meta` 提供 `id/cwd/timestamp`
2. `turn_context` 提供 `model`
3. assistant message 计入 daily/project/hourly
4. `function_call exec_command` + `exec_command_end completed exit_code 0`
5. `function_call exec_command` + `exec_command_end failed exit_code 1`
6. `custom_tool_call apply_patch` + `patch_apply_end success true`
7. `token_count.info.last_token_usage.total_tokens`
8. 文件结束 pending tool 计 missing result
9. 时间过滤能排除范围外事件
10. 多 session 文件能正确按 session id 去重

建议测试函数：

```go
func TestParseCodexSessionFileAggregate(t *testing.T)
func TestParseCodexSessionsWithFilter(t *testing.T)
func TestCodexToolStructuredResultsDoNotDoubleCount(t *testing.T)
func TestBuildFullCacheCodexProvider(t *testing.T)
func TestDetectProvider(t *testing.T)
```

关键断言：

- `TotalSessions == fixture session 数`
- `DailyActivity[date]` 与 assistant message 数一致
- `ModelUsage["gpt-5.5"].Tokens == sum(last_token_usage.total_tokens)`
- `ToolStats["exec_command"].CallCount == success + failure + missing`
- `ToolStats["exec_command"].FailureCount` 不因 `function_call_output` + `exec_command_end` 双重计数
- `ToolStats["apply_patch"].SuccessCount` 来自 `patch_apply_end.success`

## 实施步骤

建议按以下顺序落地，降低破坏现有 Claude 功能的风险：

1. 增加 provider 配置与自动检测，但默认仍为 Claude，确保现有测试不变。
2. 抽象文件发现函数，让 cache builder 能按 provider 获取 JSONL 文件列表。
3. 新增 Codex types 与单文件 parser，不接入 API，先用单元测试验证 aggregate。
4. 接入 `ParseProjectsConcurrentOnceFromDir` 等调用点，按 provider 分发到 Claude/Codex parser。
5. 接入 cache builder，确保 Codex 文件也能增量缓存。
6. 调整启动日志、页面标题和 API 返回，显示当前 provider。
7. 补充 README：`./cc-insights -provider codex -data ~/.codex`。
8. 跑全量测试，手动用真实 `~/.codex` 启动 dashboard 验证性能与图表。

## 需要注意的隐私与安全

Codex session JSONL 里包含用户输入、assistant 回复、命令输出、路径、仓库地址，甚至可能包含环境片段。实现 dashboard 时应遵循当前项目的做法：统计聚合为主，不展示完整对话和完整工具输出。

建议：

- failure sample 只保留短 preview，沿用现有 `previewString` 限制长度。
- 不读取 `auth.json`。
- 不默认读取 attachments。
- 不在 API 返回完整 `function_call.arguments` 或 `function_call_output.output`。
- 对命令样例保留截断版本。

## 最终判断

Codex 支持不是一次“大改前端”，而是一次“新增 provider/parser + 复用聚合模型”的后端扩展。当前 `ProjectAggregate` 抽象已经足够通用，Codex 的结构化事件甚至比 Claude debug 文本更适合做工具成功率、命令失败率和 token 用量分析。

第一版实现后，`cc-insights` 可以从 Claude Code 专用工具升级为通用 code agent 本地使用分析工具。后续再考虑重命名产品文案，比如从 `Claude Code Dashboard` 改成 `Code Agent Insights` 或 `CC/Codex Insights`。
