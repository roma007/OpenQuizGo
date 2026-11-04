# OpenQuizGo 文档中心

本项目遵循两条铁律，详见根目录 [`AGENTS.md`](../AGENTS.md)：

1. 所有对话都留档；
2. 所有开发都先出设计方案与实施计划，并按计划推进。

## 目录导航

| 目录 | 用途 | 入口 |
| --- | --- | --- |
| `docs/idea.txt` | 产品需求草案（第 4–8 节，待补 1–3 节） | [idea.txt](idea.txt) |
| `api/openapi.yaml` | OpenAPI 3.0.3 接口契约 | [openapi.yaml](../api/openapi.yaml) |
| `docs/data-model.md` | 数据模型与迁移说明 | [data-model.md](data-model.md) |
| `docs/deployment.md` | 部署与运维手册 | [deployment.md](deployment.md) |
| `docs/security-checklist.md` | 上线安全自查 | [security-checklist.md](security-checklist.md) |
| `docs/acceptance/phase1.md` | 第一期验收报告 | [phase1.md](acceptance/phase1.md) |
| `docs/conversations/` | 对话留档（Markdown + 原始 JSON） | [INDEX.md](conversations/INDEX.md) |
| `docs/design/` | 设计方案 | [TEMPLATE.md](design/TEMPLATE.md) |
| `docs/plans/` | 实施计划 | [TEMPLATE.md](plans/TEMPLATE.md)、[BACKLOG.md](plans/BACKLOG.md) |

## 常用命令

```bash
# 归档对话（会话结束前执行；会话开始时执行 --all 补档）
node scripts/archive-conversation.mjs
node scripts/archive-conversation.mjs --all
```

## 项目状态

| 编号 | 主题 | 设计 | 计划 | 状态 |
| --- | --- | --- | --- | --- |
| — | 项目规范与文档机制建立 | — | — | done |
| 001 | 项目总体设计（答题解锁电视使用时长） | [设计](design/001-项目总体设计方案.md) | [计划](plans/001-项目总体实施计划.md) | in-progress（T1–T12 完成，T0 首台通过） |
| 002 | Android TV 守护端 | [设计](design/002-Android TV 守护端设计.md) | [计划](plans/002-Android TV 守护端实施计划.md) | done（T1–T11，单机型，v0.2.0） |
| 003 | 家长端 App | [设计](design/003-家长端%20App%20设计.md) | [计划](plans/003-家长端%20App%20实施计划.md) | done（v0.3.1，模拟器走查） |
| 004 | 家长端告警中心 | [设计](design/004-家长端告警中心设计.md) | [计划](plans/004-家长端告警中心实施计划.md) | done（v0.4.0） |

> 新增设计/计划后，请同步更新此表。
