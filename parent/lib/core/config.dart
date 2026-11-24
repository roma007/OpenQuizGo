class AppConfig {
  /// 默认连生产（086 起 `https://quiz.openapp.asia/` 公网可用，DigiCert 证书）。
  ///
  /// 本地联调用构建参数覆盖，例如：
  /// `flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8089/`
  /// （安卓模拟器访问宿主机；真机填局域网 IP）。
  static const apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'https://quiz.openapp.asia/',
  );
}
