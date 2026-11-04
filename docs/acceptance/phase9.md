# 009 孩子端每日一题验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.9.0（家长端） |
| 日期 | 2026-11-04 |
| 范围 | 009：孩子模式 · 每日一题 |
| 设计/计划 | [设计](../design/009-孩子每日一题设计.md) / [计划](../plans/009-孩子每日一题实施计划.md) |

## 1. 结论

**通过。** 家长端内可完成每日一题，积分/连续天数落库，重复作答被唯一约束拦截。

## 2. 交付

- 迁移 `00009_daily_answer.sql`；
- `GET /v1/family/children/{id}/daily`、`POST .../daily/answers`；
- 家长端「每日一题」入口与大字号答题页；
- 每日题目按 `md5(question_id || date)` 确定性选择；答对 +10 分；连续天数按日期计算。

## 3. 验证

```text
go test ./internal/daily       passed
flutter test                   24 passed
npm run build（管理端）         passed
```

接口实测：

```text
GET  daily  → answered=false，题目「导入题 930: 3+9=?」
POST answer → correct=true, points=10
GET  daily  → answered=true, points=10, streak=1, total=10
```

App 实测：

| 步骤 | 结果 |
| --- | --- |
| 孩子卡片「每日一题」 | 进入答题页，显示题干/选项/提交按钮 |
| 选 a.12 → 提交 | 结果页「回答正确 +10 分」 |
| 数据库 | `child_daily_answer` correct=true, points=10 |
| 再次进入 | 显示已答结果与连续天数 |

截图：`docs/acceptance/phase9/01-daily-question.png`、`02-daily-result.png`。

## 4. 限制

- 每日题目仅在 choice/image 中选；
- 孩子模式使用家长令牌，退出由家长控制，暂无儿童锁；
- 连续天数/积分暂无排行榜或兑换。

## 5. 后续

- 每日一题推送提醒；
- 积分兑换/勋章；
- 独立孩子端 / 小程序（需产品与微信主体）。
