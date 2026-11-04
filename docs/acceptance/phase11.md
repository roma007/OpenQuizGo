# 011 专题题包组卷验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.11.0 |
| 日期 | 2026-11-04 |
| 范围 | 011：专题题包接入组卷 |
| 设计/计划 | [设计](../design/011-专题题包组卷设计.md) / [计划](../plans/011-专题题包组卷实施计划.md) |

## 1. 结论

**通过。** 在线且匹配的专题包会优先参与组卷；下线、过期、学科/学段不匹配或题目不足 3 道时自动回退普通组卷。

## 2. 交付

- `internal/quiz/repository.go`：`activeTopicPack`（状态/时间/学段/学科/题量匹配）、`topicCandidates`（专题候选题、近 30 天排除）；
- `internal/quiz/selector.go`：专题抽题分支、`topic_mode`/`repeat_question:topic`/`topic_shortage` 警告；
- 组卷结果新增 `topic_id`、`topic_name`；
- 不新增接口，TV/家长端无协议变化。

## 3. 验证

```text
go test ./...                                         passed
TEST_DATABASE_URL go test ./internal/quiz             passed（含专题匹配用例）
```

专题用例覆盖：

| 场景 | 结果 |
| --- | --- |
| 在线 + 学段/学科匹配 + 5 题 | 选中 5 题，全部来自专题 |
| 专题下线 | 回退普通组卷 |
| 已过期 | 回退普通组卷 |
| 学科不匹配 | 回退普通组卷 |
| 学段不匹配 | 回退普通组卷 |
| 专题仅剩 2 题 | 回退普通组卷 |

接口实测（管理端预览 `POST /v1/admin/quiz/preview`）：

```text
创建并上线专题，绑定 5 道数学题
preview → topic_name=E2E 组卷专题, total=5, selected 全部在专题内
  warnings: ["topic_mode:E2E 组卷专题", "repeat_question:topic"]
专题下线
preview → topic_id 为空, total=5（普通组卷）
```

> `repeat_question:topic` 表示孩子近 30 天做过部分专题题，已在放宽限制后重新使用。

## 4. 限制

- 专题模式不按学科配额抽题，也不再校验加权门槛；
- 同时匹配多个专题时只取 `updated_at` 最新的一个；
- 专题包不足 3 题时不会被启用。

## 5. 后续

- 管理端增加「预览组卷」按钮，直接展示专题模式结果；
- 支持按孩子/设备指定专题包；
- 专题包与会员/付费权益联动。
