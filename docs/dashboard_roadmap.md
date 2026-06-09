# Dashboard 功能扩展路线图

> 本文档记录 Claude Code Dashboard 可以新增的统计功能和分析维度

**当前状态**: Dashboard v1.5 已实现基础统计、核心扩展、缓存预聚合和 API 超时保护

---

## 📊 当前已实现功能

### ✅ 核心统计

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
| 缓存预聚合 | projects/debug 数据 | cache/cache.db |
| API 超时保护 | HTTP context | 60 秒超时 |

---

## ✅ 已完成的扩展功能

### 1️⃣ 项目活跃度排名

**数据源**: `projects/*.jsonl` 中的 `cwd` 字段

**统计内容**:
- 各项目使用次数排名
- Top 15 项目展示
- 项目路径简化显示
- 时间范围内项目活跃度变化

**示例数据**:
```
chestnut/docs/01_thesis_proposal:  892次
university-crawler:                651次
mind:                              565次
pdfget:                            525次
article_mcp:                       476次
```

**可视化方案**:
- 图表类型: 柱状图
- 交互: tooltip 显示完整项目路径

**状态**: 已实现

---

### 2️⃣ 模型使用统计

**数据源**: `projects/*.jsonl` assistant message 中的 `model` 和 `usage`

**统计内容**:
- 各模型请求次数
- 各模型 Token 总量
- 平均每次请求 Token 数

**可视化方案**:
- 图表类型: 柱状图 + 折线图
- 双轴: 请求数 (左轴) + Token 数 (右轴)

**状态**: 已实现

---

### 3️⃣ 会话统计分析

**数据源**: `projects/*.jsonl` 中的 `sessionId`

**统计内容**:
- 总会话数
- 每日会话数趋势
- 峰值/谷值日期

**可视化方案**:
- 图表类型: 折线图 + 统计卡片
- 交互: 悬停显示每日会话数

**状态**: 已实现

---

## 🆕 可新增的统计功能

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

### 已完成

| 功能 | 状态 | 说明 |
|------|------|------|
| 项目活跃度排名 | ✅ 已完成 | 从 `projects/*.jsonl` 聚合并支持缓存筛选 |
| 模型使用统计 | ✅ 已完成 | 统计请求数和 Token 总量 |
| 会话统计 | ✅ 已完成 | 从一次遍历的 aggregate 提取，避免重复 I/O |
| 星期分布 | ✅ 已完成 | 输出 7 天活动分布 |
| 工作时段统计 | ✅ 已完成 | 统计 9-18 点与非工作时段占比 |
| 缓存预聚合 | ✅ 已完成 | `BuildFullCache` 生成 `cache.db` |
| API 超时保护 | ✅ 已完成 | `/api/data` 60 秒超时 |

### Phase 2: 增强功能

| 功能 | 优先级 | 复杂度 | 预计工作量 |
|------|--------|--------|------------|
| 工具调用密度 | ⭐⭐ | 中等 | 2-3小时 |
| Agent 使用统计 | ⭐⭐ | 中等 | 3-4小时 |
| 活动峰值分析 | ⭐ | 中等 | 2-3小时 |

### Phase 3: 高级功能

| 功能 | 优先级 | 复杂度 | 预计工作量 |
|------|--------|--------|------------|
| Shell 命令统计 | ⭐ | 中等 | 2-3小时 |
| Token 效率分析 | ⭐ | 中等 | 3-4小时 |

---

## 📋 技术实施要点

### 当前架构

- `ParseProjectsConcurrentOnce` 一次遍历 `projects/*.jsonl`，同时产出项目、日期、会话、星期、小时和模型统计。
- `BuildFullCache` 基于 aggregate 和 debug 日志构建本地缓存，API 优先通过 `QueryByTimeRange` 读取缓存。
- `/api/data` 在缓存失败时会降级为实时解析，并有 60 秒超时保护。
- 前端图表集中在 `cmd/insights/static/app.js`，后端响应结构集中在 `DashboardData`。

### 已知限制

- `IncrementalUpdate` 目前检测到数据更新后仍执行完整重建，尚未做真正的增量合并。
- MCP 工具统计来自 `debug/` 日志，缓存中没有按日分解，所以时间范围筛选下仍是全局统计。
- 部分旧的 go-echarts 服务端渲染代码仍保留，但当前 Dashboard 主要走前端 ECharts 渲染。

---

*最后更新: 2026-06-09*
