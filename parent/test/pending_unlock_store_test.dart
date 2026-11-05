import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/storage/pending_unlock_store.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  test('保存、读取并清除待补发解锁', () async {
    final store = PendingUnlockStore();
    await store.save('device-1', 15);

    final pending = await store.read('device-1');
    expect(pending, isNotNull);
    expect(pending!.minutes, 15);
    expect(pending.savedAt.millisecondsSinceEpoch, greaterThan(0));

    await store.clear('device-1');
    expect(await store.read('device-1'), isNull);
  });

  test('每台设备只保留最新一条', () async {
    final store = PendingUnlockStore();
    await store.save('device-1', 10);
    await store.save('device-1', 30);

    final pending = await store.read('device-1');
    expect(pending!.minutes, 30);
    expect(await store.read('device-2'), isNull);
  });

  test('超过 TTL 自动过期并清除', () async {
    final store = PendingUnlockStore();
    await store.save('device-1', 10);

    final expired = await store.read(
      'device-1',
      now: DateTime.now().add(const Duration(minutes: 11)),
    );
    expect(expired, isNull);

    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('pending-unlock:device-1'), isNull);
  });

  test('队列不包含 PIN 字段', () async {
    final store = PendingUnlockStore();
    await store.save('device-1', 10);
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString('pending-unlock:device-1') ?? '';
    expect(raw.contains('pin'), isFalse);
  });
}
