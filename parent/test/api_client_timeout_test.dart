import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';

class _ThrowingStore implements TokenStore {
  @override
  Future<String?> accessToken() async =>
      throw StateError('keychain unavailable');

  @override
  Future<String?> refreshToken() async => null;

  @override
  Future<void> save({required String access, required String refresh}) async {}

  @override
  Future<void> clear() async {}
}

void main() {
  test('读取令牌抛错时请求立即失败，而不是一直等待', () async {
    final client = ApiClient(_ThrowingStore(), baseUrl: 'http://127.0.0.1:1/');
    final started = DateTime.now();
    await expectLater(
      client.getJson('v1/family/children/x/rule'),
      throwsA(isA<ApiException>()),
    );
    expect(DateTime.now().difference(started).inSeconds, lessThan(3));
  });
}
