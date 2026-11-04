import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/rule_repository.dart';

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

Map<String, dynamic> _ruleJson() => {
  'unlock_minutes': 20,
  'pass_threshold': 3,
  'subject_weight': {'math': 1.5, 'chinese': 1.5, 'fun': 0.5},
  'fun_ratio_cap': 2,
  'fun_enabled': true,
  'weight_preset': 'balanced',
  'time_window': {
    'allow': [
      {'start': '07:00', 'end': '21:00'},
    ],
  },
  'daily_limit': 60,
  'offline_fallback_min': 10,
  'scope_mode': 'media',
  'allowed_apps': <String>[],
  'timezone': 'Asia/Shanghai',
};

void main() {
  test('规则读取、预览与保存请求', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? updateBody;
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      if (request.method == 'GET' && request.uri.path.endsWith('/rule')) {
        request.response.write(jsonEncode({'rule': _ruleJson(), 'version': 2}));
      } else if (request.method == 'POST' &&
          request.uri.path.endsWith('/rule/preview')) {
        request.response.write(
          jsonEncode({
            'n': 5,
            'quota': {'math': 2, 'chinese': 2},
            'fun_slots': 1,
            'max_score': 7.5,
            'warnings': ['门槛接近可达上限'],
          }),
        );
      } else if (request.method == 'PUT' &&
          request.uri.path.endsWith('/rule')) {
        updateBody =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
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
    final repository = RuleRepository(api);

    final loaded = await repository.getRule('c1');
    expect(loaded.version, 2);
    expect(loaded.rule.unlockMinutes, 20);
    expect(loaded.rule.subjectWeight['math'], 1.5);
    expect(loaded.rule.timeWindow.single.start, '07:00');

    final preview = await repository.preview('c1', loaded.rule);
    expect(preview.questionCount, 5);
    expect(preview.maxScore, 7.5);
    expect(preview.warnings, isNotEmpty);

    await repository.update('c1', '1234', loaded.rule);
    expect(updateBody?['pin'], '1234');
    final rule = updateBody?['rule'] as Map<String, dynamic>;
    expect(rule['unlock_minutes'], 20);
    expect((rule['subject_weight'] as Map)['math'], 1.5);
  });
}
