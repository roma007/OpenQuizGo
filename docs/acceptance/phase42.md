# 042 共享 Redis 订阅验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.39.0 |
| 日期 | 2026-11-05 |
| 范围 | 042：同一实例内共享 Redis PubSub |
| 设计/计划 | [设计](../design/042-共享Redis订阅设计.md) / [计划](../plans/042-共享Redis订阅实施计划.md) |

## 1. 结论

**通过。** 同一实例内同一对局的多个 SSE 订阅共享 1 个 Redis PubSub，通过引用计数管理生命周期；Redis 不可用时回退进程内总线，原有行为不变。

## 2. 交付

- `eventBroker` 增加 `shared map[battleID]*sharedSubscription`；
- 首个订阅者创建 PubSub 并等待确认，后续订阅者复用；
- 广播在锁内快照、锁外在途非阻塞投递；
- 最后一个订阅者取消时关闭 PubSub 并清理；
- Redis 发布失败或订阅失败时回退本地总线；
- 并发首订阅允许短暂多建连接，后完成者立即关闭。

## 3. 验证

```text
go build ./...                                                   passed
go test ./internal/battle                                        passed
TEST_REDIS_URL=... go test ./internal/battle -run 'TestRedisEventBroker|TestEventBroker|TestShared'   passed
TEST_REDIS_URL=... go test -race ./internal/battle -run '...'    passed
go vet ./internal/battle                                         passed
```

新增用例覆盖：同 broker 两个订阅者都收到；取消一个后另一个仍收到；全部取消后清理；共享订阅与 Redis 跨实例事件。

## 4. 限制

- 并发首订阅存在瞬时额外连接（功能正确）；
- 最后订阅者取消瞬间到达的事件可能丢失（客户端已断开）；
- 未实现事件版本号与 Redis Cluster 适配。

## 5. 后续

- 事件版本号；
- Redis Cluster；
- 订阅指标与告警。
