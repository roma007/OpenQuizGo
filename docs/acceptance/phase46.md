# 046 新授权关闭旧 active 授权验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.40.0 |
| 日期 | 2026-11-05 |
| 范围 | 046：同一孩子同一设备最多一条 active 授权 |
| 设计/计划 | [设计](../design/046-新授权关闭旧授权设计.md) / [计划](../plans/046-新授权关闭旧授权实施计划.md) |

## 1. 结论

**通过。** 答题签发与家长解锁在插入新授权前，先撤销同一 `child_id + device_id` 的旧 `active` 授权；撤销发生在额度计算之前，替换授权不会损失可签发额度；事务回滚保证失败时旧授权不变。

## 2. 交付

- 新增事务内 `revokeActiveGrants`；
- `issue`：session 重放检查 → 撤销旧 active → 额度计算 → 插入；
- `issueParent`：撤销旧 active → 额度计算 → 插入；
- 不影响其他设备或其他孩子；
- 重放同一 session 直接返回旧记录，不撤销当前 active。

## 3. 验证

```text
gofmt -w internal/watch                    passed
go build ./...                             passed
go test ./internal/watch -count=1          passed（6 项）
```

测试覆盖：

- 连续两次签发后旧授权 `revoked`，仅最新一条 active；
- 旧 15 分钟 + 请求 60 分钟按替换语义得到 60 分钟；
- 其他设备的授权不受影响；
- 重放旧 session 不撤销当前 active；
- 100 并发下仅 1 条 active，且 `已用 + active 分钟 <= 每日额度`。

## 4. 限制

- 撤销不记录独立原因字段；
- 授权历史查询未实现；
- 多设备同时观看仍按设备各自 active 控制。

## 5. 后续

- 授权历史与撤销原因；
- 多设备观看策略；
- 管理端授权排查。
