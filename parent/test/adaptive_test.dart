import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/layout/adaptive.dart';

void main() {
  test('按最短边区分手机与平板', () {
    expect(formFactorOf(const Size(390, 844)), FormFactor.phone);
    expect(formFactorOf(const Size(844, 390)), FormFactor.phone);
    expect(formFactorOf(const Size(320, 1024)), FormFactor.phone);
    expect(formFactorOf(const Size(820, 1180)), FormFactor.tablet);
    expect(formFactorOf(const Size(1180, 820)), FormFactor.tablet);
    expect(formFactorOf(const Size(1366, 1024)), FormFactor.tablet);
  });

  test('内容区宽度决定卡片列数', () {
    expect(gridColumns(500), 1);
    expect(gridColumns(639), 1);
    expect(gridColumns(640), 2);
    expect(gridColumns(900), 2);
    expect(gridColumns(1079), 2);
    expect(gridColumns(1080), 3);
    expect(gridColumns(1400), 3);
  });
}
