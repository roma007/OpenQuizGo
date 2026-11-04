# 007 覆盖率告警与导出验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.7.0（admin-web） |
| 日期 | 2026-11-04 |
| 范围 | 007：覆盖率告警与 CSV 导出 |
| 设计/计划 | [设计](../design/007-覆盖率告警与导出设计.md) / [计划](../plans/007-覆盖率告警与导出实施计划.md) |

## 1. 结论

**通过。** 覆盖率不足会告警，数据可导出 CSV，满足第 3 期「看板可告警」退出条件。

## 2. 验证

```text
GET /v1/admin/coverage
  total: 2007
  warnings: ["语文/小学低年级 在线题量 1，低于建议 50"]

GET /v1/admin/coverage/export
  Content-Type: text/csv; charset=utf-8
  Content-Disposition: attachment; filename="coverage.csv"
  subject,stage,type,status,difficulty,count
  chinese,low,choice,online,2,1
  math,low,choice,draft,2,1
```

浏览器：

- 覆盖率页显示「覆盖不足告警（1 项）」与具体文案；
- 「导出 CSV」按钮调用导出接口。

`npm run build` 通过。

## 3. 限制

- 阈值固定 50，后续做成可配置；
- 导出为当前聚合结果，不含题目明细；
- 告警目前通过页面展示，未接入推送。

## 4. 后续

- 阈值按学段/学科配置；
- 告警接入 004 消息中心 / 007 推送；
- 覆盖率趋势图与按 topic 导出。
