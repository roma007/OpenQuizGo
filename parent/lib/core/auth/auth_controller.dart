import 'package:flutter/foundation.dart';

import '../storage/token_store.dart';

/// 登录状态：驱动 go_router 重定向。
class AuthController extends ChangeNotifier {
  AuthController(this._store);

  final TokenStore _store;

  bool _ready = false;
  bool _signedIn = false;

  bool get ready => _ready;
  bool get signedIn => _signedIn;

  Future<void> restore() async {
    final token = await _store.accessToken();
    _signedIn = token != null && token.isNotEmpty;
    _ready = true;
    notifyListeners();
  }

  Future<void> setSignedIn(bool value) async {
    _signedIn = value;
    _ready = true;
    notifyListeners();
  }

  Future<void> signOut() async {
    await _store.clear();
    await setSignedIn(false);
  }
}
