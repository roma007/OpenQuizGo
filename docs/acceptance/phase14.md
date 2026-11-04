# 014 会员与权益基础验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.14.0（家长端 / 管理端 0.14.0） |
| 日期 | 2026-11-04 |
| 范围 | 014：会员套餐、订单、模拟支付、会员状态 |
| 设计/计划 | [设计](../design/014-会员与权益基础设计.md) / [计划](../plans/014-会员与权益基础实施计划.md) |

## 1. 结论

**通过。** 管理端可维护套餐；家长端可查看状态与套餐、下单并模拟支付；开通后会员状态与到期时间正确，重复支付幂等，续费按到期时间累加。

## 2. 交付

- 迁移 `00011_membership.sql`：`membership_plan`、`membership_order`、`account_membership`，种子月卡/年卡；
- 新增权限 `membership.read` / `membership.write`（超级管理员全量，运营只读）；
- 服务端 `internal/membership`：套餐 CRUD、下单、模拟支付、状态计算；
- 管理端「会员套餐」页面：列表、新建/编辑、删除；
- 家长端设置页「会员中心」：状态卡、套餐卡、模拟支付确认、订单记录。

## 3. 验证

```text
go test ./...                                      passed
TEST_DATABASE_URL go test ./internal/membership    passed
flutter test                                       28 passed
npm run build（管理端）                             passed
```

接口实测：

```text
GET  /v1/admin/membership/plans   200，2 个种子套餐
POST /v1/admin/membership/plans   201
PUT  /v1/admin/membership/plans   200
DELETE /v1/admin/membership/plans 200（无订单引用）
POST /v1/family/membership/orders 201 pending，¥19.90 / 30 天
POST .../pay                      200 active，到期 2026-12-04
重复 pay                          到期时间不变（幂等）
```

管理端浏览器走查：登录后可见月度/年度套餐；新建「E2E 套餐」→ 列表显示草稿 → 删除确认 → 列表移除。

家长端模拟器走查（Android TV 平板模拟器）：

| 步骤 | 结果 |
| --- | --- |
| 设置 → 会员中心 | 显示「尚未开通会员」、月度 ¥19.90、年度 ¥169.00 |
| 月度会员 → 开通 | 弹出「模拟支付」确认，注明不会真实扣款 |
| 确认支付 | 状态变为「会员有效」，有效期至 2026-12-04（剩余 30 天），按钮变为「续费」 |
| 数据库 | `account_membership` active，`membership_order` paid |

截图：`docs/acceptance/phase14/01-membership-plans.png`、`02-membership-active.png`。

## 4. 限制

- 仅模拟支付，未接入微信/支付宝；
- 权益字段只存储与展示，尚未控制具体功能；
- 无退款、发票、自动续费、优惠券；
- 当前每个账号一条会员记录，套餐变更只影响到期时间与展示名称。

## 5. 后续

- 接入真实支付回调与订单对账；
- 权益按 `features` 控制专题/报告等功能；
- 到期提醒与自动续费；
- 管理端订单查询与退款。
