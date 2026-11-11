# 074 TV 待答题状态感知家长解锁验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.65.2（TV 0.4.3） |
| 日期 | 2026-11-11 |
| 范围 | 074：`QUIZ_REQUIRED` 状态下轮询家长新授权并恢复 |
| 设计/计划 | [设计](../design/074-TV待答题感知家长解锁设计.md) / [计划](../plans/074-TV待答题感知家长解锁实施计划.md) |
| 环境 | 创维/酷开 3T223_P31，Android 9（SDK 28），Wi-Fi 2.4GHz |

## 1. 结论

**通过（真机验证 + 构建 + 单测）。**

真机上家长解锁后，电视分别在 **7.3 秒**、**3.6 秒**内采纳并恢复，满足 ≤10 秒的验收标准。

## 2. 交付

- `GuardEngine` 新增 `unlockPollJob`：仅在 `QUIZ_REQUIRED` 且无可用授权时每 10 秒轮询 `GET /v1/watch/grants/active`；
- `syncActiveGrant` 增加 `syncingGrant` 互斥，避免与 30 秒心跳并发重复请求；
- 新增 `quiet` 模式：待答题轮询的空结果不打印 WARN，避免日志刷屏；
- 401 时进入 `UNBOUND`，避免令牌失效后无限轮询；
- 采纳授权时若媒体仍在播放，直接进入 `WATCHING`，避免误提示「按确认键继续观看」；
- TV 版本 0.4.3（versionCode 16）。

## 3. 验证

```text
TV :app:testDebugUnitTest              BUILD SUCCESSFUL
TV :app:assembleDebug（局域网地址）      BUILD SUCCESSFUL
TV :app:assembleRelease                BUILD SUCCESSFUL
```

### 3.1 真机正测

```text
11-11 11:33:23.893 I grant   : refreshed id=2c26b5e4-... remaining=300 status=active quiet=true
11-11 11:33:23.945 I OQG_GUARD: state QUIZ_REQUIRED -> GRANTED
```

- 解锁签发到电视采纳：**3.6 秒**（前一轮为 7.3 秒）；
- 电视界面：守护状态「已授权」，剩余授权「5 分 0 秒」；
- 对照截图：
  - [解锁前：需要答题、0 分 0 秒](phase74/01-quiz-required.png)
  - [解锁后：已授权、5 分 0 秒](phase74/02-unlock-adopted.png)

### 3.2 反向验证

- 授权过期并撤销后重启电视端：状态保持「需要答题」，30 秒内无状态变化；
- 轮询空结果只在启动恢复时打印一次 `no active grant`，10 秒轮询本身静默；
- 无重复 Toast、无状态抖动。

### 3.3 回归

- 无障碍重新开启后，`quiz required, armed for confirm key` 正常；
- 答题提示与 90 秒兜底代码未改动，沿用 0.4.2 真机结论；
- `BLOCKED` 状态逻辑未改动，阻挡页家长解锁入口保持原行为。

## 4. 限制

- 未新增 `GuardEngine` 单测：其依赖均为具体类（`GuardPreferences`、`ForegroundDetector`、`WatchRepository`），抽取接口超出本次范围；
- 轮询间隔固定 10 秒，暂未做指数退避；
- 真机测试使用 debug 包（release 仅允许 HTTPS），版本号与 release 一致。

## 5. 后续

- 评估长时间停留时 10s → 30s 退避；
- 评估是否给 `BLOCKED` 也加轮询（当前由阻挡页入口覆盖）；
- 在真实第三方视频 App 上复核 `isMusicActive` 的媒体识别覆盖度。
