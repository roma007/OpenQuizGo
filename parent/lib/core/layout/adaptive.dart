import 'package:flutter/material.dart';

class Breakpoints {
  static const double medium = 720;
  static const double expanded = 1100;
  static const double contentMax = 1200;
  static const double formMax = 720;
  static const double listMax = 1000;
}

bool isExpandedWidth(double width) => width >= Breakpoints.expanded;

/// 居中并限制最大宽度，避免平板/横屏内容被拉伸。
class ContentFrame extends StatelessWidget {
  const ContentFrame({
    super.key,
    required this.child,
    this.maxWidth = Breakpoints.contentMax,
    this.padding = EdgeInsets.zero,
  });

  final Widget child;
  final double maxWidth;
  final EdgeInsets padding;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: maxWidth),
        child: Padding(padding: padding, child: child),
      ),
    );
  }
}
