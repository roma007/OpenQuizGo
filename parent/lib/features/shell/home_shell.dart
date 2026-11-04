import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../alerts/alerts_page.dart';
import '../child/child_page.dart';
import '../device/device_page.dart';
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

  static const _destinations = [
    NavigationRailDestination(icon: Icon(Icons.child_care), label: Text('孩子')),
    NavigationRailDestination(icon: Icon(Icons.tv), label: Text('设备')),
    NavigationRailDestination(icon: Icon(Icons.insights), label: Text('报告')),
    NavigationRailDestination(icon: Icon(Icons.settings), label: Text('设置')),
  ];

  @override
  Widget build(BuildContext context) {
    final alerts = ref.watch(alertsProvider);
    final unread = alerts.whenOrNull(data: (data) => data.unread) ?? 0;
    return LayoutBuilder(
      builder: (context, constraints) {
        final wide = isExpandedWidth(constraints.maxWidth);
        return Scaffold(
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
          body: wide
              ? Row(
                  children: [
                    NavigationRail(
                      selectedIndex: _index,
                      labelType: NavigationRailLabelType.all,
                      onDestinationSelected: (value) =>
                          setState(() => _index = value),
                      destinations: _destinations,
                    ),
                    const VerticalDivider(width: 1),
                    Expanded(
                      child: ContentFrame(
                        child: IndexedStack(index: _index, children: _pages),
                      ),
                    ),
                  ],
                )
              : ContentFrame(
                  child: IndexedStack(index: _index, children: _pages),
                ),
          bottomNavigationBar: wide
              ? null
              : NavigationBar(
                  selectedIndex: _index,
                  onDestinationSelected: (value) =>
                      setState(() => _index = value),
                  destinations: const [
                    NavigationDestination(
                      icon: Icon(Icons.child_care),
                      label: '孩子',
                    ),
                    NavigationDestination(icon: Icon(Icons.tv), label: '设备'),
                    NavigationDestination(
                      icon: Icon(Icons.insights),
                      label: '报告',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.settings),
                      label: '设置',
                    ),
                  ],
                ),
        );
      },
    );
  }
}
