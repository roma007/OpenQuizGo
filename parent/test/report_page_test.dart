import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/cache/cache_store.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/membership_repository.dart';
import 'package:openquizgo_parent/data/report_repository.dart';
import 'package:openquizgo_parent/features/report/report_page.dart';

class _FakeReportRepository implements ReportRepository {
  _FakeReportRepository(this.reports);

  final List<GrowthReport> reports;

  @override
  Future<List<GrowthReport>> listReports(ReportQuery query) async => reports;
}

GrowthReport _report() => GrowthReport(
  id: 'r1',
  period: 'week',
  periodStart: DateTime(2026, 11, 2),
  periodEnd: DateTime(2026, 11, 9),
  subjectStats: {'math': SubjectStat(answered: 4, correct: 3, accuracy: 0.75)},
  funStats: SubjectStat(answered: 0, correct: 0, accuracy: 0),
  watchMinutes: 35,
);

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('报告页展示统计与学科正确率', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          childrenProvider.overrideWith(
            (ref) async =>
                Cached.fresh([Child(id: 'c1', nickname: '小明', stage: 'low')]),
          ),
          reportRepositoryProvider.overrideWithValue(
            _FakeReportRepository([_report()]),
          ),
          membershipCenterProvider.overrideWith(
            (ref) async => MembershipCenter(membership: null, plans: const []),
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: ReportPage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.text('35'), findsOneWidget);
    expect(find.text('数学'), findsOneWidget);
    expect(find.textContaining('3/4'), findsOneWidget);
  });

  testWidgets('报告页空态', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          childrenProvider.overrideWith(
            (ref) async =>
                Cached.fresh([Child(id: 'c1', nickname: '小明', stage: 'low')]),
          ),
          reportRepositoryProvider.overrideWithValue(
            _FakeReportRepository(const []),
          ),
          membershipCenterProvider.overrideWith(
            (ref) async => MembershipCenter(membership: null, plans: const []),
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: ReportPage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.textContaining('还没有'), findsWidgets);
  });

  testWidgets('会员解锁进阶分析', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          childrenProvider.overrideWith(
            (ref) async =>
                Cached.fresh([Child(id: 'c1', nickname: '小明', stage: 'low')]),
          ),
          reportRepositoryProvider.overrideWithValue(
            _FakeReportRepository([_report()]),
          ),
          membershipCenterProvider.overrideWith(
            (ref) async => MembershipCenter(
              membership: MembershipInfo(
                active: true,
                status: 'active',
                planName: '月度会员',
                expiresAt: DateTime(2026, 12, 4),
                daysRemaining: 30,
                entitlements: const ['report_advanced'],
              ),
              plans: const [],
            ),
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: ReportPage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.text('进阶分析'), findsOneWidget);
    expect(find.textContaining('薄弱学科：数学'), findsOneWidget);
    expect(find.textContaining('还有提升空间'), findsOneWidget);
  });
}
