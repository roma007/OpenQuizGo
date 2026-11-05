# 030 对战事件推送验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.28.0（家长端 0.28.0） |
| 日期 | 2026-11-05 |
| 范围 | 030：SSE 对战事件、10 秒兜底轮询 |
| 设计/计划 | [设计](../design/030-对战事件推送设计.md) / [计划](../plans/030-对战事件推送实施计划.md) |

## 1. 结论

**通过。** 房间加入/提交后通过 SSE 即时通知另一端；家长端在孩子提交后 1 秒内进入家长答题；SSE 断开仍保留 10 秒兜底轮询。

## 2. 交付

- 服务端内存事件总线，Join/Submit 成功后发布；
- `GET /v1/family/children/{childID}/battles/{battleID}/events` SSE；
- 初始事件 + 20 秒心跳，断开自动取消订阅；
- 客户端 `ApiClient.eventStream` / `BattleRepository.events`；
- 对战页订阅事件，保留 10 秒兜底轮询；
- OpenAPI 补 SSE 接口。

## 3. 验证

```text
go test ./...                       passed（含事件总线单测）
flutter test                        40 passed
curl SSE + join                    initial + join = 2 个 event: update
模拟器                              孩子提交后 1 秒内切换「家长答题」
```

截图：`docs/acceptance/phase30/01-sse-parent-turn.png`。

## 4. 限制

- 事件总线为单实例内存实现，多实例需 Redis/NATS；
- 事件不携带状态，客户端收到后重新拉取完整状态；
- SSE 断开依赖 10 秒兜底轮询，非即时。

## 5. 后续

- Redis PubSub/WebSocket 多实例；
- 事件版本号与断线补发；
- 推送通知复用事件总线。
