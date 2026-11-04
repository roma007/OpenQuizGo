import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../data/membership_repository.dart';

const entitlementLabels = <String, String>{
  'topic_pack': '付费专题题包',
  'report_advanced': '进阶报告',
  'customer_service': '专属客服',
};

class MembershipPage extends ConsumerStatefulWidget {
  const MembershipPage({super.key});

  @override
  ConsumerState<MembershipPage> createState() => _MembershipPageState();
}

class _MembershipPageState extends ConsumerState<MembershipPage> {
  MembershipCenter? _center;
  List<MembershipOrder> _orders = const [];
  bool _loading = true;
  String? _payingPlanId;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final results = await Future.wait([
        ref.read(membershipRepositoryProvider).center(),
        ref.read(membershipRepositoryProvider).orders(),
      ]);
      if (!mounted) return;
      setState(() {
        _center = results[0] as MembershipCenter;
        _orders = results[1] as List<MembershipOrder>;
      });
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _buy(MembershipPlan plan) async {
    final active = _center?.membership?.active == true;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('模拟支付'),
        content: Text(
          '这是模拟支付，不会产生真实扣款。\n\n'
          '确认支付 ${formatYuan(plan.priceCents)} ${active ? '续费' : '开通'}'
          '「${plan.name}」（${plan.durationDays} 天）？',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('确认支付'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _payingPlanId = plan.id);
    try {
      final order = await ref
          .read(membershipRepositoryProvider)
          .createOrder(plan.id);
      final result = await ref
          .read(membershipRepositoryProvider)
          .payOrder(order.id);
      await _load();
      _toast('开通成功，有效期至 ${formatDate(result.membership.expiresAt)}');
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _payingPlanId = null);
    }
  }

  void _toast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('会员中心')),
      body: ContentFrame(
        maxWidth: 900,
        child: _loading && _center == null
            ? const Center(child: CircularProgressIndicator())
            : RefreshIndicator(
                onRefresh: _load,
                child: ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    _statusCard(context),
                    const SizedBox(height: 20),
                    Text(
                      '会员套餐',
                      style: Theme.of(context).textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    const SizedBox(height: 8),
                    if ((_center?.plans ?? const []).isEmpty)
                      const Card(
                        child: Padding(
                          padding: EdgeInsets.all(20),
                          child: Text('暂无可用套餐，请稍后再试。'),
                        ),
                      )
                    else
                      for (final plan in _center!.plans)
                        _planCard(context, plan),
                    const SizedBox(height: 20),
                    Text(
                      '最近订单',
                      style: Theme.of(context).textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    const SizedBox(height: 8),
                    if (_orders.isEmpty)
                      const Card(
                        child: Padding(
                          padding: EdgeInsets.all(20),
                          child: Text('还没有订单。'),
                        ),
                      )
                    else
                      for (final order in _orders) _orderCard(context, order),
                  ],
                ),
              ),
      ),
    );
  }

  Widget _statusCard(BuildContext context) {
    final membership = _center?.membership;
    final active = membership?.active == true;
    final color = active ? const Color(0xFF1B8A6B) : const Color(0xFF8B8B8B);
    final title = active
        ? '会员有效'
        : membership?.status == 'expired'
        ? '会员已到期'
        : '尚未开通会员';
    final subtitle = active
        ? '${membership!.planName} · 有效期至 ${formatDate(membership.expiresAt)}（剩余 ${membership.daysRemaining} 天）'
        : membership?.status == 'expired'
        ? '${membership!.planName} · 到期时间 ${formatDate(membership.expiresAt)}'
        : '选择下方套餐，使用模拟支付立即开通。';
    return Card(
      color: active ? const Color(0xFFE7F8EF) : null,
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Row(
          children: [
            Icon(Icons.workspace_premium_outlined, size: 48, color: color),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: 24,
                      fontWeight: FontWeight.bold,
                      color: color,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(subtitle),
                  if (active && membership!.entitlements.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        for (final code in membership.entitlements)
                          Chip(
                            avatar: const Icon(Icons.check_circle, size: 16),
                            label: Text(entitlementLabels[code] ?? code),
                          ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _planCard(BuildContext context, MembershipPlan plan) {
    final active = _center?.membership?.active == true;
    final paying = _payingPlanId == plan.id;
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    plan.name,
                    style: const TextStyle(
                      fontSize: 22,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                Text(
                  formatYuan(plan.priceCents),
                  style: const TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.bold,
                    color: Color(0xFF1B8A6B),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text('${plan.durationDays} 天 · ${plan.description}'),
            if (plan.entitlementCodes.isNotEmpty) ...[
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final code in plan.entitlementCodes)
                    Chip(
                      avatar: const Icon(Icons.lock_open, size: 16),
                      label: Text('功能：${entitlementLabels[code] ?? code}'),
                    ),
                ],
              ),
            ],
            if (plan.features.isNotEmpty) ...[
              const SizedBox(height: 12),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final feature in plan.features)
                    Chip(label: Text(feature)),
                ],
              ),
            ],
            const SizedBox(height: 12),
            Align(
              alignment: Alignment.centerRight,
              child: FilledButton(
                onPressed: paying ? null : () => _buy(plan),
                child: paying
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(active ? '续费' : '开通'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _orderCard(BuildContext context, MembershipOrder order) {
    final paid = order.status == 'paid';
    final label = switch (order.status) {
      'paid' => '已支付',
      'refunded' => '已退款',
      'cancelled' => '已取消',
      _ => '待支付',
    };
    final reason = order.status == 'refunded' && order.refundReason.isNotEmpty
        ? '\n退款原因：${order.refundReason}'
        : '';
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Icon(
          paid ? Icons.check_circle_outline : Icons.receipt_long_outlined,
          color: paid ? const Color(0xFF1B8A6B) : null,
        ),
        title: Text('${order.planName} · ${formatYuan(order.amountCents)}'),
        subtitle: Text(
          '${formatDateTime(order.createdAt)} · ${order.orderNo}\n$label$reason',
        ),
        isThreeLine: true,
      ),
    );
  }
}
