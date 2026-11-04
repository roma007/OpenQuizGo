# 013 亲子对战验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.13.0（家长端 0.13.0） |
| 日期 | 2026-11-04 |
| 范围 | 013：亲子对战（同设备轮流） |
| 设计/计划 | [设计](../design/013-亲子对战设计.md) / [计划](../plans/013-亲子对战实施计划.md) |

## 1. 结论

**通过。** 家长端可创建 5 题对战，孩子先答、交接后家长答，服务端判分并返回胜负与逐题解析；重复提交被拦截，数据落库一致。

## 2. 交付

- 迁移 `00010_family_battle.sql`：`family_battle`、`family_battle_answer`；
- `internal/battle`：创建、查询、提交、判分、轮次控制；
- 接口：`POST/GET /v1/family/children/{id}/battles`、`POST .../battles/{id}/answers`；
- 家长端孩子卡片「亲子对战」入口，支持答题、交接、结果与逐题解析；
- 未结束前不返回正确答案与解析。

## 3. 验证

```text
go test ./...                                     passed
TEST_DATABASE_URL go test ./internal/battle       passed
flutter test                                      26 passed
npm run build（管理端）                            passed
```

服务端用例覆盖：创建 5 题、孩子提交、重复提交 409、答案缺题 422、家长提交、胜负计算、非本家庭 404、题库不足 409。

模拟器实测（Android TV 平板模拟器）：

| 步骤 | 结果 |
| --- | --- |
| 孩子卡片进入亲子对战 | 显示轮流答题说明与「开始对战」 |
| 开始对战 | 服务端创建对局，显示第 1/5 题 |
| 孩子答完 5 题 | 提交成功，进入交接页，显示孩子得分 5/5 |
| 家长答题 | 5 题全部选择错误选项 |
| 结果页 | 「孩子获胜 5:0」，逐题显示孩子答对/家长答错与正确答案 |

数据库：

```text
family_battle: status=finished, child_score=5, parent_score=0, question_ids=5
family_battle_answer: child 5 行（5 对），parent 5 行（0 对）
```

截图：`docs/acceptance/phase13/01-battle-intro.png` ~ `04-battle-result.png`。

## 4. 限制

- MVP 为同一设备轮流答题，不是实时双设备对战；
- 每题 1 分，暂无计时、连胜或奖励积分；
- 仅支持选择题/图片题；
- 中途退出后重新进入需重新创建对局（当前未做继续入口）。

## 5. 后续

- 第二台设备输入房间码加入；
- 倒计时、连胜、家庭排行榜；
- 对战结果推送与成长报告联动。
