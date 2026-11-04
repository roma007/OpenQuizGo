import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/alert_repository.dart';

class _MemoryStore implements TokenStore {
  String? access = 'token';
  String? refresh = 'refresh';

  @override
  Future<String?> accessToken() async => access;

  @override
  Future<String?> refreshToken() async => refresh;

  @override
  Future<void> save({required String access, required String refresh}) async {}

  @override
  Future<void> clear() async {}
}

void main() {
  test('告警列表与已读请求', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    var marked = false;
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      if (request.method == 'GET' && request.uri.path == '/v1/family/alerts') {
        request.response.write(
          jsonEncode({
            'items': [
              {
                'id': 1,
                'type': 'permission_revoked',
                'detail': {'permission': 'accessibility'},
                'reported_at': '2026-11-04T08:00:00Z',
                'child_name': '小明',
                'device_id': 'tv-1',
                'read': false,
              },
            ],
            'unread': 1,
          }),
        );
      } else if (request.method == 'POST' &&
          request.uri.path == '/v1/family/alerts/read') {
        marked = true;
        request.response.write(jsonEncode({'status': 'ok'}));
      } else {
        request.response.statusCode = HttpStatus.notFound;
      }
      await request.response.close();
    });

    final api = ApiClient(
      _MemoryStore(),
      baseUrl: 'http://127.0.0.1:${server.port}/',
    );
    final repository = AlertRepository(api);

    final list = await repository.list();
    expect(list.unread, 1);
    expect(list.items, hasLength(1));
    expect(
      alertTitle(list.items.first.type, list.items.first.detail),
      '无障碍服务被关闭',
    );

    await repository.markAllRead();
    expect(marked, isTrue);
  });
}
