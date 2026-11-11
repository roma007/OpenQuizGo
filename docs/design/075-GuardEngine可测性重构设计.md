# 075 GuardEngine 可测性重构设计

| 项目 | 内容 |
| --- | --- |
| 编号 | 075 |
| 状态 | done |
| 作者 | AI |
| 创建日期 | 2026-11-11 |
| 最后更新 | 2026-11-11 |
| 对应计划 | [075-GuardEngine可测性重构实施计划](../plans/075-GuardEngine可测性重构实施计划.md) |
| 前置 | 074 待答题感知家长解锁 |

## 1. 背景与目标

- 现状与问题：`GuardEngine` 直接依赖 `GuardPreferences`、`ForegroundDetector`、`WatchRepository` 三个具体类，且内部调用 `System.currentTimeMillis()` 与固定 1s/10s/30s 定时器，JVM 单测无法注入替身或控制时间；074 的解锁轮询、授权采纳、401 降级等关键分支只能靠真机日志验证。
- 本次目标（可验证）：
  1. 抽出三组最小端口接口，生产实现保持不变；
  2. 注入时钟，使授权过期、心跳节流可在虚拟时间下测试；
  3. 新增 `GuardEngineTest`，覆盖待答题轮询、媒体中解锁、401、心跳撤销、授权耗尽等分支；
  4. 不改变任何线上行为与状态迁移语义。
- 非目标（明确不做什么）：
  - 不改服务端、不改 UI、不改轮询/心跳间隔；
  - 不引入 Mockito/MockK 等 mock 框架；
  - 不为 `QuizActivity`、`BattleActivity` 补测试。

## 2. 需求与验收标准

| 编号 | 需求 | 验收标准 | 优先级 |
| --- | --- | --- | --- |
| R1 | 依赖可替换 | `GuardEngine` 构造参数全部为接口/函数类型，生产实现无行为变化 | P0 |
| R2 | 时间可控 | 授权有效期、心跳节流使用注入时钟 | P0 |
| R3 | 轮询分支可测 | 单测覆盖 `QUIZ_REQUIRED` 下 10s 内采纳家长授权 | P0 |
| R4 | 媒体分支可测 | 媒体播放中采纳授权直接进入 `WATCHING` | P0 |
| R5 | 异常分支可测 | 401 进入 `UNBOUND`，心跳撤销进入 `BLOCKED` | P0 |
| R6 | 回归验证 | `:app:testDebugUnitTest` 与 `:app:assembleDebug` 通过 | P0 |

## 3. 方案对比与选型

| 方案 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- |
| A. 抽端口接口 + 注入时钟 | 无第三方依赖，编译期约束，测试稳定 | 需改构造参数与少量类型声明 | 采用 |
| B. MockK/Mockito mock 具体类 | 改动小 | 新增依赖，对 final 类/Android 类型支持不稳，测试可读性差 | 不采用 |
| C. 模拟器 instrumentation 测试 | 最接近真机 | 慢、依赖 ADB/网络，无法覆盖定时器边界 | 不采用 |

选型理由：端口接口是最小侵入且长期可维护的方案；时钟注入只改 6 处 `System.currentTimeMillis()`，风险可控。

## 4. 总体架构

- 模块划分与职责：
  - `domain/GuardPorts.kt`：`GuardStorage`、`ForegroundSource`、`WatchGateway` 三个端口；
  - `GuardPreferences : GuardStorage`、`ForegroundDetector : ForegroundSource`、`WatchRepository : WatchGateway` 仅增加接口声明；
  - `GuardEngine` 构造参数改为端口 + `clock: () -> Long`，移除未使用的 `DiagnosticsReporter`；
  - `AppContainer` 删除 `diagnosticsReporter` 实参。
- 关键流程：状态机、轮询、心跳、离线补报逻辑完全不变。
- 目录结构影响：仅 TV 模块 `domain/`、`di/`、三个 data/platform 类与测试目录。

## 5. 数据模型

无新增实体与存储；`StoredGrant`、`StoredLease`、`PendingOffline` 结构不变。

## 6. 接口设计

| 类型 | 名称 | 入参 | 出参 | 说明 |
| --- | --- | --- | --- | --- |
| Kotlin | `GuardStorage.deviceToken` | — | `Flow<String?>` | 设备令牌 |
| Kotlin | `GuardStorage.guardSnapshot()` | — | `Pair<String, Long>` | 状态与观看秒数 |
| Kotlin | `GuardStorage.storedGrant()` | — | `StoredGrant?` | 本地授权 |
| Kotlin | `GuardStorage.storedLease()` | — | `StoredLease?` | 离线租约 |
| Kotlin | `GuardStorage.pendingOffline()` | — | `PendingOffline?` | 待补报用量 |
| Kotlin | `ForegroundSource.state` | — | `StateFlow<ForegroundState>` | 前台/媒体状态 |
| Kotlin | `WatchGateway.activeGrant()` | — | `WatchGrant?` | 有效授权 |
| Kotlin | `WatchGateway.heartbeat()` | grantId、foregroundApp、paused | `HeartbeatResult` | 心跳 |
| Kotlin | `WatchGateway.offlineLease()` | — | `OfflineLease?` | 租约 |
| Kotlin | `WatchGateway.reportOffline()` | leaseId、entries | `OfflineReportResult` | 补报 |

错误约定：沿用 `WatchApiException.httpStatus`，不新增异常类型。

## 7. 非功能要求

- 性能：无新增运行时开销，接口调用为静态分发。
- 安全与权限：不变。
- 可观测性：日志文案与级别不变。
- 兼容性与可维护性：生产类仅增加接口声明；测试使用 JUnit4 + `kotlinx-coroutines-test`。

## 8. 风险与对策

| 风险 | 影响 | 概率 | 对策 |
| --- | --- | --- | --- |
| 接口与实现签名漂移 | 编译失败 | 低 | 生产类显式实现接口，CI 编译兜底 |
| 虚拟时间与系统时钟不一致 | 测试误判 | 中 | 测试时钟直接读取 `testScheduler.currentTime + base` |
| `android.util.Log` 在 JVM 抛异常 | 单测失败 | 高 | 开启 `unitTests.isReturnDefaultValues` |
| 重构改变状态语义 | 线上回归 | 低 | 仅替换类型与时钟调用，逐行对照 diff；补 8 个用例 |

## 9. 未决问题

- [ ] 是否将 `DiagnosticsReporter` 重新接入引擎用于异常上报（当前未使用，非本次范围）。

## 10. 变更记录

| 日期 | 变更内容 | 原因 |
| --- | --- | --- |
| 2026-11-11 | 初稿并确认 | 074 遗留单测缺口 |
