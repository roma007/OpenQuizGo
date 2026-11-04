import 'package:flutter/material.dart';

import '../child/child_page.dart';
import '../device/device_page.dart';
import '../report/report_page.dart';
import '../settings/settings_page.dart';

class HomeShell extends StatefulWidget {
  const HomeShell({super.key});

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;

  static const _titles = ['孩子', '设备', '报告', '设置'];

  static const _pages = [
    ChildPage(),
    DevicePage(),
    ReportPage(),
    SettingsPage(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(_titles[_index])),
      body: IndexedStack(index: _index, children: _pages),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (value) => setState(() => _index = value),
        destinations: const [
          NavigationDestination(icon: Icon(Icons.child_care), label: '孩子'),
          NavigationDestination(icon: Icon(Icons.tv), label: '设备'),
          NavigationDestination(icon: Icon(Icons.insights), label: '报告'),
          NavigationDestination(icon: Icon(Icons.settings), label: '设置'),
        ],
      ),
    );
  }
}
