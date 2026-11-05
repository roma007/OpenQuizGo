class AppConfig {
  /// 通过 `--dart-define=API_BASE_URL=https://...` 指定。
  static const apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://127.0.0.1:8080/',
  );

  static const appVersion = '0.29.0';
}
