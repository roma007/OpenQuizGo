import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/providers.dart';

class BindDevicePage extends ConsumerStatefulWidget {
  const BindDevicePage({super.key});

  @override
  ConsumerState<BindDevicePage> createState() => _BindDevicePageState();
}

class _BindDevicePageState extends ConsumerState<BindDevicePage> {
  final _code = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  String? _childId;
  bool _submitting = false;

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    if (_childId == null) {
      _snack('请选择要绑定的孩子');
      return;
    }
    setState(() => _submitting = true);
    try {
      await ref
          .read(deviceRepositoryProvider)
          .activate(code: _code.text.trim(), childId: _childId!);
      if (mounted) Navigator.of(context).pop(true);
    } on ApiException catch (error) {
      if (mounted) _snack(error.message);
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
    final children = ref.watch(childrenProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('绑定电视')),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            const Text('在电视上打开 OpenQuizGo 守护端，首页会显示 6 位激活码。'),
            const SizedBox(height: 16),
            TextFormField(
              controller: _code,
              keyboardType: TextInputType.number,
              maxLength: 6,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              decoration: const InputDecoration(
                labelText: '激活码',
                counterText: '',
                hintText: '例如 123456',
                prefixIcon: Icon(Icons.pin_outlined),
              ),
              validator: (value) {
                final code = value?.trim() ?? '';
                if (code.length != 6) return '请输入 6 位激活码';
                return null;
              },
            ),
            const SizedBox(height: 20),
            children.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => Text(
                '加载孩子失败：$error',
                style: const TextStyle(color: Colors.red),
              ),
              data: (items) {
                if (items.isEmpty) {
                  return const Text(
                    '还没有孩子档案，请先在「孩子」页创建孩子后再绑定。',
                    style: TextStyle(color: Colors.orange),
                  );
                }
                _childId ??= items.first.id;
                return DropdownButtonFormField<String>(
                  initialValue: _childId,
                  decoration: const InputDecoration(labelText: '绑定到哪个孩子'),
                  items: [
                    for (final child in items)
                      DropdownMenuItem(
                        value: child.id,
                        child: Text(child.nickname),
                      ),
                  ],
                  onChanged: (value) => setState(() => _childId = value),
                );
              },
            ),
            const SizedBox(height: 28),
            FilledButton(
              onPressed: _submitting ? null : _submit,
              child: _submitting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('确认绑定'),
            ),
            const SizedBox(height: 12),
            Text(
              '激活码有效期为 5 分钟；绑定后电视会自动进入守护状态。',
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ],
        ),
      ),
    );
  }
}
