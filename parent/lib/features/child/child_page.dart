import 'package:flutter/material.dart';

import '../../core/widgets/placeholder_card.dart';

class ChildPage extends StatelessWidget {
  const ChildPage({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: const [
        PlaceholderCard(
          icon: Icons.child_care,
          title: '孩子档案',
          detail: 'T3：孩子列表、新增与编辑',
        ),
      ],
    );
  }
}
