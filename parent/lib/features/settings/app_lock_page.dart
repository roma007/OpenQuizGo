import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';

class AppLockPage extends ConsumerWidget {
  const AppLockPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final enabled = ref.watch(appLockEnabledProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('儿童锁')),
      body: ContentList(
        children: [
          const Padding(
            padding: EdgeInsets.all(16),
            child: Text(
              '儿童锁只保护本机家长端 App，与服务端家长 PIN 独立。开启后每次启动或回到前台都需要输入。',
            ),
          ),
          enabled.when(
            loading: () => const ListTile(title: Text('加载中…')),
            error: (error, _) => ListTile(title: Text('加载失败：$error')),
            data: (isEnabled) => Column(
              children: [
                SwitchListTile(
                  title: const Text('启用儿童锁'),
                  subtitle: Text(isEnabled ? '已开启' : '未开启'),
                  value: isEnabled,
                  onChanged: (value) async {
                    if (value) {
                      final pin = await _promptNewPin(context);
                      if (pin == null) return;
                      await ref.read(appLockStoreProvider).setPin(pin);
                    } else {
                      final pin = await _promptPin(
                        context,
                        title: '关闭儿童锁',
                        action: '关闭',
                      );
                      if (pin == null) return;
                      final ok = await ref
                          .read(appLockStoreProvider)
                          .verify(pin);
                      if (!ok) {
                        if (context.mounted) {
                          _snack(context, 'PIN 不正确');
                        }
                        return;
                      }
                      await ref.read(appLockStoreProvider).disable();
                    }
                    ref.invalidate(appLockEnabledProvider);
                    if (context.mounted) {
                      _snack(context, value ? '儿童锁已开启' : '儿童锁已关闭');
                    }
                  },
                ),
                if (isEnabled)
                  ListTile(
                    leading: const Icon(Icons.password),
                    title: const Text('修改儿童锁 PIN'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () async {
                      final pin = await _promptNewPin(context);
                      if (pin == null) return;
                      await ref.read(appLockStoreProvider).setPin(pin);
                      ref.invalidate(appLockEnabledProvider);
                      if (context.mounted) _snack(context, '儿童锁 PIN 已更新');
                    },
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Future<String?> _promptNewPin(BuildContext context) async {
    final first = TextEditingController();
    final second = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('设置儿童锁 PIN'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: first,
              autofocus: true,
              obscureText: true,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              maxLength: 6,
              decoration: const InputDecoration(
                labelText: '新 PIN（4–6 位）',
                counterText: '',
              ),
            ),
            TextField(
              controller: second,
              obscureText: true,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              maxLength: 6,
              decoration: const InputDecoration(
                labelText: '确认 PIN',
                counterText: '',
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () {
              final a = first.text.trim();
              final b = second.text.trim();
              if (a.length < 4 || a.length > 6 || a != b) {
                Navigator.pop(context);
                return;
              }
              Navigator.pop(context, a);
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
    first.dispose();
    second.dispose();
    return result;
  }

  Future<String?> _promptPin(
    BuildContext context, {
    required String title,
    required String action,
  }) async {
    final controller = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          autofocus: true,
          obscureText: true,
          keyboardType: TextInputType.number,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          maxLength: 6,
          decoration: const InputDecoration(
            labelText: '儿童锁 PIN',
            counterText: '',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: Text(action),
          ),
        ],
      ),
    );
    controller.dispose();
    return result;
  }

  void _snack(BuildContext context, String message) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }
}

class ContentList extends StatelessWidget {
  const ContentList({super.key, required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return ListView(children: children);
  }
}
