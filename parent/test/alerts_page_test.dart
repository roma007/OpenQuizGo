import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/alert_repository.dart';
import 'package:openquizgo_parent/features/alerts/alerts_page.dart';

class _FakeAlertRepository implements AlertRepository {
  @override
  Future<AlertList> list({int limit = 50}) async =>
      AlertList(items: const [], unread: 0);

  @override
  Future<void> markAllRead() async {}
}

void main() {
  testWidgets('告警页展示事件类型', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          alertsProvider.overrideWith(
            (ref) async => AlertList(
              items: [
                AlertItem(
                  id: 1,
                  type: 'permission_revoked',
                  detail: const {'permission': 'accessibility'},
                  reportedAt: DateTime(2026, 11, 4, 8),
                  childName: '小明',
                  deviceId: 'tv-1',
                  read: false,
                ),
              ],
              unread: 1,
            ),
          ),
          alertRepositoryProvider.overrideWithValue(_FakeAlertRepository()),
        ],
        child: const MaterialApp(home: AlertsPage()),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('无障碍服务被关闭'), findsOneWidget);
    expect(find.textContaining('小明'), findsOneWidget);
  });

  testWidgets('告警页空态', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          alertsProvider.overrideWith(
            (ref) async => AlertList(items: const [], unread: 0),
          ),
          alertRepositoryProvider.overrideWithValue(_FakeAlertRepository()),
        ],
        child: const MaterialApp(home: AlertsPage()),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('暂无告警'), findsOneWidget);
  });
}
