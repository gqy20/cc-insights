# Claude Code 工作指南

## 项目定位

cc-insights 是面向 Claude Code 的使用诊断工具。它读取 Claude Code 历史数据、项目会话、工具调用、失败样例、Token 消耗和 Session 生命周期，把这些数据整理成可解释的证据、判断和改进方向。

本项目的核心目标不是继续堆统计图，也不是为每类问题新增命令，而是帮助用户回答：

- Claude Code 经常失败在哪里？
- 失败更像环境、路径、依赖、权限、网络、工具参数，还是任务拆分问题？
- 哪些项目、Session、命令族和工具调用消耗了最多时间与 Token？
- 应该优化 `CLAUDE.md`、hooks、MCP 配置，还是封装新的稳定工具？

## 命令职责

CLI 命令必须保持收敛：

- `sum`：全局概览。
- `rec`：诊断、解释、证据摘要、触发条件、根因候选、建议动作和下钻命令。
- `why`：失败样例下钻。
- `cmd`：Bash 命令族、具体命令和高风险命令下钻。
- `tok`：Token、模型、项目和会话消耗下钻。
- `ses`：Session 生命周期、长会话、高失败会话和 Plan/Task 信号下钻。
- `web`：启动本地 Dashboard。

新增分析能力时，优先增强 `rec` 的解释层；只有当能力属于稳定、通用的原始证据入口时，才考虑新增或调整下钻命令。

## 项目结构

主代码位于 `cmd/insights/`，使用单个 Go `main` 包。

- `cli.go`、`cli_*.go`：CLI 参数、报告构建和输出。
- `parser.go`、`project_parser.go`、`history_parser.go`、`debug_parser.go`：数据解析。
- `aggregate.go`、`aggregate_finalize.go`：聚合和最终分析结果生成。
- `cache.go`、`cache_builder.go`：缓存结构、文件级快照、变化检测和快速查询。
- `*_analysis.go`：成本、Session、Skill、命令、失败等专项分析。
- `command_file_analysis.go`：Bash 命令解析，支持多段 `&&`/`;` 链式命令独立统计（含引号感知分割、同链去重、结果分发和逐段风险检测）。
- `cmd/insights/static/dist/`：Web Dashboard 构建产物（Vite 输出），由 `web/` 生成后经 `go:embed` 内联进单二进制。
- `web/`：Web Dashboard 前端源码（React + TypeScript + Vite + Tailwind + Recharts + TanStack Query），`pnpm build` 产物落到 `cmd/insights/static/dist/`。设计方向见 `docs/plan/frontend-react-refactor.md`。
- `cmd/insights/rules/bash.yml`：内置 Bash 命令分类规则。
- `cmd/insights/rules/diagnostics.yml`：`rec` 诊断规则说明，包括指标、阈值、数据来源和触发解释。
- `docs/`：路线图、说明和截图。

## 常用命令

```bash
make build          # 构建：先 web-build（pnpm 前端）再 go build（static + UPX），输出单二进制 cc-insights
make run            # 构建并运行
make run-dev        # 使用 ../data 开发运行
make test           # production tags 下运行测试
go test ./...       # 开发时快速测试
make clean          # 清理构建产物
make release        # 构建多平台发布包
cd web && pnpm dev  # 前端开发（Vite 热更新，代理 /api 到 Go :8080；改前端用这个）
```

常用 CLI 验证：

```bash
go run ./cmd/insights rec -p 7d -n 3
go run ./cmd/insights rec -p 7d --detail
go run ./cmd/insights why -p 7d --reason timeout -n 5
go run ./cmd/insights ses -p 7d -n 5
go run ./cmd/insights web --addr :8932
```

## 开发原则

- Go 文件提交前运行 `gofmt`。
- 构建和验证编译优先用 `make build`（输出 `cc-insights`，static + strip + UPX）。不要在仓库根裸 `go build ./cmd/insights`：它默认按路径末段输出名为 `insights` 的二进制，且未被 `.gitignore` 忽略，会污染工作区。需要快速编译检查时用 `make build` 或 `go build -o /tmp/insights ./cmd/insights`。
- 保持实现靠近所属领域：CLI 放 `cli_*.go`，聚合放 `aggregate*.go`，缓存放 `cache*.go`，前端放 `web/`（构建产物落到 `cmd/insights/static/dist/`，勿手改；改 UI 在 `web/src/` 后 `pnpm build`）。
- 不要引入重复命令；优先复用 `rec`、`why`、`cmd`、`tok`、`ses` 的职责边界。
- 对结构化数据优先使用现有聚合和缓存结构，不要临时扫描或拼字符串。
- 修改 Bash 分类规则时优先编辑 `cmd/insights/rules/bash.yml`，避免把可配置规则硬编码进 Go。
- 修改诊断阈值说明、指标来源和触发解释时优先编辑 `cmd/insights/rules/diagnostics.yml`。
- 不提交真实 Claude Code 数据、缓存文件或日志。

## 测试要求

测试使用 Go 标准 `testing` 包，文件命名为 `*_test.go`，与实现同目录。新增或修改以下能力时应补测试：

- CLI 命令、输出结构和过滤参数。
- 缓存构建、轻量诊断缓存和时间范围查询。
- Bash 命令分类规则和多段链式命令解析（`splitChainSegments`、`extractShellSegments`、结果分发）。
- 失败原因、Session 生命周期、Token 成本、性能诊断。

提交前至少运行：

```bash
go test ./...
```

涉及 production tags、发布流程或嵌入静态资源时，再运行：

```bash
make test
make build
```

## 提交约定

提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org)，格式 `<type>(<scope>): <subject>`，已由 `.pre-commit-config.yaml` 的 commit-msg 钩子强制校验（首次启用需 `pre-commit install --hook-type commit-msg`）。

允许的 type：`feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`revert`。

例如：

```text
feat(web): apply Anthropic-style fonts (Newsreader + Geist)
fix(parser): handle quoted && chain segments
chore(ci): enforce conventional commit messages
```

每个提交聚焦一个清晰主题。涉及 Web UI 或 CLI 输出格式时，在最终说明中附验证命令、截图结论或关键输出摘要。
