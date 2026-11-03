import '../api/api_client.dart';
import '../api/models.dart';
import '../storage/token_store.dart';

class AuthRepository {
  AuthRepository(this._api, this._store);

  final ApiClient _api;
  final TokenStore _store;

  Future<void> requestCode(String phone) async {
    await _api.postJson('/v1/auth/sms/code', {'phone': phone}, noAuth: true);
  }

  Future<void> login({required String phone, required String code}) async {
    final json = await _api.postJson('/v1/auth/login', {
      'phone': phone,
      'code': code,
    }, noAuth: true);
    final result = LoginResult.fromJson(json);
    await _store.save(access: result.accessToken, refresh: result.refreshToken);
  }

  Future<void> logout() async {
    try {
      await _api.postJson('/v1/auth/logout', {});
    } catch (_) {
      // 本地退出优先。
    }
    await _store.clear();
  }
}
