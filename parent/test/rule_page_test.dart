import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/rule_repository.dart';
import 'package:openquizgo_parent/features/rule/rule_page.dart';

Rule _rule() => Rule(
  unlockMinutes: 20,
  passThreshold: 3,
  subjectWeight: const {'math': 1.5, 'chinese': 1.5, 'fun': 0.5},
  funRatioCap: 2,
  funEnabled: true,
  weightPreset: 'balanced',
  timeWindow: [TimeWindow(start: '07:00', end: '21:00')],
  dailyLimit: 60,
  offlineFallbackMin: 10,
  scopeMode: 'media',
  allowedApps: const [],
  timezone: 'Asia/Shanghai',
);

class _FakeRuleRepository implements RuleRepository {
  @override
  Future<RuleWithVersion> getRule(String childId) async =>
      RuleWithVersion(rule: _rule(), version: 3);

  @override
  Future<RulePreview> preview(String childId, Rule rule) async => RulePreview(
    questionCount: 5,
    maxScore: 7.5,
    funSlots: 1,
    quota: const {},
    warnings: const [],
  );

  @override
  Future<void> update(String childId, String pin, Rule rule) async {}
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('规则页渲染核心字段', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ruleRepositoryProvider.overrideWithValue(_FakeRuleRepository()),
        ],
        child: MaterialApp(
          home: RulePage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.runAsync(() async {
      await Future<void>.delayed(const Duration(milliseconds: 50));
    });
    await tester.pump();
    expect(find.text('小明 的观看规则'), findsOneWidget);
    expect(find.text('规则版本 v3'), findsOneWidget);
    expect(find.text('解锁时长（分钟）'), findsOneWidget);
    expect(find.text('加权门槛'), findsOneWidget);
    await tester.scrollUntilVisible(
      find.text('保存规则'),
      300,
      scrollable: find.byType(Scrollable).first,
    );
    expect(find.widgetWithText(FilledButton, '保存规则'), findsOneWidget);
  });
}
