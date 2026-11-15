import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/tv_repository.dart';
import 'package:openquizgo_parent/features/device/device_page.dart';

TvDevice _tv({
  String id = 'tv-pk-1',
  String name = '客厅电视',
  String childName = '小明',
  bool quizEnabled = true,
}) {
  return TvDevice(
    id: id,
    name: name,
    deviceId: 'tv-living-01',
    childId: 'c1',
    childName: childName,
    gradeTag: 'g4',
    online: true,
    lastSeenAt: DateTime.utc(2026, 11, 15, 8),
    usedMinutes: 20,
    config: TvConfig(
      quizEnabled: quizEnabled,
      limitEnabled: true,
      unlockMinutes: 15,
      dailyLimit: 60,
      subjects: const ['math'],
      passThreshold: 3,
      quizScope: 'random',
      hardStopGrace: 10,
      version: 1,
    ),
  );
}

void main() {
  testWidgets('电视列表展示孩子、年级与答题间隔', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          tvsProvider.overrideWith((ref) async => [_tv()]),
        ],
        child: const MaterialApp(home: Scaffold(body: DevicePage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('电视设备'), findsOneWidget);
    expect(find.text('客厅电视'), findsOneWidget);
    expect(find.text('小明 · 四年级'), findsOneWidget);
    expect(find.text('每 15 分钟答题'), findsOneWidget);
    expect(find.text('今日已看 20 / 60 分钟'), findsOneWidget);
  });

  testWidgets('电视列表空态提示电视端登录', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [tvsProvider.overrideWith((ref) async => <TvDevice>[])],
        child: const MaterialApp(home: Scaffold(body: DevicePage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('还没有电视登录'), findsOneWidget);
    expect(find.widgetWithText(OutlinedButton, '刷新'), findsOneWidget);
  });

  testWidgets('未指定孩子时展示随机出题', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          tvsProvider.overrideWith(
            (ref) async => [_tv(childName: '', quizEnabled: false)],
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: DevicePage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('未指定孩子 · 全库随机出题'), findsOneWidget);
    expect(find.text('做题已关闭'), findsOneWidget);
  });
}
