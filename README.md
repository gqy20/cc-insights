# cc-insights

> AI 驱动的 Claude Code 使用数据分析工具 - 可视化数据、发现模式、获取智能推荐

cc-insights 是一个基于 Go + ECharts 的 Claude Code 使用数据分析工具，通过本地 Web Dashboard 展示使用统计，并支持时间范围筛选、缓存预聚合和并发解析优化。

**核心特点：**
- **单文件部署** - 静态资源嵌入二进制，~2MB 完全便携
- **高性能并发** - Producer-Consumer 模式 + Worker Pool
- **跨平台兼容** - 静态链接构建，无外部依赖
- **缓存加速** - 启动时构建 `cache/cache.db`，API 优先读取预聚合数据
- **AI 分析** - 基于使用数据提供智能洞察和推荐（规划中）

## ✨ 功能特性

- 📊 **可视化图表**
  - 每日活动趋势（折线图）
  - Slash Commands 使用统计（柱状图）
  - MCP 工具调用统计（饼图）
  - 每日会话趋势（折线图）
  - 项目活跃度排名（柱状图）
  - 星期活动分布（柱状图）
  - 模型使用分析（柱状图 + 折线图）
  - 工作时段分布（柱状图）

- ⏱️ **时间范围筛选**
  - 快捷预设：7天、30天、90天、全部
  - 自定义范围：选择起止日期
  - 实时统计信息显示

- 🤖 **AI 智能分析（规划中）**
  - **Commands 推荐** - 基于使用模式推荐可能有用的新命令
  - **MCP 工具推荐** - 根据工作内容推荐合适的 MCP 服务器
  - **效率洞察** - 分析工作习惯，提供优化建议
  - **模式识别** - 发现重复操作、使用高峰等模式

## 📦 快速开始

### 1. 构建

```bash
make build-static-compress  # 静态+UPX压缩（~2MB，默认推荐）
make build-static           # 静态链接（~7MB）
make build                  # 动态链接（~6MB）
```

### 2. 运行

```bash
# 使用默认数据目录（可执行文件所在目录下的 data）
./cc-insights

# 指定数据目录
./cc-insights -data /path/to/data

# 指定监听地址
./cc-insights -addr :9090
```

### 3. 访问

打开浏览器访问:
- 首页: http://localhost:8080
- Dashboard: http://localhost:8080/dashboard
- API: http://localhost:8080/api/data?preset=7d

## 📁 数据目录结构

```
data/
├── history.jsonl        # 命令历史 (2.5MB, 10K+条)
├── stats-cache.json     # 统计缓存 (12.5KB)
├── projects/            # Claude Code 项目会话 JSONL
└── debug/               # Debug 日志目录 (1.1GB, 2848个文件)
```

## 🎨 Dashboard 预览

![Dashboard Preview](docs/dashboard.png)

## 📡 API 接口

### GET /api/data

获取筛选后的数据：

```
GET /api/data?preset=7d
GET /api/data?preset=custom&start=2025-12-01&end=2026-01-08
```

**参数：**
- `preset`: `7d` | `30d` | `90d` | `all` | `custom`
- `start`: 开始日期 (自定义范围时使用，格式: `2025-12-01`)
- `end`: 结束日期 (自定义范围时使用，格式: `2026-01-08`)

**响应：**

```json
{
  "success": true,
  "data": {
    "timestamp": "2026-01-08 16:02:07",
    "time_range": {
      "preset": "7d",
      "start": "2026-01-01",
      "end": "2026-01-08"
    },
    "commands": [
      {"Command": "/tdd", "Count": 115},
      {"Command": "/gh", "Count": 31}
    ],
    "hourly_counts": {
      "10": 53, "11": 44, "15": 137
    },
    "daily_trend": {
      "dates": ["2026-01-02", "2026-01-03"],
      "counts": [7765, 7849]
    },
    "mcp_tools": [
      {"Tool": "search_web", "Server": "jina", "Count": 1543}
    ],
    "sessions": {
      "total_sessions": 89,
      "peak_date": "2026-01-04",
      "peak_count": 23,
      "valley_date": "2026-01-01",
      "valley_count": 2
    },
    "project_stats": {
      "projects": [
        {"project": "/path/to/project", "session_count": 0, "message_count": 892}
      ],
      "total_messages": 15420,
      "total_sessions": 89
    },
    "model_usage": [
      {"model": "claude-sonnet-4.5", "count": 120, "tokens": 450000}
    ]
  }
}
```

### GET /api/stats

旧版 API（保持兼容），返回全部数据。

## 🔧 命令行参数

```
-data <path>    数据目录路径（默认: 可执行文件所在目录/data）
-cache <path>   缓存目录路径（默认: 可执行文件所在目录/cache）
-addr <addr>    监听地址（默认: :8080）
```

## 📊 性能测试结果

### 最近 7 天数据

```
history.jsonl: 0.05s
debug/ 日志:   5.22s
总计:          ~5.3s
```

### 全部数据

```
history.jsonl: 0.05s
debug/ 日志:   4.96s
总计:          ~5.0s
```

**测试环境：** 72核 CPU，2.2G 数据

## 🚀 开发

```bash
# 安装依赖
make deps

# 开发模式运行
make run-dev

# 运行测试
make test

# 性能测试
make bench
make bench-all

# 构建压缩版本
make build-static-compress
```

## 📝 注意事项

1. **数据路径灵活**: 使用 `-data` 参数指定任意数据目录
2. **无需外部数据库**: 直接读取 JSON/JSONL 和日志文件，缓存写入本地 JSON 文件
3. **并发解析**: projects 和 debug 日志使用 worker 并发解析
4. **单文件部署**: 所有依赖编译进单一可执行文件
5. **时间筛选**: 支持多种时间范围选择方式
6. **静态链接**: 默认构建无外部依赖，可在任意 Linux 系统运行
7. **测试状态**: `make test` 使用自包含 fixture，可在没有真实 Claude Code 数据目录的环境中运行

## 🔧 故障排查

### Dashboard 无法启动

```bash
# 检查数据目录
ls -la data/history.jsonl

# 检查端口占用
lsof -i :8080

# 查看日志
./cc-insights -data ../data 2>&1 | tee debug.log
```

### 图表不显示

1. 打开浏览器开发者工具 (F12)
2. 检查 Console 是否有错误
3. 检查 Network 是否有 API 请求失败

### 数据加载慢

```bash
# 运行性能测试
make bench

# 检查并发度
nproc  # Linux 系统CPU核心数
```

## 📄 License

MIT
