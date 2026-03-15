# 数据库表结构

数据库使用 `SQLite`，ORM 使用 `GORM`。设计目标是先满足单机版稳定落地，同时保留未来升级到 MySQL / PostgreSQL 的迁移空间。

## 1. 设计原则

- 价格数据与分析数据分表，避免写入热点互相影响。
- 所有核心记录保留 `source`、`captured_at`、`created_at`。
- 定时任务结果可追溯。
- AI 报告、预测结果、评分结果分离存储。

## 2. 表清单

| 表名 | 用途 |
| --- | --- |
| `data_sources` | 数据源配置 |
| `gold_price_ticks` | 实时黄金价格原始点位 |
| `gold_price_candles` | 聚合 K 线 |
| `factor_definitions` | 因子定义 |
| `factor_snapshots` | 因子时间序列快照 |
| `news_articles` | 新闻与行业事件 |
| `analysis_reports` | AI 每日分析报告 |
| `report_predictions` | 报告中结构化预测结论 |
| `report_scores` | 报告准确率评分 |
| `job_runs` | 定时任务执行记录 |

## 3. 详细表结构

### 3.1 `data_sources`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `code` | TEXT UNIQUE | 数据源编码 |
| `name` | TEXT | 数据源名称 |
| `category` | TEXT | `gold/news/macro/fx/event` |
| `base_url` | TEXT | 数据源地址 |
| `is_enabled` | BOOLEAN | 是否启用 |
| `priority` | INTEGER | 优先级，数值越小越优先 |
| `rate_limit_per_min` | INTEGER | 每分钟限频 |
| `created_at` | DATETIME | 创建时间 |
| `updated_at` | DATETIME | 更新时间 |

### 3.2 `gold_price_ticks`

存储最细粒度实时价格。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `source_id` | INTEGER INDEX | 数据源 ID |
| `symbol` | TEXT INDEX | 建议固定为 `AU_CNY_G` |
| `price` | DECIMAL(10,3) | 人民币/克 |
| `change_amount` | DECIMAL(10,3) | 相对上一采样点涨跌 |
| `change_rate` | DECIMAL(10,4) | 涨跌幅 |
| `currency` | TEXT | 固定 `CNY` |
| `unit` | TEXT | 固定 `g` |
| `captured_at` | DATETIME INDEX | 抓取时间 |
| `created_at` | DATETIME | 入库时间 |

索引建议：

- `idx_gold_tick_symbol_captured_at(symbol, captured_at desc)`

### 3.3 `gold_price_candles`

用于图表和回测展示。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `symbol` | TEXT INDEX | `AU_CNY_G` |
| `interval` | TEXT INDEX | `1m/5m/15m/1h/4h/1d` |
| `open_price` | DECIMAL(10,3) | 开盘价 |
| `high_price` | DECIMAL(10,3) | 最高价 |
| `low_price` | DECIMAL(10,3) | 最低价 |
| `close_price` | DECIMAL(10,3) | 收盘价 |
| `avg_price` | DECIMAL(10,3) | 均价 |
| `sample_count` | INTEGER | 样本数量 |
| `window_start` | DATETIME INDEX | 周期起始 |
| `window_end` | DATETIME | 周期结束 |
| `created_at` | DATETIME | 创建时间 |

唯一约束建议：

- `(symbol, interval, window_start)`

### 3.4 `factor_definitions`

定义影响黄金的因子元数据。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `code` | TEXT UNIQUE | 因子编码 |
| `name` | TEXT | 中文名称 |
| `category` | TEXT | `macro/market/event/demand` |
| `description` | TEXT | 因子说明 |
| `value_type` | TEXT | `number/text/score/percent` |
| `unit` | TEXT | 单位 |
| `default_weight` | DECIMAL(6,3) | 默认权重 |
| `impact_direction_rule` | TEXT | 影响方向规则说明 |
| `created_at` | DATETIME | 创建时间 |
| `updated_at` | DATETIME | 更新时间 |

预置因子编码建议：

- `usd_index`
- `fed_rate`
- `inflation`
- `cny_fx`
- `safe_haven_sentiment`
- `central_bank_gold_buying`
- `oil_price`
- `equity_market`
- `geopolitics`
- `physical_demand`

### 3.5 `factor_snapshots`

统一存储因子时间序列，无论来源是数值型数据还是事件型评分。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `factor_id` | INTEGER INDEX | 因子 ID |
| `source_id` | INTEGER INDEX | 数据源 ID |
| `value_num` | DECIMAL(14,4) NULL | 数值型值 |
| `value_text` | TEXT NULL | 文本型值 |
| `score` | DECIMAL(6,2) NULL | 统一评分，建议 `-100` 到 `100` |
| `impact_direction` | TEXT | `bullish/bearish/neutral` |
| `impact_strength` | DECIMAL(6,2) | 影响强度 |
| `confidence` | DECIMAL(5,2) | 置信度 |
| `summary` | TEXT | 快照摘要 |
| `captured_at` | DATETIME INDEX | 快照时间 |
| `created_at` | DATETIME | 创建时间 |

索引建议：

- `idx_factor_snapshot_factor_time(factor_id, captured_at desc)`

### 3.6 `news_articles`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `source_id` | INTEGER INDEX | 数据源 ID |
| `title` | TEXT INDEX | 标题 |
| `summary` | TEXT | AI 摘要 |
| `content_hash` | TEXT UNIQUE | 用于去重 |
| `url` | TEXT | 原文地址 |
| `published_at` | DATETIME INDEX | 发布时间 |
| `captured_at` | DATETIME | 抓取时间 |
| `region` | TEXT | `CN/US/EU/Global` |
| `category` | TEXT | `macro/policy/geopolitics/market/industry` |
| `sentiment` | TEXT | `positive/negative/neutral` |
| `importance` | INTEGER | 1-5 |
| `impact_score` | DECIMAL(6,2) | 对黄金影响分值 |
| `related_factors_json` | TEXT | 关联因子数组 JSON |
| `created_at` | DATETIME | 创建时间 |

### 3.7 `analysis_reports`

存储 AI 生成的每日分析报告。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `report_date` | DATE UNIQUE | 报告日期 |
| `title` | TEXT | 报告标题 |
| `trend` | TEXT | `bullish/bearish/range/volatile` |
| `confidence` | DECIMAL(5,2) | 报告置信度 |
| `summary` | TEXT | 摘要 |
| `full_content` | TEXT | 完整内容 |
| `key_drivers_json` | TEXT | 关键驱动因素 |
| `risk_points_json` | TEXT | 风险提示 |
| `input_snapshot_json` | TEXT | 当日输入快照 |
| `ai_provider` | TEXT | AI 服务商 |
| `model_name` | TEXT | 模型名 |
| `prompt_version` | TEXT | 提示词版本 |
| `generated_at` | DATETIME | 生成时间 |
| `created_at` | DATETIME | 创建时间 |
| `updated_at` | DATETIME | 更新时间 |

### 3.8 `report_predictions`

将报告中的核心预测结构化，便于评分。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `report_id` | INTEGER INDEX | 报告 ID |
| `target_date` | DATE INDEX | 预测目标日期 |
| `predicted_direction` | TEXT | `up/down/flat` |
| `predicted_low` | DECIMAL(10,3) | 预测低点 |
| `predicted_high` | DECIMAL(10,3) | 预测高点 |
| `predicted_close` | DECIMAL(10,3) NULL | 预测收盘 |
| `factor_focus_json` | TEXT | 重点关注因子 |
| `created_at` | DATETIME | 创建时间 |

### 3.9 `report_scores`

记录每日评分结果。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `report_id` | INTEGER UNIQUE INDEX | 报告 ID |
| `scored_date` | DATE INDEX | 评分日期 |
| `direction_score` | DECIMAL(6,2) | 方向得分 |
| `range_score` | DECIMAL(6,2) | 区间得分 |
| `factor_hit_score` | DECIMAL(6,2) | 因子命中得分 |
| `risk_score` | DECIMAL(6,2) | 风险提示得分 |
| `total_score` | DECIMAL(6,2) | 总分 `0-100` |
| `actual_close` | DECIMAL(10,3) | 实际收盘 |
| `actual_high` | DECIMAL(10,3) | 实际最高 |
| `actual_low` | DECIMAL(10,3) | 实际最低 |
| `score_explanation` | TEXT | 评分说明 |
| `created_at` | DATETIME | 创建时间 |

### 3.10 `job_runs`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER PK | 主键 |
| `job_name` | TEXT INDEX | 任务名 |
| `job_type` | TEXT | `collector/report/scoring/cleanup` |
| `status` | TEXT | `success/failed/running` |
| `started_at` | DATETIME | 开始时间 |
| `finished_at` | DATETIME | 结束时间 |
| `duration_ms` | INTEGER | 耗时 |
| `message` | TEXT | 结果摘要 |
| `error_detail` | TEXT | 错误信息 |
| `created_at` | DATETIME | 创建时间 |

## 4. 关系说明

- `data_sources` 1 对多 `gold_price_ticks`
- `data_sources` 1 对多 `factor_snapshots`
- `data_sources` 1 对多 `news_articles`
- `factor_definitions` 1 对多 `factor_snapshots`
- `analysis_reports` 1 对多 `report_predictions`
- `analysis_reports` 1 对 1 `report_scores`

## 5. GORM 模型拆分建议

- `backend/internal/model/source.go`
- `backend/internal/model/gold_price.go`
- `backend/internal/model/factor.go`
- `backend/internal/model/news.go`
- `backend/internal/model/report.go`
- `backend/internal/model/job_run.go`

## 6. 初始化数据建议

项目初始化时至少预置：

- 10 个影响因子定义
- 2 个以上价格数据源配置
- 2 个以上新闻源配置
- 默认评分权重配置

## 7. 当前代码落地状态

当前首版代码已接通 `GORM + SQLite` 启动链路，并完成以下表对应模型的 AutoMigrate 骨架：

- `data_sources`
- `gold_price_ticks`
- `gold_price_candles`
- `factor_definitions`
- `news_articles`
- `analysis_reports`
- `job_runs`

说明：

- 这是第一阶段启动骨架，字段已覆盖核心主干。
- `gold_price_candles` 与 `job_runs` 已接入真实持久化。
- `factor_snapshots`、`report_predictions`、`report_scores` 仍建议在下一阶段继续补齐为真实持久化模型。
