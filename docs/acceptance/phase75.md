# 075 GuardEngine 可测性重构验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.65.3（TV 0.4.4） |
| 日期 | 2026-11-11 |
| 范围 | 075：端口接口抽取、时钟注入、GuardEngine 单测 |
| 设计/计划 | [设计](../design/075-GuardEngine可测性重构设计.md) / [计划](../plans/075-GuardEngine可测性重构实施计划.md) |

## 1. 结论

**通过（单测 + 构建 + 模拟器冒烟）。** 重构未改变任何状态迁移与轮询/心跳间隔，074 的关键分支已被单测固定。

## 2. 交付

- 新增 `domain/GuardPorts.kt`：
  - `GuardStorage`：令牌、快照、授权、租约、离线待补报；
  - `ForegroundSource`：前台/媒体状态；
  - `WatchGateway`：有效授权、心跳、租约、离线补报。
- `GuardPreferences`、`ForegroundDetector`、`WatchRepository` 显式实现对应端口，生产逻辑未改；
- `GuardEngine` 改为依赖端口，注入 `clock: () -> Long`，移除未使用的 `DiagnosticsReporter` 参数；
- `AppContainer` 同步删除多余实参；
- 新增 `GuardEngineTest`（9 个用例）与 `kotlinx-coroutines-test` 依赖；
- 开启 `unitTests.isReturnDefaultValues`，使 `android.util.Log` 在 JVM 下安全返回；
- TV 版本 0.4.4（versionCode 17）。

## 3. 单元测试

```text
./gradlew :app:testDebugUnitTest
BUILD SUCCESSFUL

GuardEngineTest（9/9）
  pollAdoptsParentGrantInQuizRequired
  pollAdoptsGrantWhileMediaActiveGoesStraightToWatching
  pollKeepsQuizRequiredWhenNoGrant
  pollUnauthorizedMovesToUnbound
  heartbeatRevokedMovesToBlocked
  grantExhaustionReturnsToQuizRequired
  quizPassedAdoptsGrant
  adoptGrantWhileMediaActiveEntersWatching
  restoreExpiredGrantFallsBackToQuizRequired

其余既有用例（9/9）
  LogStoreTest 4、ApiFactoryTest 3、DiagnosticsReporterTest 2
```

共计 18 个单测，0 失败。

## 4. 构建

```text
:app:assembleDebug（http://10.0.2.2:18080/）   BUILD SUCCESSFUL
:app:assembleRelease（https://tv.openquizgo.example/） BUILD SUCCESSFUL
aapt2 dump badging → versionCode=17 versionName=0.4.4
```

## 5. 模拟器冒烟

- 覆盖安装 0.4.4 后保留绑定，启动即拉取有效授权；
- 家长端签发 5 分钟授权后，电视显示「剩余授权 5 分 0 秒」；
- 模拟媒体播放后 `state IDLE -> WATCHING`，停止播放后 `WATCHING -> IDLE`；
- 日志与 0.4.3 一致，无新增异常。

## 6. 限制

- 单测使用端口替身与虚拟时间，不覆盖真实 Retrofit/OkHttp 序列化；
- `GuardEngine` 的离线租约补报仍未被单测覆盖，留待后续；
- 未新增 instrumentation 测试，真机交互仍依赖人工冒烟。

## 7. 后续

- 为离线补报与租约耗尽补测试；
- 评估把 `DiagnosticsReporter` 重新接入引擎，用于状态机异常上报。
