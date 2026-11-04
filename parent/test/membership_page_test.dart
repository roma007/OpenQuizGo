import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/membership_repository.dart';
import 'package:openquizgo_parent/features/membership/membership_page.dart';

MembershipPlan _plan() => MembershipPlan(
  id: 'p1',
  code: 'monthly',
  name: '月度会员',
  description: '按月解锁',
  durationDays: 30,
  priceCents: 1990,
  features: const ['权益 A'],
  entitlementCodes: const ['topic_pack', 'report_advanced'],
  status: 'online',
  sort: 10,
);

MembershipInfo _active() => MembershipInfo(
  active: true,
  status: 'active',
  planName: '月度会员',
  expiresAt: DateTime(2026, 12, 4, 12),
  daysRemaining: 30,
  entitlements: const ['topic_pack', 'report_advanced'],
);

class _FakeMembershipRepository implements MembershipRepository {
  int payCalls = 0;
  bool paid = false;

  @override
  Future<MembershipCenter> center() async =>
      MembershipCenter(membership: paid ? _active() : null, plans: [_plan()]);

  @override
  Future<List<MembershipOrder>> orders() async => const [];

  @override
  Future<MembershipOrder> createOrder(String planId) async => MembershipOrder(
    id: 'o2',
    orderNo: 'M2',
    planName: '月度会员',
    amountCents: 1990,
    durationDays: 30,
    status: 'pending',
    createdAt: DateTime(2026, 11, 4, 11),
    paidAt: null,
  );

  @override
  Future<MembershipPayResult> payOrder(String orderId) async {
    payCalls++;
    paid = true;
    return MembershipPayResult(
      order: MembershipOrder(
        id: 'o2',
        orderNo: 'M2',
        planName: '月度会员',
        amountCents: 1990,
        durationDays: 30,
        status: 'paid',
        createdAt: DateTime(2026, 11, 4, 11),
        paidAt: DateTime(2026, 11, 4, 11, 0, 1),
      ),
      membership: _active(),
    );
  }
}

void main() {
  testWidgets('会员中心模拟支付开通', (tester) async {
    final repository = _FakeMembershipRepository();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [membershipRepositoryProvider.overrideWithValue(repository)],
        child: const MaterialApp(home: MembershipPage()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('尚未开通会员'), findsOneWidget);
    expect(find.text('月度会员'), findsOneWidget);
    expect(find.text('¥19.90'), findsOneWidget);

    await tester.tap(find.text('开通'));
    await tester.pumpAndSettle();
    expect(find.text('模拟支付'), findsOneWidget);
    expect(find.textContaining('不会产生真实扣款'), findsOneWidget);

    await tester.tap(find.text('确认支付'));
    await tester.pumpAndSettle();
    expect(repository.payCalls, 1);
    expect(find.text('会员有效'), findsOneWidget);
    expect(find.textContaining('有效期至 2026-12-04'), findsWidgets);
  });
}
