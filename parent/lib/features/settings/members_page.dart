import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/api/api_client.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';

class MembersPage extends ConsumerWidget {
  const MembersPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final members = ref.watch(membersProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('家庭成员')),
      body: ContentFrame(
        maxWidth: 1000,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () => _invite(context, ref),
                      icon: const Icon(Icons.person_add_alt),
                      label: const Text('生成邀请码'),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () => _join(context, ref),
                      icon: const Icon(Icons.group_add),
                      label: const Text('加入家庭'),
                    ),
                  ),
                ],
              ),
            ),
            Expanded(
              child: members.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text('加载成员失败：$error'),
                      const SizedBox(height: 12),
                      OutlinedButton(
                        onPressed: () => ref.invalidate(membersProvider),
                        child: const Text('重试'),
                      ),
                    ],
                  ),
                ),
                data: (items) => ListView.separated(
                  padding: const EdgeInsets.all(16),
                  itemCount: items.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 12),
                  itemBuilder: (context, index) {
                    final member = items[index];
                    return Card(
                      child: ListTile(
                        leading: CircleAvatar(
                          child: Icon(
                            member.role == 'owner'
                                ? Icons.admin_panel_settings
                                : Icons.person,
                          ),
                        ),
                        title: Text(member.phone),
                        subtitle: Text(
                          '${_roleLabel(member.role)}'
                          '${member.joinedAt == null ? '' : ' · 加入于 ${DateFormat('yyyy-MM-dd').format(member.joinedAt!.toLocal())}'}',
                        ),
                      ),
                    );
                  },
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _roleLabel(String role) => switch (role) {
    'owner' => '家庭管理员',
    'member' => '家庭成员',
    _ => role,
  };

  Future<void> _invite(BuildContext context, WidgetRef ref) async {
    try {
      final invite = await ref.read(familyRepositoryProvider).inviteMember();
      if (!context.mounted) return;
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('家庭邀请码'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                invite.code,
                style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                  letterSpacing: 8,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 12),
              const Text('邀请码 7 天内有效，在另一台手机登录后选择「加入家庭」。'),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () async {
                await Clipboard.setData(ClipboardData(text: invite.code));
                if (context.mounted) Navigator.pop(context);
              },
              child: const Text('复制'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } on ApiException catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
      }
    }
  }

  Future<void> _join(BuildContext context, WidgetRef ref) async {
    final controller = TextEditingController();
    final code = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('加入家庭'),
        content: TextField(
          controller: controller,
          autofocus: true,
          keyboardType: TextInputType.number,
          maxLength: 6,
          decoration: const InputDecoration(
            labelText: '6 位邀请码',
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
            child: const Text('加入'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (code == null || code.isEmpty) return;
    try {
      await ref.read(familyRepositoryProvider).joinFamily(code);
      ref.invalidate(childrenProvider);
      ref.invalidate(devicesProvider);
      ref.invalidate(membersProvider);
      ref.invalidate(ruleProvider);
      ref.invalidate(reportsProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('已加入家庭')));
      }
    } on ApiException catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
      }
    }
  }
}
