# 015 会员权益生效验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.15.0（家长端 / 管理端 0.15.0） |
| 日期 | 2026-11-04 |
| 范围 | 015：付费专题门禁、进阶报告、权益配置 |
| 设计/计划 | [设计](../design/015-会员权益生效设计.md) / [计划](../plans/015-会员权益生效实施计划.md) |

## 1. 结论

**通过。** 套餐权益改为结构化代码并可配置；付费专题在有效会员下参与组卷、无会员或到期后自动跳过；报告页按权益显示进阶分析或锁定提示。

## 2. 交付

- 迁移 `00012_membership_entitlements.sql`：`entitlement_codes`，月卡/年卡回填 `topic_pack`、`report_advanced`；
- 会员服务：套餐读写权益代码、白名单校验；会员响应返回 `entitlements`；
- 组卷：付费专题要求家庭内有效会员且套餐包含 `topic_pack`；
- 管理端：套餐表新增「功能权益」，弹窗支持多选；
- 家长端：会员中心展示权益标签；报告页新增「进阶分析」（薄弱学科、较上期变化、练习建议），无权益显示锁定提示。

## 3. 验证

```text
go test ./...                                             passed
TEST_DATABASE_URL go test ./internal/quiz                passed（付费专题 3 场景）
TEST_DATABASE_URL go test ./internal/membership          passed（权益 CRUD/到期）
flutter test                                             29 passed
npm run build（管理端）                                   passed
```

接口实测（付费专题 + 5 道数学题）：

```text
会员有效 → POST /v1/admin/quiz/preview
         → topic_name=E2E 付费专题，selected 全部来自专题
会员到期 → 同参数 preview
         → topic_id 为空，total=5（普通组卷）
```

管理端浏览器：套餐列表显示「付费专题题包 / 进阶报告」；编辑弹窗多选已回填。

家长端模拟器：

| 状态 | 报告页结果 |
| --- | --- |
| 会员有效 | 显示「进阶分析」：薄弱学科数学 75%、练习建议 |
| 会员到期 | 显示锁图标与「开通会员后可查看薄弱学科、较上期变化与练习建议」 |

截图：`docs/acceptance/phase15/01-report-advanced.png`、`02-report-locked.png`。

## 4. 限制

- 进阶分析目前基于单份报告与上一份报告，未做更细的日趋势；
- 付费专题只在组卷时判断家庭会员，管理端预览未指定孩子时不命中付费专题；
- 权益代码暂为固定三项，新增需改代码与前端文案；
- `customer_service` 仅展示，不控制功能。

## 5. 后续

- 家庭会员共享策略可配置（当前为家庭内任一有效会员）；
- 权益中心页面与到期提醒；
- 真实支付回调与退款后权益回收。
