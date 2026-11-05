# 065 退款回调验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.57.0 |
| 日期 | 2026-11-05 |
| 范围 | 065：HMAC 退款回调、幂等退款与渠道退款单号 |
| 设计/计划 | [设计](../design/065-退款回调设计.md) / [计划](../plans/065-退款回调实施计划.md) |

## 1. 结论

**通过（服务端测试 + 接口）。** 新增支付渠道退款回调，HMAC 校验后复用后台退款逻辑，记录渠道退款单号；重复回调不重复回退会员时长。

## 2. 交付

- 迁移 `00028_refund_callback.sql`（已应用到开发库，版本 28）；
- `POST /v1/payments/refund-callback`，HMAC-SHA256；
- 密钥 `REFUND_CALLBACK_SECRET`，未配置回退 `PAYMENT_CALLBACK_SECRET`；
- 已退款订单幂等返回，不重复回退；
- 记录 `external_refund_no` 唯一索引；
- 订单响应增加 `external_refund_no`。

## 3. 验证

```text
go test -p 1 ./...（测试库/Redis）      全部通过
go run ./cmd/migrate up                 OK 00028
bad signature                          401 SIGNATURE_INVALID
```

测试覆盖合法回调退款、会员到期回退、重复回调幂等、错误签名、未配置密钥 503。

## 4. 限制

- 未处理退款失败/撤销状态；
- 未实现渠道对账文件导入；
- 回调无重试/告警；
- 真实商户回调需配置密钥并联调。

## 5. 后续

- 退款失败/撤销；
- 渠道对账与重试告警；
- 真实支付渠道联调。
