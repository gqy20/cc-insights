# 更新日志

这个文件记录项目中值得关注的版本变化。

项目使用带 `v` 前缀的语义化版本标签，例如 `v0.1.0`。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 新增
- **核心 Dashboard**：基于 Go + go-echarts 的 Web 仪表盘，可视化 Claude Code 使用数据，单文件部署（~2MB，UPX 压缩），静态链接无外部依赖
- **时间范围过滤**：支持 7天 / 30天 / 90天 / 全部 / 自定义 时间范围筛选
- **项目活跃度排名**：按消息数统计各项目活跃度，生成水平条形图
- **每日活动趋势**：折线图展示每日消息数量变化
- **24 小时分布**：柱状图展示全天各小时的活动分布
- **星期分布统计**：查看一周中每天的活动频率
- **工作时段热力图**：分析工作时段与非工作时段的使用模式
- **模型使用统计**：展示不同 Claude 模型的使用占比
- **会话统计**：按会话维度统计消息数、工具调用数等指标
- **MCP 工具调用统计**：从 debug 日志解析 MCP 工具调用频次
- **命令使用排行**：从 history.jsonl 统计斜杠命令使用频率
- **运行时分析面板**：解析 debug 日志中的 runtime 事件、agent 工具调用和工具调用失败记录，在 Dashboard 展示运行时统计
- **并发解析引擎**：
  - history.jsonl：Producer-Consumer 模式 + Worker Pool（1000 条/批）
  - projects/*.jsonl：`ParseProjectsConcurrentOnce` 单次遍历聚合全部统计
  - debug 日志：信号量控制并发（CPU 核心 × 4，最多 64）
- **缓存系统**：`CacheBuilder` 预聚合数据持久化，按时间范围查询，基于文件修改时间自动判断重建
- **API 接口**：`/api/data`、`/api/stats` 返回 JSON 数据，`/api/reload` 手动触发缓存刷新
- **AssistantMessage thinking 解析**：支持 Claude thinking 类型内容解析

### 修复

### 变更

### 性能

### 移除

### 工具链

### 文档
