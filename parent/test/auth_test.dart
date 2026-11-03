import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/auth/auth_controller.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';

class MemoryTokenStore implements TokenStore {
  MemoryTokenStore({this.access, this.refresh});

  String? access;
  String? refresh;

  @override
  Future<String?> accessToken() async => access;

  @override
  Future<String?> refreshToken() async => refresh;

  @override
  Future<void> save({required String access, required String refresh}) async {
    this.access = access;
    this.refresh = refresh;
  }

  @override
  Future<void> clear() async {
    access = null;
    refresh = null;
  }
}

void main() {
  test('AuthController 根据本地令牌恢复登录态', () async {
    final store = MemoryTokenStore(access: 'token', refresh: 'refresh');
    final controller = AuthController(store);
    await controller.restore();
    expect(controller.ready, isTrue);
    expect(controller.signedIn, isTrue);

    await controller.signOut();
    expect(controller.signedIn, isFalse);
    expect(store.access, isNull);
  });

  test('401 自动刷新令牌并重试原请求', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    server.listen((request) async {
      if (request.uri.path == '/v1/auth/refresh') {
        final body = await utf8.decoder.bind(request).join();
        expect(body, contains('r1'));
        request.response.headers.contentType = ContentType.json;
        request.response.write(
          jsonEncode({
            'access_token': 'access2',
            'refresh_token': 'r2',
            'expires_in': 900,
          }),
        );
        await request.response.close();
        return;
      }
      if (request.headers.value('authorization') == 'Bearer access2') {
        request.response.headers.contentType = ContentType.json;
        request.response.write(jsonEncode({'ok': true}));
        await request.response.close();
        return;
      }
      request.response.statusCode = HttpStatus.unauthorized;
      request.response.headers.contentType = ContentType.json;
      request.response.write(
        jsonEncode({'code': 'AUTH_INVALID_TOKEN', 'message': '令牌无效'}),
      );
      await request.response.close();
    });

    final store = MemoryTokenStore(access: 'expired', refresh: 'r1');
    final api = ApiClient(store, baseUrl: 'http://127.0.0.1:${server.port}/');

    final result = await api.getJson('/v1/data');
    expect(result['ok'], isTrue);
    expect(store.access, 'access2');
    expect(store.refresh, 'r2');
  });
}
