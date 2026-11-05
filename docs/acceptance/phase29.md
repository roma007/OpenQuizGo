# 029 排行榜周期筛选验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.27.0（家长端 0.27.0） |
| 日期 | 2026-11-05 |
| 范围 | 029：家庭排行榜总榜/本周/本月 |
| 设计/计划 | [设计](../design/029-排行榜周期筛选设计.md) / [计划](../plans/029-排行榜周期筛选实施计划.md) |

## 1. 结论

**通过。** 排行榜支持总榜/本周/本月切换；周/月按 `finished_at` 过滤，积分、连胜与排名基于所选周期。

## 2. 交付

- `GET /v1/family/battles/ranking?period=all|week|month`，默认 all，非法 422；
- 周期过滤使用数据库 `date_trunc`；
- 家长端排行榜标题右侧 Segmented，切换后重新统计；
- 对战记录不随周期变化；
- OpenAPI 补 period 参数。

## 3. 验证

```text
go test ./...                                      passed
TEST_DATABASE_URL go test ./internal/battle        passed（40 天前对局仅计入总榜）
flutter test                                       40 passed
```

接口实测：

```text
period=all    → 200
period=week   → 200
period=month  → 200
period=year   → 422
```

Widget 测试：点击「本周」后仓库收到 `period=week`。

## 4. 限制

- 周/月边界按数据库时区；
- 排行榜实时查询，未缓存；
- 不支持自定义日期区间或赛季。

## 5. 后续

- 自定义区间/赛季；
- 奖励积分与勋章；
- 榜单分享。
