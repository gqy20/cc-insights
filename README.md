<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/logo/cc-insights-dark.svg" />
    <img src="docs/logo/cc-insights.svg" width="100" height="100" alt="cc-insights" />
  </picture>
</p>

<h1 align="center">cc-insights</h1>

<p align="center">面向 Claude Code 的使用诊断工具：把历史数据变成可解释的证据、判断和改进方向</p>

<p align="center">
  <a href="https://github.com/gqy20/cc-insights/actions/workflows/ci.yml"><img src="https://github.com/gqy20/cc-insights/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/gqy20/cc-insights/releases"><img src="https://img.shields.io/github/v/release/gqy20/cc-insights" alt="Release" /></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white" alt="Go" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT" /></a>
</p>

cc-insights 是一个 Claude Code 使用诊断 CLI：读取历史数据、项目会话、工具调用、失败样例、Token 消耗和 Session 生命周期，整理成**可解释的证据、诊断结论和改进方向**。默认输出可读摘要，也提供本地 Web Dashboard，并支持时间范围筛选、缓存预聚合和并发解析。

## 目录

- [为什么用 cc-insights](#为什么用-cc-insights)
- [快速开始](#快速开始)
- [命令速查](#命令速查)
- [与 AI 协作](#与-ai-协作)
- [配置与规则](#配置与规则)
- [Dashboard](#dashboard)
- [开发](#开发)
- [故障排查](#故障排查)
- [文档](#文档)

## 为什么用 cc-insights

cc-insights 的目标不是展示「用了多少 token、跑了多少命令」，而是回答**为什么 Claude Code 变慢、变贵或频繁失败**，以及**下一步该优化哪里**。

| 传统方式（翻 JSONL / 看终端日志） | 用 cc-insights |
|----------|-------------|
| 手动 grep `~/.claude` 找失败 | `cc-insights rec -p 7d` → 给根因候选、证据摘要和下钻命令 |
| 数不清 Token 花在哪 | `cc-insights tok -p 30d -j` → 按项目 / 模型 / 会话拆解 |
| Bash 命令频繁失败，不知是哪段 | `cc-insights cmd -p 30d` → 按命令族 + 链式逐段 + 高风险命令 |
| 长会话失控难发现 | `cc-insights ses` → 找长会话 / 高失败会话 / Plan·Task 信号 |
| 想看可视化要自己搭 | `cc-insights web` → 单二进制本地起 Dashboard |
| 让 AI 帮分析但数据是乱的 | 所有命令 `-j` 输出结构化 JSON → AI 直接消费 |

**核心设计原则：**

- **诊断优先** — `rec` 先给判断，再给证据和可执行下钻命令；不为每种解释继续堆命令
- **AI 友好** — JSON / Markdown / Table 三种输出，AI 直接解析继续分析
- **单文件部署** — 静态资源内嵌二进制（UPX 后约 2.7MB），无外部依赖、无数据库
- **规则可配置** — Bash 分类与诊断阈值用 YAML 配置，无需改代码
- **缓存加速** — 启动时构建 `~/.cc-insights/cache/`，API 优先读预聚合数据

## 快速开始

### 1. 安装

**下载预编译二进制（推荐，无需 Go / Node 环境）**

从 [Releases](https://github.com/gqy20/cc-insights/releases) 下载对应平台包，解压后放到 `~/.local/bin`：

| 平台 | 文件 |
|------|------|
| macOS · Apple Silicon | `cc-insights_<ver>_darwin_arm64.tar.gz` |
| macOS · Intel | `cc-insights_<ver>_darwin_amd64.tar.gz` |
| Linux · amd64 | `cc-insights_<ver>_linux_amd64.tar.gz` |
| Linux · arm64 | `cc-insights_<ver>_linux_arm64.tar.gz` |
| Windows · amd64 | `cc-insights_<ver>_windows_amd64.zip` / `.exe` |

macOS / Linux 命令行获取最新版并安装到 `~/.local/bin`：

```bash
VERSION=$(curl -fsSL https://api.github.com/repos/gqy20/cc-insights/releases/latest | grep -m1 '"tag_name"' | cut -d'"' -f4)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')   # darwin / linux
ARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
curl -fsSL -o /tmp/cc-insights.tar.gz \
  "https://github.com/gqy20/cc-insights/releases/download/${VERSION}/cc-insights_${VERSION}_${OS}_${ARCH}.tar.gz"
mkdir -p ~/.local/bin && tar xzf /tmp/cc-insights.tar.gz -C ~/.local/bin --strip-components=1 "cc-insights_${VERSION}_${OS}_${ARCH}/cc-insights"
chmod +x ~/.local/bin/cc-insights
```

Windows 直接下载 `cc-insights_<ver>_windows_amd64.exe`，放到 `~/.local/bin`（或任意 PATH 目录）；也可下 `.zip`（含 README/LICENSE）。

**源码构建（需要 Go 1.21+，会自动构建前端）**

```bash
make build          # 先 pnpm 前端再 go build（static + strip + UPX），输出 ./cc-insights
```

Windows 原生 PowerShell / cmd 也可直接用 Go 构建：

```powershell
go build -trimpath -tags=prod -o cc-insights.exe ./cmd/insights
```

### 2. 运行

```bash
cc-insights                # 默认输出最近 30 天 CLI 摘要
cc-insights web            # 启动 Web Dashboard（默认 :8932）
cc-insights web --addr :9090 --data /path/to/data
cc-insights rec -p 7d      # 诊断：根因 + 证据 + 下钻命令
```

### 3. 数据来源

默认读取 `~/.claude`（Claude Code 数据目录），可用 `--data` 指定任意目录：

```
~/.claude/
├── history.jsonl        # 命令历史
├── stats-cache.json     # 统计缓存
├── projects/            # Claude Code 项目会话 JSONL
├── tasks/               # Task 数据
└── debug/               # runtime 事件日志目录
```

## 命令速查

| 命令 | 作用 | 示例 |
|------|------|------|
| `sum` | 全局概览 | `cc-insights sum -p 7d` |
| `rec` | 诊断结论、证据、触发条件、根因候选、建议动作、下钻命令 | `cc-insights rec -p 7d` / `rec --detail` / `rec --prompts` |
| `why` | 按原因 / 工具 / 模型 / 项目 / Session 下钻失败样例 | `cc-insights why -p 7d --reason timeout -n 5` |
| `cmd` | Bash 命令族、具体命令、高风险命令（含链式 `&&`/`;` 逐段解析） | `cc-insights cmd -p 30d -j` |
| `tok` | Token、模型、项目和会话消耗 | `cc-insights tok -p 30d -j` |
| `ses` | Session 生命周期、长会话、高失败会话、Plan/Task 信号 | `cc-insights ses -p 7d -n 5` |
| `err` | 失败来源：失败原因、失败工具和模型组合 | `cc-insights err -p 7d -j` |
| `web` | 启动 Web Dashboard | `cc-insights web --addr :8932` |

`rec` 是主诊断入口，其余命令是稳定的原始证据下钻。新增分析能力优先进入 `rec` 的解释层，而非新增命令。

**全局 flags：**

| Flag | 说明 |
|------|------|
| `-p, --preset` | 时间范围：`24h`、`7d`、`30d`、`90d`、`all` |
| `--start / --end` | 自定义日期范围（`YYYY-MM-DD`） |
| `-f, --format` | 输出格式：`table`、`json`、`markdown` |
| `-j` / `-m` | 输出 JSON / 输出 Markdown |
| `-n` / `--limit` | Top N 数量（`why` 表示样例数） |
| `--detail` | 展开 `rec` 的触发条件、根因候选和优化目标 |
| `--id <id>` | 按诊断 ID 精确过滤 `rec` 输出 |
| `--prompts` | 在 `rec` 中分析用户提示词画像、协作偏好和候选规则 |
| `--reason / --category / --tool / --model / --project / --session` | 多维过滤 |
| `--data <path>` | 数据目录（默认 `~/.claude`） |
| `--cache <path>` | 缓存目录（默认 `~/.cc-insights/cache`） |
| `--rules <path>` | Bash 分类规则（默认内置 `rules/bash.yml`，也读 `~/.cc-insights/bash.yml`） |

## 与 AI 协作

cc-insights 本身就是为 AI 协作设计的诊断工具。所有命令支持 `-j` / `--format json`，输出结构化数据供 Claude Code、Codex 等 AI 直接解析和继续推理：

```bash
cc-insights err -p 7d -j
cc-insights why -p 7d --reason timeout -n 20 -j
cc-insights tok -p 30d -j
cc-insights cmd -p 30d -j
cc-insights ses -p 30d -j
cc-insights rec -p 30d -j
cc-insights rec -p 30d --id performance.slowest_call -j
```

> **典型协作流**：先让 AI 跑 `rec -p 7d -j` 拿到诊断结论与根因候选 → 对感兴趣的 ID 跑 `rec --id <id> -j` 或对应下钻命令（`why` / `cmd` / `tok` / `ses`）取原始证据 → AI 基于证据给出优化 `CLAUDE.md` / hooks / MCP 配置或封装工具的建议。

## 配置与规则

| 文件 | 作用 |
|------|------|
| `cmd/insights/rules/bash.yml` | Bash 命令分类规则（模式匹配、风险等级标记、链式命令分段） |
| `cmd/insights/rules/diagnostics.yml` | `rec` 诊断规则：指标定义、阈值、数据来源、触发解释 |

两者均可直接修改后热重载，无需重新编译。`--rules` 可加载自定义 Bash 规则，也会自动读取 `~/.cc-insights/bash.yml`。

## Dashboard

`cc-insights web` 启动本地 Dashboard，一个全局筛选状态驱动整屏分析，支持暗色模式。

![Dashboard Preview](docs/dashboard.png)

**能力一览：**

- 每日活动 / 会话趋势（折线）、项目活跃度排名、星期与工作时段分布（柱状）
- Slash Commands 使用、模型使用分析（含 Token）
- 诊断建议卡（severity + 证据 + 动作）与证据下钻面板（失败 / 命令 / Token / Session / 工具 5 个 tab）
- 时间范围快捷预设 + 自定义；模型 / 工具 / 原因 / 项目 / Session 多维联动

打开浏览器访问：

- Dashboard: http://localhost:8932/dashboard
- API: http://localhost:8932/api/data?preset=7d

接口参数、响应结构和可信度（`coverage`）约定见 [API 文档](docs/api.md)。

## 开发

```bash
make run-dev        # 开发模式运行（使用 ../data）
make test           # production tags 下运行测试（自包含 fixture）
go test ./...       # 开发时快速测试
make bench          # 性能测试
make release        # 多平台发布包（Linux/macOS/Windows）
```

前端源码在 `web/`（React + TypeScript + Vite + Tailwind + Recharts + TanStack Query），改 UI 后 `pnpm build` 产物落到 `cmd/insights/static/dist/` 经 `go:embed` 内联。详见 [前端重构计划](docs/plan/frontend-react-refactor.md)。

性能参考：72 核机器、约 2.2G 数据，全量解析约 5 秒，`history.jsonl` 解析约 0.05 秒。

## 故障排查

**Dashboard 无法启动**

```bash
ls -la ~/.claude/history.jsonl     # 确认数据目录存在
lsof -i :8932                      # 确认端口未被占用
cc-insights --data ~/.claude 2>&1 | tee debug.log
```

**图表不显示 / 数据加载慢**

1. F12 打开开发者工具，检查 Console 与 Network 是否有请求失败。
2. 运行 `make bench` 看并发解析耗时；`nproc` 查看可用核心数。

## 发布

GitHub Actions 的 Release workflow 在打 `v*` tag 时构建并发布 5 个平台包：

- Linux: `amd64`、`arm64`
- macOS: Intel (`amd64`)、Apple Silicon (`arm64`)
- Windows: `amd64`

本地 `make release` 也能生成同样的包（依赖 Unix shell 工具，Windows 建议用 Git Bash / WSL）。普通用户直接下载 [Release 产物](https://github.com/gqy20/cc-insights/releases)即可。

## 文档

- [Web Dashboard API](docs/api.md) — 接口、参数、响应、可信度约定
- [架构说明](docs/architecture.md)
- [诊断体系](docs/diagnostics.md)
- [容错与降级策略](docs/fallbacks.md)
- [路线图](docs/roadmap.md)
- [Codex 数据源调研](docs/archive/codex.md)

## License

[MIT](LICENSE)
