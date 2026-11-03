import 'package:flutter/material.dart';

import '../../core/widgets/placeholder_card.dart';

class DevicePage extends StatelessWidget {
  const DevicePage({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: const [
        PlaceholderCard(
          icon: Icons.tv,
          title: '电视设备',
          detail: 'T4：激活码绑定、设备列表、解绑；T7：远程解锁',
        ),
      ],
    );
  }
}
