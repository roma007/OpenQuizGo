# 047 覆盖率按专题导出验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.41.0（管理端 0.41.0） |
| 日期 | 2026-11-05 |
| 范围 | 047：专题维度覆盖率聚合与 CSV 导出 |
| 设计/计划 | [设计](../design/047-覆盖率按专题导出设计.md) / [计划](../plans/047-覆盖率按专题导出实施计划.md) |

## 1. 结论

**通过。** 新增专题覆盖聚合接口与 CSV 导出；同一题属于多个专题时分别计数，未加入专题的题目单独汇总；管理端覆盖率页新增专题表格与导出按钮。默认矩阵导出保持不变。

## 2. 交付

- `GET /v1/admin/coverage/topics?threshold=50`（`question.read`）；
- 返回 `cells`（topic_id/topic_name/status/count）、去重 `total`、`warnings`；
- `GET /v1/admin/coverage/export?dimension=topic` → `topic_id,topic_name,status,count`；
- 默认或非 `topic` 维度仍导出原矩阵 CSV；
- 管理端「专题覆盖」表格 + 「导出专题 CSV」按钮 + 专题告警。

## 3. 验证

```text
gofmt -w internal/question                                  passed
TEST_DATABASE_URL=... go test ./internal/question           passed
cd admin-web && npm run build                               passed
```

测试覆盖：多专题分别计数、未分专题兜底、去重 total、低于阈值告警、高于阈值不告警。

## 4. 限制

- 未实现覆盖率趋势快照/趋势图；
- 完全没有题目或仅草稿的专题不会进入 cells，因此不会产生 0 题告警；
- 未使用 `question.topic_id` 单值字段，仅以 `topic_question` 多对多关系为准。

## 5. 后续

- 覆盖率趋势快照；
- 0 题专题告警；
- 按专题筛选题目。
