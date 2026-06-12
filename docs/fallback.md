# Fallback 行为梳理

本文档整理当前项目中显式或隐式的 fallback / graceful degradation / 兼容路径。范围包括后端 API、缓存、解析器、CLI 和前端展示。

## 总览

项目的主要降级策略是：优先使用预聚合缓存；缓存不可用时实时解析；单个数据源失败时返回空结果或部分结果；单条坏记录不阻断整个文件解析；前端无数据时展示空态。

这些 fallback 大多是为了保证 Dashboard 和 CLI 在本地 Claude Code 数据不完整、缓存损坏、某些目录缺失或单条 JSONL 损坏时仍能工作。

## 当前 `~/.claude` 数据观察

以下数据基于当前机器的 `~/.claude` 只读聚合统计，不包含任何正文内容。

### 数据规模

- `history.jsonl`: 23289 行
- `projects/**/*.jsonl`: 3433 个文件，281087 行
- `tasks/**/*.json`: 826 个文件
- `debug/*.txt`: 0 个文件
- `~/.claude/stats-cache.json`: 存在且 JSON 有效
- `~/.cc-insights/cache/cache.db`: 存在且 JSON 有效，版本 `2.9`

### 数据质量

- `history.jsonl`:
  - 坏 JSON 行数：0
  - 坏 timestamp：0
  - slash command 行数：1322
- `projects/**/*.jsonl`:
  - assistant 消息：124296
  - user 消息：62576
  - attachment 事件：27138
  - assistant message JSON 解析失败：0
  - assistant model 为空：0
  - assistant timestamp 缺失：0
  - assistant cwd 为空：0
- `tasks/**/*.json`:
  - `completed`: 573
  - `pending`: 241
  - `in_progress`: 12
  - 未知 status：0

### 实际触发较多的 fallback 信号

- `projects` 中有 46434 条记录缺失 timestamp，但都集中在非 assistant/user 的运行时事件类型，例如 `last-prompt`、`permission-mode`、`file-history-snapshot`、`ai-title`、`mode` 等。
- `projects` 中有 56982 条记录缺失 cwd，也集中在上述运行时事件类型；assistant 记录没有 cwd 缺失。
- tool 调用对齐问题存在但比例较低：
  - `tool_use`: 52991
  - `tool_result`: 52955
  - orphan `tool_result`: 153
  - 文件结束时仍 pending 的 `tool_use`: 189
  - 有 pending tool 的文件数：126
  - 单文件最大 pending 数：9
- `debug/*.txt` 为 0，因此旧 debug 日志解析路径在当前数据上没有贡献。

### 基于真实数据的判断

- `history` 和 `tasks` 的容错主要是防御性代码，目前真实数据没有触发坏 JSON 或未知状态。
- `tool_result` orphan / missing fallback 应保留，它确实覆盖了少量真实不对齐数据，且比例低，适合作为质量信号而不是错误。
- `Unknown project` 对 assistant 统计不是问题，因为当前 assistant 记录没有空 cwd；大量空 cwd 来自运行时事件，应避免误判为核心消息质量问题。
- `debug` 目录 fallback 可以弱化或标记 legacy，因为当前主数据已经主要来自 `projects`。
- 对未知 preset 回退全量数据与真实数据质量无关，已清理为显式错误。

## 缓存到实时解析

### API 数据源选择

- 位置：`cmd/insights/api.go`
- 入口：`buildDashboardData`
- 触发条件：
  - `globalCache == nil`
  - 或 `buildDataFromCache` 返回错误
- 行为：
  - 有缓存时优先走 `buildDataFromCache`
  - 缓存读取失败时记录 `Warn("缓存读取失败，降级到实时解析")`
  - 随后调用 `buildDataFromParsing`
- 影响：
  - API 仍返回数据，但耗时会变长
  - 响应数据来源从 `cache` 变为 `parsing`

### Web / CLI 初始化缓存失败

- 位置：`cmd/insights/main.go`, `cmd/insights/cli.go`
- 入口：
  - `runWebServer`
  - `prepareCLIData`
- 触发条件：
  - `initializeCache()` 返回错误，例如缓存目录不可写、缓存构建失败
- 行为：
  - 记录 `Warn("缓存初始化失败，将使用实时解析模式")`
  - 不终止进程
  - 后续请求通过实时解析兜底
- 影响：
  - Dashboard / CLI 可继续使用
  - 首次请求可能较慢

### 缓存重建判定

- 位置：`cmd/insights/cache_builder.go`
- 入口：`CacheBuilder.NeedsRebuild`
- 触发条件与行为：
  - 缓存文件不存在：返回 `true`
  - 缓存文件无法加载或 JSON 损坏：返回 `true`
  - 缓存版本不等于 `CacheVersion`：返回 `true`
  - 无法获取数据最后修改时间：返回 `true`
  - 数据文件比缓存新：返回 `true`
- 影响：
  - 采取保守策略，尽量重建缓存而不是使用可能过期或损坏的数据

### 增量更新回退完整构建

- 位置：`cmd/insights/cache_builder.go`
- 入口：`CacheBuilder.IncrementalUpdate`
- 触发条件：
  - `LoadCacheFile` 失败
  - 或检测到数据已更新
- 行为：
  - 缓存不存在时执行 `BuildFullCache`
  - 数据已更新时也执行 `BuildFullCache`
- 影响：
  - 当前“增量更新”本质上回退为完整重建
  - 逻辑简单可靠，但大数据目录下成本较高

### 文件级缓存复用与重解析

- 位置：`cmd/insights/cache_builder.go`
- 入口：`buildProjectAggregateIncremental`
- 触发条件：
  - 旧缓存中存在同一路径文件缓存
  - 且文件大小、修改时间一致
- 行为：
  - 命中时复用 `ProjectFileCache`
  - 未命中时重新解析该 JSONL 文件
  - 缓存版本变化时丢弃旧缓存，全部重解析
- 影响：
  - 这是当前真正生效的文件级增量 fallback

## 数据源级容错

### 实时解析返回部分数据

- 位置：`cmd/insights/api.go`
- 入口：`buildDataFromParsing`
- 数据源：
  - `history.jsonl`
  - `projects/**/*.jsonl`
  - `debug/*.txt`
  - `tasks/*/*.json`
- 行为：
  - 四类数据源并行解析
  - 每个数据源走 `safeParse*` 包装
  - 单个数据源失败时使用空结果，不让整体 API 失败
- 影响：
  - Dashboard 可能部分图表为空
  - API 仍返回 `success: true`

### history 解析失败

- 位置：`cmd/insights/api.go`, `cmd/insights/concurrent.go`
- 入口：`safeParseHistoryConcurrent`
- 触发条件：
  - `history.jsonl` 不存在、不可读或解析入口返回错误
- 行为：
  - 记录 warn
  - 返回空命令列表和空小时统计 map
- 影响：
  - Slash Commands、命令调用和部分 hourly 数据为空

### projects 解析失败

- 位置：`cmd/insights/api.go`, `cmd/insights/project_parser.go`
- 入口：`safeParseProjectsOnce`
- 触发条件：
  - `projects` 目录不存在或不可读
  - `ParseProjectsConcurrentOnce` 返回错误
- 行为：
  - 记录 warn
  - 返回 `emptyProjectAggregate`
- 影响：
  - 项目、模型、运行时、成本、失败、会话等依赖 projects 的图表为空
  - Dashboard 不会 panic

### debug 日志解析失败

- 位置：`cmd/insights/api.go`, `cmd/insights/concurrent.go`
- 入口：`safeParseDebugLogs`
- 触发条件：
  - `debug` 目录不存在或不可读
- 行为：
  - `debug` 目录不存在时直接返回空 MCP 工具列表
  - `debug` 目录存在但不可读时经 `safeParseDebugLogs` 记录 warn 并返回空 MCP 工具列表
  - 返回空 MCP 工具列表
- 影响：
  - 旧版 MCP debug 统计为空
  - projects 中解析出的 runtime tool analysis 不受该 debug 目录影响

### tasks 目录解析失败

- 位置：`cmd/insights/api.go`, `cmd/insights/task_parser.go`, `cmd/insights/cache_builder.go`
- 入口：
  - `safeParseTasksOnce`
  - `ParseTasksConcurrentFromDir`
  - `buildProjectAggregateIncremental`
- 触发条件：
  - `tasks` 目录不存在
  - task 目录不可读
  - task JSON 解析失败
- 行为：
  - `tasks` 目录不存在时返回空 `TaskAnalysisData`
  - 单个 session task 目录读失败时跳过
  - 单个 task JSON 损坏时跳过
  - 缓存构建中 tasks 扫描失败只记录 warn
  - 缓存构建会使用 `CacheBuilder.DataDir` 对应的 `tasks/`，不再回退到全局 `cfg.DataDir`
- 影响：
  - Task / Plan 中 task 子模块为空
  - 不影响其他 Dashboard 数据

### session 统计提取失败或无 aggregate

- 位置：`cmd/insights/api.go`, `cmd/insights/session_parser.go`
- 入口：
  - `extractSessionStatsFromAggregate`
  - `safeParseSessionStats`
- 触发条件：
  - aggregate 为 nil
  - aggregate 中没有 `DailySessions`
  - session 解析函数返回错误
- 行为：
  - 返回 `TotalSessions: 0` 和空 `DailySessionMap`
- 影响：
  - 会话统计为空，但 API 不失败

## 记录级容错

### JSONL 单条坏记录跳过

- 位置：`cmd/insights/history_parser.go`, `cmd/insights/concurrent.go`, `cmd/insights/project_parser.go`, `cmd/insights/cache_builder.go`
- 触发条件：
  - JSON decoder 读到非 EOF 错误
  - 单条 record 的 `message` 无法 unmarshal
  - timestamp 无法解析
- 行为：
  - 跳过当前记录
  - 继续解析后续记录
- 影响：
  - 局部数据缺失
  - 不阻断整个文件或请求

### 项目文件读失败跳过

- 位置：`cmd/insights/project_parser.go`, `cmd/insights/cache_builder.go`
- 触发条件：
  - `os.Open(filePath)` 失败
  - `WalkDir` 遇到单个路径错误
  - 单个项目目录读取失败
- 行为：
  - 跳过对应文件或目录
  - 继续处理其他项目文件
- 影响：
  - 某些项目统计缺失
  - 整体解析继续

### debug 文件时间提取失败

- 位置：`cmd/insights/concurrent.go`
- 入口：`ParseDebugLogsConcurrentFromDir`
- 触发条件：
  - 无法从 debug 文件第一行提取时间戳
- 行为：
  - 使用文件修改时间 `ModTime` 作为后备时间
- 影响：
  - 时间过滤仍可执行
  - 精度取决于文件修改时间

### debug 单文件打开失败跳过

- 位置：`cmd/insights/debug_parser.go`, `cmd/insights/concurrent.go`
- 入口：
  - `parseDebugFile`
  - `parseDebugFileOptimized`
- 触发条件：
  - 单个 `.txt` 文件无法打开
- 行为：
  - 直接返回
  - 不影响其他 debug 文件

### 缺失 tool_result 计为 missing

- 位置：`cmd/insights/project_parser.go`, `cmd/insights/parser.go`
- 触发条件：
  - 文件解析结束后 `pendingTools` 仍有未匹配的 `tool_use`
- 行为：
  - 调用 `addMissingToolResultLocked`
  - 统计 `MissingResultCount`
  - 工具性能中记录 `durationMs=-1`
- 影响：
  - 不当作解析失败
  - 作为质量信号进入 Tool / Session / Agent 分析

### tool_result 无前序 tool_use

- 位置：`cmd/insights/parser.go`
- 入口：`parseToolResults`
- 触发条件：
  - 遇到 `tool_result`，但 `pendingTools` 中找不到对应 `tool_use_id`
- 行为：
  - 构造一个 fallback `pendingToolCall`
  - `Tool` 和 `Model` 设为 `unknown`
  - 继续分类与统计结果
- 影响：
  - 失败/成功结果不会丢失
  - 工具名和模型维度会归入 `unknown`

## 字段兼容与默认值

### 时间范围 preset 默认 all / 30d

- 位置：`cmd/insights/api.go`, `cmd/insights/filter.go`, `cmd/insights/cli.go`
- 行为：
  - API 未传 `preset/start/end` 时使用全部数据
  - API 传入未知 `preset` 时返回错误，不再回退为全部数据
  - CLI 默认 `--preset 30d`
  - CLI 明确传不支持的 preset 会返回错误
- 影响：
  - API 和 CLI 对显式未知 preset 均采用显式错误

### 项目名默认 Unknown

- 位置：`cmd/insights/project_parser.go`, `cmd/insights/cost_analysis.go`
- 触发条件：
  - record 中 `cwd` 或 project 为空
- 行为：
  - 项目维度使用 `Unknown`
- 影响：
  - 避免空 key
  - 无法还原真实项目路径

### agent 默认 main / sidechain:unknown

- 位置：`cmd/insights/parser.go`, `cmd/insights/cost_analysis.go`
- 触发条件：
  - `agentID` 为空
- 行为：
  - 主链默认 `main`
  - sidechain 默认 `sidechain:unknown`
- 影响：
  - Agent 分析和成本分析保持可聚合

### tool / model / failure reason 默认 unknown

- 位置：`cmd/insights/parser.go`
- 触发条件：
  - 工具名、模型名、失败分类字段为空
- 行为：
  - 使用 `unknown`
- 影响：
  - 维度不会丢失，但会聚合到未知桶

### session 字段多来源兼容

- 位置：`cmd/insights/session_analysis.go`
- 行为：
  - `last-prompt`、`ai-title`、`custom-title`、`queue-operation`、`permission-mode`、`mode`、`result` 等事件会依次从顶层字段、`content`、`message` 中取值
  - 取不到时回退为 `unknown` 或跳过
- 影响：
  - 兼容不同 Claude Code 日志结构
  - 避免字段迁移导致统计完全缺失

### session title 优先级

- 位置：`cmd/insights/session_analysis.go`
- 行为：
  - `custom-title` 优先于其他 title
  - 无 title 时可使用 `last-prompt` 预览作为标题
- 影响：
  - 展示层尽量有可读标题

### assistant usage 字段兼容

- 位置：`cmd/insights/types.go`
- 行为：
  - `Usage` 同时支持常见 token 字段和嵌套 `server_tool_use`
  - 新增字段通过 `omitempty` 保持 API 兼容
- 影响：
  - 不同 Claude Code 版本的 usage 结构更容易被解析

## API 兼容

### `/api/stats` 旧响应格式

- 位置：`cmd/insights/main.go`, `cmd/insights/api.go`
- 入口：`statsAPIHandler`
- 行为：
  - 复用 `/api/data` 的缓存优先数据管道
  - 输出旧版顶层字段，而不是 `{success,data}` 包装
- 影响：
  - 保持老客户端兼容
  - 新逻辑和旧接口共享同一数据源

### 请求超时保护

- 位置：`cmd/insights/api.go`
- 入口：`handleDataAPI`
- 触发条件：
  - 数据处理超过 60 秒
  - 或客户端上下文取消
- 行为：
  - 返回 HTTP 408 和 JSON 错误
- 影响：
  - 防止长请求无限占用连接
  - 不是数据 fallback，但属于服务可用性保护

## CLI 兼容与默认值

### 默认命令

- 位置：`cmd/insights/cli.go`
- 行为：
  - 无命令或首个参数是 flag 时默认执行 `sum`
- 影响：
  - `cc-insights -p 7d` 等价于 summary 查询

### 命令别名 / 简写

- 位置：`cmd/insights/cli.go`
- 当前命令：
  - `sum`
  - `err`
  - `why`
  - `tok`
  - `web`
- 行为：
  - `-p` 兼容 `--preset`
  - `-f` 兼容 `--format`
  - `-j` 强制 JSON
  - `-m` 强制 Markdown
  - `-n` 同时作为 Top N / 样例数量
- 影响：
  - 提供短命令体验
  - 非支持命令会直接报错，不再静默 fallback

### CLI 参数默认值

- 位置：`cmd/insights/cli.go`
- 默认值：
  - `Preset: 30d`
  - `Format: table`
  - `Limit: 10`
  - `Samples: Limit`
- 行为：
  - limit 或 samples 小于等于 0 时回退默认值
- 影响：
  - 避免空输出或负数切片风险

### CLI 输出格式

- 位置：`cmd/insights/cli.go`
- 行为：
  - 支持 `table`、`json`、`markdown` / `md`
  - 未知格式返回错误
- 影响：
  - 输出格式不做静默降级，避免用户误读

## 前端展示降级

### 空图表状态

- 位置：`cmd/insights/static/app_core.js`
- 入口：
  - `renderCharts`
  - `renderEmptyChart`
  - `collapseEmptyCharts`
- 触发条件：
  - 某个 chart definition 的 `hasData(data)` 返回 false
  - 或图表没有渲染出 canvas/svg 且 insight 文案包含“暂无”
- 行为：
  - 给 chart wrapper 加 `empty-chart`
  - 使用图表定义中的 `emptyText`
  - 可标记 `healthy-empty`
- 影响：
  - 某些数据源为空时页面保持完整
  - 用户看到明确空态，而不是空白图表

### Summary 卡片默认值

- 位置：`cmd/insights/static/app_core.js`
- 行为：
  - 缺少 `project_stats` 时用 `daily_trend.counts` 求总消息数
  - 缺少 `tool_analysis` 时用 `mcp_tools` 求工具调用数
  - 缺少模型、项目、失败率等字段时显示 `-`
- 影响：
  - 支持部分 API 数据结构
  - 缓存/实时解析返回字段不完整时仍能展示概要

## 非 fallback 的硬失败

以下场景当前不会降级为可用结果：

- 数据根目录 `cfg.DataDir` 不存在：
  - Web / CLI 都会返回错误并停止当前启动或命令
- API 未知 preset 或自定义时间范围格式错误：
  - 返回错误响应
- CLI 自定义时间范围只传 `--start` 或只传 `--end`：
  - 返回错误
- CLI preset 不在 `24h|7d|30d|90d|all`：
  - 返回错误
- CLI 输出格式不在 `table|json|markdown|md`：
  - 返回错误
- `projects` 目录不存在时：
  - 直接调用 `ParseProjectsConcurrentOnce` 会返回错误
  - 但 API/CLI 数据管道经 `safeParseProjectsOnce` 会转为空结果

## 风险与改进建议

1. `buildDataFromParsing` 假设 `aggregate` 非 nil。当前通过 `safeParseProjectsOnce` 保证这一点；后续改动时应保留这个约束。
2. `debug` 文件时间提取失败回退 `ModTime`，可能导致时间范围过滤与真实事件时间不一致。
3. 单条 JSONL 解析错误目前静默跳过。对排查数据质量有帮助的话，可以增加抽样计数或 debug 日志。
4. 大量未知字段会归到 `unknown`，保证稳定但可能掩盖上游 schema 变化。建议在一致性检查或 debug 模式中统计 unknown 比例。
