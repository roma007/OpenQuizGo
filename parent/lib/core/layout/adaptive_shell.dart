import 'package:flutter/material.dart';

import 'adaptive.dart';

class AdaptiveNavItem {
  const AdaptiveNavItem({required this.icon, required this.label});

  final IconData icon;
  final String label;
}

/// 统一导航骨架：
/// - 手机（含横屏、分屏窄窗口）：底部 NavigationBar；
/// - 平板：侧边 NavigationRail，宽度足够时展开显示文字。
class AdaptiveShell extends StatelessWidget {
  const AdaptiveShell({
    super.key,
    required this.appBar,
    required this.body,
    required this.items,
    required this.selectedIndex,
    required this.onSelected,
  });

  final PreferredSizeWidget appBar;
  final Widget body;
  final List<AdaptiveNavItem> items;
  final int selectedIndex;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    final size = MediaQuery.sizeOf(context);
    if (!isTabletSize(size)) {
      return Scaffold(
        appBar: appBar,
        body: body,
        bottomNavigationBar: NavigationBar(
          selectedIndex: selectedIndex,
          onDestinationSelected: onSelected,
          destinations: [
            for (final item in items)
              NavigationDestination(icon: Icon(item.icon), label: item.label),
          ],
        ),
      );
    }

    final extended = size.width >= Breakpoints.extendedRail;
    return Scaffold(
      appBar: appBar,
      body: Row(
        children: [
          NavigationRail(
            extended: extended,
            selectedIndex: selectedIndex,
            labelType: extended
                ? NavigationRailLabelType.none
                : NavigationRailLabelType.all,
            onDestinationSelected: onSelected,
            destinations: [
              for (final item in items)
                NavigationRailDestination(
                  icon: Icon(item.icon),
                  label: Text(item.label),
                ),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(child: body),
        ],
      ),
    );
  }
}
