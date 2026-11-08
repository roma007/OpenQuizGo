# 073 TV 对战媒体与 SSE 验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.65.0（TV 0.4.1） |
| 日期 | 2026-11-08 |
| 范围 | 073：对战题干/选项媒体、设备 SSE、轮询兜底 |
| 设计/计划 | [设计](../design/073-TV对战媒体与SSE设计.md) / [计划](../plans/073-TV对战媒体与SSE实施计划.md) |

## 1. 结论

**通过（单元测试 + 构建 + 接口冒烟）。**

## 2. 交付

- 对局题目新增 `stem_media` 下发；
- `GET /v1/device/battles/{battleID}/events` 设备鉴权 SSE；
- 家长/设备 SSE 共用 `streamEvents`，20 秒心跳；
- TV 题干图片、选项图片（最多 3 张）与选项音频；
- TV SSE 订阅、版本去重、3 秒重连与状态刷新兜底；
- 共享 `RemoteImage` / `resolveMediaUrl`，答题页与对战页复用；
- TV 版本 0.4.1（versionCode 14）。

## 3. 验证

```text
go test -p 1 ./...（测试库/Redis）     全部通过
go vet ./...                          通过
TV testDebugUnitTest                  BUILD SUCCESSFUL
TV assembleRelease                    BUILD SUCCESSFUL

接口冒烟：
  无令牌 GET /device/battles/{id}/events        401
  不存在/他人对局                                404 BATTLE_NOT_FOUND
  本孩子对局                                    200，返回
    id: 0
    event: update
    data: {"version":0}
```

## 4. 限制

- 未做 TV 实机遥控、图片解码与音频播放走查；
- SSE 断线重连为 3 秒退避，未做指数退避；
- 图片无缓存与大小限制；
- 家长侧媒体渲染沿用既有实现，本次未改动。

## 5. 后续

- TV 实机验收；
- 图片缓存与尺寸限制；
- 家长侧对局媒体一致性检查。
