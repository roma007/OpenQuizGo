import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/storage/pending_rule_store.dart';
import 'package:openquizgo_parent/data/rule_repository.dart';
import 'package:shared_preferences/shared_preferences.dart';

Rule testRule(int unlockMinutes) => Rule.fromJson({
  'unlock_minutes': unlockMinutes,
  'pass_threshold': 3,
  'daily_limit': 60,
  'fun_ratio_cap': 2,
  'fun_enabled': true,
  'weight_preset': 'balanced',
  'time_window': {
    'allow': [
      {'start': '07:00', 'end': '21:00'},
    ],
  },
  'offline_fallback_min': 10,
  'scope_mode': 'media',
  'allowed_apps': <String>[],
  'timezone': 'Asia/Shanghai',
});

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  test('保存、读取并清除待同步规则', () async {
    final store = PendingRuleStore();
    await store.save('child-1', testRule(30));

    final pending = await store.read('child-1');
    expect(pending, isNotNull);
    expect(pending!.rule.unlockMinutes, 30);
    expect(pending.rule.dailyLimit, 60);
    expect(pending.savedAt.millisecondsSinceEpoch, greaterThan(0));

    await store.clear('child-1');
    expect(await store.read('child-1'), isNull);
  });

  test('每个孩子只保留最新一条规则', () async {
    final store = PendingRuleStore();
    await store.save('child-1', testRule(20));
    await store.save('child-1', testRule(45));

    final pending = await store.read('child-1');
    expect(pending!.rule.unlockMinutes, 45);
    expect(await store.read('child-2'), isNull);
  });

  test('队列不包含 PIN 字段', () async {
    final store = PendingRuleStore();
    await store.save('child-1', testRule(20));
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString('pending-rule:child-1') ?? '';
    expect(raw.contains('pin'), isFalse);
  });
}
