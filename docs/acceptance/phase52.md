# 052 覆盖率趋势图验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.44.0（管理端 0.44.0） |
| 日期 | 2026-11-05 |
| 范围 | 052：覆盖率按天快照与趋势 |
| 设计/计划 | [设计](../design/052-覆盖率趋势图设计.md) / [计划](../plans/052-覆盖率趋势图实施计划.md) |

## 1. 结论

**通过。** 新增 `coverage_snapshot` 表、今日快照采集与近 N 天趋势接口；worker 每个报告周期自动采集；管理端覆盖率页展示趋势与「记录今日快照」按钮。

## 2. 交付

- 迁移 `00022_coverage_snapshot.sql`；
- `POST /v1/admin/coverage/snapshot`（`question.write`）采集/刷新今日快照；
- `GET /v1/admin/coverage/trend?days=30`（`question.read`）按日期升序返回快照；
- 同日 UPSERT，不产生重复行；
- worker 每次报告周期后自动采集；
- 管理端趋势条 + 手动快照按钮。

## 3. 验证

```text
gofmt / go build                          通过
go test -p 1 ./...（测试库/Redis）         全部通过
go run ./cmd/migrate up                   OK 00022
cd admin-web && npm run build             通过
cd admin-web && npm run test:e2e          7/7 通过
```

接口实测：

```text
POST /admin/coverage/snapshot   200 {captured_on:"2026-11-05",total:2007,online:2003,...}
再次 POST                       200 同日覆盖，数值一致
GET /admin/coverage/trend?days=7 200 返回该日快照
```

集成测试覆盖同日 UPSERT、趋势返回、editor 可采集。

## 4. 限制

- 趋势从首次采集开始积累，不回溯历史；
- 仅总量与状态维度，未按学科/专题拆分；
- 按数据库日期聚合；
- 未实现趋势导出。

## 5. 后续

- 学科/专题维度快照；
- 趋势导出；
- 覆盖率变化告警。
