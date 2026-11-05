import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/battle_repository.dart';

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

Map<String, dynamic> _battle({
  String status = 'child_turn',
  int childScore = 0,
  int parentScore = 0,
  bool finished = false,
}) => {
  'id': 'b1',
  'status': status,
  'question_count': 1,
  'questions': [
    {
      'id': 'q1',
      'stem': '2+3=?',
      'type': 'choice',
      'options': [
        {'id': 'a', 'text': '5'},
        {'id': 'b', 'text': '6'},
      ],
    },
  ],
  'child_score': childScore,
  'parent_score': parentScore,
  if (finished)
    'result': {
      'winner': 'child',
      'child_score': childScore,
      'parent_score': parentScore,
    },
  if (finished)
    'answers': [
      {
        'side': 'child',
        'question_id': 'q1',
        'answer': ['a'],
        'correct': true,
        'correct_answer': ['a'],
        'explain': '三加二等于五',
      },
      {
        'side': 'parent',
        'question_id': 'q1',
        'answer': ['b'],
        'correct': false,
        'correct_answer': ['a'],
        'explain': '三加二等于五',
      },
    ],
};

void main() {
  test('创建、查询与提交对战', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? submitted;
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      final path = request.uri.path;
      if (request.method == 'POST' && path.endsWith('/battles')) {
        request.response.write(jsonEncode(_battle()));
      } else if (request.method == 'GET' && path.contains('/battles/b1')) {
        request.response.write(
          jsonEncode(_battle(status: 'parent_turn', childScore: 1)),
        );
      } else if (request.method == 'GET' && path.endsWith('/battles')) {
        request.response.write(
          jsonEncode({
            'items': [
              {
                'id': 'b1',
                'mode': 'room',
                'status': 'finished',
                'question_count': 5,
                'child_score': 5,
                'parent_score': 5,
                'created_at': '2026-11-05T09:30:00+08:00',
                'finished_at': '2026-11-05T09:32:00+08:00',
              },
            ],
          }),
        );
      } else if (request.method == 'POST' && path.endsWith('/answers')) {
        submitted =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        final side = submitted?['side'];
        request.response.write(
          jsonEncode(
            side == 'child'
                ? _battle(status: 'parent_turn', childScore: 1)
                : _battle(
                    status: 'finished',
                    childScore: 1,
                    parentScore: 0,
                    finished: true,
                  ),
          ),
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
    final repository = BattleRepository(api);

    final created = await repository.create('c1');
    expect(created.status, 'child_turn');
    expect(created.questions.single.stem, '2+3=?');
    expect(created.questions.single.options.first.text, '5');

    final history = await repository.history('c1');
    expect(history.single.mode, 'room');
    expect(history.single.childScore, 5);
    expect(history.single.parentScore, 5);

    final loaded = await repository.get('c1', 'b1');
    expect(loaded.childTurn, isFalse);
    expect(loaded.childScore, 1);

    final afterChild = await repository.submit('c1', 'b1', 'child', {
      'q1': 'a',
    });
    expect(submitted?['side'], 'child');
    expect((submitted?['answers'] as List).single['answer'], ['a']);
    expect(afterChild.status, 'parent_turn');

    final finished = await repository.submit('c1', 'b1', 'parent', {'q1': 'b'});
    expect(finished.finished, isTrue);
    expect(finished.result?.winner, 'child');
    expect(finished.answers.length, 2);
    expect(finished.answers.first.explain, '三加二等于五');
  });
}
