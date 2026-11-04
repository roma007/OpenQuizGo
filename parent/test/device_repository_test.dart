import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/device_repository.dart';

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
  test('设备列表、绑定与解绑请求', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? bindBody;
    String? deletedPath;
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      if (request.method == 'GET' && request.uri.path == '/v1/family/devices') {
        request.response.write(
          jsonEncode([
            {
              'id': 'd1',
              'device_id': 'tv-1',
              'platform': 'tv',
              'child_id': 'c1',
              'child_name': '小明',
              'bound_at': '2026-11-04T08:00:00Z',
            },
          ]),
        );
      } else if (request.method == 'POST' &&
          request.uri.path == '/v1/family/devices/activate') {
        bindBody =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        request.response.write(jsonEncode({'status': 'confirmed'}));
      } else if (request.method == 'DELETE' &&
          request.uri.path.startsWith('/v1/family/devices/')) {
        deletedPath = request.uri.path;
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
    final repository = DeviceRepository(api);

    final devices = await repository.listDevices();
    expect(devices, hasLength(1));
    expect(devices.first.childName, '小明');
    expect(devices.first.platform, 'tv');

    await repository.activate(code: '123456', childId: 'c1');
    expect(bindBody?['code'], '123456');
    expect(bindBody?['child_id'], 'c1');

    await repository.unbind('d1');
    expect(deletedPath, '/v1/family/devices/d1');
  });
}
