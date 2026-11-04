import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/daily_repository.dart';

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
  test('每日一题接口解析与提交', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? body;
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      if (request.method == 'GET' && request.uri.path.endsWith('/daily')) {
        request.response.write(
          jsonEncode({
            'date': '2026-11-04',
            'question': {
              'id': 'q1',
              'stem': '2+3=?',
              'type': 'choice',
              'options': [
                {'id': 'a', 'text': '5'},
                {'id': 'b', 'text': '6'},
              ],
            },
            'answered': false,
            'streak': 2,
            'total_points': 20,
          }),
        );
      } else if (request.method == 'POST' &&
          request.uri.path.endsWith('/daily/answers')) {
        body =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        request.response.write(
          jsonEncode({
            'date': '2026-11-04',
            'question': {
              'id': 'q1',
              'stem': '2+3=?',
              'type': 'choice',
              'options': [],
            },
            'answered': true,
            'correct': true,
            'correct_answer': ['a'],
            'points': 10,
            'streak': 3,
            'total_points': 30,
          }),
        );
      } else {
        request.response.statusCode = HttpStatus.notFound;
      }
      await request.response.close();
    });

    final api = ApiClient(
      _MemoryStore(),
      baseUrl: 'http://127.0.0.1:${server.port}/',
    );
    final repository = DailyRepository(api);

    final state = await repository.get('c1');
    expect(state.question?.stem, '2+3=?');
    expect(state.streak, 2);

    final result = await repository.answer('c1', 'a');
    expect(body?['answer'], ['a']);
    expect(result.correct, isTrue);
    expect(result.points, 10);
  });
}
