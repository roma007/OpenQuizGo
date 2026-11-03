import 'package:flutter/material.dart';

import '../../core/widgets/placeholder_card.dart';

class ReportPage extends StatelessWidget {
  const ReportPage({super.key});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: const [
        PlaceholderCard(
          icon: Icons.insights,
          title: '成长报告',
          detail: 'T6：周/月答题与观看统计',
        ),
      ],
    );
  }
}
