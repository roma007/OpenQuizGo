# 068 时钟漂移与签名校验验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.60.0（TV 0.3.8） |
| 日期 | 2026-11-05 |
| 范围 | 068：clock_skew 与 package_tampered 本地检测 |
| 设计/计划 | [设计](../design/068-时钟漂移与签名校验设计.md) / [计划](../plans/068-时钟漂移与签名校验实施计划.md) |

## 1. 结论

**通过（单元测试 + 构建级）。** TV 心跳时比较服务端时间，偏差 >5 分钟上报 `clock_skew`（每小时最多一次）；支持通过构建参数注入签名 SHA-256，启动时校验并上报 `package_tampered`。

## 2. 交付

- 心跳成功后比较 `server_time`，偏差 >5 分钟上报，1 小时内去重；
- `-PexpectedSignature=<sha256>` 注入 `BuildConfig.EXPECTED_SIGNATURE`；
- 启动时校验 APK 签名证书 SHA-256（API 28+ signingInfo，旧版 signatures）；
- 不匹配上报 `package_tampered`（含 expected/actual）；
- TV 版本 0.3.8（versionCode 11）。

## 3. 验证

```text
./gradlew :app:testDebugUnitTest        BUILD SUCCESSFUL
./gradlew :app:assembleRelease          BUILD SUCCESSFUL
```

单元测试覆盖时钟差计算与空/非法时间忽略。

## 4. 限制

- `uninstall_attempt` 无法由自身 App 感知，仍需 Device Owner/系统权限，单列待办；
- debug/未配置签名 pin 时跳过签名校验；
- 未做 TV 实机时钟篡改/重签验证；
- 上报失败仅记本地日志。

## 5. 后续

- Device Owner 卸载拦截；
- 服务端下发签名/版本基线；
- 时钟漂移用户提示。
