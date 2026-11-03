# 设备管控 PoC（T0）

验证「第三方视频 App 的暂停/恢复与遮罩拦截」在目标电视上的可行性。纯 Java、零第三方依赖，APK 约 0.8 MB。

## 构建

方式一（Android Studio）：打开 `android/` 目录，直接运行 `app`。

方式二（命令行，需 Gradle 9.x + JDK 17+）：

```bash
cd android
./gradlew assembleDebug
# 产物：app/build/outputs/apk/debug/app-debug.apk
```

> `local.properties` 中的 `sdk.dir` 需指向本机 Android SDK，该文件不入库。

## 安装

```bash
adb connect <电视IP>:5555     # 电视需开启开发者模式与网络调试
adb install -r app-debug.apk
```

## 使用步骤

1. 打开 **OpenQuizGo PoC**。
2. 依次点击并授权：
   - 无障碍服务（识别前台应用）
   - 悬浮窗（全屏遮罩）
   - 通知使用权（读取 MediaSession，用于最精准的暂停/恢复）
   - 使用情况访问（验证前台应用统计）
3. 打开目标视频 App 播放视频，按 Home 回到 PoC（或从最近任务切回）。
4. 依次测试：
   - `A. MediaSession 暂停 / 恢复`
   - `B. 请求音频焦点 / 释放`
   - `C. 媒体键暂停 / 播放`
   - `5. 显示全屏遮罩`（验证是否盖住播放画面、遥控器焦点是否被拦截）
5. 观察界面状态区：
   - 前台应用（无障碍上报）
   - MediaSession 包名与播放状态
   - 每次操作的耗时（毫秒）

日志同时输出到 logcat：

```bash
adb logcat -s OQG_POC
```

## 结果记录表

每个 App 填一行。「暂停/恢复」记录成功与否、耗时与异常现象。

| App | 版本 | MediaSession 可读 | A 暂停 | A 恢复 | B 暂停 | B 恢复 | C 暂停 | C 恢复 | 遮罩覆盖 | 识别延迟 | 备注 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 爱奇艺 | | | | | | | | | | | |
| 腾讯视频 | | | | | | | | | | | |
| 优酷 | | | | | | | | | | | |
| 芒果 TV | | | | | | | | | | | |
| 云视听小电视 | | | | | | | | | | | |
| Netflix | | | | | | | | | | | |
| YouTube | | | | | | | | | | | |

## 判定口径（暂定）

- 暂停成功率 ≥ 90%、恢复成功率 ≥ 85%，且从操作到画面暂停 ≤ 500ms。
- 遮罩需能覆盖全屏播放画面，遥控器方向键/确认键不得穿透到下层 App。
- 关闭无障碍/悬浮窗后，PoC 能检测到（状态区变 ✗）。

## 已知边界

- 仅验证标准模式（无障碍 + 悬浮窗 + 使用统计），不含 Device Owner。
- 部分 App 不暴露 MediaSession 或不响应媒体键，需在表中如实记录。
- 系统设置、HDMI 输入、投屏不在本 PoC 覆盖范围。

---

## 实测记录

### 2026-11-03 · 创维 3T223_P31（Android 9 / SDK 28 / MT9255 / armv7）

| 项 | 结果 |
| --- | --- |
| PoC 安装 | ✅ `adb install` 成功 |
| 第三方视频 App 安装（VLC 3.7.1） | ❌ `INSTALL_FAILED_USER_RESTRICTED`（创维拦截，PoC 本身可装） |
| 悬浮窗权限 | ✅ appops allow |
| 无障碍服务 | ✅ 已绑定，前台包名识别正常 |
| 使用情况访问 | ✅ |
| 通知使用权 | ❌ 系统无该设置页，`enabled_notification_listeners` 写入后不生效 |
| MediaSession | ❌ 播放中 `dumpsys media_session` 为 0 个会话 |
| 音频焦点 | ❌ 播放器收到 `LOSS_TRANSIENT` 但仍为 `state:started` |
| 媒体键 PAUSE/PLAY | ❌ `dispatchMediaKeyEvent` 后仍为 `state:started` |
| 普通悬浮窗覆盖桌面 | ✅ 可覆盖创维桌面 |
| 普通悬浮窗覆盖视频 | ❌ 系统日志：`sky_black: inAddViewBlackList pkg:com.openquizgo.poc`，窗口未添加，声音继续 |
| 无障碍悬浮层（TYPE_ACCESSIBILITY_OVERLAY） | ❌ 同样被系统拦截 |
| 无障碍读取播放器节点 | ❌ `getRootInActiveWindow()` 返回 null，无法点击暂停按钮 |
| ADB 注入 DPAD_CENTER | ✅ 视频暂停（shell 权限；普通 App 无 INJECT_EVENTS，无法复用） |
| 设备 root | ❌ 生产版本，无 su |
| 全屏 QuizActivity 接管 | ✅ 播放器停止并释放 MediaPlayer |
| 退出 QuizActivity 后恢复 | ✅ 进度保留、续播；但创维会重播一次片头广告（无可访问的跳过节点） |
| HOME 逃逸看门狗 | ✅ 拦截模式下 0.5s 内重新拉起答题界面 |

**结论（v4 修订）：悬浮窗与媒体控制路径不可用，但「全屏 Activity 接管」可用——标准模式在该机型成立（受限）。**

失败路径：

1. 创维 `CooCaaWhiteList` / `inAddViewBlackList` 拦截非白名单应用的悬浮窗（含无障碍悬浮层），视频上无法加遮罩；
2. 播放器不暴露 MediaSession，忽略音频焦点与媒体键；
3. 播放器窗口不可被无障碍读取，无法用节点点击兜底；
4. 第三方视频 App 被系统安装策略拦截（VLC 实测）。

成功路径：

1. 由无障碍服务或后台广播拉起全屏 `QuizActivity`，播放器失焦后停止并释放；
2. 退出 Activity 后播放器自动恢复播放；
3. 拦截模式下按 HOME 会被看门狗重新拉起答题界面。
4. 2026-11-03 12:30 用户实际全屏播放复测：拉起答题界面后 MediaPlayer 消失；返回后焦点回到 `DetailVideoActivity`，`MediaPlayer state:started`。
5. 2026-11-03 12:50 正片复测：不透明 Activity 退出后**进度保留（续播）**，但会重新播放一次片头广告；广告窗口无可访问的「跳过」控件。

剩余限制：

- 第三方视频 App 无法侧载，只能使用创维商店/预装应用（爱奇艺、腾讯等需从商店安装）；
- 后台 Activity 启动在 Android 10+ 受限，需逐机型验证；
- 部分播放器退出后可能不自动续播，需按 App 建立恢复策略；
- 创维恢复时会重播片头广告（实测无跳过节点），需要厂商白名单或接受该体验。

### 复测命令

```bash
adb connect <TV_IP>:5555
adb install -r dist/OpenQuizGo-PoC-0.1.3.apk
adb shell appops set com.openquizgo.poc SYSTEM_ALERT_WINDOW allow
adb shell appops set com.openquizgo.poc GET_USAGE_STATS allow
adb shell settings put secure enabled_accessibility_services com.openquizgo.poc/com.openquizgo.poc.GuardAccessibilityService
adb shell settings put secure accessibility_enabled 1
# 打开全屏答题界面（视频会停止，返回后恢复）
adb shell am broadcast -n com.openquizgo.poc/.CommandReceiver -a com.openquizgo.poc.CMD --es cmd open_quiz
# 拦截模式：非本应用前台时自动拉起答题界面
adb shell am broadcast -n com.openquizgo.poc/.CommandReceiver -a com.openquizgo.poc.CMD --es cmd block_on
```
