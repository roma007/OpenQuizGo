import 'package:flutter/material.dart';

/// 设备形态：手机与平板使用不同的导航与栅格策略。
enum FormFactor { phone, tablet }

class Breakpoints {
  /// 最短边达到该值按平板处理，避免 iPhone 横屏被误判为平板。
  static const double tabletShortestSide = 600;

  /// 内容区宽度达到该值使用两列卡片。
  static const double twoColumn = 640;

  /// 内容区宽度达到该值报告页左右分栏。
  static const double twoPane = 900;

  /// 内容区宽度达到该值使用三列卡片。
  static const double threeColumn = 1080;

  /// 平板宽度达到该值展开侧边导航（显示文字）。
  static const double extendedRail = 1200;

  static const double contentMax = 1200;
  static const double formMax = 720;
  static const double listMax = 1000;
}

FormFactor formFactorOf(Size size) =>
    size.shortestSide >= Breakpoints.tabletShortestSide
    ? FormFactor.tablet
    : FormFactor.phone;

bool isTabletSize(Size size) => formFactorOf(size) == FormFactor.tablet;

bool isTablet(BuildContext context) => isTabletSize(MediaQuery.sizeOf(context));

/// 根据内容区可用宽度返回卡片列数。
int gridColumns(double width) {
  if (width >= Breakpoints.threeColumn) return 3;
  if (width >= Breakpoints.twoColumn) return 2;
  return 1;
}

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
