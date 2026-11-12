import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/layout/adaptive.dart';
import '../../core/layout/adaptive_shell.dart';
import '../../core/providers.dart';
import '../alerts/alerts_page.dart';
import '../child/child_page.dart';
import '../device/device_page.dart';
import '../membership/membership_banner.dart';
import '../report/report_page.dart';
import '../settings/settings_page.dart';

class HomeShell extends ConsumerStatefulWidget {
  const HomeShell({super.key});

  @override
  ConsumerState<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends ConsumerState<HomeShell> {
  int _index = 0;

  static const _titles = ['孩子', '设备', '报告', '设置'];

  static const _pages = [
    ChildPage(),
    DevicePage(),
    ReportPage(),
    SettingsPage(),
  ];

  static const _items = [
    AdaptiveNavItem(icon: Icons.child_care, label: '孩子'),
    AdaptiveNavItem(icon: Icons.tv, label: '设备'),
    AdaptiveNavItem(icon: Icons.insights, label: '报告'),
    AdaptiveNavItem(icon: Icons.settings, label: '设置'),
  ];

  @override
  Widget build(BuildContext context) {
    final alerts = ref.watch(alertsProvider);
    final unread = alerts.whenOrNull(data: (data) => data.unread) ?? 0;
    return AdaptiveShell(
      selectedIndex: _index,
      onSelected: (value) => setState(() => _index = value),
      items: _items,
      appBar: AppBar(
        title: Text(_titles[_index]),
        actions: [
          IconButton(
            tooltip: '消息中心',
            onPressed: () async {
              await Navigator.of(
                context,
              ).push(MaterialPageRoute(builder: (_) => const AlertsPage()));
              ref.invalidate(alertsProvider);
            },
            icon: Badge.count(
              count: unread,
              isLabelVisible: unread > 0,
              child: const Icon(Icons.notifications_outlined),
            ),
          ),
        ],
      ),
      body: ContentFrame(
        child: Column(
          children: [
            const MembershipBanner(),
            Expanded(
              child: IndexedStack(index: _index, children: _pages),
            ),
          ],
        ),
      ),
    );
  }
}
