# 第二期 Android TV 守护端验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.2.0 |
| 日期 | 2026-11-03 |
| 范围 | 第二期：Android TV 守护端（P2-T1 – P2-T11） |
| 设计基线 | [002-Android TV 守护端设计](../design/002-Android TV 守护端设计.md) |
| 计划基线 | [002-Android TV 守护端实施计划](../plans/002-Android TV 守护端实施计划.md) |
| 服务端基线 | v0.1.0（[第一期验收](phase1.md)） |

## 1. 结论

**第二期守护端功能验收：通过（单机型）。**

- P2-T1 – P2-T11 全部完成；
- 创维 `3T223_P31`（Android 9 / SDK 28）实机跑通完整闭环：
  激活绑定 → 前台识别 → 观看计时 → 确认键暂停 → 服务端答题 → 授权签发 → 心跳计量 → 暂停不计量 → 额度耗尽 → 重新答题；
- 答题恢复**无广告、进度保留**（同一 MediaPlayer 实例）；
- 阻挡页 HOME 逃逸重新拉起实测 **0.84s**（要求 ≤2s）；
- 升级安装验证通过（0.1.0 → 0.2.0，数据保留）。

**限制**：第二台机型（G4）未提供；第三方视频 App 矩阵测试按用户要求延后。

## 2. 交付物

| 类别 | 交付 |
| --- | --- |
| 安装包 | `dist/OpenQuizGo-TV-0.2.0.apk`（sha256 见同目录 `.sha256`） |
| 源码 | `tv/`（Kotlin + Compose for TV） |
| 文档 | 设计 002 v3、实施计划 002、本报告、部署手册 TV 章节 |
| 服务端接口 | `/v1/quiz/*`、`/v1/watch/*`、`/v1/offline/lease`、`/v1/device/capability`、`/v1/device/tamper` |

## 3. 实机环境

| 项 | 值 |
| --- | --- |
| 机型 | 创维 `3T223_P31` |
| 系统 | Android 9（SDK 28），armv7 |
| 分辨率 | 1920×1080 |
| 包名 | `com.openquizgo.quiz` |
| 签名 | 内部验收包使用 debug 签名（生产签名见 §8） |
| 联调方式 | `adb reverse tcp:18080 tcp:18080` |

## 4. 任务完成情况

| 任务 | 内容 | 状态 | 证据 |
| --- | --- | --- | --- |
| P2-T1 | 工程骨架 | ✅ | 可编译安装；Compose TV 渲染 |
| P2-T2 | 激活与设备身份 | ✅ | 6 位激活码 → 家长确认 → 令牌 Keystore 加密持久化 |
| P2-T3 | 前台识别与计时 | ✅ | 无障碍事件 + UsageStats 校正；识别创维影视/本地媒体 |
| P2-T4 | 守护状态机 | ✅ | 状态机 + 前台服务 + 开机自启 + 状态恢复 |
| P2-T5 | 接管与答题 UI | ✅ | 确认键暂停 → 半透明答题 → 确认键续播；无广告 |
| P2-T6 | 答题流程 | ✅ | 服务端组卷 5 题、逐题判分、结算授权、409 断线恢复 |
| P2-T7 | 授权与心跳 | ✅ | 30s 心跳；暂停 60s 不扣减；断网补报 3 分钟 |
| P2-T8 | 阻挡与看门狗 | ✅ | 全屏阻挡、HOME 重拉 0.84s、家长解锁、90s 超时回退 |
| P2-T9 | 权限引导与诊断 | ✅ | 自检 4/4、诊断日志、能力/篡改上报 |
| P2-T10 | 机型/App 矩阵 | ⚠️ 部分 | 仅创维；第三方 App 矩阵延后 |
| P2-T11 | 发布与验收 | ✅ | 签名 APK、升级验证、本报告 |

## 5. 验收指标

| 指标 | 要求 | 实测 | 结论 |
| --- | --- | --- | --- |
| 接管延迟（确认键 → 答题界面） | ≤1s | 0.58s | ✅ |
| 恢复播放进度 | 100% 保留 | 同一播放器实例、原进度 | ✅ |
| 恢复广告 | 无 | 无广告 | ✅ |
| 看门狗重新拉起 | ≤2s | 0.84s | ✅ |
| 心跳周期 | 30s | 30s | ✅ |
| 暂停计量 | 不计时 | 暂停 60s 剩余不变 | ✅ |
| 断网补报 | 恢复后对账 | 停服 2 分钟后 `reported=3m` | ✅ |
| 超时兜底 | 可接管 | 90.4s 自动拉起答题 | ✅ |
| 升级安装 | 保留数据 | 0.1.0 → 0.2.0 成功，绑定保留 | ✅ |

## 6. 关键链路实测记录

### 6.1 答题恢复（T5/T6）

```text
state WATCHING -> QUIZ_REQUIRED
confirm key, armed=true
quiz activity launched
state QUIZ_REQUIRED -> QUIZZING
answer pos=0..4 correct=true   (7.5 / 3)
quiz finished state=passed grant=20
state QUIZZING -> GRANTED
播放器：ID 55 state:paused -> state:started（同一实例）
```

### 6.2 授权与心跳（T7）

```text
heartbeat status=active remaining=1140 paused=false
...
18:07:28 remaining=540 paused=false   ← 暂停前
18:07:58 remaining=540 paused=true    ← 暂停
18:08:28 remaining=540 paused=true
18:08:33 state IDLE -> WATCHING
offline reported=3m remaining=37m clamped=false
```

### 6.3 阻挡与看门狗（T8）

```text
blocker back ignored
watchdog: blocker missing pkg=com.tianci.movieplatform
foreground=...BlockerActivity       ← 0.84s 后
parent unlock grant=9c77097e… minutes=10
state BLOCKED -> GRANTED
watchdog: quiz fallback after 90454ms
```

### 6.4 能力与篡改（T9）

```text
PUT /v1/device/capability 200
{"usage_stats":true,"accessibility":true,"notifications":true,
 "foreground_service":true,"boot_receiver":true}
POST /v1/device/tamper 200
permission_revoked {"permission":"accessibility"}
```

## 7. 已知限制与偏差

1. **第二机型缺失**：小米/当贝/TCL/索尼等未测试，G4 待补。
2. **第三方 App 矩阵延后**：爱奇艺、腾讯、云视听等未验证；当前仅验证创维影视与本地媒体。
3. **应用更新后无障碍失效**：该机型安装/升级后需重启电视并重设 `enabled_accessibility_services`。
4. **超时兜底可能重播广告**：90s 未按键时全屏接管会停止播放器，属接受的兜底路径。
5. **电视端家长 PIN 未实现**：当前家长解锁依赖家长端/后台签发新授权，电视轮询生效；PIN 输入待 003。
6. **篡改上报仅覆盖权限关闭**：`clock_skew`、`uninstall_attempt`、`package_tampered` 未实现。
7. **日志仅内存**：最近 120 行，未落盘/远程上传。
8. **标准模式防卸载有限**：未申请 Device Owner，卸载依赖家长端告警。

## 8. 发布、升级与回滚

### 8.1 构建

```bash
cd tv
./gradlew :app:assembleRelease -PapiBaseUrl=https://<生产域名>/
```

- 生产签名：仓库根目录放置 `keystore.properties`（已被 `.gitignore` 忽略），字段：
  `storeFile` / `storePassword` / `keyAlias` / `keyPassword`；
- 未提供时回退 debug 签名，仅用于内部验收，**不可用于正式发布**；
- release 构建必须使用 HTTPS 地址（明文 HTTP 仅 debug 构建允许）。

### 8.2 安装 / 升级

```bash
adb install -r dist/OpenQuizGo-TV-0.2.0.apk
# 安装后开启无障碍与使用情况访问：
adb shell settings put secure enabled_accessibility_services \
  com.openquizgo.quiz/com.openquizgo.guard.platform.GuardAccessibilityService
adb shell settings put secure accessibility_enabled 1
adb shell appops set com.openquizgo.quiz GET_USAGE_STATS allow
```

- 同签名升级保留 DataStore（激活令牌、授权、离线记录）；
- 更换签名需先卸载，**会丢失本地绑定**，需重新激活；
- 升级后如无障碍未生效：重启电视并重设上述 settings。

### 8.3 回滚

- 保留上一版 APK，同签名 `adb install -r -d` 回退；
- 服务端可远程吊销授权/解绑设备，客户端下次心跳进入 `BLOCKED`/`UNBOUND`。

## 9. 后续建议

1. 第二机型到位后补 T10 矩阵，重点验证后台 Activity 启动限制与按键过滤能力。
2. 第三方视频 App 逐一验证：暂停/恢复、广告、切集、HDR/会员内容。
3. 家长端（003）上线临时解锁与 PIN，电视端接入。
4. 增加日志落盘与远程诊断上传。
5. 评估 Device Owner / 厂商预装，消除广告重播与卸载绕过。
