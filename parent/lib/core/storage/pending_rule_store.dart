import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import '../../data/rule_repository.dart';

class PendingRule {
  PendingRule({
    required this.rule,
    required this.savedAt,
  });

  final Rule rule;
  final DateTime savedAt;

  Map<String, dynamic> toJson() => {
    'saved_at': savedAt.millisecondsSinceEpoch,
    'rule': rule.toJson(),
  };

  factory PendingRule.fromJson(Map<String, dynamic> json) => PendingRule(
    rule: Rule.fromJson(Map<String, dynamic>.from(json['rule'] as Map)),
    savedAt: DateTime.fromMillisecondsSinceEpoch(
      (json['saved_at'] as num?)?.toInt() ?? 0,
    ),
  );
}

/// 离线规则保存队列：每个孩子只保留最新一条待同步规则，不保存 PIN。
class PendingRuleStore {
  static const _prefix = 'pending-rule:';

  Future<void> save(String childId, Rule rule) async {
    final prefs = await SharedPreferences.getInstance();
    final pending = PendingRule(rule: rule, savedAt: DateTime.now());
    await prefs.setString('$_prefix$childId', jsonEncode(pending.toJson()));
  }

  Future<PendingRule?> read(String childId) async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString('$_prefix$childId');
    if (raw == null) return null;
    try {
      return PendingRule.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    } catch (_) {
      return null;
    }
  }

  Future<void> clear(String childId) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('$_prefix$childId');
  }
}
