import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/config.dart';
import '../../core/providers.dart';
import 'members_page.dart';
import 'pin_page.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Card(
          child: ListTile(
            leading: const Icon(Icons.lock_outline),
            title: const Text('家长 PIN'),
            subtitle: const Text('修改 PIN（规则保存与远程解锁需要）'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () async {
              final saved = await Navigator.of(
                context,
              ).push<bool>(MaterialPageRoute(builder: (_) => const PinPage()));
              if (saved == true && context.mounted) {
                ScaffoldMessenger.of(
                  context,
                ).showSnackBar(const SnackBar(content: Text('家长 PIN 已更新')));
              }
            },
          ),
        ),
        const SizedBox(height: 12),
        Card(
          child: ListTile(
            leading: const Icon(Icons.group_outlined),
            title: const Text('家庭成员'),
            subtitle: const Text('邀请另一位家长、加入家庭'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => Navigator.of(
              context,
            ).push(MaterialPageRoute(builder: (_) => const MembersPage())),
          ),
        ),
        const SizedBox(height: 12),
        Card(
          child: ListTile(
            leading: const Icon(Icons.info_outline),
            title: const Text('关于'),
            subtitle: Text('OpenQuizGo 家长端 ${AppConfig.appVersion}'),
          ),
        ),
        const SizedBox(height: 24),
        OutlinedButton.icon(
          onPressed: () async {
            await ref.read(authRepositoryProvider).logout();
            await ref.read(authControllerProvider).signOut();
          },
          icon: const Icon(Icons.logout),
          label: const Text('退出登录'),
        ),
      ],
    );
  }
}
