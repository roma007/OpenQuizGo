# 056 电视端家长 PIN 解锁验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.48.0（TV 0.3.6） |
| 日期 | 2026-11-05 |
| 范围 | 056：电视端家庭 PIN 直接解锁 |
| 设计/计划 | [设计](../design/056-电视端家长PIN解锁设计.md) / [计划](../plans/056-电视端家长PIN解锁实施计划.md) |

## 1. 结论

**通过（接口/单元测试 + 构建级）。** 新增设备鉴权解锁接口，电视拦截页可通过数字键盘输入家庭 owner PIN 直接解锁 10 分钟；原家长端轮询放行保留。无 TV 实机，未做遥控键盘走查。

## 2. 交付

- `POST /v1/device/unlock`（设备鉴权）body `{pin,minutes}`；
- 通过孩子所属家庭 owner 账号校验 PIN，复用解锁频控、规则额度、家长授权与审计；
- TV 拦截页家长解锁增加数字键盘、清除/删除、解锁按钮；
- 成功后 `adoptGrant` 并关闭拦截页；失败显示错误；
- TV 版本 0.3.6（versionCode 9）。

## 3. 验证

```text
gofmt / go build                       通过
go test -p 1 ./...（测试库/Redis）      全部通过
TV testDebugUnitTest / assembleRelease BUILD SUCCESSFUL
未带设备令牌 POST /device/unlock        401
```

watch 测试覆盖错误 PIN 拒绝、正确 PIN 签发 10 分钟授权、active grant 可读取。

## 4. 限制

- PIN 为家庭 owner 的家长 PIN，非 owner 家长独立 PIN 不生效；
- 电视遥控输入较慢，固定 10 分钟；
- 未做 TV 实机遥控/视觉验收；
- 未实现多家长 PIN 与解锁通知。

## 5. 后续

- 选择解锁时长；
- 多家长 PIN；
- 解锁审批通知与实机验收。
