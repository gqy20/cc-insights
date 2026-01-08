# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Code Dashboard 是一个基于 Go + go-echarts 的单文件可执行程序，用于可视化 Claude Code 的使用数据。它读取 Claude Code 的数据目录（history.jsonl、stats-cache.json、projects/*.jsonl、debug/日志），生成交互式图表展示使用统计。

**核心特点：**
- **单文件部署** - 所有静态资源（HTML/JS）通过 Go embed 嵌入二进制，部署时只需一个 `cc-insights` 文件
- **静态链接 + UPX 压缩** - 默认构建方式，~2MB 完全便携，无外部依赖
- **高性能并发解析** - Producer-Consumer 模式 + Worker Pool，支持大规模数据快速处理
- **一次遍历聚合** - `ParseProjectsConcurrentOnce` 函数在单次遍历中同时获取项目、日期、小时、星期、模型使用等所有统计，大幅提升性能
- **缓存系统** - `CacheBuilder` 支持预聚合数据持久化，可提升重复加载性能

## Build Commands

**默认构建（推荐）：静态链接 + UPX压缩**

```bash
# 默认构建：静态链接 + UPX压缩（~2MB，无外部依赖，推荐用于所有场景）
make build-static-compress
```

**其他构建选项：**

```bash
# 开发运行（使用上级目录的data，快速迭代）
make run-dev

# 运行测试
make test

# 性能测试
make bench          # 最近7天
make bench-all      # 全部数据

# 其他构建方式（一般不需要）
make build          # 标准构建（动态链接，~6MB）
make build-static   # 静态构建（无UPX压缩，~7MB）

# 多平台发布
make release-static     # 静态链接版本（容器/任意Linux）
make release            # 动态链接版本
```

## Architecture

### 代码组织

所有代码在 `cmd/insights/` 目录下，使用单个 `main` 包（非标准Go项目布局，简化依赖管理）。

**核心模块：**

| 文件 | 职责 |
|------|------|
| `main.go` | HTTP服务器、路由、HTML模板、静态资源embed |
| `config.go` | 命令行参数解析、配置管理 |
| `filter.go` | 时间范围过滤（7d/30d/90d/all/custom） |
| `parser.go` | 数据解析（history.jsonl、stats-cache.json、debug日志、projects/*.jsonl） |
| `concurrent.go` | 并发解析优化（Producer-Consumer、Worker Pool） |
| `api.go` | API接口（/api/data、/api/stats） |
| `charts.go` | go-echarts图表生成 |
| `cache.go` | 缓存文件结构和时间范围查询 |
| `cache_builder.go` | 缓存构建器（完整构建、增量更新、过期检查） |
| `benchmark_main.go` | 性能测试工具 |

### 数据流

```
HTTP请求 → handleDataAPI (api.go)
    ↓
解析参数创建 TimeFilter (filter.go)
    ↓
ParseHistoryConcurrent (concurrent.go)
    ├─ Producer: 读取history.jsonl → 批量(1000条/批) → channel
    └─ Consumer: Worker pool并发解析 → 时间过滤 → 聚合
    ↓
ParseProjectsConcurrentOnce (parser.go)
    └─ 一次遍历获取项目、日期、小时、星期、模型等所有统计
    ↓
ParseStatsCacheWithFilter (parser.go)
    └─ 读取stats-cache.json → 时间过滤 → 提取每日活动
    ↓
ParseDebugLogsConcurrent (parser.go)
    └─ FilterDebugFiles预过滤 → 信号量控制并发 → 正则匹配MCP工具调用
    ↓
DashboardData组装 → JSON响应
```

### 并发处理模型

**history.jsonl解析** (`concurrent.go`):
- Producer-Consumer模式，1000条/批
- Worker数量 = CPU核心数/2
- 每个worker本地聚合，最后合并减少锁竞争

**debug日志解析** (`parser.go`):
- 信号量控制：CPU核心数×4，最多64个并发
- 预过滤：解析前根据文件修改时间过滤（节省50%+解析时间）
- 大缓冲区扫描：64KB-1MB

### 时间过滤系统

`filter.go` 定义了统一的 `TimeFilter` 结构，支持：

```go
type TimeFilter struct {
    Start *time.Time  // nil表示无限制
    End   *time.Time  // nil表示无限制
}

// 预设范围
NewTimeFilterFromPreset(Range7Days|Range30Days|Range90Days|RangeAll)

// 自定义范围
NewTimeFilterCustom("2025-12-01", "2026-01-08")

// 过滤检查
tf.Contains(time)  // bool
```

所有解析函数都接受 `TimeFilter` 参数，在解析时同步过滤，避免解析后二次过滤。

### 单文件部署实现

`main.go` 中使用 Go embed 嵌入静态资源：

```go
//go:embed static/*
var staticFS embed.FS

// 使用嵌入的文件系统
staticSub, _ := fs.Sub(staticFS, "static")
http.Handle("/static/", http.FileServer(http.FS(staticSub)))
```

**注意**: `static/` 目录必须在 `cmd/insights/` 下，因为 `//go:embed` 路径相对于源文件。

### 数据结构

**输入数据格式**:
- `history.jsonl`: JSONL格式，每行一个 `HistoryRecord`（包含timestamp、display、project等）
- `stats-cache.json`: JSON格式，包含 `DailyActivity[]`、`ModelUsage`、`HourCounts`
- `projects/<uuid>/*.jsonl`: 项目级别的对话记录，包含完整的消息内容和token使用信息
- `debug/*.log`: 文本日志，文件名格式包含时间戳，包含MCP工具调用记录

**缓存系统** (`cache.go`, `cache_builder.go`):
- `CacheFile` 结构定义了缓存文件格式，支持版本控制和过期检查
- `CacheBuilder` 提供完整构建和增量更新功能
- `TimeRange.Contains()` 方法用于时间范围查询和过滤
- 缓存数据包含：每日统计（`DayAggregate`）、小时统计（`HourAggregate`）、项目统计、模型使用、MCP工具统计
- `NeedsRebuild()` 检查缓存是否需要重建（基于文件修改时间）

**输出API** (`api.go`):
```json
{
  "success": true,
  "data": {
    "timestamp": "2026-01-08 16:02:07",
    "time_range": {"preset": "7d", "start": "...", "end": "..."},
    "commands": [{"Command": "/tdd", "Count": 115}],
    "hourly_counts": {"10": 53, "11": 44},
    "daily_trend": {"dates": [...], "counts": [...]},
    "mcp_tools": [{"Tool": "search_web", "Server": "jina", "Count": 1543}],
    "sessions": {...}  // 可选
  }
}
```

### 前端交互

HTML硬编码在 `main.go` 的handler中（`indexHandler`、`dashboardPageHandler`），JavaScript通过 `/static/app.js` 提供。

前端使用AJAX调用 `/api/data`，通过 `preset` 或 `start/end` 参数控制时间范围，图表使用ECharts（CDN加载）。

## Common Tasks

### 添加新的API端点

1. 在 `main.go` 的路由中添加 `http.HandleFunc`
2. 在 `api.go` 中实现handler函数
3. 使用 `TimeFilter` 支持时间过滤

### 添加新的数据解析

1. 在 `parser.go` 中添加解析函数，接受 `TimeFilter` 参数
2. 如需高性能，在 `concurrent.go` 中实现并发版本
3. 在 `api.go` 中调用并返回JSON

### 修改图表

1. 在 `charts.go` 中修改或添加 `Create*Chart` 函数
2. 使用 go-echarts v2 API
3. 在 `api.go` 中调用并返回JSON

### 性能优化

当前已实现：
- 批量处理（1000条/批）
- 并发解析（Worker Pool、信号量控制）
- 预过滤（文件时间过滤）

如需进一步优化，参考 `benchmark_main.go` 进行性能测试。

## Testing

```bash
# 运行所有测试
make test

# 运行单个测试
go test -v ./cmd/insights -run TestParseHistory

# 运行特定测试
go test -v ./cmd/insights -run TestParseProjectsConcurrentOnce

# 性能测试
make bench          # 使用Range7Days
make bench-all      # 修改为RangeAll
```

**测试覆盖的关键功能**:
- `TestParseProjectStats` - 项目统计解析
- `TestParseProjectsConcurrentOnce` - 一次遍历聚合功能（数据一致性验证）
- `TestProjectStatsByWeekday` - 星期分布统计
- `TestParseDailyActivityFromProjects` - 每日活动数据生成
- `TestParseHourlyCountsFromProjects` - 24小时统计
- `TestParseModelUsageFromProjects` - 模型使用统计
- `TestParseWorkHoursStats` - 工作时段分析

**缓存系统测试**:
- `TestCacheSaveLoad` - 缓存文件保存和加载
- `TestCacheExpiration` - 缓存过期检查
- `TestCacheTimeRangeQuery` - 时间范围查询
- `TestCacheBuilderNeedsRebuild` - 重建条件检查

## Static Linking

**本项目使用静态链接作为默认构建方式**，通过 `CGO_ENABLED=0` 构建完全无外部依赖的二进制文件，配合 UPX 压缩实现最小体积。

```bash
make build-static-compress  # 默认推荐：静态+UPX（~2MB）
```

**优势：**
- 无外部依赖（不需要 glibc）
- 可在任意 Linux 系统运行（包括 Alpine、容器、musl libc）
- 体积小（~2MB），易于分发
- 启动稍慢但运行时性能无差异

**其他场景：**
```bash
make build-static           # 静态链接无压缩（~7MB，开发调试）
make release-static         # 多平台静态版本（跨平台发布）
```

## Development Workflow

### 命令行参数

```bash
-data <path>    # 指定数据目录（默认: ./data）
-addr <addr>    # 指定监听地址（默认: :8080）
```

### 开发模式运行
使用 `make run-dev` 可以快速迭代开发，它会使用上级目录的 `data` 目录：
```bash
make run-dev    # 相当于 go run -tags=prod ./cmd/insights -data ../data
```

### 静态资源开发

开发时如果需要修改 `static/app.js`：
- 修改完成后使用 `make build` 重新嵌入
- 或使用 `-tags !prod` 直接运行（二进制会读取本地 static/ 目录）

### Build Tags 说明
- `-tags=prod`: 启用静态资源嵌入（生产模式）
- `-tags !prod`: 禁用静态资源嵌入（开发/测试模式，加速编译）
- Makefile 中 `make build` 和 `make run-dev` 使用 `prod` tag
- `make bench` 使用 `!prod` tag 避免嵌入静态文件

### 添加新的统计维度

当需要添加新的统计功能时，优先考虑在 `ParseProjectsConcurrentOnce` 中添加，以保持一次遍历的性能优势：

1. 在 `ProjectAggregate` 结构中添加新字段（parser.go:109-122）
2. 在 `parseProjectFileAggregate` 函数中添加统计逻辑（parser.go:1213-1291）
3. 在 `finalize` 方法中生成输出格式数据（parser.go:1294-1374）
4. 在 `api.go` 的 `handleDataAPI` 中将数据加入响应
5. 在前端 `static/app.js` 中添加图表渲染

### 代码组织原则

- **单包结构** - 所有代码在 `cmd/insights/` 单个 `main` 包中，简化依赖管理
- **文件按职责分离** - 虽然是单包，但按功能模块组织文件
- **时间过滤统一** - 所有解析函数都接受 `TimeFilter` 参数，在解析时同步过滤
- **并发优先** - 大量数据解析使用并发模式（Producer-Consumer、Worker Pool、信号量控制）

### 缓存系统设计

**缓存文件结构** (`cache.go`):
- `CacheFile` 结构包含版本、时间范围、预聚合数据
- `DayAggregate` 包含每日的详细统计（消息数、会话数、工具调用数、小时分布）
- `TimeRange.Contains()` 统一时间范围检查逻辑
- `QueryByTimeRange()` 支持按时间范围查询缓存数据

**缓存构建器** (`cache_builder.go`):
- `BuildFullCache()` 完整构建缓存（遍历所有数据源）
- `IncrementalUpdate()` 增量更新（当前简化为完整重建）
- `NeedsRebuild()` 基于文件修改时间检查缓存有效性
- `GetLastDataModified()` 递归扫描数据目录获取最新修改时间

**缓存与实时数据的选择**:
- 当前实现主要使用实时解析（`ParseProjectsConcurrentOnce`）
- 缓存系统预留了接口，可用于未来性能优化场景

### 关键设计模式

**一次遍历聚合** (`parser.go:1142-1374`):
- `ProjectAggregate` 结构包含所有统计数据的中间格式
- `parseProjectFileAggregate` 在单个文件遍历中更新所有统计
- `finalize` 方法将中间格式转换为输出格式
- 避免多次遍历同一文件，性能提升显著

**并发解析** (`concurrent.go`):
- history.jsonl: Producer-Consumer 模式，批量处理（1000条/批），Worker数量 = CPU核心数/2
- debug日志: 信号量控制，CPU核心数×4，最多64个并发，预过滤减少50%+解析时间

**时间过滤** (`filter.go`):
- `TimeFilter` 结构支持 nil 表示无限制
- `Contains` 方法统一时间范围检查
- `FilterDebugFiles` 预过滤避免解析无用文件

## Key Data Structures

### HistoryRecord (parser.go:18-24)
来自 `history.jsonl`，包含用户操作历史：
```go
type HistoryRecord struct {
    Display        string            // 用户输入的命令或消息
    PastedContents map[string]string // 粘贴的内容
    Timestamp      int64             // 毫秒时间戳
    Project        string            // 所属项目路径
}
```

### ProjectRecord (parser.go:124-137)
来自 `projects/<uuid>/*.jsonl`，包含完整的对话记录：
```go
type ProjectRecord struct {
    ParentUUID  string          // 父会话UUID
    Cwd         string          // 当前工作目录（作为项目名）
    SessionID   string          // 会话ID
    Type        string          // "user" | "assistant"
    Message     json.RawMessage // 消息内容（原始JSON）
    Timestamp   string          // RFC3339Nano格式时间戳
}
```

### AssistantMessage (parser.go:139-154)
解析后的assistant消息，包含模型使用信息：
```go
type AssistantMessage struct {
    Model   string // 模型名称，如 "glm-4.6"
    Content []struct {
        Type string // "text"
        Text string
    }
    Usage struct {
        InputTokens          int // 输入token数
        OutputTokens         int // 输出token数
        CacheReadInputTokens int // 缓存读取的token数
    }
}
```

### ProjectAggregate (parser.go:108-122)
一次遍历获取的所有统计数据聚合：
```go
type ProjectAggregate struct {
    ProjectStats      map[string]*ProjectStatItem // 项目统计
    DailyActivity     map[string]int              // 每日活动（日期 -> 消息数）
    HourlyCounts      [24]int                     // 24小时统计
    WeekdayData       [7]WeekdayItem              // 星期统计（0=周一）
    ModelUsage        map[string]*ModelUsageItem  // 模型使用统计
    // ... 还有输出格式字段
}
```

### CacheFile (cache.go:11-28)
缓存文件结构，用于持久化预聚合数据：
```go
type CacheFile struct {
    Version    string                  // 缓存格式版本
    LastUpdate time.Time               // 最后缓存时间戳
    TimeRange  TimeRange               // 缓存覆盖的时间范围
    DailyStats  map[string]*DayAggregate // "2026-01-08" -> 当天所有统计
    HourlyStats [24]*HourAggregate     // 每小时统计
    TotalMessages int                  // 总消息数
    TotalSessions int                  // 总会话数
    ProjectStats  map[string]*ProjectStatItem
    ModelUsage    map[string]*ModelUsageItem
    WeekdayStats  [7]*WeekdayItem
    MCPToolStats  map[string]int
}
```

## Performance Considerations

### 当前性能指标（72核CPU，2.2GB数据）
- 最近7天: history.jsonl ~0.05s, debug日志 ~5.22s
- 全部数据: history.jsonl ~0.05s, debug日志 ~4.96s

### 性能优化要点
1. **批量处理** - history.jsonl使用1000条/批减少channel开销
2. **本地聚合** - 每个worker先本地聚合，最后合并减少锁竞争
3. **预过滤** - debug日志在解析前根据文件时间过滤
4. **大缓冲区** - debug文件扫描使用64KB-1MB缓冲区
5. **一次遍历** - `ParseProjectsConcurrentOnce` 避免重复读取文件

### 性能瓶颈识别
- 使用 `make bench` 运行性能测试
- `benchmark_main.go` 提供了基准测试框架
- 主要瓶颈通常在debug日志解析（文件数量多，~2848个文件）

## Roadmap & Future Work

详见 `docs/dashboard_roadmap.md`，主要扩展方向：

**Phase 2: 核心扩展**（已部分实现）
- ✅ 项目活跃度排名 - `ParseProjectStatsWithFilter`
- ✅ 模型使用统计 - `ParseModelUsageFromProjects`
- ✅ 会话统计 - `ParseSessionStatsWithFilter`
- ✅ 星期分布统计 - `ParseProjectStatsByWeekday`
- ✅ 工作时段分析 - `ParseWorkHoursStats`

**Phase 3: 增强功能**
- Agent使用统计 - 需解析history.jsonl中的agent调用记录
- 工具调用密度分析 - toolCallCount/messageCount比率
- 活动峰值/低谷分析

**Phase 4: 高级功能**
- Shell命令统计 - 解析shell-snapshots/目录
- Token效率分析 - 基于dailyModelTokens数据
- 缓存系统集成 - 使用CacheBuilder提升首次加载性能

## Project Metadata

- **Go 版本**: 1.21+
- **主要依赖**: `github.com/go-echarts/go-echarts/v2 v2.3.3`
- **模块名**: `cc-insights`
- **二进制名**: `cc-insights` (原名 `cc-dashboard`)
- **数据目录**: 默认 `./data`，可通过 `-data` 参数指定
- **监听地址**: 默认 `:8080`，可通过 `-addr` 参数指定
- **缓存目录**: 默认 `./cache`，用于存放预聚合数据
