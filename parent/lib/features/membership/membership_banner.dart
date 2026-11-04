import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';
import 'membership_page.dart';

class MembershipBanner extends ConsumerWidget {
  const MembershipBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final membership = ref.watch(membershipCenterProvider).value?.membership;
    if (membership == null) return const SizedBox.shrink();

    final expiring = membership.active && membership.daysRemaining <= 7;
    final expired = !membership.active && membership.status == 'expired';
    if (!expiring && !expired) return const SizedBox.shrink();

    final days = membership.daysRemaining.clamp(1, 7);
    final background = expiring
        ? const Color(0xFFFFF3E0)
        : const Color(0xFFFFEBEE);
    final foreground = expiring
        ? const Color(0xFF9A5B00)
        : const Color(0xFFC62828);
    final title = expiring ? '会员将在 $days 天后到期' : '会员已到期，会员权益已暂停';
    final subtitle = expiring ? '续费后继续享受专题题包与进阶报告权益' : '点击续费，恢复会员权益';

    return Material(
      color: background,
      child: InkWell(
        onTap: () async {
          await Navigator.of(
            context,
          ).push(MaterialPageRoute(builder: (_) => const MembershipPage()));
          ref.invalidate(membershipCenterProvider);
        },
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              Icon(
                expiring ? Icons.schedule : Icons.error_outline,
                color: foreground,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: TextStyle(
                        color: foreground,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(subtitle, style: TextStyle(color: foreground)),
                  ],
                ),
              ),
              Text(
                '去续费',
                style: TextStyle(
                  color: foreground,
                  fontWeight: FontWeight.bold,
                ),
              ),
              Icon(Icons.chevron_right, color: foreground),
            ],
          ),
        ),
      ),
    );
  }
}
