# 004 家长端告警中心验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.4.0 |
| 日期 | 2026-11-04 |
| 范围 | 004：权限异常告警闭环 |
| 设计/计划 | [设计](../design/004-家长端告警中心设计.md) / [计划](../plans/004-家长端告警中心实施计划.md) |

## 1. 结论

**通过。** TV 上报的 `permission_revoked` 已能在家长端看到，未读徽标与已读状态闭环。

## 2. 交付

- 迁移 `00008_alert_read.sql`（`tamper_event.read_at` + 索引）；
- 服务端 `GET /v1/family/alerts`、`POST /v1/family/alerts/read`；
- 家长端消息中心（铃铛徽标 + 列表 + 空态 + 已读）；
- Android `dist/OpenQuizGo-Parent-0.4.0.apk`；iOS Runner.app（无签名）。

## 3. 验证

```text
go test ./...        passed
flutter test         21 passed
dart analyze         无问题
```

模拟器实测：

| 步骤 | 结果 |
| --- | --- |
| 插入 `permission_revoked` | 首页铃铛显示未读 1 |
| 打开消息中心 | 显示「无障碍服务被关闭 / 解锁测试 / tv-parent-e2e2 / 时间」 |
| 自动已读 | `POST /alerts/read 200`，数据库 `read_at` 非空 |
| 返回首页 | 徽标清零 |

截图：`docs/acceptance/phase4/01-badge.png`、`02-alerts.png`、`03-read.png`。

## 4. 限制

- 未接入 FCM/APNs 系统推送，需家长打开 App 查看；
- 未读数仅在启动/进入列表/下拉刷新时更新；
- 告警类型文案目前覆盖权限、卸载、完整性、时钟四类。

## 5. 后续

- 005：FCM/APNs 推送（需要 Firebase 项目与 APNs Key）；
- 告警分类订阅、按孩子筛选。
