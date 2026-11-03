import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/providers.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _phone = TextEditingController();
  final _code = TextEditingController();
  final _formKey = GlobalKey<FormState>();

  bool _submitting = false;
  int _countdown = 0;
  Timer? _timer;

  @override
  void dispose() {
    _timer?.cancel();
    _phone.dispose();
    _code.dispose();
    super.dispose();
  }

  Future<void> _sendCode() async {
    final phone = _phone.text.trim();
    if (phone.length != 11) {
      _snack('请输入 11 位手机号');
      return;
    }
    setState(() => _countdown = 60);
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) return;
      setState(() {
        _countdown -= 1;
        if (_countdown <= 0) timer.cancel();
      });
    });
    try {
      await ref.read(authRepositoryProvider).requestCode(phone);
      _snack('验证码已发送（开发环境固定 123456）');
    } on ApiException catch (error) {
      _snack(error.message);
    }
  }

  Future<void> _login() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    try {
      await ref
          .read(authRepositoryProvider)
          .login(phone: _phone.text.trim(), code: _code.text.trim());
      await ref.read(authControllerProvider).setSignedIn(true);
    } on ApiException catch (error) {
      _snack(error.message);
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  void _snack(String message) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 32),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'OpenQuizGo',
                      style: theme.textTheme.headlineMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: theme.colorScheme.primary,
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text('家长端', style: theme.textTheme.titleMedium),
                    const SizedBox(height: 32),
                    TextFormField(
                      controller: _phone,
                      keyboardType: TextInputType.phone,
                      maxLength: 11,
                      decoration: const InputDecoration(
                        labelText: '手机号',
                        counterText: '',
                        prefixIcon: Icon(Icons.phone_android),
                      ),
                      validator: (value) {
                        final phone = value?.trim() ?? '';
                        if (!RegExp(r'^1[3-9]\d{9}$').hasMatch(phone)) {
                          return '请输入正确的手机号';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 16),
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Expanded(
                          child: TextFormField(
                            controller: _code,
                            keyboardType: TextInputType.number,
                            maxLength: 6,
                            decoration: const InputDecoration(
                              labelText: '验证码',
                              counterText: '',
                              prefixIcon: Icon(Icons.sms_outlined),
                            ),
                            validator: (value) =>
                                (value?.trim().length ?? 0) >= 4
                                ? null
                                : '请输入验证码',
                          ),
                        ),
                        const SizedBox(width: 12),
                        SizedBox(
                          height: 52,
                          child: OutlinedButton(
                            onPressed: _countdown > 0 ? null : _sendCode,
                            child: Text(
                              _countdown > 0 ? '${_countdown}s' : '获取验证码',
                            ),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 28),
                    FilledButton(
                      onPressed: _submitting ? null : _login,
                      child: _submitting
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('登录'),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      '登录即表示同意《用户协议》与《隐私政策》',
                      style: theme.textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
