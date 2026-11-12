import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/features/auth/login_screen.dart';

void main() {
  testWidgets('登录页渲染并校验手机号', (tester) async {
    await tester.pumpWidget(
      const ProviderScope(child: MaterialApp(home: LoginScreen())),
    );

    expect(find.text('电视时间管家'), findsOneWidget);
    expect(find.text('帮孩子管好电视时间'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, '登录'), findsOneWidget);

    await tester.tap(find.widgetWithText(FilledButton, '登录'));
    await tester.pump();

    expect(find.text('请输入正确的手机号'), findsOneWidget);
    expect(find.text('请输入验证码'), findsOneWidget);
  });
}
