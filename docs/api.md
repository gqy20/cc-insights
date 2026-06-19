# Web Dashboard API

cc-insights 的 Web Dashboard 基于本地 HTTP API，默认监听 `:8932`，所有接口返回统一 JSON。本页是接口参考；快速上手见 [README](../README.md)。

## 主数据接口

### GET /api/data

获取筛选后的主数据（驱动 Dashboard 全部图表）。

```
GET /api/data?preset=7d
GET /api/data?preset=custom&start=2025-12-01&end=2026-01-08
GET /api/data?preset=7d&project=cc-insights&model=claude-sonnet-4-6&reason=timeout
```

**参数：**

| 参数 | 说明 |
|------|------|
| `preset` | `24h` \| `7d` \| `30d` \| `90d` \| `all` \| `custom` |
| `start` / `end` | 自定义范围起止日期（`YYYY-MM-DD`），仅 `preset=custom` 时生效 |
| `project` | 按项目路径片段过滤 |
| `model` | 按模型名过滤 |
| `tool` | 按工具名过滤 |
| `reason` | 按失败原因过滤 |
| `session` | 按 Session ID 过滤 |

**响应示例：**

```json
{
  "success": true,
  "data": {
    "timestamp": "2026-06-15 16:02:07",
    "time_range": {
      "preset": "7d",
      "start": "2026-06-08",
      "end": "2026-06-15"
    },
    "commands": [
      {"Command": "/tdd", "Count": 115},
      {"Command": "/gh", "Count": 31}
    ],
    "hourly_counts": { "10": 53, "11": 44, "15": 137 },
    "daily_trend": {
      "dates": ["2026-06-09", "2026-06-10"],
      "counts": [7765, 7849]
    },
    "runtime_tools": [
      {"Tool": "search_web", "Server": "jina", "Count": 1543}
    ],
    "sessions": {
      "total_sessions": 89,
      "peak_date": "2026-06-12",
      "peak_count": 23,
      "valley_date": "2026-06-08",
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
      {"model": "claude-sonnet-4-6", "count": 120, "tokens": 450000}
    ]
  }
}
```

## 交互式分析接口

用于 Dashboard 的下钻面板和大屏联动，复用同一组过滤参数：

```
GET /api/overview?preset=7d
GET /api/diagnostics?preset=7d&detail=true
GET /api/diagnostics?preset=7d&id=performance.slowest_call
GET /api/detail/failures?preset=7d&reason=timeout
GET /api/detail/commands?preset=7d&family=test
GET /api/detail/tokens?preset=7d&project=cc-insights
GET /api/detail/sessions?preset=7d&session=<id>
GET /api/detail/tools?preset=7d&tool=Bash
GET /api/timeline?preset=all
```

## 元数据与可信度

所有交互式接口返回统一 `meta`，包含数据源、缓存版本、时间范围、过滤条件和运行耗时。`/api/data` 接受同一组过滤参数，前端会用同一个 filter 同步刷新主图表和下钻面板。

Dashboard 响应会附带 `coverage` 元数据，说明每个图在当前筛选下的可信度：

- `exact`：可由缓存索引精确计算。
- `sample`：只能基于样例下钻，不能代表完整总体。
- `unavailable`：当前组合筛选没有精确数据支撑，前端会显示空态原因。

每日趋势已支持时间范围、项目、Session、工具、失败原因和模型的部分组合精确联动，例如 `project + tool`、`project + reason`、`project + model`、`session + tool`、`session + reason`、`session + model`。无法精确重算的图表不会展示全局数据，避免造成假联动。
