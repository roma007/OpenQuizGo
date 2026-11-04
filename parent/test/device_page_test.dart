import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/features/device/device_page.dart';

void main() {
  testWidgets('设备页展示已绑定设备', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          devicesProvider.overrideWith(
            (ref) async => [
              DeviceInfo(
                id: 'd1',
                deviceId: 'tv-1',
                platform: 'tv',
                childId: 'c1',
                childName: '小明',
                boundAt: DateTime.utc(2026, 11, 4, 8),
              ),
            ],
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: DevicePage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('电视设备'), findsOneWidget);
    expect(find.text('小明'), findsOneWidget);
  });

  testWidgets('设备页空态提示绑定', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          devicesProvider.overrideWith((ref) async => <DeviceInfo>[]),
        ],
        child: const MaterialApp(home: Scaffold(body: DevicePage())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('还没有绑定电视'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, '绑定电视'), findsWidgets);
  });
}
