# 051 校准趋势图与批量回滚验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.43.0（管理端 0.43.0） |
| 日期 | 2026-11-05 |
| 范围 | 051：校准趋势聚合与批量撤销最近一次校准 |
| 设计/计划 | [设计](../design/051-校准趋势图与批量回滚设计.md) / [计划](../plans/051-校准趋势图与批量回滚实施计划.md) |

## 1. 结论

**通过。** 新增按天校准趋势接口与管理端趋势条；新增批量撤销最近一次校准接口，撤销写 `rollback=true` 审计，重复撤销自动跳过，避免来回切换。

## 2. 交付

- `GET /v1/admin/calibration/trend?days=30`（`question.read`）：按天返回 `changes/up/down`；
- `POST /v1/admin/calibration/rollback`（`question.write`）：批量撤销最近一次非回滚校准，最多 200 题；
- 仅当当前难度等于最近一次校准目标难度时撤销，否则跳过；
- 撤销审计 `after_data.rollback=true`，历史接口返回 `rollback` 标记；
- 管理端校准页新增「近 30 天校准趋势」、行选择批量「撤销最近校准」、历史弹窗「回滚」标签。

## 3. 验证

```text
gofmt -w internal/question                         passed
go test -p 1 ./...（测试库/Redis）                  passed
cd admin-web && npm run build                      passed
cd admin-web && npm run test:e2e                   7/7 passed
```

接口实测：

```text
题目难度 2 → apply 3                     200 applied=1
POST /admin/calibration/rollback         200 applied=1 skipped=0，难度回到 2
再次 rollback                            200 applied=0 skipped=1
GET /admin/calibration/trend?days=7      200 返回每日 changes/up/down
```

集成测试覆盖：撤销后难度回退、历史新增 rollback 记录、重复撤销跳过、趋势聚合。

## 4. 限制

- 批量撤销只回退一步，不做多步历史选择；
- 趋势按数据库会话时区按天聚合；
- 未实现趋势图折线/导出；
- 撤销不记录独立原因字段。

## 5. 后续

- 多步历史回滚；
- 趋势图可视化升级与导出；
- 撤销原因与审批。
