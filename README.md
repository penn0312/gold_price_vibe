# 黄金价格走势分析项目

一个面向中文用户的黄金价格走势分析系统，聚焦 `人民币/克` 实时金价、影响因子联动、新闻事件抓取、AI 日报生成、预测复盘评分与历史准确率展示。

## 1. 项目目标

本项目需要同时解决 6 个核心问题：

1. 实时采集人民币计价黄金价格，并展示多周期走势图。
2. 自动抓取国内外影响黄金走势的新闻、政策与行业事件。
3. 内置并量化影响黄金走势的关键因素，形成可视化因子面板。
4. 由 AI 每天自动生成未来走势分析报告。
5. 每天自动对前一天的分析进行准确率评分，输出 `0-100` 分。
6. 可视化历史预测准确率曲线，用于评估模型与策略稳定性。

## 2. 技术栈

- 后端：`Go` + `Gin` + `GORM` + `SQLite`
- 前端：`Vue 3` + `Vite` + `ECharts`
- 调度：`cron` / 后端内置定时任务
- AI：可接入兼容大模型接口，用于报告生成、新闻摘要、事件归因、评分解释
- 风格：纯白简约、金融信息优先、低噪音展示

## 3. 核心功能

### 3.1 实时黄金价格

- 采集人民币/克实时价格
- 存储 tick 与 K 线聚合数据
- 展示 `1D / 7D / 30D / 90D / 1Y` 走势
- 支持分时线、均线、涨跌幅、最高最低价

### 3.2 新闻与行业事件

- 自动抓取国内外财经新闻、央行动态、地缘政治事件
- 自动抽取关键词、情绪、关联因子、影响方向
- 支持按时间、来源、因子、影响等级筛选

### 3.3 影响黄金走势因子

系统内置以下因子并统一量化：

- 美元指数
- 美联储利率
- 通胀
- 人民币汇率
- 避险情绪
- 央行购金
- 石油
- 股市
- 地缘政治
- 实物需求

### 3.4 AI 分析报告

- 每日自动生成黄金未来走势分析报告
- 输出趋势判断、关键驱动因素、风险提示、置信度
- 保留报告版本、模型版本、输入快照

### 3.5 准确率评分

- 每日对前一日报告做自动评分
- 从方向、幅度、关键因素命中、风险提示命中等维度打分
- 形成历史准确率曲线

## 4. 推荐目录结构

```text
gold_price/
├── README.md
├── CHANGELOG.md
├── Makefile
├── api/
│   └── openapi.yaml
├── backend/
│   ├── cmd/server/
│   ├── internal/
│   │   ├── api/
│   │   ├── config/
│   │   ├── cron/
│   │   ├── model/
│   │   ├── repository/
│   │   ├── service/
│   │   └── source/
│   └── migrations/
├── docs/
│   ├── 01-architecture.md
│   ├── 02-database-schema.md
│   ├── 03-api.md
│   ├── 04-todolist.md
│   ├── 05-test-cases.md
│   └── 06-change-sync.md
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── stores/
│   │   └── styles/
│   └── public/
└── scripts/
    └── verify_sync.sh
```

## 5. 页面建议

### 5.1 首页总览

- 实时金价卡片
- 当日涨跌幅
- 最新 AI 趋势判断
- 关键因子看板
- 最新重要新闻

### 5.2 价格分析页

- 实时走势图
- K 线 / 分时切换
- 周期切换
- 成交与波动辅助信息

### 5.3 因子分析页

- 十大因子最新值
- 单因子历史走势
- 因子对金价方向影响标签
- 因子相关性热力图

### 5.4 新闻事件页

- 新闻时间流
- 国内 / 国际 / 宏观 / 地缘政治筛选
- AI 摘要与影响因子标注

### 5.5 AI 报告页

- 每日分析报告列表
- 报告详情
- 前一日报告评分
- 历史准确率曲线

## 6. 交付顺序

本文档配套输出已按以下顺序落盘：

1. `README.md`
2. `docs/01-architecture.md`
3. `docs/02-database-schema.md`
4. `docs/03-api.md`
5. `docs/04-todolist.md`
6. `docs/05-test-cases.md`
7. `docs/06-change-sync.md`

## 7. 当前状态

当前仓库已完成：

- 项目级设计文档与接口草案
- `Go + Gin + GORM + SQLite` 后端启动骨架
- `Vue3 + Vite + ECharts` 前端首页骨架
- mock 数据驱动的报告、准确率 API
- 实时金价写库、K 线聚合、自动采集定时任务
- 价格主源失败自动切换备用源
- 价格采集标准化、异常值过滤与 `CNY/g` 单位统一
- `/prices/stream` SSE 持续推送
- 新闻抓取、分类映射、去重入库与 `/news` 查询接口
- 因子定义初始化、历史快照预热与 `/factors` 真实读库接口
- `update-factors` 因子更新任务与 SQLite 快照持久化
- 报告生成、结构化预测、评分入库与 `/reports` 真实读库接口
- `generate-report`、`score-report` 后台任务与历史准确率曲线持久化
- 自动任务定义中心、统一调度、失败重试与告警钩子
- 变更同步规范、版本日志与校验脚本

## 8. 已实现的首版代码能力

### 后端

- 启动 Gin 服务
- 初始化 SQLite 与 GORM AutoMigrate
- 提供首页、价格、新闻、因子、报告、评分曲线、后台任务接口骨架
- 金价采集器支持 `mock` / `remote` 两种模式
- `remote` 模式下主源失败会自动回退到 `mock` 备用源
- 实时金价会写入 SQLite
- 采集结果统一标准化为 `CNY/g`
- 异常跳变与过期时间戳会在入库前拦截
- 自动聚合 `1m / 5m / 15m / 1h / 1d` K 线
- 启动后自动预热 1 日历史价格并定时采集
- 提供 `SSE` 实时价格流接口
- 新闻系统支持 `mock` / `remote` 两种抓取模式
- 启动时自动预热新闻数据，支持手动触发新闻抓取任务
- 新闻支持分页、分类、地区、重要级别和关联因子筛选
- 新闻标题和正文会生成哈希，避免重复入库
- 当前摘要、情绪和影响因子为规则映射版，后续可升级为 AI 版
- 因子系统会自动初始化 10 个核心因子定义
- 启动时若无因子数据，会自动预热近 90 天历史快照
- `/factors/latest`、`/factors/history`、`/factors/definitions` 已切到 SQLite 真实回读
- `update-factors` 会结合最新价格、新闻热度与规则引擎生成最新因子快照
- 因子评分统一输出 `score`、`impact_direction`、`impact_strength`、`confidence`
- 报告系统会自动预热近 30 天日报、结构化预测与历史评分
- `/reports/latest`、`/reports`、`/reports/:id`、`/reports/accuracy/curve` 已切到 SQLite 真实回读
- `generate-report` 支持指定 `report_date` 生成日报，`score-report` 支持指定日期重算评分
- 服务启动时会初始化 `job_definitions`，统一维护自动任务频率、超时和重试参数
- 新闻抓取、因子更新、日报生成、评分任务已支持自动调度
- 自动任务运行会记录 `trigger_mode`、`attempt`、`max_attempts`、`scheduled_for`
- `/admin/jobs/definitions` 可查看任务中心配置与最近状态
- 当前日报与评分为本地规则引擎版本，后续可平滑替换为真实 AI 模型

### 前端

- 首页金融风大盘
- 实时金价卡片
- 1 日分时走势图
- 因子面板
- 新闻事件列表
- 准确率曲线

## 9. 本地运行

### 9.1 启动后端

```bash
go mod tidy
make run-backend
```

### 9.2 启动前端

```bash
cd frontend
npm install
npm run dev
```

### 9.3 环境变量

参考：

```text
.env.example
```

关键变量：

- `APP_PORT`
- `APP_DB_PATH`
- `GOLD_SOURCE_MODE`
- `GOLD_API_URL`
- `GOLD_API_KEY`
- `NEWS_SOURCE_MODE`
- `NEWS_FEED_URL`
- `NEWS_API_KEY`
- `USD_CNY_RATE`
- `PRICE_COLLECT_INTERVAL_SEC`
- `NEWS_FETCH_ENABLED`
- `NEWS_FETCH_INTERVAL_SEC`
- `FACTOR_UPDATE_ENABLED`
- `FACTOR_UPDATE_INTERVAL_SEC`
- `REPORT_GENERATE_ENABLED`
- `REPORT_GENERATE_TIME`
- `REPORT_SCORE_ENABLED`
- `REPORT_SCORE_TIME`
- `JOB_RETRY_LIMIT`
- `JOB_RETRY_BACKOFF_SEC`
- `JOB_TIMEOUT_SEC`
- `JOB_ALERT_WEBHOOK`
- `VITE_API_BASE`

## 10. 下一步建议

建议下一阶段按以下顺序继续：

1. 进入 Phase 7，补齐前端加载态、空态、错误态和重试交互
2. 将日报生成器和新闻摘要从规则版升级为 AI 版
3. 对接真实宏观因子源，替换当前本地规则因子引擎
4. 完成联调、端到端验证与 `v0.1.0` 发布收口
5. 增加鉴权、告警增强和部署脚本
