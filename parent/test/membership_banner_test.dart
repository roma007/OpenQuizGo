import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/membership_repository.dart';
import 'package:openquizgo_parent/features/membership/membership_banner.dart';

MembershipInfo _membership({
  required bool active,
  required String status,
  int days = 30,
}) => MembershipInfo(
  active: active,
  status: status,
  planName: '月度会员',
  expiresAt: DateTime(2026, 12, 4),
  daysRemaining: days,
  entitlements: active ? const ['topic_pack'] : const [],
);

Widget _app(MembershipInfo? membership) => ProviderScope(
  overrides: [
    membershipCenterProvider.overrideWith(
      (ref) async => MembershipCenter(membership: membership, plans: const []),
    ),
  ],
  child: const MaterialApp(home: Scaffold(body: MembershipBanner())),
);

void main() {
  testWidgets('7 天内显示到期提醒', (tester) async {
    await tester.pumpWidget(
      _app(_membership(active: true, status: 'active', days: 3)),
    );
    await tester.pumpAndSettle();
    expect(find.text('会员将在 3 天后到期'), findsOneWidget);
    expect(find.text('去续费'), findsOneWidget);
  });

  testWidgets('到期后显示暂停提醒', (tester) async {
    await tester.pumpWidget(
      _app(_membership(active: false, status: 'expired', days: 0)),
    );
    await tester.pumpAndSettle();
    expect(find.text('会员已到期，会员权益已暂停'), findsOneWidget);
  });

  testWidgets('有效期充足时不显示', (tester) async {
    await tester.pumpWidget(
      _app(_membership(active: true, status: 'active', days: 30)),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('会员'), findsNothing);
  });

  testWidgets('无会员时不显示', (tester) async {
    await tester.pumpWidget(_app(null));
    await tester.pumpAndSettle();
    expect(find.textContaining('会员'), findsNothing);
  });
}
