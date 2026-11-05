# 023 房间码过期与倒计时验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.23.0（家长端 0.23.0） |
| 日期 | 2026-11-05 |
| 范围 | 023：房间码 10 分钟有效、倒计时、过期提示 |
| 设计/计划 | [设计](../design/023-房间码过期与倒计时设计.md) / [计划](../plans/023-房间码过期与倒计时实施计划.md) |

## 1. 结论

**通过。** 房间码默认 10 分钟有效；家长端等待页显示剩余时间；到期后停止轮询并提示重新创建；过期加入返回 409。

## 2. 交付

- 迁移 `00019_battle_room_expiry.sql`：`join_code_expires_at` + 部分索引；
- 创建 room 写入 `now()+10 分钟`，同设备模式为 NULL；
- `State` 返回 `join_code_expires_at`（仅未结束 room）；
- `Join` 过期返回 409 `ROOM_EXPIRED`，状态保持 waiting；
- 家长端等待页「房间码剩余 mm:ss」，到期显示「房间码已过期，请重新创建」并停止轮询。

## 3. 验证

```text
go test ./...                                      passed
flutter test                                       38 passed
```

接口实测：

```text
创建房间 → status=waiting，expires_at=2026-11-05T12:44:06（约 10 分钟）
客户端显示「房间码剩余 09:53」
修改过期时间为过去 → Join 返回 409 ROOM_EXPIRED
```

模拟器截图：`docs/acceptance/phase23/01-room-countdown.png`、`02-room-expired.png`。

## 4. 限制

- 过期房间未自动清理，仅拒绝加入；
- 倒计时依赖服务端时间，客户端仅展示；
- 已加入房间的重连不受过期影响。

## 5. 后续

- 定时清理过期房间；
- 房间码手动关闭/重新生成；
- WebSocket 替代轮询。
