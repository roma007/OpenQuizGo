# 072 TV 端加入对战房间验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.64.0（TV 0.4.0） |
| 日期 | 2026-11-06 |
| 范围 | 072：TV 输入房间码加入亲子对战（孩子侧） |
| 设计/计划 | [设计](../design/072-TV加入对战房间设计.md) / [计划](plans/072-TV加入对战房间实施计划.md) |

## 1. 结论

**通过（接口/单元测试 + 构建级）。** 新增设备鉴权对战接口；TV 可输入 6 位房间码加入、完成孩子答题、轮询等待家长并展示结果。

## 2. 交付

- `POST /v1/device/battles/join`（设备鉴权 + 房间码）；
- `GET /v1/device/battles/{battleID}`；
- `POST /v1/device/battles/{battleID}/answers`（固定孩子侧）；
- `JoinAsChild` 复用房间加入逻辑，不要求家长账号；
- TV 加入页（屏幕字母数字键盘）、答题页、等待页、结果页；
- 首页「加入亲子对战」入口；
- TV 版本 0.4.0（versionCode 13）。

## 3. 验证

```text
go test -p 1 ./...（测试库/Redis）      全部通过
TV testDebugUnitTest / assembleRelease BUILD SUCCESSFUL
未带设备令牌 POST /device/battles/join  401
```

battle 测试覆盖设备加入、错误房间码、过期房间、孩子提交后进入家长回合。

## 4. 限制

- TV 仅作为孩子侧加入；家长仍在家长端作答；
- 对战题图片未在 TV 端渲染（仅文字/选项）；
- 轮询 2 秒，未接 SSE；
- 未做 TV 实机遥控走查。

## 5. 后续

- 对战题图片/音频渲染；
- SSE 实时事件；
- 家长侧 TV 加入与实机验收。
