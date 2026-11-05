import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/storage/app_lock_store.dart';
import 'package:openquizgo_parent/features/lock/app_lock_gate.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('开启儿童锁后启动显示锁屏，正确 PIN 进入', (tester) async {
    SharedPreferences.setMockInitialValues({});
    await AppLockStore().setPin('1234');

    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: AppLockGate(child: Text('HOME')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('儿童锁'), findsWidgets);
    expect(find.text('HOME'), findsNothing);

    await tester.enterText(find.byType(TextField), '1234');
    await tester.tap(find.text('解锁'));
    await tester.pumpAndSettle();

    expect(find.text('HOME'), findsOneWidget);
  });

  testWidgets('未开启儿童锁直接显示内容', (tester) async {
    SharedPreferences.setMockInitialValues({});

    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: AppLockGate(child: Text('HOME')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('HOME'), findsOneWidget);
  });
}
