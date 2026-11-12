import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../data/membership_repository.dart';
import '../membership/membership_page.dart';
import 'app_lock_page.dart';
import 'members_page.dart';
import 'pin_page.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final membership = ref.watch(membershipCenterProvider).value?.membership;
    final packageInfo = ref
        .watch(packageInfoProvider)
        .whenOrNull(data: (value) => value);
    final membershipSubtitle = membership?.active == true
        ? '${membership!.planName} · 有效期至 ${formatDate(membership.expiresAt)}'
        : membership?.status == 'expired'
        ? '会员已到期，点击续费'
        : '查看会员状态、套餐与订单（模拟支付）';
    return ContentFrame(
      maxWidth: 800,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Card(
            child: ListTile(
              leading: const Icon(Icons.lock_outline),
              title: const Text('家长 PIN'),
              subtitle: const Text('修改 PIN（规则保存与远程解锁需要）'),
              trailing: const Icon(Icons.chevron_right),
              onTap: () async {
                final saved = await Navigator.of(context).push<bool>(
                  MaterialPageRoute(builder: (_) => const PinPage()),
                );
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
              leading: const Icon(Icons.child_care_outlined),
              title: const Text('儿童锁'),
              subtitle: const Text('保护本机家长端 App，防止孩子修改规则'),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => Navigator.of(
                context,
              ).push(MaterialPageRoute(builder: (_) => const AppLockPage())),
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
              leading: const Icon(Icons.workspace_premium_outlined),
              title: const Text('会员中心'),
              subtitle: Text(membershipSubtitle),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => Navigator.of(
                context,
              ).push(MaterialPageRoute(builder: (_) => const MembershipPage())),
            ),
          ),
          const SizedBox(height: 12),
          Card(
            child: ListTile(
              leading: const Icon(Icons.info_outline),
              title: const Text('关于'),
              subtitle: Text(
                packageInfo == null
                    ? '电视时间管家'
                    : '电视时间管家 v${packageInfo.version} '
                          '(${packageInfo.buildNumber})',
              ),
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
      ),
    );
  }
}
