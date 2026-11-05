# 037 多实例事件总线验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.35.0 |
| 日期 | 2026-11-05 |
| 范围 | 037：Redis Pub/Sub 多实例 SSE 事件 |
| 设计/计划 | [设计](../design/037-多实例事件总线设计.md) / [计划](../plans/037-多实例事件总线实施计划.md) |

## 1. 结论

**通过。** 两个 broker 通过 Redis Pub/Sub 可跨实例收发事件；Redis 不可用时回退进程内总线；SSE 协议与心跳不变。

## 2. 交付

- `eventBroker` 支持可选 Redis；
- `PUBLISH battle:events:<id>` / `SUBSCRIBE`，订阅等待确认；
- 发布失败回退本地；
- `battle.New(pool, rdb)`，测试可传 nil；
- SSE 先订阅再发送初始事件，减少竞态。

## 3. 验证

```text
go test ./...                                      passed
TEST_DATABASE_URL+TEST_REDIS_URL go test ./internal/battle
  TestEventBroker        passed（本地）
  TestRedisEventBroker   passed（跨 broker）
curl SSE + join          initial + join = 2 个 event: update（Redis 模式）
```

## 4. 限制

- 每个 SSE 连接一个 Redis 订阅，连接数多时需共享订阅；
- 事件仅唤醒信号，客户端重新拉取状态；
- 未处理 Redis Cluster/断线重连的高级场景。

## 5. 后续

- 共享 PubSub + 本地分发；
- 事件版本号/断线补发；
- Redis Cluster 与监控。
