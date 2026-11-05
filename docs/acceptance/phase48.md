# 048 对战事件版本号验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.41.0 |
| 日期 | 2026-11-05 |
| 范围 | 048：SSE 事件版本号与去重基线 |
| 设计/计划 | [设计](../design/048-对战事件版本号设计.md) / [计划](../plans/048-对战事件版本号实施计划.md) |

## 1. 结论

**通过。** 每个对局事件版本由 Redis `INCR` 生成（无 Redis 时本地原子计数兜底），SSE 通过 `id:` 与 `data.version` 暴露；共享订阅和跨实例版本序列保持一致。

## 2. 交付

- 版本键 `battle:events:ver:<battleID>`；
- `publish` 返回并分发版本，通道改为 `chan int64`；
- Redis 消息体为版本字符串，共享 PubSub 解析后广播；
- SSE 输出 `id: <version>` + `event: update` + `data: {"version":<version>}`；
- 订阅建立后下发当前版本基线。

## 3. 验证

```text
gofmt -w internal/battle                                      passed
go build ./...                                                passed
go test ./internal/battle -count=1                            passed
TEST_REDIS_URL=... go test -race ./internal/battle -run ...   passed
```

测试覆盖：本地连续发布递增、两个 broker 经 Redis 版本一致、Redis 失败本地兜底、SSE 格式、共享订阅清理。

## 4. 限制

- Redis 故障期间各实例本地计数可能重复或跳号；
- 本地版本计数 map 未随对局结束回收；
- 通道容量 1，高频事件会合并并跳过中间版本。

## 5. 后续

- SSE Last-Event-ID 补发；
- 事件持久化；
- 版本计数生命周期清理。
