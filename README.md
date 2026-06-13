# cc-insights

> 面向 Claude Code 的使用诊断工具：把历史数据变成可解释的证据、判断和改进方向

cc-insights 是一个 Claude Code 使用诊断 CLI，默认输出可读摘要，也提供本地 Web Dashboard 展示使用统计，并支持时间范围筛选、缓存预聚合和并发解析优化。

## 核心理念

cc-insights 的目标不是简单展示“用了多少 token、跑了多少命令”，而是帮助人和 AI 看懂 Claude Code 的真实工作方式，并据此优化项目协作方式。

核心问题是：

- Claude Code 经常失败在什么地方？
- 是环境、路径、依赖、权限、网络、工具参数，还是任务拆分方式的问题？
- 哪些项目、session、命令族和工具调用消耗了最多时间与 token？
- 应该优化 `CLAUDE.md`、hooks、MCP 配置，还是应该把高频动作封装成更稳定的工具？

因此 CLI 命令保持收敛：

- `rec` 负责诊断、解释和给出下一步方向。
- `why`、`cmd`、`tok`、`ses` 负责证据下钻。
- `sum` 负责全局概览。
- `web` 负责本地可视化。

不会为每一种解释继续增加新命令。新的分析能力优先进入 `rec` 的诊断解释层，原始证据则通过少量稳定的下钻命令查看。这样既方便人阅读，也方便 AI 用 JSON / Markdown 输出继续分析。

**核心特点：**
- **单文件部署** - 静态资源嵌入二进制，完全便携
- **高性能并发** - Producer-Consumer 模式 + Worker Pool
- **跨平台兼容** - 静态链接构建，无外部依赖
- **缓存加速** - 启动时构建 `~/.cc-insights/cache/cache.db`，API 优先读取预聚合数据
- **AI 友好** - 支持 JSON / Markdown / Table 输出，便于 AI 直接调用分析结果
- **诊断优先** - `rec` 输出异常、证据、解释、排查方向和可执行下钻命令

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

- 🤖 **AI 友好 CLI**
  - `sum` - 快速概览 Claude Code 使用情况
  - `err` - 输出失败原因、失败工具和模型组合
  - `why` - 按原因、工具、模型、项目或 Session 下钻失败样例
  - `tok` - 输出 Token、模型、项目和会话消耗
  - `cmd` - 输出 Bash 命令族、具体命令和高风险命令
  - `ses` - 输出 Session 生命周期、长会话、高失败会话和 Plan/Task 信号
  - `rec` - 输出诊断结论、证据、解释、下一步排查方向和下钻命令
  - `web` - 启动 Web Dashboard
  - 支持 `--format json|markdown|table`

## 📦 快速开始

### 1. 构建

```bash
make build  # 静态链接构建；如果本机安装了 UPX，会自动压缩
```

Windows 原生 PowerShell/cmd 环境可直接使用 Go 构建：

```powershell
go build -trimpath -tags=prod -o cc-insights.exe ./cmd/insights
```

### 2. 运行

```bash
# 默认输出最近 30 天 CLI 摘要
./cc-insights

# 启动 Web Dashboard
./cc-insights web

# 指定数据目录
./cc-insights --data /path/to/data

# 指定监听地址
./cc-insights web --addr :9090

# 给 AI 使用的 JSON 输出
./cc-insights err -p 7d -j
./cc-insights why -p 7d --reason error_text -n 20 -j
./cc-insights tok -p 30d -j
./cc-insights cmd -p 30d -j
./cc-insights ses -p 30d -j
./cc-insights rec -p 30d -j
```

### 3. 访问

打开浏览器访问:
- 首页: http://localhost:8932
- Dashboard: http://localhost:8932/dashboard
- API: http://localhost:8932/api/data?preset=7d

## 📁 数据目录结构

```
~/.claude/
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

## 🔧 CLI 用法

```
cc-insights
cc-insights err -p 7d -j
cc-insights why -p 7d --reason error_text -n 20 -j
cc-insights tok -p 30d -j
cc-insights cmd -p 30d -j
cc-insights ses -p 30d --session SESSION_ID -j
cc-insights rec -p 30d -j
cc-insights web [--addr :8932]

命令：
sum  总览
err  失败来源
why  失败样例下钻
tok  Token 与成本
cmd  Bash 命令分析
ses  Session 生命周期
rec  诊断、解释与下钻命令
web  Web Dashboard

--data <path>     数据目录路径（默认: ~/.claude）
--cache <path>    缓存目录路径（默认: ~/.cc-insights/cache）
--rules <path>    Bash 命令分类规则（默认内置 rules/bash.yml；也会读取 ~/.cc-insights/bash.yml）
-p, --preset      时间范围：24h、7d、30d、90d、all
--start <date>    自定义开始日期（YYYY-MM-DD）
--end <date>      自定义结束日期（YYYY-MM-DD）
-f, --format      输出格式：table、json、markdown
-j                输出 JSON
-m                输出 Markdown
-n                Top N 数量；对 why 表示样例数量
--limit <n>       Top N 数量
--samples <n>     下钻样例数量
--reason <value>  按失败 reason 过滤
--category <val>  按失败 category 过滤
--tool <name>     按工具名过滤
--model <name>    按模型名过滤
--project <text>  按项目路径片段过滤
--session <id>    按 Session ID 过滤
--addr <addr>     Web 监听地址（默认: :8932）
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
# 开发模式运行
make run-dev

# 运行测试
make test

# 性能测试
make bench
make bench-all

# 构建版本
make build

# 多平台发布包（Linux/macOS/Windows）
make release
```

## 📝 注意事项

1. **数据路径灵活**: 使用 `-data` 参数指定任意数据目录
2. **无需外部数据库**: 直接读取 JSON/JSONL 和日志文件，缓存写入本地 JSON 文件
3. **并发解析**: projects 和 debug 日志使用 worker 并发解析
4. **单文件部署**: 所有依赖编译进单一可执行文件
5. **时间筛选**: 支持多种时间范围选择方式
6. **静态链接**: 默认构建无外部依赖，Release 会产出 Linux、macOS 和 Windows 包
7. **测试状态**: `make test` 使用自包含 fixture，可在没有真实 Claude Code 数据目录的环境中运行

## 🔧 故障排查

### Dashboard 无法启动

```bash
# 检查数据目录
ls -la ~/.claude/history.jsonl

# 检查端口占用
lsof -i :8932

# 查看日志
./cc-insights -data ~/.claude 2>&1 | tee debug.log
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

## 🚢 Release 发布

GitHub Actions 的 Release workflow 会在打 `v*` tag 时构建并发布以下平台包：

- Linux: `amd64`, `arm64`
- macOS: Intel (`amd64`), Apple Silicon (`arm64`)
- Windows: `amd64`

本地 `make release` 也能生成同样的发布包，但它依赖 Unix shell 工具。Windows 本机建议使用 Git Bash、MSYS2 或 WSL 执行，普通用户直接下载 GitHub Release 产物即可。

## 📄 License

MIT
