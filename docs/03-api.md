# API 文档

Base URL:

```text
/api/v1
```

统一返回格式：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

## 1. 系统接口

### 1.1 健康检查

- Method: `GET`
- Path: `/health`

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "status": "up",
    "server_time": "2026-03-15T21:00:00+08:00"
  }
}
```

## 2. 首页总览

### 2.1 获取首页概览

- Method: `GET`
- Path: `/dashboard/overview`

返回内容：

- 最新黄金价格
- 今日涨跌
- 最新 AI 报告摘要
- 最新因子摘要
- 最新重要新闻

说明：

- `realtime_price` 当前优先读取 SQLite 最新采集结果。

## 3. 黄金价格接口

### 3.1 获取实时金价

- Method: `GET`
- Path: `/prices/realtime`

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "symbol": "AU_CNY_G",
    "price": 562.318,
    "change_amount": 1.246,
    "change_rate": 0.22,
    "currency": "CNY",
    "unit": "g",
    "captured_at": "2026-03-15T20:59:40+08:00"
  }
}
```

说明：

- 返回值已统一标准化为 `CNY/g`。
- 数据优先读取 SQLite 内最新有效 tick。

### 3.2 获取价格历史走势

- Method: `GET`
- Path: `/prices/history`

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `range` | string | 是 | `1d/7d/30d/90d/1y` |
| `interval` | string | 否 | `1m/5m/15m/1h/1d` |

说明：

- 当前 `1d` 周期优先读取数据库内的真实采集与聚合数据。
- 若长周期历史数据尚未积累充分，服务会临时回退到样例数据，保证页面可展示。
- 历史 K 线全部基于清洗后的标准化 tick 聚合。

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "symbol": "AU_CNY_G",
    "interval": "1m",
    "items": [
      {
        "time": "2026-03-15T20:00:00+08:00",
        "open": 560.100,
        "high": 560.600,
        "low": 559.980,
        "close": 560.420
      }
    ]
  }
}
```

### 3.3 获取价格实时流

- Method: `GET`
- Path: `/prices/stream`
- 协议：`SSE`

连接行为：

- 建立连接后立即返回一次 `price_status` 连接状态事件。
- 最新价格发生变化时推送 `price_tick`。
- 空闲期间推送 `price_status` 心跳事件。

事件类型：

- `price_tick`
- `price_status`

`price_tick` 示例：

```json
{
  "symbol": "AU_CNY_G",
  "price": 562.318,
  "change_amount": 1.246,
  "change_rate": 0.22,
  "currency": "CNY",
  "unit": "g",
  "captured_at": "2026-03-15T20:59:40+08:00"
}
```

`price_status` 示例：

```json
{
  "status": "alive",
  "server_time": "2026-03-15T21:00:00+08:00"
}
```

## 4. 新闻与事件接口

### 4.1 获取新闻列表

- Method: `GET`
- Path: `/news`

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `page` | int | 否 | 页码 |
| `page_size` | int | 否 | 每页数量 |
| `category` | string | 否 | 新闻分类 |
| `region` | string | 否 | 地区 |
| `importance` | int | 否 | 重要级别 |
| `factor_code` | string | 否 | 关联因子编码 |

说明：

- 返回分页结构：`items`、`page`、`page_size`、`total`。
- `summary`、`sentiment`、`importance`、`impact_score` 当前为规则化生成结果。

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "id": 1,
        "source_name": "Mock Macro Desk",
        "title": "美元指数回落，黄金短线获得支撑",
        "summary": "美元指数回调压低持有黄金的机会成本，市场对黄金短线配置意愿有所回升。",
        "url": "https://example.com/news/usd-gold",
        "region": "US",
        "category": "market",
        "sentiment": "positive",
        "importance": 4,
        "impact_score": 82,
        "related_factors": ["usd_index"],
        "published_at": "2026-03-15T20:25:00+08:00",
        "captured_at": "2026-03-15T21:00:00+08:00"
      }
    ],
    "page": 1,
    "page_size": 10,
    "total": 1
  }
}
```

### 4.2 获取新闻详情

- Method: `GET`
- Path: `/news/:id`

响应说明：

- 返回单条新闻详情，额外包含 `content` 正文内容。

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 1,
    "source_name": "Mock Macro Desk",
    "title": "美元指数回落，黄金短线获得支撑",
    "summary": "美元指数回调压低持有黄金的机会成本，市场对黄金短线配置意愿有所回升。",
    "content": "美元指数回调压低持有黄金的机会成本，市场对黄金短线配置意愿有所回升。",
    "url": "https://example.com/news/usd-gold",
    "region": "US",
    "category": "market",
    "sentiment": "positive",
    "importance": 4,
    "impact_score": 82,
    "related_factors": ["usd_index"],
    "published_at": "2026-03-15T20:25:00+08:00",
    "captured_at": "2026-03-15T21:00:00+08:00"
  }
}
```

## 5. 因子接口

### 5.1 获取最新因子面板

- Method: `GET`
- Path: `/factors/latest`

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "code": "usd_index",
      "name": "美元指数",
      "value": 104.23,
      "unit": "",
      "score": -62.4,
      "impact_direction": "bearish",
      "impact_strength": 81.2,
      "confidence": 86.0,
      "captured_at": "2026-03-15T20:30:00+08:00"
    }
  ]
}
```

### 5.2 获取单因子历史

- Method: `GET`
- Path: `/factors/history`

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `code` | string | 是 | 因子编码 |
| `range` | string | 是 | `7d/30d/90d/1y` |

### 5.3 获取因子定义列表

- Method: `GET`
- Path: `/factors/definitions`

## 6. AI 报告接口

### 6.1 获取最新报告

- Method: `GET`
- Path: `/reports/latest`

### 6.2 获取报告列表

- Method: `GET`
- Path: `/reports`

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `page` | int | 否 | 页码 |
| `page_size` | int | 否 | 每页大小 |
| `start_date` | string | 否 | 起始日期 |
| `end_date` | string | 否 | 结束日期 |

### 6.3 获取报告详情

- Method: `GET`
- Path: `/reports/:id`

### 6.4 获取准确率曲线

- Method: `GET`
- Path: `/reports/accuracy/curve`

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `range` | string | 是 | `30d/90d/180d/1y` |

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "avg_score": 78.6,
    "items": [
      {
        "report_date": "2026-03-10",
        "score": 82.0
      },
      {
        "report_date": "2026-03-11",
        "score": 74.5
      }
    ]
  }
}
```

## 7. 后台任务接口

### 7.1 手动触发金价采集

- Method: `POST`
- Path: `/admin/jobs/collect-price`

当前行为：

- 拉取当前金价
- 写入 `gold_price_ticks`
- 更新 `gold_price_candles`
- 写入 `job_runs`

### 7.2 手动触发新闻抓取

- Method: `POST`
- Path: `/admin/jobs/fetch-news`

### 7.3 手动触发因子更新

- Method: `POST`
- Path: `/admin/jobs/update-factors`

### 7.4 手动生成日报

- Method: `POST`
- Path: `/admin/jobs/generate-report`

请求体示例：

```json
{
  "report_date": "2026-03-15"
}
```

### 7.5 手动执行评分

- Method: `POST`
- Path: `/admin/jobs/score-report`

请求体示例：

```json
{
  "report_date": "2026-03-14"
}
```

### 7.6 获取任务执行记录

- Method: `GET`
- Path: `/admin/jobs/runs`

## 8. 状态码约定

| code | 含义 |
| --- | --- |
| `0` | 成功 |
| `4001` | 参数错误 |
| `4004` | 资源不存在 |
| `5001` | 内部服务错误 |
| `5002` | 外部数据源不可用 |
| `5003` | AI 报告生成失败 |
| `5004` | 评分失败 |

## 9. 接口开发约束

- 所有列表接口必须支持分页。
- 所有时间字段统一使用 ISO 8601。
- 图表接口默认按时间升序返回。
- 后台触发接口要记录 `job_runs`。
- 如接口变更，必须同步更新 `api/openapi.yaml` 与本文件。
