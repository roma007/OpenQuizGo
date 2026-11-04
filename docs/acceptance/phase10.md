# 010 专题题包验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.10.0（管理端 0.10.0） |
| 日期 | 2026-11-04 |
| 范围 | 010：专题题包管理与选题 |
| 设计/计划 | [设计](../design/010-专题题包设计.md) / [计划](../plans/010-专题题包实施计划.md) |

## 1. 结论

**通过。** 管理端可创建专题、维护元数据、搜索并绑定题目、上下线、删除；接口与服务层测试覆盖 CRUD、去重、排序与外键校验。

## 2. 交付

- 服务端 `internal/topic`：列表、详情、新建、修改、删除、全量设置题目；
- 路由挂在管理员鉴权组下：`GET/POST /v1/admin/topics`、`GET/PUT/DELETE /v1/admin/topics/{id}`、`PUT /v1/admin/topics/{id}/questions`；
- 管理端「专题题包」菜单：列表、新建/编辑弹窗、搜索选题、删除二次确认；
- 复用现有 `topic_pack` / `topic_question` 表，无新增迁移。

## 3. 验证

```text
go test ./...                                  passed
TEST_DATABASE_URL go test ./internal/topic     passed
npm run build（管理端）                         passed
```

接口实测（管理员 `admin`）：

```text
POST /v1/admin/topics                        201 draft
PUT  /v1/admin/topics/{id}/questions         200 question_count=3 / question_ids 顺序一致
PUT  /v1/admin/topics/{id}                   200 online, question_count=3
GET  /v1/admin/topics/{id}                   200 名称/题目数正确
DELETE /v1/admin/topics/{id}                 200
```

浏览器实测（`http://127.0.0.1:5174/topics`）：

| 步骤 | 结果 |
| --- | --- |
| 新建专题「E2E 寒假专题」 | 列表显示 节日 / 全学科 / 长期 / 免费 / 0 题 / 草稿 |
| 选题搜索「导入题」并添加 3 题 | 保存后题目数变为 3 |
| 编辑为已上线并开启付费 | 列表显示 付费 / 3 题 / 已上线 |
| 删除并确认 | 列表回到「还没有专题题包」 |

> 说明：本次验收时桌面浏览器窗口处于最小化状态，无法截图；以上结果来自浏览器 DOM 文本、页面交互与接口日志。后续在可见窗口下补图。

## 4. 限制

- 专题包尚未接入组卷/每日一题选题，仅完成管理与绑定；
- 选题弹窗按关键词搜索，单次最多展示 20 条；
- 专题范围字段暂只做存储与展示，未参与推荐过滤。

## 5. 后续

- 组卷时支持按专题包抽题；
- 专题包与付费/会员体系联动；
- 批量导入专题题目、拖拽排序。
