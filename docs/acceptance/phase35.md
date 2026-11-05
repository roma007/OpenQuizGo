# 035 题目图片上传验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.33.0（服务端 / 管理端 0.33.0） |
| 日期 | 2026-11-05 |
| 范围 | 035：题干图片上传、静态读取、管理端预览 |
| 设计/计划 | [设计](../design/035-题目图片上传设计.md) / [计划](../plans/035-题目图片上传实施计划.md) |

## 1. 结论

**通过。** 管理端可上传 JPEG/PNG/WebP/GIF 图片（≤5MB），返回公开 URL；题目保存 `stem_media` 后可回读；管理端表单显示上传按钮与预览。

## 2. 交付

- `internal/media`：multipart 上传、类型/大小校验、随机文件名、静态读取；
- `POST /v1/admin/media`（`question.write`）、`GET /v1/media/{name}`；
- 配置 `UPLOAD_DIR`（默认 `./data/uploads`）、`UPLOAD_MAX_MB`（默认 5）；
- QuestionEdit 增加「上传图片」、缩略图预览、移除；
- 保存/编辑回显 `stem_media.url`；
- OpenAPI 补媒体接口。

## 3. 验证

```text
go test ./...                              passed（media 单测）
npm run build（管理端）                     passed
```

接口实测：

```text
上传 67 字节 PNG → {"url":"/v1/media/5b2f...png","content_type":"image/png"}
GET 该 URL → 200 image/png
创建题目 stem_media={url} → 201
GET 题目 → stem_media 回读一致
删除题目 → 200
```

管理端页面：`/questions/new` 存在「上传图片」按钮与 1 个 file input。

## 4. 限制

- 当前本地磁盘存储，未接对象存储/CDN；
- 仅题干单图，选项图片未支持；
- TV 端尚未渲染 `stem_media`（仅管理端预览）；
- 无图片压缩/裁剪。

## 5. 后续

- S3/对象存储与 CDN；
- TV 端图片渲染（Coil）；
- 选项图片与多图；
- 压缩/格式转换与清理任务。
