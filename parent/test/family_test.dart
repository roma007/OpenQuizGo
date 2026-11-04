import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/data/family_repository.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';

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
  test('孩子列表与新增请求格式', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? createdBody;
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      if (request.method == 'GET' &&
          request.uri.path == '/v1/family/children') {
        request.response.write(
          jsonEncode([
            {
              'id': 'c1',
              'nickname': '小明',
              'stage': 'low',
              'birth_year': 2018,
              'birth_month': 6,
            },
          ]),
        );
      } else if (request.method == 'POST' &&
          request.uri.path == '/v1/family/children') {
        createdBody =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        request.response.statusCode = HttpStatus.created;
        request.response.write(
          jsonEncode({
            'id': 'c2',
            'nickname': createdBody!['nickname'],
            'stage': createdBody!['stage'],
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
    final repository = FamilyRepository(api);

    final children = await repository.listChildren();
    expect(children, hasLength(1));
    expect(children.first.nickname, '小明');
    expect(children.first.birthYear, 2018);

    final created = await repository.createChild(
      const ChildDraft(nickname: '小红', stage: 'preschool', birthYear: 2020),
    );
    expect(created.id, 'c2');
    expect(createdBody?['nickname'], '小红');
    expect(createdBody?['stage'], 'preschool');
    expect(createdBody?['birth_year'], 2020);
  });
}
