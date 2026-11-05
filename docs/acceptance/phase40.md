# 040 选项图片验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.38.0（管理端 0.38.0 / TV 0.3.1） |
| 日期 | 2026-11-05 |
| 范围 | 040：选项图片上传、保存、TV 渲染 |
| 设计/计划 | [设计](../design/040-选项图片设计.md) / [计划](../plans/040-选项图片实施计划.md) |

## 1. 结论

**通过（构建级）。** 选项支持可选 `image`；管理端可逐项上传/预览/移除，允许纯图片选项；TV 选项按钮可渲染图片。无 TV 实机，未做视觉验收。

## 2. 交付

- 选项 JSON 增加 `image`；
- 管理端 OptionItem 增加 `image`，选项行上传/预览/移除；
- 保存校验允许 `text` 为空但必须有 `image`；
- TV QuizOption 解析 `image`，选项按钮内渲染（最大高度 100dp）；
- TV 版本 0.3.1（versionCode 4）。

## 3. 验证

```text
npm run build（管理端）                 passed
./gradlew :app:compileDebugKotlin       BUILD SUCCESSFUL
./gradlew :app:assembleRelease          BUILD SUCCESSFUL
```

接口实测：

```text
创建纯图片选项题目 → 201
GET → options[0] = {id:"a", text:"", image:"/v1/media/...png"}
DELETE → 200
```

## 4. 限制

- 仅单张选项图片，未支持多图/选项音频；
- 无图片无障碍描述；
- TV 端未实机验证图片显示。

## 5. 后续

- 多图/选项音频；
- 图片裁剪/压缩；
- TV 真机视觉验收。
