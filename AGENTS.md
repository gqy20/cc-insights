# 仓库指南

## 项目理念

cc-insights 是面向 Claude Code 的使用诊断工具，不只是统计或可视化工具。核心目标是把历史数据转化为可解释的证据、判断和改进方向，帮助人和 AI 看清失败、耗时、Token、Session、Bash 命令和工具调用背后的原因。

命令体系保持收敛：`rec` 负责诊断、解释、触发条件、根因候选、建议动作和下一步方向；`why`、`cmd`、`tok`、`ses` 负责证据下钻；`sum` 负责全局概览；`web` 负责本地可视化。不要为每一种解释继续增加新命令，新的分析能力优先增强 `rec`。

## 项目结构

主代码位于 `cmd/insights/`，使用单个 Go `main` 包。CLI 入口和报告构建在 `cli.go`、`cli_*.go`；解析、聚合、缓存和分析逻辑分布在 `parser.go`、`aggregate*.go`、`cache*.go`、`*_analysis.go`。前端静态资源在 `cmd/insights/static/`；规则文件在 `cmd/insights/rules/`，其中 `bash.yml` 负责 Bash 分类，`diagnostics.yml` 负责诊断阈值说明；文档和截图在 `docs/`。

## 构建、测试与运行

- `make build`：构建静态 `cc-insights` 二进制文件。
- `make run`：构建并用默认数据目录运行。
- `make run-dev`：使用 `go run -tags=prod ./cmd/insights -data ../data` 开发运行。
- `make test`：使用 production tags 运行完整测试。
- `go test ./...`：开发时快速运行测试。
- `make clean`：清理二进制和发布目录。
- `make release`：构建多平台发布包。

常用验证命令：`go run ./cmd/insights rec -p 7d -n 3`，`go run ./cmd/insights why -p 7d --reason timeout -n 5`。

## 代码风格

Go 文件提交前运行 `gofmt`。新增功能应放在对应领域文件中：CLI 报告放 `cli_*.go`，聚合放 `aggregate*.go`，缓存放 `cache*.go`，前端交互放 `cmd/insights/static/`。命名使用清晰的动宾结构，例如 `buildCLISessionReport`、`cloneToolAnalysis`。CLI 命令保持三个字符左右且稳定，当前主命令为 `sum`、`err`、`why`、`tok`、`cmd`、`ses`、`rec`、`web`。

## 测试要求

测试使用 Go 标准 `testing` 包，文件命名为 `*_test.go`，与实现文件同目录。新增 CLI 报告、解析逻辑、缓存路径、规则分类和诊断逻辑时，应补充聚焦测试。优先使用已有 fixture 和 helper，不依赖真实 `~/.claude` 数据目录。提交前至少运行 `go test ./...`；涉及 release 或 production tags 时运行 `make test`。

## 提交与 PR

提交信息使用简短祈使句，例如 `Add fast diagnostic drilldowns`。每次提交聚焦一个清晰主题。PR 应说明问题、变更摘要、测试结果；涉及 Web 或输出格式时附截图或 CLI 输出；有关联 issue 时一并链接。

## 安全与配置

不要提交真实 Claude Code 数据、缓存或日志。默认数据目录为 `~/.claude/`，本地缓存位于 `~/.cc-insights/cache/`。自定义 Bash 规则可放在 `~/.cc-insights/bash.yml`，内置默认规则位于 `cmd/insights/rules/bash.yml`；诊断规则说明位于 `cmd/insights/rules/diagnostics.yml`。
