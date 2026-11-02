# OpenQuizGo 文档中心

本项目遵循两条铁律，详见根目录 [`AGENTS.md`](../AGENTS.md)：

1. 所有对话都留档；
2. 所有开发都先出设计方案与实施计划，并按计划推进。

## 目录导航

| 目录 | 用途 | 入口 |
| --- | --- | --- |
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
| 001 | 待定（等待首个需求） | — | — | — |

> 新增设计/计划后，请同步更新此表。
