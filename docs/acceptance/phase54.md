# 054 TV 远程诊断上传验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.46.0（TV 0.3.5） |
| 日期 | 2026-11-05 |
| 范围 | 054：TV 日志上传与管理端查询 |
| 设计/计划 | [设计](../design/054-TV远程诊断上传设计.md) / [计划](../plans/054-TV远程诊断上传实施计划.md) |

## 1. 结论

**通过（接口/单元测试 + 构建级）。** TV 诊断页可上传最近日志；服务端保存并支持管理端按设备查询列表与详情；日志大小限制 256KB。无 TV 实机，未做真机上传验证。

## 2. 交付

- 迁移 `00023_device_diagnostic.sql`（已应用到开发库，版本 23）；
- `POST /v1/device/diagnostics`（设备鉴权，1–256KB，note ≤200 字）；
- `GET /v1/admin/diagnostics?device_id=&limit=20`（`family.read`）；
- `GET /v1/admin/diagnostics/{id}`（`family.read`）；
- TV `LogExporter.combined` 合并轮转日志并保留尾部 256KB；
- 诊断页「上传诊断」按钮，ViewModel 展示上传结果。

## 3. 验证

```text
gofmt / go build                          通过
go test -p 1 ./...（测试库/Redis）         全部通过
go run ./cmd/migrate up                   OK 00023
TV testDebugUnitTest / assembleRelease    BUILD SUCCESSFUL
管理端诊断列表（空）                      200 []
不存在详情                                404
```

watch 测试覆盖上传、列表、详情、空内容/超大内容拒绝。

## 4. 限制

- 未做 TV 真机上传；
- 未实现自动上传、保留期清理、管理端下载/搜索；
- 日志合并按字节保留尾部，极端情况下可能截断多字节字符；
- 未实现家长端查看。

## 5. 后续

- 异常自动上传；
- 保留期与容量清理；
- 管理端查看/下载页面。
