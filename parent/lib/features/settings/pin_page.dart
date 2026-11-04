import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/providers.dart';

class PinPage extends ConsumerStatefulWidget {
  const PinPage({super.key});

  @override
  ConsumerState<PinPage> createState() => _PinPageState();
}

class _PinPageState extends ConsumerState<PinPage> {
  final _formKey = GlobalKey<FormState>();
  final _current = TextEditingController();
  final _next = TextEditingController();
  final _confirm = TextEditingController();
  bool _saving = false;

  @override
  void dispose() {
    _current.dispose();
    _next.dispose();
    _confirm.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _saving = true);
    try {
      await ref
          .read(familyRepositoryProvider)
          .setPin(currentPin: _current.text.trim(), newPin: _next.text.trim());
      if (mounted) {
        Navigator.of(context).pop(true);
      }
    } on ApiException catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('家长 PIN')),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            const Text('PIN 用于修改规则和远程解锁。首次设置时「当前 PIN」留空。'),
            const SizedBox(height: 20),
            _pinField(_current, '当前 PIN（首次设置留空）', required: false),
            const SizedBox(height: 12),
            _pinField(_next, '新 PIN（4–6 位数字）'),
            const SizedBox(height: 12),
            _pinField(_confirm, '确认新 PIN', confirm: true),
            const SizedBox(height: 28),
            FilledButton(
              onPressed: _saving ? null : _save,
              child: _saving
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('保存 PIN'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _pinField(
    TextEditingController controller,
    String label, {
    bool required = true,
    bool confirm = false,
  }) {
    return TextFormField(
      controller: controller,
      obscureText: true,
      keyboardType: TextInputType.number,
      inputFormatters: [FilteringTextInputFormatter.digitsOnly],
      maxLength: 6,
      decoration: InputDecoration(labelText: label, counterText: ''),
      validator: (value) {
        final pin = value?.trim() ?? '';
        if (!required && pin.isEmpty) return null;
        if (!RegExp(r'^\d{4,6}$').hasMatch(pin)) {
          return '请输入 4–6 位数字';
        }
        if (confirm && pin != _next.text.trim()) {
          return '两次输入不一致';
        }
        return null;
      },
    );
  }
}
