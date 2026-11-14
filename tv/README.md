# 电视时间管家（Android TV 守护端）

正式电视端工程。包名 `com.openquizgo.quiz`（`namespace` 仍为 `com.openquizgo.guard`）。

> 创维等机型会拦截部分包名/新包安装，`com.openquizgo.guard`、`com.openquizgo.tv` 实测会被
> `INSTALL_FAILED_USER_RESTRICTED` 拦截；`com.openquizgo.quiz` 可安装。若更换包名需重新实测。

## 当前模式（080，服务端模式）

TV 端是守护端，**所有业务数据以服务端为唯一真源**，本地只保存设备令牌与最小状态。

- **激活绑定**：首屏出 6 位激活码，家长端输入并选择孩子后，TV 自动拿到设备令牌。
- **规则来自服务端**：能不能看、看多久、答对解锁多少分钟（默认 15）、每日上限、
  允许时段，全部由 `/v1/child/context` 下发的 `Rule` 决定，TV 不硬编码任何时长。
  家长端改规则后最迟 60 秒生效。
- **题库与判分在服务端**：`/v1/quiz/sessions` 组卷下发，`/answers` 判分，`/finish` 达标后签发解锁授权。
  APK 内不含题库。
- **观看计量**：播放期间每 30 秒一次 `/v1/watch/grants/{id}/heartbeat`，暂停不发心跳。
- **拦截**：额度用尽（`daily_limit`）或不在允许时段（`time_window`）进入拦截页；
  可用家庭 PIN 调 `/v1/device/unlock` 现场解锁，也可由家长端远程解锁。
- **断网降级**：联网时预取 `/v1/offline/lease`，断网只能消耗租约内的分钟数，
  **不产生新的答题解锁**；恢复网络后 `/v1/watch/logs` 对账扣减当日额度。
- **状态上报**：`/v1/device/capability` 上报守护模式与权限状态，权限被关时 `/v1/device/tamper`。

> 077（2026-11-12）曾一度改为纯本地单机；080 起回归服务端模式，本地题库与本地判分已删除。

## 构建

```bash
cd tv
./gradlew :app:assembleDebug
# 产物：app/build/outputs/apk/debug/app-debug.apk

./gradlew :app:assembleRelease
# 产物：app/build/outputs/apk/release/app-release.apk
```

### 服务端地址（`BuildConfig.API_BASE_URL`）

默认 `http://100.65.110.80:8082/`（服务器 Tailscale 地址）。覆盖方式：

```bash
./gradlew :app:assembleRelease -PapiBaseUrl=https://quiz.openapp.asia/
```

- 备案通过前，nginx 只放行 `127.0.0.1` 与 `100.64.0.0/10`，**家庭电视必须接入同一 tailnet 才能连上**。
- 备案通过、80/443 放开后，用 `-PapiBaseUrl=https://quiz.openapp.asia/` 重新构建即可对外。
- debug 构建通过 `app/src/debug/AndroidManifest.xml` 允许明文 HTTP；release 请用 HTTPS。

## 测试

```bash
./gradlew :app:testDebugUnitTest
```

覆盖：状态机迁移（UNBOUND/IDLE/WATCHING/QUIZ_REQUIRED/QUIZZING/GRANTED/BLOCKED）、
授权耗尽、领取授权、401 降级、离线租约、令牌加解密、API 工厂、诊断上报、日志滚动。

## 安装与调试

安装后必须启动一次 App（Android 新装应用的 stopped 状态限制），之后即可后台常驻并在开机时自启。
可以使用一键脚本：

```bash
# 安装 -> 无界面初始化（不显示 App 界面）；ENABLE_A11Y=1 同时通过 adb 开启无障碍
ENABLE_A11Y=1 ./scripts/install_and_start.sh
```

手动安装：

```bash
adb connect <TV_IP>:5555
adb install -r app/build/outputs/apk/debug/app-debug.apk

# 若遇到 INSTALL_FAILED_USER_RESTRICTED，可重试或：
adb push app/build/outputs/apk/debug/app-debug.apk /data/local/tmp/q.apk
adb shell pm install -r -d /data/local/tmp/q.apk

# 启动
adb shell am start -n com.openquizgo.quiz/com.openquizgo.guard.ui.MainActivity
```

### 立即触发答题（仅 debug）

```bash
adb shell am broadcast \
  -a com.openquizgo.guard.DEBUG_FORCE_QUIZ \
  -n com.openquizgo.quiz/com.openquizgo.guard.debug.DebugReceiver
```

### 关键权限

- **网络**：`INTERNET` + `ACCESS_NETWORK_STATE`（服务端模式必需）。
- **无障碍服务**：到点自动拉起答题界面、拦截页回到前台。未开启时首页会提示。
- **使用情况访问**（`PACKAGE_USAGE_STATS`）：识别前台视频应用，用于 `scope_mode=allowed_apps`。

> 注意：部分机型更新 APK 后会注销无障碍服务，需重启电视或从设置重新开启。

## 目录

```
app/src/main/java/com/openquizgo/guard/
├── GuardApplication.kt
├── di/AppContainer.kt
├── data/remote/ApiFactory.kt, GuardApi.kt      # Retrofit + 全部服务端接口
├── data/local/GuardPreferences.kt, TokenCipher.kt
├── data/foreground/ForegroundDetector.kt       # UsageStats + 媒体播放检测
├── data/quiz/QuizRepository.kt                 # 会话创建/作答/结束
├── data/watch/WatchRepository.kt               # 授权、心跳、租约、对账
├── domain/GuardEngine.kt                       # 守护状态机（服务端为准）
├── platform/GuardService.kt                    # 前台服务 + 看门狗
├── platform/GuardAccessibilityService.kt
└── ui/MainActivity.kt, QuizActivity.kt, BlockerActivity.kt, BattleActivity.kt
```

## 设备令牌

令牌用 Android Keystore（别名 `guard_token`）+ AES-GCM 加密后存入 DataStore，
明文不落盘、不进日志。异常处理见 `data/local/TokenCipher.kt`。

设计与计划：[`080-TV端回归服务端模式设计`](../docs/design/080-TV端回归服务端模式设计.md)。
