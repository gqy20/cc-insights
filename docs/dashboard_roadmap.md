# Dashboard 功能扩展路线图

> 本文档记录 Claude Code Dashboard 可以新增的统计功能和分析维度

**当前状态**: Dashboard v2.0 已实现基础统计、核心分析、质量与生命周期分析（M0-M4 全部完成）

---

## 📊 当前已实现功能（M0-M4 全部完成）

### ✅ M0: 基础统计

| 功能 | 数据源 | 可视化类型 |
|------|--------|------------|
| Slash Commands 使用统计 | history.jsonl | 横向柱状图 (Top 15) |
| 每日活动趋势 | projects/*.jsonl / cache | 折线图 |
| MCP 工具调用统计 | debug/ 目录 | 饼图 (Top 10) |
| 每日会话趋势 | projects/*.jsonl / cache | 折线图 |
| 项目活跃度排名 | projects/*.jsonl / cache | 柱状图 (Top 15) |
| 星期活动分布 | projects/*.jsonl / cache | 柱状图 |
| 模型使用分析 | projects/*.jsonl / cache | 柱状图 + 折线图 |
| 工作时段分布 | projects/*.jsonl / cache | 柱状图 |
| 时间范围筛选 | - | 快捷预设 + 自定义 |
| 缓存预聚合 | projects/debug 数据 | CacheFile JSON |
| API 超时保护 | HTTP context | 60 秒超时 |

### ✅ M1: 核心分析模块

| # | 模块 | 数据源 | 关键指标 |
|---|------|--------|---------|
| 1 | **工具调用分析** ToolAnalysis | attachment events | 成功率/失败率/Missing Result，按工具+模型细分 |
| 2 | **失败原因细分** FailureAnalysis | tool_use error | 按 reason/tool/model 三维聚合，含样例 |
| 3 | **运行事件与 Hook** EventAnalysis | runtime events | 事件类型/Hook 成功取消错误/权限模式/IDE打开文件 |
| 4 | **Agent 分析** AgentAnalysis | agent events | 主会话 vs subagent 工具调用，sidechain 占比 |
| 5 | **Bash 与文件操作** CommandAnalysis | bash/file_operation | 风险等级分类(high/medium/low)，Edit 失败原因 |
| 6 | **Token 与成本** CostAnalysis | usage tokens | 按模型/项目/会话/agent 统计，预算时间线 |

### ✅ M2: Session 生命周期复盘

**数据源**: `projects/*.jsonl` sessionId 分组

**统计内容**:
- Top 失败 Session（工具失败 + Missing Result）
- 高耗时 Session 排名
- Outcome 状态分布（success/error/cancelled）
- Queue 操作统计

**状态**: 已实现 (`SessionAnalysisData`, `initSessionAnalysisChart`)

### ✅ M3: 文件与编辑质量分析

**数据源**: attachment events (read_text_file/write_file/edit_text_file/file-history-snapshot/edited_text_file)

**5 维度输出**:
1. **Hot Files** — 按 Read/Edit/Write 操作次数排序，含失败率
2. **Edit Failures** — 按文件+失败原因聚合（permission denied/read-only/conflict 等）
3. **Snapshots** — file-history-snapshot 跨会话热度、版本号追踪
4. **Edited Files** — edited_text_file 编辑量（行数/字符数）、平均编辑规模
5. **Totals** — 唯一文件数、总操作量、整体 Edit 失败率

**状态**: 已实现 (`FileAnalysisData`, `initFileAnalysisChart`)

### ✅ M4: Task / Plan 结构分析

**双数据源设计**:

| 数据源 | 类型 | 内容 |
|--------|------|------|
| Plan 分析 (P0) | Attachment 事件零额外 IO | plan_mode 生命周期(37进/34出/7重入)、plan_file_reference(32条含markdown)、goal_status(3条)、reminder频率(3408 task/808 todo) |
| Task 分析 (P1) | tasks/ 目录独立并发扫描 | 825个JSON文件、417个session目录、状态分布(69.5%完成率)、per-session统计 |

**5 网格图表布局**:
1. 左上: Plan Mode 生命周期 (Enter/Exit/Reentry 分组柱状)
2. 右上: 计划文件引用 Top 10 (横向柱状)
3. 左下: Task 状态分布 (Completed/Pending/InProgress 堆叠)
4. 右下: Reminder 频率 Top session (Task vs Todo 分组)
5. 底部全宽: Per-session Task 数 Top 10 (堆叠横向)

**状态**: 已实现 (`TaskPlanAnalysisData`, `initTaskPlanChart`)

---

## 🆕 未来可新增的功能

### 4️⃣ 工具调用密度分析 (中优先级)

**数据源**: `dailyActivity` 中的 `toolCallCount / messageCount`

**统计内容**:
- 工具调用与消息比率趋势
- 高密度时段识别
- 工具使用效率分析

**示例数据**:
```
2025-12-07: 工具调用率 38%
2025-12-21: 工具调用率 37%
2025-12-05: 工具调用率 37%
```

**可视化方案**:
- 图表类型: 双轴折线图
- 左轴: 消息数
- 右轴: 工具调用数
- 区域填充: 工具调用率

**实施难度**: ⭐⭐ 中等
**价值评估**: ⭐⭐⭐⭐

---

### 5️⃣ Agent 使用统计 (中优先级)

**数据源**: `history.jsonl` 中的 agent 调用记录

**统计内容**:
- 各 agent 调用次数排名
- Agent 类型分布
- Agent 协作模式

**可用 Agent** (19个):
```
- task-executor / task-checker
- git-expert
- human-centric-coder
- jobs-designer
- knuth-style-algorithmist
- liheng-scientific-engineer
- linus-style-programmer
- literature-analysis-expert
- literature-manager
- master-writing-craftsman
- mermaid-diagram-architect
- mermaid-expert-consultant
- pragmatic-linus-programmer
- pytest-expert
- uncle-bob-craftsman
- canvas-design
- pdf-anthropic
```

**可视化方案**:
- 图表类型: 横向柱状图
- 分组: 按 Agent 类型归类
- 交互: 显示 Agent 详细描述

**实施难度**: ⭐⭐ 中等 (需要文本解析)
**价值评估**: ⭐⭐⭐

---

### 6️⃣ Shell 命令统计 (低优先级)

**数据源**: `shell-snapshots/` 目录 (1,046个文件)

**统计内容**:
- Shell 类型分布 (bash/zsh)
- Shell 快照时间分析
- 快照频率趋势

**可视化方案**:
- 图表类型: 饼图 + 折线图
- 交互: 切换不同 shell 类型

**实施难度**: ⭐⭐ 中等
**价值评估**: ⭐⭐

---

### 7️⃣ 活动峰值/低谷分析 (低优先级)

**数据源**: `dailyActivity`

**统计内容**:
- 峰值日期标识
- 谷值日期标识
- 异常活动检测
- 活动波动分析

**示例数据**:
```
峰值: 2026-01-04 (12,702 消息, 23 会话)
谷值: 2025-11-19 (69 消息, 2 会话)
```

**可视化方案**:
- 图表类型: 带标注的折线图
- 标注: 峰值/谷值点
- 交互: 点击显示详情

**实施难度**: ⭐⭐ 中等
**价值评估**: ⭐⭐⭐

---

### 8️⃣ Token 效率分析 (低优先级)

**数据源**: `dailyModelTokens`

**统计内容**:
- Token 消耗趋势
- 模型切换效率
- 缓存效率分析
- 成本优化建议

**可视化方案**:
- 图表类型: 多系列折线图
- 交互: 模型对比

**实施难度**: ⭐⭐ 中等
**价值评估**: ⭐⭐⭐

---

## 🎯 实施优先级

### ✅ 已完成（M0-M4）

| Milestone | 功能 | 状态 | 关键文件 |
|-----------|------|------|---------|
| M0 | 基础统计 (Commands/Trend/MCP/Projects/Weekday/Model/WorkHours) | ✅ | parser.go, charts.go |
| M1 | 核心分析 (Tool/Failure/Event/Agent/Command/Cost) | ✅ | types.go, aggregate_finalize.go, charts_runtime.js |
| M2 | Session 生命周期复盘 | ✅ | session_stats.go, charts_runtime.js |
| M3 | 文件与编辑质量分析 | ✅ | parser.go (attachment), charts_runtime.js |
| M4 | Task / Plan 结构分析 | ✅ | task_parser.go, parser.go (plan events), charts_runtime.js |

### 未来方向

| 功能 | 优先级 | 复杂度 | 数据源 |
|------|--------|--------|--------|
| Shell 命令统计 | ⭐ | 中等 | shell-snapshots/ 目录 |
| Token 效率分析 | ⭐⭐ | 中等 | 缓存命中率 / 成本优化建议 |
| 工具调用密度分析 | ⭐⭐ | 低 | toolCallCount/messageCount 比率 |
| 活动峰值/低谷分析 | ⭐ | 中等 | 异常活动检测 |
| Agent 协作模式分析 | ⭐⭐ | 高 | agent 调用链路追踪 |

---

## 📋 技术架构要点

### 当前架构

- **单次遍历聚合**: `ParseProjectsConcurrentOnce` 一次遍历 `projects/*.jsonl`，同时产出项目、日期、会话、星期、小时、模型、工具、失败、事件、agent、命令、成本、文件、plan 等 14+ 维度统计
- **双数据源设计**: M4 引入独立数据源模式 — Plan 分析复用 JSONL 零额外 IO，Task 分析从 `tasks/` 目录独立并发扫描
- **四数据源并行**: API 层 history/projects/debug/tasks 四个 goroutine 并行解析，整体耗时由最慢的数据源决定
- **缓存系统**: `BuildFullCache` → CacheFile JSON → `QueryByTimeRange` 时间过滤 → 优雅降级实时解析
- **前端渲染**: ECharts CDN + wonderland theme，15 个图表模块各自独立 init 函数
- **优雅降级**: 每个数据源有 safeParse* 容错包装，任一源失败返回空结果不影响其他模块

### 已知限制

- `IncrementalUpdate` 目前检测到数据更新后仍执行完整重建，尚未做真正的增量合并
- MCP 工具统计来自 `debug/` 日志，缓存中没有按日分解，时间范围筛选下仍是全局统计
- Task 时间过滤使用 mtime 近似值，跨天 session 可能不精确
- Task UUID 与 Project UUID 不重叠，无法直接关联任务到具体项目

### 新增文件清单（M1-M4）

| 文件 | 职责 |
|------|------|
| `task_parser.go` | tasks/ 目录并发扫描（M4 新增） |
| `static/charts_runtime.js` | 全部前端 ECharts 图表初始化函数 |
| `static/app_core.js` | DOM 容器创建 + 图表注册调用 |

---

*最后更新: 2026-06-11 (M4: Task/Plan 分析完成)*
