import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// 令牌存储抽象，便于测试注入内存实现。
abstract class TokenStore {
  Future<String?> accessToken();

  Future<String?> refreshToken();

  Future<void> save({required String access, required String refresh});

  Future<void> clear();
}

/// 安全存储令牌：Android Keystore / iOS Keychain。
class SecureTokenStore implements TokenStore {
  SecureTokenStore([FlutterSecureStorage? storage])
    : _storage = storage ?? const FlutterSecureStorage();

  final FlutterSecureStorage _storage;

  static const _accessKey = 'access_token';
  static const _refreshKey = 'refresh_token';

  @override
  Future<String?> accessToken() => _storage.read(key: _accessKey);

  @override
  Future<String?> refreshToken() => _storage.read(key: _refreshKey);

  @override
  Future<void> save({required String access, required String refresh}) async {
    await _storage.write(key: _accessKey, value: access);
    await _storage.write(key: _refreshKey, value: refresh);
  }

  @override
  Future<void> clear() async {
    await _storage.delete(key: _accessKey);
    await _storage.delete(key: _refreshKey);
  }
}
