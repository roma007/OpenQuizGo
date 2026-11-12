# 电视时间管家（Android TV 本地版）

正式电视端工程。包名 `com.openquizgo.quiz`（`namespace` 仍为 `com.openquizgo.guard`）。

> 创维等机型会拦截部分包名/新包安装，`com.openquizgo.guard`、`com.openquizgo.tv` 实测会被
> `INSTALL_FAILED_USER_RESTRICTED` 拦截；`com.openquizgo.quiz` 可安装。若更换包名需重新实测。

## 当前模式（077）

- **完全离线**：不连接服务器、不需要激活、不需要家长端；安装并开启无障碍服务后即可使用。
- 内置 120 道本地单选题（`app/src/main/assets/questions.json`），不按学龄筛选，全部混合随机出题。
- 首页只有一个开关「定时答题」，默认开启。
- 累计视频播放 **15 分钟**自动弹出 1 道题：
  - 答对：显示解析，点「继续观看」退出，计时清零；
  - 答错：停留本题，返回键无效，直到答对；
  - 切到桌面/关闭答题界面：守护服务看门狗会自动重新拉起。
- 关闭开关后不再弹题；重新开启后从 0 开始计时。
- 开关状态与累计秒数保存在本地 DataStore，重启后恢复。

## 构建

```bash
cd tv
./gradlew :app:assembleDebug
# 产物：app/build/outputs/apk/debug/app-debug.apk

./gradlew :app:assembleRelease
# 产物：app/build/outputs/apk/release/app-release.apk
```

不再需要 `-PapiBaseUrl`，也没有明文 HTTP 配置。

## 测试

```bash
./gradlew :app:testDebugUnitTest
```

覆盖：15 分钟计时、暂停不计时、答对清零、开关关闭、恢复累计时间、题库结构（120 题、4 选项、答案合法）。

## 安装与调试

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

- **无障碍服务**：用于到点自动拉起答题界面。未开启时首页会提示，无法自动弹题。
- 不再需要「使用情况访问」与网络权限。

> 注意：部分机型更新 APK 后会注销无障碍服务，需重启电视或从设置重新开启。

## 目录

```
app/src/main/
├── assets/questions.json                  # 本地题库
├── java/com/openquizgo/guard/
│   ├── GuardApplication.kt
│   ├── di/AppContainer.kt
│   ├── data/local/QuestionBank.kt         # 题库加载与随机取题
│   ├── data/local/LocalSettings.kt        # 开关与计时持久化
│   ├── data/foreground/MediaPlaybackDetector.kt
│   ├── domain/LocalGuardEngine.kt         # 本地状态机
│   ├── platform/GuardService.kt           # 前台服务 + 看门狗
│   ├── platform/GuardAccessibilityService.kt
│   └── ui/MainActivity.kt, QuizActivity.kt
└── res/xml/guard_accessibility.xml
```

## 题库维护

题库为普通 JSON，字段与约束见
[`docs/design/077-TV本地题库与定时答题设计.md`](../docs/design/077-TV本地题库与定时答题设计.md)。
替换 `assets/questions.json` 后重新构建 APK 即可，无需改代码。
