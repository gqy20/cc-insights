# 诊断体系

`rec` 是 cc-insights 的核心命令。它不是简单输出统计 Top N，而是把聚合数据解释成可行动的诊断报告。

## 设计原则

- `rec` 负责判断和解释。
- `why`、`cmd`、`tok`、`ses` 负责证据下钻。
- 新能力优先增强 `rec`，不新增重复命令。
- 诊断结果必须能回答“为什么触发、证据是什么、应该改哪里”。

## Finding 结构

每条诊断 finding 包含：

- `id`：稳定诊断 ID，可用 `--id` 精确过滤。
- `category`：`failure`、`command`、`performance`、`context`、`workflow` 等。
- `severity` / `confidence`：严重度和置信度。
- `summary` / `interpretation`：面向人的摘要和解释。
- `evidence`：聚合证据，例如失败率、耗时、Session、项目。
- `trigger`：触发指标、当前值、阈值、数据来源和触发解释。
- `root_causes`：根因候选，例如 `missing_path`、`browser_missing`、`task_scope_too_large`。
- `examples`：相关失败样例摘要。
- `targets`：建议优化方向。
- `actions`：结构化建议动作。
- `drilldown_commands`：可复制执行的下钻命令。

## 规则配置

诊断规则说明位于：

```text
cmd/insights/rules/diagnostics.yml
```

该文件集中描述：

- `id` 或 `id_prefix`
- 触发指标 `metric`
- 阈值说明 `threshold`
- 数据来源 `source`
- 触发解释 `rationale`

Go 代码仍负责实际计算和根因推断。不要把高频调整的阈值说明散落在输出代码里。

## 根因候选

当前根因候选会结合 reason 和失败样例 preview 进行细化，包括：

- `missing_path`
- `wrong_workdir`
- `service_not_running`
- `port_conflict`
- `dependency_missing`
- `browser_missing`
- `network_4xx_5xx`
- `permission_or_auth`
- `task_scope_too_large`
- `unsafe_command_requires_guardrail`
- `bash_rule_misclassification`
- `chain_command_masking`：`&&`/`;` 链式中后续段被丢弃（已修复，现逐段独立统计）
- `large_context_or_repeated_exploration`

这些根因不是最终事实，而是基于现有证据的候选判断。后续应通过更多样例和恢复结果提升置信度。

## 优化目标

`targets` 和 `actions` 把诊断连接到实际改进位置：

- `CLAUDE.md`：项目结构、常用入口、命令边界、任务拆分。
- `hook`：危险命令、前置检查、端口/路径/权限提示。
- `MCP`：工具依赖、认证、浏览器运行时、网络和参数契约。
- `tool`：高频慢命令、重复流程和稳定脚本封装。
- `workflow`：计划确认、阶段拆分、失败恢复和验收点。

## 使用方式

```bash
cc-insights rec -p 7d
cc-insights rec -p 7d --detail
cc-insights rec -p 30d --prompts
cc-insights rec -p 7d --id performance.slowest_call -j
```

`rec --prompts` 读取原始 `projects/**/*.jsonl`，只保留真实用户输入，过滤工具结果回传、恢复摘要、探针和明显自动化噪声，用于生成提示词画像、协作偏好和可沉淀到 `CLAUDE.md` / hooks / 工具规则的候选建议。

默认 table 输出保持简洁；`--detail` 展开触发条件、根因候选、样例和建议动作；JSON 输出始终包含结构化字段，适合 AI 继续分析。

## 增加新诊断

新增诊断时建议按以下顺序：

1. 在对应 `build*Diagnostics` 函数里生成 finding。
2. 为 finding 补稳定 `id`、`evidence`、`interpretation` 和 `next_steps`。
3. 在 `diagnostics.yml` 增加指标、阈值、来源和触发解释。
4. 在 root cause / action 逻辑里补目标映射。
5. 增加 focused test。
