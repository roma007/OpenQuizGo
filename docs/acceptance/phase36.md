# 036 TV 图片题干渲染验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.34.0（TV 0.3.0） |
| 日期 | 2026-11-05 |
| 范围 | 036：TV 答题页渲染 stem_media |
| 设计/计划 | [设计](../design/036-TV图片题干渲染设计.md) / [计划](../plans/036-TV图片题干渲染实施计划.md) |

## 1. 结论

**通过（构建级）。** TV 端解析并展示 `stem_media.url`，相对/绝对 URL 均支持；加载失败有提示，不阻塞答题。无 TV 设备，未做真机视觉验收。

## 2. 交付

- `SessionQuestion` 增加 `stem_media: {url}`；
- `RemoteImage`：IO 下载、Bitmap 渲染、最大高度 240dp；
- 加载中/失败文案；
- TV 版本 0.3.0（versionCode 3）；
- APK `dist/OpenQuizGo-TV-0.3.0.apk`。

## 3. 验证

```text
./gradlew :app:compileDebugKotlin   BUILD SUCCESSFUL
./gradlew :app:assembleRelease      BUILD SUCCESSFUL
```

服务端会话已包含 `stem_media`（既有字段），本次无需改动。

## 4. 限制

- 无 TV 实机/模拟器，未验证实际图片显示与性能；
- 未接图片缓存/采样，大图可能占用内存；
- 仅题干图片，选项图片未支持。

## 5. 后续

- 真机验证与 Coil 缓存；
- 图片预加载；
- 选项图片。
