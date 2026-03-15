# 每次变更自动同步规范

目标：任何功能变更都必须同步更新文档、接口文档、版本日志、测试用例，避免“代码变了，说明没变”。

## 1. 必须同步的文件

| 变更类型 | 必须同步更新 |
| --- | --- |
| 数据库模型变更 | `docs/02-database-schema.md`、`CHANGELOG.md`、`docs/05-test-cases.md` |
| API 路由或请求响应变更 | `docs/03-api.md`、`api/openapi.yaml`、`CHANGELOG.md`、`docs/05-test-cases.md` |
| 采集逻辑 / 因子逻辑变更 | `README.md`、`docs/01-architecture.md`、`CHANGELOG.md`、`docs/05-test-cases.md` |
| AI 报告 / 评分逻辑变更 | `README.md`、`docs/01-architecture.md`、`docs/03-api.md`、`CHANGELOG.md`、`docs/05-test-cases.md` |
| 前端页面结构或交互变更 | `README.md`、必要时 `docs/03-api.md`、`CHANGELOG.md`、`docs/05-test-cases.md` |

## 2. 执行机制

建议每次提交前执行：

```bash
make check-sync
```

该命令会调用：

```bash
bash scripts/verify_sync.sh
```

## 3. 校验规则

### 3.1 当以下目录发生变更时

- `backend/internal/model`
- `backend/internal/api`
- `backend/internal/service`
- `frontend/src`

### 3.2 至少检查以下文档是否同步更新

- `README.md`
- `docs/02-database-schema.md`
- `docs/03-api.md`
- `api/openapi.yaml`
- `docs/05-test-cases.md`
- `CHANGELOG.md`

## 4. 研发流程要求

每个需求或缺陷修复都应遵循：

1. 先改代码。
2. 同步改文档。
3. 同步改接口文档。
4. 同步补测试用例。
5. 更新版本日志。
6. 运行联动校验。

## 5. PR 检查清单

提交前必须确认：

- [ ] 需求说明与实现一致
- [ ] 接口文档已更新
- [ ] 数据库表结构文档已更新
- [ ] 测试用例已补齐
- [ ] `CHANGELOG.md` 已更新
- [ ] 已执行 `make check-sync`

## 6. 建议版本日志格式

每次更新请按以下分组维护：

- `Added`
- `Changed`
- `Fixed`
- `Docs`
- `Tests`

## 7. 失败处理

如果联动校验失败，不允许合并，必须先补齐：

- 缺失文档
- 缺失接口说明
- 缺失测试用例
- 缺失版本日志
