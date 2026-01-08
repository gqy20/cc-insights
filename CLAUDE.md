# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Code Dashboard 是一个基于 Go + go-echarts 的单文件可执行程序，用于可视化 Claude Code 的使用数据。它读取 Claude Code 的数据目录（history.jsonl、stats-cache.json、debug/日志），生成交互式图表展示使用统计。

**核心特点：**
- **单文件部署** - 所有静态资源（HTML/JS）通过 Go embed 嵌入二进制，部署时只需一个 `cc-dashboard` 文件
- **静态链接 + UPX 压缩** - 默认构建方式，~2MB 完全便携，无外部依赖
- **高性能并发解析** - Producer-Consumer 模式 + Worker Pool，支持大规模数据快速处理

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

所有代码在 `cmd/dashboard/` 目录下，使用单个 `main` 包（非标准Go项目布局，简化依赖管理）。

**核心模块：**

| 文件 | 职责 |
|------|------|
| `main.go` | HTTP服务器、路由、HTML模板、静态资源embed |
| `config.go` | 命令行参数解析、配置管理 |
| `filter.go` | 时间范围过滤（7d/30d/90d/all/custom） |
| `parser.go` | 数据解析（history.jsonl、stats-cache.json、debug日志） |
| `concurrent.go` | 并发解析优化（Producer-Consumer、Worker Pool） |
| `api.go` | API接口（/api/data、/api/stats） |
| `charts.go` | go-echarts图表生成 |
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

**注意**: `static/` 目录必须在 `cmd/dashboard/` 下，因为 `//go:embed` 路径相对于源文件。

### 数据结构

**输入数据格式**:
- `history.jsonl`: JSONL格式，每行一个 `HistoryRecord`（包含timestamp、display等）
- `stats-cache.json`: JSON格式，包含 `DailyActivity[]`、`ModelUsage`、`HourCounts`
- `debug/*.log`: 文本日志，文件名格式包含时间戳

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
go test -v ./cmd/dashboard -run TestParseHistory

# 性能测试
make bench          # 使用Range7Days
make bench-all      # 修改为RangeAll
```

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
