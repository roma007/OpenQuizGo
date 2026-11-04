import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/report_repository.dart';

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
  test('报告列表解析与查询参数', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    String? query;
    server.listen((request) async {
      query = request.uri.query;
      request.response.headers.contentType = ContentType.json;
      request.response.write(
        jsonEncode({
          'items': [
            {
              'id': 'r1',
              'period': 'week',
              'period_start': '2026-11-02T00:00:00+08:00',
              'period_end': '2026-11-09T00:00:00+08:00',
              'subject_stats': {
                'math': {'answered': 4, 'correct': 3, 'accuracy': 0.75},
                'chinese': {'answered': 2, 'correct': 1, 'accuracy': 0.5},
              },
              'fun_stats': {'answered': 1, 'correct': 1, 'accuracy': 1.0},
              'watch_minutes': 35,
            },
          ],
        }),
      );
      await request.response.close();
    });

    final api = ApiClient(
      _MemoryStore(),
      baseUrl: 'http://127.0.0.1:${server.port}/',
    );
    final repository = ReportRepository(api);
    final reports = await repository.listReports(
      ReportQuery(childId: 'c1', period: 'week'),
    );

    expect(query, contains('period=week'));
    expect(query, contains('limit=8'));
    expect(reports, hasLength(1));
    final report = reports.first;
    expect(report.watchMinutes, 35);
    expect(report.answered, 7);
    expect(report.correct, 5);
    expect(report.subjectStats['math']?.accuracy, 0.75);
  });
}
