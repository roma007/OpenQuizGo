# OpenQuizGo 守护端（Android TV）

正式电视端工程。包名 `com.openquizgo.quiz`（`namespace` 仍为 `com.openquizgo.guard`）。

> 创维等机型会拦截部分包名/新包安装，`com.openquizgo.guard`、`com.openquizgo.tv` 实测会被
> `INSTALL_FAILED_USER_RESTRICTED` 拦截；`com.openquizgo.quiz` 可安装。若更换包名需重新实测。

## 构建

```bash
cd tv
./gradlew :app:assembleDebug
# 产物：app/build/outputs/apk/debug/app-debug.apk
```

API 地址默认 `http://192.168.1.27:8080/`，可通过 Gradle 属性覆盖。

### 本地联调（推荐 adb reverse）

部分电视/防火墙会阻止电视反向访问开发机，用 `adb reverse` 把电视的
localhost 映射到开发机，可绕过该问题：

```bash
adb reverse tcp:18080 tcp:18080
./gradlew :app:assembleDebug -PapiBaseUrl=http://127.0.0.1:18080/
```

debug 构建通过 `src/debug/AndroidManifest.xml` 允许明文 HTTP；release 必须使用 HTTPS。

## 测试

```bash
./gradlew :app:testDebugUnitTest
```

## 安装

```bash
adb connect <TV_IP>:5555
adb install -r app/build/outputs/apk/debug/app-debug.apk

# 若遇到 INSTALL_FAILED_USER_RESTRICTED，可重试或：
adb push app/build/outputs/apk/debug/app-debug.apk /data/local/tmp/q.apk
adb shell pm install -r -d /data/local/tmp/q.apk

# 启动
adb shell am start -n com.openquizgo.quiz/com.openquizgo.guard.ui.MainActivity
```

## 当前进度

- P2-T1 工程骨架：Compose for TV、DataStore、Retrofit、手工 DI、CI
- 下一步 P2-T2：设备激活与令牌

## 目录

```
app/src/main/java/com/openquizgo/guard/
├── GuardApplication.kt
├── di/AppContainer.kt
├── data/local/GuardPreferences.kt
├── data/remote/GuardApi.kt, ApiFactory.kt
└── ui/MainActivity.kt
```
