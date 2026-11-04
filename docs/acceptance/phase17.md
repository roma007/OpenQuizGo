# 017 退款审计与原因验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.17.0（家长端 / 管理端 0.17.0） |
| 日期 | 2026-11-05 |
| 范围 | 017：退款原因、操作人、审计日志 |
| 设计/计划 | [设计](../design/017-退款审计与原因设计.md) / [计划](../plans/017-退款审计与原因实施计划.md) |

## 1. 结论

**通过。** 管理端退款必须填写原因；订单保存原因与操作人；退款写入审计日志，审计页可见；家长端订单展示退款原因。

## 2. 交付

- 迁移 `00014_membership_refund_audit.sql`：`refund_reason`、`refunded_by`；
- 退款接口要求原因 1–200 字，已退款订单幂等返回首次原因；
- 退款成功后写入 `audit_log`：`membership.order.refund`，含订单号、金额、时长、原因、操作人、IP；
- 管理端退款改为原因弹窗，订单表新增「退款原因」列；
- 家长端已退款订单展示退款原因；
- 审计接口取消仅限题目类型，会员退款记录可在审计页看到。

## 3. 验证

```text
go test ./...                                      passed
TEST_DATABASE_URL go test ./internal/membership    passed
flutter test                                       30 passed
npm run build（管理端）                             passed
```

接口/数据库实测：

```text
管理员退款（原因「用户申请退款：误购」）
→ membership_order.refund_reason 保存原因
→ refunded_by 非空
→ 会员到期从 2027-01-03 回退到 2026-12-04
→ audit_log: membership.order.refund，after_data 含 order_no/amount/duration/reason
→ GET /v1/admin/audit 返回该审计记录
```

管理端浏览器：

| 步骤 | 结果 |
| --- | --- |
| 会员订单 → 退款 | 弹窗显示订单信息与原因输入框 |
| 填写原因并确认 | 订单状态「已退款」，退款原因列显示原因 |
| 审计日志 | 出现 `membership.order.refund` 记录 |

家长端 Widget 测试：已退款订单显示「已退款」与「退款原因：用户申请退款：误购」。

## 4. 限制

- 审计写入失败仅忽略，不阻断退款（与现有题库审计一致）；
- 审计未记录退款前后会员到期时间的完整快照；
- 原因仅管理员与家长可见，未做敏感词过滤。

## 5. 后续

- 审计记录补充 before/after 会员状态；
- 退款审核流与二次确认人；
- 真实支付渠道退款单号与对账。
