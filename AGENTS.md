# OpenQuizGo 项目规约（AGENTS.md）

本文件是项目的最高约束。任何会话、任何代理、任何贡献者开始工作前，必须先阅读并遵守。

---

## 一、项目铁律

### 铁律 1：所有对话必须留档

- 每一次会话（无论长短、无论是否产生代码）都必须归档到 `docs/conversations/`。
- 归档内容 = **可读的 Markdown 记录** + **原始 JSON**（`docs/conversations/raw/`），二者缺一不可。
- 归档时机：
  - 会话结束前必须归档；
  - 长会话在每个里程碑完成后增量归档（重跑脚本会覆盖同一会话的旧归档）。
- 归档命令：

  ```bash
  node scripts/archive-conversation.mjs              # 归档当前项目最近一次会话
  node scripts/archive-conversation.mjs --session ses_xxx   # 指定会话
  node scripts/archive-conversation.mjs --all        # 全量补档
  node scripts/archive-conversation.mjs --sanitize   # 脱敏后归档
  ```

- **会话开始时先补档**：执行一次 `--all`，确保上一次会话的收尾消息也进入归档。
- 归档文件不得手工删改；如需更正，在文末追加「归档勘误」小节并注明时间。
- `docs/conversations/INDEX.md` 由脚本自动维护，是对话检索入口，不要手工编辑。

### 铁律 2：先设计、再计划、后实施

任何功能开发、重构、架构调整，必须依次产出并推进：

1. **设计方案** → `docs/design/NNN-<主题>.md`（模板：`docs/design/TEMPLATE.md`）
   - 必须包含：背景与目标、需求/验收标准、方案对比与选型、总体架构、数据模型、接口设计、非功能要求、风险与对策、未决问题。
2. **实施计划** → `docs/plans/NNN-<主题>.md`（模板：`docs/plans/TEMPLATE.md`）
   - 必须包含：任务拆分（含依赖与顺序）、每项任务的验收标准、验证方式、里程碑、回滚方案。
3. **按计划实施** → 在计划文件中实时更新任务状态、实际耗时、偏差与决策记录。
4. **验收与复盘** → 回填实际结果、遗留问题与后续建议。

**禁止跳过设计与计划直接写代码。** 计划未经确认（状态不是 `approved`）不得进入实施。

**例外（轻量变更）**：错别字、注释、纯文档修正、无行为变化的单行修改，可免设计与计划，但必须在 `docs/plans/BACKLOG.md` 登记一行（日期 / 内容 / 原因）。

---

## 二、文档体系

```
docs/
├── README.md                 # 文档导航与项目状态总览
├── conversations/            # 对话留档（铁律 1）
│   ├── INDEX.md              # 自动生成的归档索引
│   ├── raw/                  # 原始 JSON，完整无损
│   └── YYYY-MM-DD-NNN-<标题>.md
├── design/                   # 设计方案（铁律 2 第一步）
│   ├── TEMPLATE.md
│   └── NNN-<主题>.md
└── plans/                    # 实施计划（铁律 2 第二步）
    ├── TEMPLATE.md
    ├── BACKLOG.md            # 轻量变更登记
    └── NNN-<主题>.md
```

- 设计文档与实施计划**共用编号**：`NNN` 为三位递增序号，一个主题一份设计 + 一份计划。
- 文档内互相引用：设计文档头部写「对应计划」，计划文档头部写「对应设计」。

## 三、状态流转

`draft`（草稿）→ `review`（待评审）→ `approved`（已确认）→ `in-progress`（实施中）→ `done`（已完成）

异常分支：`cancelled`（取消，需写明原因）。

- 只有 `approved` 的计划可以开始实施。
- 实施中发现方案需变更：先改设计与计划并重新确认，再继续编码，同时在「决策记录」中留痕。

## 四、命名与提交

- 对话归档：`YYYY-MM-DD-NNN-<会话标题>.md`（脚本自动生成）。
- 设计与计划：`NNN-<kebab-case 或中文主题>.md`，同一主题编号一致。
- 代码提交信息引用编号，例如：`feat(003): 实现题库导入`。
- 提交前自检：本次变更是否有对应设计与计划？计划状态是否已更新？

## 五、当前技术栈与约定

- **服务端**：Go（模块化单体）、PostgreSQL 16、Redis 7、S3 兼容对象存储。
- **电视端**：Android TV（Kotlin + Compose for TV + 无障碍/UsageStats），定位为**守护端**，不承载内容播放；答题时**优先拉起全屏答题 Activity** 使播放器失焦停止（创维实测可用），MediaSession/音频焦点/媒体键作为支持机型上的补充。
- **家长端**：Flutter（Android / iOS）。
- **管控对象**：第三方视频 App（播放器不可控），设备级管控方案见设计 001 第 4.6 节。
- 构建/测试：`make build`、`make test`、`make test-int`（需 `TEST_DATABASE_URL` / `TEST_REDIS_URL`）。
- 本地启动：`make dev`；迁移：`make migrate-up`。
- 接口契约：`api/openapi.yaml`；部署手册：`docs/deployment.md`。

## 六、版本管理与远端

| 远端 | 地址 | 分支 |
| --- | --- | --- |
| `github` | `git@github.com:roma007/OpenQuizGo.git` | `main` |
| `gitee` | `git@gitee.com:roma007007/open-quiz-go.git` | `master` |

- 本地主干分支为 `main`，两个远端保持同步：
  ```bash
  git push github main
  git push gitee main:master
  ```
- 不要在单一远端上产生分叉；如发生冲突，先在本地合并解决，再同时推送。
- 公开仓库注意：归档前若对话含密钥、口令、个人信息，先用 `--sanitize` 归档并复查 `docs/conversations/raw/`，敏感内容不得提交。

