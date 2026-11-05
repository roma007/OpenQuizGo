import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

class PendingUnlock {
  PendingUnlock({required this.minutes, required this.savedAt});

  final int minutes;
  final DateTime savedAt;

  DateTime get expiresAt => savedAt.add(PendingUnlockStore.ttl);

  Map<String, dynamic> toJson() => {
    'minutes': minutes,
    'saved_at': savedAt.millisecondsSinceEpoch,
  };

  factory PendingUnlock.fromJson(Map<String, dynamic> json) => PendingUnlock(
    minutes: (json['minutes'] as num?)?.toInt() ?? 10,
    savedAt: DateTime.fromMillisecondsSinceEpoch(
      (json['saved_at'] as num?)?.toInt() ?? 0,
    ),
  );
}

/// 解锁补发队列：每台设备只保留最新一条，10 分钟过期，不保存 PIN。
class PendingUnlockStore {
  static const ttl = Duration(minutes: 10);
  static const _prefix = 'pending-unlock:';

  Future<void> save(String deviceId, int minutes) async {
    final prefs = await SharedPreferences.getInstance();
    final pending = PendingUnlock(minutes: minutes, savedAt: DateTime.now());
    await prefs.setString('$_prefix$deviceId', jsonEncode(pending.toJson()));
  }

  Future<PendingUnlock?> read(
    String deviceId, {
    DateTime? now,
    Duration? ttl,
  }) async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString('$_prefix$deviceId');
    if (raw == null) return null;
    try {
      final pending = PendingUnlock.fromJson(
        jsonDecode(raw) as Map<String, dynamic>,
      );
      final expiresAt = pending.savedAt.add(ttl ?? PendingUnlockStore.ttl);
      if (!(now ?? DateTime.now()).isBefore(expiresAt)) {
        await prefs.remove('$_prefix$deviceId');
        return null;
      }
      return pending;
    } catch (_) {
      return null;
    }
  }

  Future<void> clear(String deviceId) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('$_prefix$deviceId');
  }
}
