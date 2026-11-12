import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/cache/cache_store.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/alert_repository.dart';
import 'package:openquizgo_parent/features/child/child_page.dart';
import 'package:openquizgo_parent/features/shell/home_shell.dart';

Future<void> _pumpHome(WidgetTester tester, Size size) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        alertsProvider.overrideWith(
          (ref) async => AlertList(items: const [], unread: 0),
        ),
        childrenProvider.overrideWith(
          (ref) async =>
              Cached.fresh([Child(id: 'c1', nickname: '小明', stage: 'low')]),
        ),
      ],
      child: const MaterialApp(home: HomeShell()),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 200));
}

int _childColumns(WidgetTester tester) {
  final gridFinder = find.descendant(
    of: find.byType(ChildPage),
    matching: find.byType(GridView),
  );
  if (gridFinder.evaluate().isEmpty) return 1;
  final grid = tester.widget<GridView>(gridFinder);
  return (grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount)
      .crossAxisCount;
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('iPhone 竖屏使用底部导航和单列', (tester) async {
    await _pumpHome(tester, const Size(390, 844));

    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(NavigationRail), findsNothing);
    expect(_childColumns(tester), 1);
  });

  testWidgets('iPhone 横屏仍使用底部导航', (tester) async {
    await _pumpHome(tester, const Size(844, 390));

    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(NavigationRail), findsNothing);
  });

  testWidgets('iPad 竖屏使用侧栏和两列', (tester) async {
    await _pumpHome(tester, const Size(820, 1180));

    expect(find.byType(NavigationRail), findsOneWidget);
    expect(find.byType(NavigationBar), findsNothing);
    expect(
      tester.widget<NavigationRail>(find.byType(NavigationRail)).extended,
      isFalse,
    );
    expect(_childColumns(tester), 2);
  });

  testWidgets('iPad Pro 横屏展开侧栏并显示三列', (tester) async {
    await _pumpHome(tester, const Size(1366, 1024));

    expect(find.byType(NavigationRail), findsOneWidget);
    expect(
      tester.widget<NavigationRail>(find.byType(NavigationRail)).extended,
      isTrue,
    );
    expect(_childColumns(tester), 3);
  });

  testWidgets('iPad 10.9 横屏使用紧凑侧栏和三列', (tester) async {
    await _pumpHome(tester, const Size(1180, 820));

    expect(find.byType(NavigationRail), findsOneWidget);
    expect(
      tester.widget<NavigationRail>(find.byType(NavigationRail)).extended,
      isFalse,
    );
    expect(_childColumns(tester), 3);
  });

  testWidgets('Slide Over 窄窗口退回底部导航和单列', (tester) async {
    await _pumpHome(tester, const Size(320, 1024));

    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(NavigationRail), findsNothing);
    expect(_childColumns(tester), 1);
  });
}
