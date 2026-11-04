import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/cache/cache_store.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  test('缓存写入后可读取并带时间戳', () async {
    final store = CacheStore();
    await store.write('children', [
      {'id': 'c1', 'nickname': '小明', 'stage': 'low'},
    ]);

    final entry = await store.read('children');
    expect(entry, isNotNull);
    expect(entry!.data, isA<List<dynamic>>());
    expect(entry.cachedAt.isAfter(DateTime(2020)), isTrue);
  });

  test('无缓存返回 null', () async {
    final store = CacheStore();
    expect(await store.read('missing'), isNull);
  });
}
