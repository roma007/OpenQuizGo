import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

class Cached<T> {
  Cached.fresh(this.data) : stale = false, cachedAt = null;

  Cached.stale(this.data, this.cachedAt) : stale = true;

  final T data;
  final bool stale;
  final DateTime? cachedAt;
}

class CacheEntry {
  CacheEntry({required this.data, required this.cachedAt});

  final dynamic data;
  final DateTime cachedAt;
}

/// 轻量离线缓存：SharedPreferences 存 JSON + 时间戳。
class CacheStore {
  static const _prefix = 'cache:';

  Future<void> write(String key, Object? data) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      '$_prefix$key',
      jsonEncode({'at': DateTime.now().millisecondsSinceEpoch, 'data': data}),
    );
  }

  Future<CacheEntry?> read(String key) async {
    String? raw;
    try {
      final prefs = await SharedPreferences.getInstance();
      raw = prefs.getString('$_prefix$key');
    } catch (_) {
      return null;
    }
    if (raw == null) return null;
    try {
      final json = jsonDecode(raw) as Map<String, dynamic>;
      final at = (json['at'] as num?)?.toInt() ?? 0;
      return CacheEntry(
        data: json['data'],
        cachedAt: DateTime.fromMillisecondsSinceEpoch(at),
      );
    } catch (_) {
      return null;
    }
  }
}
