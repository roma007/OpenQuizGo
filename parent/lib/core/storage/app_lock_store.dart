import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 本地儿童锁：只保护家长端 App，不保存明文 PIN。
class AppLockStore {
  static const _enabledKey = 'app-lock:enabled';
  static const _saltKey = 'app-lock:salt';
  static const _hashKey = 'app-lock:hash';

  Future<bool> isEnabled() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getBool(_enabledKey) == true &&
        (prefs.getString(_saltKey) ?? '').isNotEmpty &&
        (prefs.getString(_hashKey) ?? '').isNotEmpty;
  }

  Future<void> setPin(String pin) async {
    final prefs = await SharedPreferences.getInstance();
    final salt = base64UrlEncode(
      List<int>.generate(16, (_) => Random.secure().nextInt(256)),
    );
    await prefs.setString(_saltKey, salt);
    await prefs.setString(_hashKey, _hash(salt, pin));
    await prefs.setBool(_enabledKey, true);
  }

  Future<bool> verify(String pin) async {
    final prefs = await SharedPreferences.getInstance();
    final salt = prefs.getString(_saltKey) ?? '';
    final hash = prefs.getString(_hashKey) ?? '';
    if (salt.isEmpty || hash.isEmpty) return false;
    return _hash(salt, pin) == hash;
  }

  Future<void> disable() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_enabledKey);
    await prefs.remove(_saltKey);
    await prefs.remove(_hashKey);
  }

  String _hash(String salt, String pin) =>
      sha256.convert(utf8.encode('$salt$pin')).toString();
}
