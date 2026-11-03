# OpenQuizGo 家长端（Flutter）

## 运行

```bash
flutter pub get
flutter run --dart-define=API_BASE_URL=http://<服务端地址>:8080/
```

- Android 调试：`flutter run -d <device>`
- 打包：`flutter build apk --debug`
- 开发环境短信验证码固定 `123456`（服务端 `SMS_MODE=mock`）。

### 国内网络

pub.dev 下载慢时使用镜像：

```bash
PUB_HOSTED_URL=https://pub.flutter-io.cn \
FLUTTER_STORAGE_BASE_URL=https://storage.flutter-io.cn \
flutter pub get
```

### 静态检查

工程路径含中文时 `flutter analyze` 的分析服务会崩溃，改用：

```bash
dart analyze lib test
```

### Android 构建依赖

`shared_preferences_android` 依赖 `jni`，需要 NDK `28.2.13676358`：

```bash
sdkmanager "ndk;28.2.13676358"
```

## 结构

```
lib/
├── app.dart              # MaterialApp.router
├── core/                 # 配置、网络、存储、主题、Provider
└── features/             # auth / child / device / report / settings
```

## 状态

- T1 工程骨架、T2 登录与令牌刷新已完成；
- 后续任务见 `docs/plans/003-家长端 App 实施计划.md`。
