import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/family_repository.dart';

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
  test('孩子、PIN 与家庭成员请求', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? createdBody;
    Map<String, dynamic>? pinBody;
    String? joinedCode;

    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      final path = request.uri.path;
      if (request.method == 'GET' && path == '/v1/family/children') {
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
      } else if (request.method == 'POST' && path == '/v1/family/children') {
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
      } else if (request.method == 'GET' && path == '/v1/family/members') {
        request.response.write(
          jsonEncode([
            {
              'account_id': 'a1',
              'phone': '139****2222',
              'role': 'owner',
              'joined_at': '2026-11-04T08:00:00Z',
            },
          ]),
        );
      } else if (request.method == 'POST' &&
          path == '/v1/family/members/invite') {
        request.response.write(
          jsonEncode({'code': '654321', 'expires_at': '2026-11-11T08:00:00Z'}),
        );
      } else if (request.method == 'POST' &&
          path == '/v1/family/members/join') {
        final body =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        joinedCode = body['code'] as String?;
        request.response.write(jsonEncode({'family_id': 'f1'}));
      } else if (request.method == 'POST' && path == '/v1/family/pin') {
        pinBody =
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
    expect(createdBody?['birth_year'], 2020);

    final members = await repository.listMembers();
    expect(members, hasLength(1));
    expect(members.first.role, 'owner');
    expect(members.first.phone, '139****2222');

    final invite = await repository.inviteMember();
    expect(invite.code, '654321');
    expect(invite.expiresAt, isNotNull);

    await repository.joinFamily('654321');
    expect(joinedCode, '654321');

    await repository.setPin(currentPin: '1234', newPin: '5678');
    expect(pinBody?['current_pin'], '1234');
    expect(pinBody?['new_pin'], '5678');
  });
}
