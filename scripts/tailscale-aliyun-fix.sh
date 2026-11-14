#!/usr/bin/env bash
# 修复 Tailscale 与云厂商「100.64.0.0/10」内网段冲突
#
# 背景：tailscaled 会创建防欺骗规则
#   -A ts-input -s 100.64.0.0/10 ! -i tailscale0 -j DROP
# 而阿里云的内网服务都落在 100.64.0.0/10，从 eth0 进来，于是返回包被内核丢弃：
#   - 实例元数据   100.100.100.200
#   - 内网 DNS     100.100.2.136 / 100.100.2.138
#   - 云安全中心   100.100.30.26 / 100.100.32.66 / 100.100.25.4
#   - 云备份 HBR   100.100.0.28
#   - OSS 内网     oss-cn-<region>-internal.aliyuncs.com → 100.115.33.x
# 症状：DNS 全废、元数据不可达、云安全中心离线、文件备份失败（备份点未生成）。
#
# 处理：在 ts-input 链最前面插入「eth0 上来自 100.64.0.0/10 的包一律放行」。
# 只对默认路由网卡生效，docker0 / br-* 等其它网卡仍受 Tailscale 的 DROP 保护。
# 幂等：已存在则直接退出；tailscaled 重启后会重建规则，由 timer 自动补回。
set -u

command -v iptables >/dev/null 2>&1 || exit 0

# tailscaled 没运行时没有 ts-input 链，直接跳过（开机早期会走这个分支）
iptables -S ts-input >/dev/null 2>&1 || exit 0

# 默认路由所在网卡（阿里云 ECS 通常是 eth0）
IFACE=$(ip route show default 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i == "dev") { print $(i + 1); exit }}')
[ -n "$IFACE" ] || exit 0

# 清理早期版本留下的窄规则（100.100.0.0/16），它覆盖不到 OSS 等 100.115.x.x
while iptables -C ts-input -s 100.100.0.0/16 -j ACCEPT 2>/dev/null; do
    iptables -D ts-input -s 100.100.0.0/16 -j ACCEPT
done

# 已有正确的规则就不重复插入
iptables -C ts-input -i "$IFACE" -s 100.64.0.0/10 -j ACCEPT 2>/dev/null && exit 0

iptables -I ts-input 1 -i "$IFACE" -s 100.64.0.0/10 -j ACCEPT
