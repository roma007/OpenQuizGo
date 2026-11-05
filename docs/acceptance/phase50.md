# 050 选项音频验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.42.0（管理端 0.42.0 / TV 0.3.4） |
| 日期 | 2026-11-05 |
| 范围 | 050：选项音频上传、保存、试听与 TV 播放 |
| 设计/计划 | [设计](../design/050-选项音频设计.md) / [计划](../plans/050-选项音频实施计划.md) |

## 1. 结论

**通过（构建 + 接口/单元测试级）。** 媒体上传支持 MP3/WAV/M4A/OGG；选项可保存 `audio`；管理端可上传/试听/移除；TV 每个选项提供独立播放按钮。无 TV 实机，未做真机听音验收。

## 2. 交付

- 媒体服务按文件头识别 MP3（ID3/帧同步）、WAV（RIFF+WAVE）、OGG（OggS）、M4A（ftyp）；
- 选项 JSON 增加 `audio`，与 text/image/images 至少一个；
- 管理端选项卡片增加音频上传、`<audio>` 试听、移除；
- TV `QuizOption` 增加 `audio`，选项区域「播放选项音频」+「选择」两个按钮；
- TV 使用 `MediaPlayer` 异步准备播放，切换/离开页面释放；
- TV 版本 0.3.4（versionCode 7）。

## 3. 验证

```text
go test ./internal/media ./internal/question       passed
go test -p 1 ./...（测试库/Redis）                 passed
cd admin-web && npm run build                      passed
cd admin-web && npm run test:e2e                   7/7 passed
TV testDebugUnitTest / assembleRelease             BUILD SUCCESSFUL
```

接口实测：

```text
POST /v1/admin/media（ID3 MP3）  → 200 {content_type:"audio/mpeg", url:"/v1/media/....mp3"}
创建纯音频选项题目              → 201
GET 题目                        → options[0].audio = "/v1/media/....mp3"
DELETE                          → 200
```

单元测试覆盖四种音频文件头识别与未知格式拒绝，以及纯音频/音频+图片选项合法、空白音频地址拒绝。

## 4. 限制

- 未做 TV 实机播放/解码验证；
- 受现有 5MB 上传上限约束；
- 未支持题干音频与音频时长/波形；
- 未实现听力题专用交互（自动播放/重播次数）。

## 5. 后续

- 题干音频；
- 音频时长与波形；
- 听力题自动播放策略与实机验收。
