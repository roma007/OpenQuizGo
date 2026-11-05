# 044 选项多图验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.40.0（管理端 0.40.0 / TV 0.3.2） |
| 日期 | 2026-11-05 |
| 范围 | 044：单个选项最多 3 张图片，兼容旧单图 |
| 设计/计划 | [设计](../design/044-选项多图设计.md) / [计划](../plans/044-选项多图实施计划.md) |

## 1. 结论

**通过（构建 + 单元测试级）。** 选项支持 `images` 数组，旧 `image` 继续兼容；管理端可上传/移除多图；TV 优先渲染多图并回退单图。无 TV 实机，未做视觉验收。

## 2. 交付

- `optionItem` 增加 `images`，保留 `image`；
- 最多 3 张，空 URL 拒绝，`text/image/images` 至少一个；
- 管理端选项卡片支持多图上传、逐张预览/移除，保存写 `images` 并把首图同步到 `image`；
- TV `QuizOption` 增加 `images`，`Row` 最多渲染 3 张（单张最大 90dp），无有效项回退 `image`。

## 3. 验证

```text
gofmt -w internal/question                                  passed
go test ./internal/question                                 passed
go test ./internal/question -run TestValidateOptionImages  6/6 passed
cd admin-web && npm run build                              passed
cd tv && ./gradlew :app:compileDebugKotlin                 BUILD SUCCESSFUL
```

单元测试覆盖：2 张图合法、4 张图拒绝、仅 `image` 合法、纯空选项非法、空白 URL、重复 id。

## 4. 限制

- 未做 TV 实机视觉验收；
- 未执行 `assembleRelease`；
- 历史数据在管理端再次保存时才升级为 `images` + 首图 `image`；
- 未支持选项音频。

## 5. 后续

- 选项音频；
- 图片压缩/裁剪；
- TV 实机验收。
