# 018 支付回调与签名验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.18.0（服务端 / 管理端 0.18.0） |
| 日期 | 2026-11-05 |
| 范围 | 018：支付回调、HMAC 签名、渠道归因 |
| 设计/计划 | [设计](../design/018-支付回调与签名设计.md) / [计划](../plans/018-支付回调与签名实施计划.md) |

## 1. 结论

**通过。** 支付渠道可用 HMAC-SHA256 签名回调；签名错误被拒绝，合法回调幂等开通/续费，订单记录渠道与外部交易号。

## 2. 交付

- 迁移 `00015_membership_payment_callback.sql`：`external_trade_no`；
- `POST /v1/payments/callback`：无需家长令牌，HMAC 校验原始请求体；
- 回调只处理 `paid`：pending→paid、记录 `channel=callback` 与外部交易号、延长会员；
- 已支付幂等、已退款拒绝、未知订单 404；
- `PAYMENT_CALLBACK_SECRET` 配置，dev 默认 `dev-payment-callback-secret`，空值禁用；
- 管理端订单表新增「渠道」列。

## 3. 验证

```text
go test ./...                                      passed
TEST_DATABASE_URL go test ./internal/membership    passed（含回调用例）
npm run build（管理端）                             passed
```

本地回调实测：

```text
创建月卡订单 M20261105004449106674（pending）
错误签名 → 401
正确签名 →
  200 {"status":"ok","paid":true}
  订单 status=paid, channel=callback, external_trade_no=CALLBACK-001
  会员到期 2027-01-03
重复回调 → 200，到期时间不变
```

签名示例：

```bash
BODY='{"order_no":"M2026...","trade_no":"T123","status":"paid"}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$PAYMENT_CALLBACK_SECRET" | awk '{print $2}')
curl -s -X POST http://127.0.0.1:18080/v1/payments/callback \
  -H "Content-Type: application/json" \
  -H "X-Payment-Signature: $SIG" \
  -d "$BODY"
```

管理端订单页：渠道列显示「渠道回调」。

## 4. 限制

- 尚未对接真实微信/支付宝签名格式与证书；
- 回调未做原文存档、失败重试队列与告警；
- 仅支持 `paid` 状态，不支持关闭/退款回调；
- 外部交易号未做唯一性约束。

## 5. 后续

- 多渠道适配与证书管理；
- 回调日志、重试与对账；
- 退款回调与订单状态同步。
