import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/family_repository.dart';
import 'package:openquizgo_parent/features/settings/members_page.dart';
import 'package:openquizgo_parent/features/settings/pin_page.dart';

void main() {
  testWidgets('PIN 页校验 4–6 位数字', (tester) async {
    await tester.pumpWidget(
      const ProviderScope(child: MaterialApp(home: PinPage())),
    );
    await tester.tap(find.widgetWithText(FilledButton, '保存 PIN'));
    await tester.pump();
    expect(find.text('请输入 4–6 位数字'), findsWidgets);
  });

  testWidgets('成员页展示角色与脱敏手机号', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          membersProvider.overrideWith(
            (ref) async => [
              FamilyMember(
                accountId: 'a1',
                phone: '139****2222',
                role: 'owner',
                joinedAt: DateTime(2026, 11, 4),
              ),
            ],
          ),
        ],
        child: const MaterialApp(home: MembersPage()),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('139****2222'), findsOneWidget);
    expect(find.textContaining('家庭管理员'), findsOneWidget);
  });
}
