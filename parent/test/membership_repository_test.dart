import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/api_client.dart';
import 'package:openquizgo_parent/core/storage/token_store.dart';
import 'package:openquizgo_parent/data/membership_repository.dart';

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

Map<String, dynamic> _plan() => {
  'id': 'p1',
  'code': 'monthly',
  'name': '月度会员',
  'description': '按月解锁',
  'duration_days': 30,
  'price_cents': 1990,
  'features': ['权益 A'],
  'status': 'online',
  'sort': 10,
};

void main() {
  test('会员中心、下单与模拟支付', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));

    Map<String, dynamic>? submitted;
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      final path = request.uri.path;
      if (request.method == 'GET' && path.endsWith('/membership')) {
        request.response.write(
          jsonEncode({
            'plans': [_plan()],
          }),
        );
      } else if (request.method == 'GET' && path.endsWith('/orders')) {
        request.response.write(
          jsonEncode({
            'items': [
              {
                'id': 'o1',
                'order_no': 'M1',
                'plan_name': '月度会员',
                'amount_cents': 1990,
                'duration_days': 30,
                'status': 'paid',
                'created_at': '2026-11-04T10:00:00+08:00',
                'paid_at': '2026-11-04T10:00:05+08:00',
              },
            ],
          }),
        );
      } else if (request.method == 'POST' && path.endsWith('/orders')) {
        submitted =
            jsonDecode(await utf8.decoder.bind(request).join())
                as Map<String, dynamic>;
        request.response.write(
          jsonEncode({
            'id': 'o2',
            'order_no': 'M2',
            'plan_name': '月度会员',
            'amount_cents': 1990,
            'duration_days': 30,
            'status': 'pending',
            'created_at': '2026-11-04T11:00:00+08:00',
          }),
        );
      } else if (request.method == 'POST' && path.endsWith('/pay')) {
        request.response.write(
          jsonEncode({
            'order': {
              'id': 'o2',
              'order_no': 'M2',
              'plan_name': '月度会员',
              'amount_cents': 1990,
              'duration_days': 30,
              'status': 'paid',
              'created_at': '2026-11-04T11:00:00+08:00',
              'paid_at': '2026-11-04T11:00:01+08:00',
            },
            'membership': {
              'active': true,
              'status': 'active',
              'plan_name': '月度会员',
              'expires_at': '2026-12-04T11:00:01+08:00',
              'days_remaining': 30,
            },
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
    final repository = MembershipRepository(api);

    final center = await repository.center();
    expect(center.membership, isNull);
    expect(center.plans.single.name, '月度会员');
    expect(center.plans.single.priceCents, 1990);

    final orders = await repository.orders();
    expect(orders.single.status, 'paid');

    final order = await repository.createOrder('p1');
    expect(submitted?['plan_id'], 'p1');
    expect(order.status, 'pending');

    final paid = await repository.payOrder('o2');
    expect(paid.order.status, 'paid');
    expect(paid.membership.active, isTrue);
    expect(formatYuan(paid.membership.active == true ? 1990 : 0), '¥19.90');
    expect(formatDate(paid.membership.expiresAt), '2026-12-04');
  });
}
