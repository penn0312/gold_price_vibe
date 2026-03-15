# Changelog

All notable changes to this project should be documented in this file.

## [0.1.0] - 2026-03-15

### Added

- 初始化黄金价格走势分析项目的顶层 README。
- 新增架构设计文档与模块说明。
- 新增数据库表结构设计文档。
- 新增 API 文档与 OpenAPI 草案。
- 新增前后端开发 TodoList。
- 新增完整测试用例文档。
- 新增变更自动同步规范文档。
- 新增 `make check-sync` 校验入口与同步脚本。
- 新增 `Go + Gin + GORM + SQLite` 后端启动骨架。
- 新增 `Vue3 + Vite + ECharts` 前端首页骨架。
- 新增 mock 数据服务与首页所需 API 路由。
- 新增 SQLite AutoMigrate 初始化与基础模型定义。
- 新增本地运行环境变量示例。
- 新增价格采集 provider、价格仓储层、K 线聚合与自动采集任务。
- 新增 `gold_price_candles` 与 `job_runs` 持久化模型。
- 新增价格链路仓储测试。

### Docs

- 明确了价格采集、新闻抓取、因子分析、AI 报告、评分与准确率曲线的设计边界。
- 补充了当前首版骨架的运行方式与实现状态。
- 补充了价格写库、K 线聚合和定时采集的当前实现状态。

### Tests

- 建立覆盖后端、前端、采集、AI、调度、性能、安全和文档联动的测试矩阵。
- 补充首版骨架启动、跨域和图表渲染测试项。
- 新增价格持久化、手动采集和定时采集测试项。
