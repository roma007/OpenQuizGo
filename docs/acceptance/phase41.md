# 041 校准历史与回滚验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.39.0 |
| 日期 | 2026-11-05 |
| 范围 | 041：难度校准历史查询与回滚 |
| 设计/计划 | [设计](../design/041-校准历史与回滚设计.md) / [计划](../plans/041-校准历史与回滚实施计划.md) |

## 1. 结论

**通过。** 新增只读校准历史接口，数据来自 `audit_log` 的 `question.calibrate`；管理端校准页可查看历史并按历史难度回滚，回滚复用现有 `calibration/apply` 并生成新审计记录。

## 2. 交付

- `GET /v1/admin/calibration/history?question_id=&limit=20`（权限 `question.read`）；
- 历史字段：`id` / `question_id` / `from` / `to` / `actor_id` / `created_at`；
- `question_id` 可选，`limit` 默认 20、上限 200；
- SQL 全参数化，过滤缺失/非法 `from`/`to`；
- 管理端校准页每行「历史」按钮，弹窗展示难度变化、时间、操作人；
- 「回滚到此难度」二次确认后调用 `calibration/apply`，成功后刷新列表。

## 3. 验证

```text
go build ./...                                        passed
go test ./internal/question（TEST_DATABASE_URL）       passed
go vet ./internal/question                            passed
cd admin-web && npm run build                         passed
```

集成用例覆盖：apply(2→4) 后历史返回 from/to；editor（`question.read`）可读；回滚后新增记录且最新在前；`limit=1000` 被截断为 200。

开发库实测：

```text
GET /v1/admin/calibration/history?limit=5 → 200
[{"id":23,"question_id":"...","from":1,"to":2,"actor_id":"...","created_at":"..."}]
```

## 4. 限制

- 历史依赖审计写入成功；审计失败时该次校准不可见；
- 回滚目标难度与当前难度相同时不产生新记录，前端提示「已回滚 0 题」；
- 未做批量回滚与校准趋势图。

## 5. 后续

- 校准趋势图；
- 批量回滚；
- 回滚审批流。
