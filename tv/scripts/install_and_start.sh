#!/usr/bin/env bash
# 安装并完成后台初始化：
#   1. 安装 APK；
#   2. 通过无界面 BootstrapActivity 启动守护服务（不显示任何前端界面，
#      同时让 Android 退出新装应用的 stopped 状态）；
#   3. 可选：通过 adb 直接开启无障碍服务（ENABLE_A11Y=1）。
#
# 用法：
#   ./scripts/install_and_start.sh [apk路径]
#   ENABLE_A11Y=1 ./scripts/install_and_start.sh
set -euo pipefail

cd "$(dirname "$0")/.."

PKG="com.openquizgo.quiz"
BOOTSTRAP="$PKG/com.openquizgo.guard.ui.BootstrapActivity"
A11Y="$PKG/com.openquizgo.guard.platform.GuardAccessibilityService"
APK="${1:-app/build/outputs/apk/release/app-release.apk}"

if [[ ! -f "$APK" ]]; then
  echo "找不到 APK：$APK"
  echo "请先执行：./gradlew :app:assembleRelease"
  exit 1
fi

echo "安装 $APK"
adb install -r "$APK"

echo "后台初始化（不显示前端）"
adb shell am start -n "$BOOTSTRAP" >/dev/null
sleep 2

if [[ "${ENABLE_A11Y:-0}" == "1" ]]; then
  echo "开启无障碍服务"
  adb shell settings put secure enabled_accessibility_services "$A11Y"
  adb shell settings put secure accessibility_enabled 1
fi

echo
echo "完成：守护服务已在后台运行，开关默认开启。"
echo "以后每次开机由 BootReceiver 自动启动，不需要再打开 App。"
echo "如需进入设置开关界面，点击电视桌面上的「电视时间管家」即可。"
