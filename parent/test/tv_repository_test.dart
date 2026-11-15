import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/tv_repository.dart';

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

Map<String, dynamic> _configJson({bool quizEnabled = true}) => {
  'quiz_enabled': quizEnabled,
  'limit_enabled': true,
  'unlock_minutes': 15,
  'daily_limit': 60,
  'subjects': ['math'],
  'pass_threshold': 3,
  'quiz_scope': 'random',
  'hard_stop_grace': 10,
  'version': 2,
};

Map<String, dynamic> _tvJson({bool quizEnabled = true}) => {
  'id': 'tv-pk-1',
  'name': '客厅电视',
  'device_id': 'tv-living-01',
  'child_id': 'c1',
  'child_name': '小明',
  'grade_tag': 'g3',
  'online': true,
  'used_minutes': 25,
  'config': _configJson(quizEnabled: quizEnabled),
};

void main() {
  test('电视列表、配置、指定孩子、放行与解绑请求', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? configBody;
    Map<String, dynamic>? childBody;
    Map<String, dynamic>? unlockBody;
    String? deletedPath;
    server.listen((request) async {
      final path = request.uri.path;
      request.response.headers.contentType = ContentType.json;
      if (request.method == 'GET' && path == '/v1/family/tvs') {
        request.response.write(jsonEncode([_tvJson()]));
      } else if (request.method == 'GET' && path == '/v1/family/tvs/tv-pk-1') {
        request.response.write(jsonEncode(_tvJson()));
      } else if (request.method == 'GET' &&
          path == '/v1/family/tvs/tv-pk-1/config') {
        request.response.write(jsonEncode(_configJson()));
      } else if (request.method == 'PUT' &&
          path == '/v1/family/tvs/tv-pk-1/config') {
        configBody =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        request.response.write(jsonEncode(_configJson(quizEnabled: false)));
      } else if (request.method == 'PUT' &&
          path == '/v1/family/tvs/tv-pk-1/child') {
        childBody =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        request.response.write(jsonEncode(_tvJson()));
      } else if (request.method == 'POST' &&
          path == '/v1/family/tvs/tv-pk-1/unlock') {
        unlockBody =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        request.response.write(
          jsonEncode({
            'grant_id': 'g1',
            'minutes': 10,
            'remaining_seconds': 600,
          }),
        );
      } else if (request.method == 'DELETE' &&
          path == '/v1/family/tvs/tv-pk-1') {
        deletedPath = path;
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
    final repository = TvRepository(api);

    final tvs = await repository.listTVs();
    expect(tvs, hasLength(1));
    expect(tvs.first.childName, '小明');
    expect(tvs.first.gradeTag, 'g3');
    expect(tvs.first.usedMinutes, 25);
    expect(tvs.first.config.quizEnabled, isTrue);
    expect(tvs.first.config.subjects, ['math']);

    final config = await repository.getConfig('tv-pk-1');
    expect(config.version, 2);
    expect(config.hardStopGrace, 10);

    final updated = await repository.updateConfig(
      deviceId: 'tv-pk-1',
      pin: '1234',
      config: config.copyWith(quizEnabled: false),
    );
    expect(updated.quizEnabled, isFalse);
    expect(configBody?['pin'], '1234');
    final sentConfig = configBody?['config'] as Map<String, dynamic>?;
    expect(sentConfig?['version'], 2);
    expect(sentConfig?['quiz_enabled'], isFalse);

    final assigned = await repository.assignChild(
      deviceId: 'tv-pk-1',
      nickname: '小明',
      gradeTag: 'g3',
    );
    expect(assigned.childName, '小明');
    expect(childBody?['nickname'], '小明');
    expect(childBody?['grade_tag'], 'g3');

    final grant = await repository.unlock(
      deviceId: 'tv-pk-1',
      pin: '1234',
      minutes: 10,
    );
    expect(grant.minutes, 10);
    expect(grant.remainingSeconds, 600);
    expect(unlockBody?['pin'], '1234');
    expect(unlockBody?['minutes'], 10);

    await repository.unbind('tv-pk-1');
    expect(deletedPath, '/v1/family/tvs/tv-pk-1');
  });
}
