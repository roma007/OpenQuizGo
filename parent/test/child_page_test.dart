import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/cache/cache_store.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/features/shell/home_shell.dart';

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('首页孩子页渲染标题与列表', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          childrenProvider.overrideWith(
            (ref) async =>
                Cached.fresh([Child(id: 'c1', nickname: '小明', stage: 'low')]),
          ),
        ],
        child: const MaterialApp(home: HomeShell()),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('孩子档案'), findsOneWidget);
    expect(find.text('小明'), findsOneWidget);
  });
}
