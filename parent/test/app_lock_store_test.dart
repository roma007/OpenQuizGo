import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/storage/app_lock_store.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  test('设置、校验并关闭儿童锁', () async {
    final store = AppLockStore();
    expect(await store.isEnabled(), isFalse);

    await store.setPin('1234');
    expect(await store.isEnabled(), isTrue);
    expect(await store.verify('1234'), isTrue);
    expect(await store.verify('0000'), isFalse);

    await store.disable();
    expect(await store.isEnabled(), isFalse);
  });

  test('不保存明文 PIN', () async {
    final store = AppLockStore();
    await store.setPin('9876');
    final prefs = await SharedPreferences.getInstance();
    final all = prefs.getKeys().map((key) => prefs.get(key)).join(',');
    expect(all.contains('9876'), isFalse);
    expect(prefs.getString('app-lock:hash'), isNotNull);
    expect(prefs.getString('app-lock:salt'), isNotEmpty);
  });
}
