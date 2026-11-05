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
  List<MembershipInvoice> _invoices = const [];
  bool _loading = true;
  bool _sharingSaving = false;
  bool _autoRenewSaving = false;
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
        ref.read(membershipRepositoryProvider).invoices(),
      ]);
      if (!mounted) return;
      setState(() {
        _center = results[0] as MembershipCenter;
        _orders = results[1] as List<MembershipOrder>;
        _invoices = results[2] as List<MembershipInvoice>;
      });
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _buy(MembershipPlan plan) async {
    final active = _center?.membership?.active == true;
    final coupon = TextEditingController();
    final confirmed = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('模拟支付'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              '这是模拟支付，不会产生真实扣款。\n\n'
              '确认支付 ${formatYuan(plan.priceCents)} ${active ? '续费' : '开通'}'
              '「${plan.name}」（${plan.durationDays} 天）？',
            ),
            TextField(
              controller: coupon,
              decoration: const InputDecoration(
                labelText: '优惠码（可选）',
                hintText: '如 SAVE100',
              ),
              textCapitalization: TextCapitalization.characters,
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(coupon.text.trim()),
            child: const Text('确认支付'),
          ),
        ],
      ),
    );
    final code = confirmed;
    coupon.dispose();
    if (code == null || !mounted) return;
    setState(() => _payingPlanId = plan.id);
    try {
      final order = await ref
          .read(membershipRepositoryProvider)
          .createOrder(plan.id, couponCode: code);
      final result = await ref
          .read(membershipRepositoryProvider)
          .payOrder(order.id);
      await _load();
      final saved = order.discountCents > 0
          ? '，已优惠 ${formatYuan(order.discountCents)}'
          : '';
      _toast('开通成功$saved，有效期至 ${formatDate(result.membership.expiresAt)}');
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _payingPlanId = null);
    }
  }

  Future<void> _applyInvoice(MembershipOrder order) async {
    final title = TextEditingController();
    final taxNo = TextEditingController();
    final email = TextEditingController();
    var titleType = 'personal';
    final submitted = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: Text('申请发票 · ${order.orderNo}'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                initialValue: titleType,
                decoration: const InputDecoration(labelText: '抬头类型'),
                items: const [
                  DropdownMenuItem(value: 'personal', child: Text('个人')),
                  DropdownMenuItem(value: 'company', child: Text('企业')),
                ],
                onChanged: (value) =>
                    setDialogState(() => titleType = value ?? 'personal'),
              ),
              TextField(
                controller: title,
                decoration: const InputDecoration(labelText: '发票抬头'),
              ),
              if (titleType == 'company')
                TextField(
                  controller: taxNo,
                  decoration: const InputDecoration(labelText: '税号'),
                ),
              TextField(
                controller: email,
                decoration: const InputDecoration(labelText: '接收邮箱（可选）'),
                keyboardType: TextInputType.emailAddress,
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text('提交申请'),
            ),
          ],
        ),
      ),
    );
    if (submitted != true || !mounted) {
      title.dispose();
      taxNo.dispose();
      email.dispose();
      return;
    }
    try {
      await ref.read(membershipRepositoryProvider).applyInvoice(
        order.id,
        titleType: titleType,
        title: title.text.trim(),
        taxNo: taxNo.text.trim(),
        email: email.text.trim(),
      );
      await _load();
      _toast('发票申请已提交');
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      title.dispose();
      taxNo.dispose();
      email.dispose();
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
                    const SizedBox(height: 12),
                    _sharingCard(context),
                    const SizedBox(height: 12),
                    _autoRenewCard(context),
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

  Future<void> _toggleSharing(bool value) async {
    setState(() => _sharingSaving = true);
    try {
      await ref.read(membershipRepositoryProvider).setSharing(value);
      await _load();
      _toast(value ? '已开启家庭会员共享' : '已关闭家庭会员共享');
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _sharingSaving = false);
    }
  }

  Future<void> _configureAutoRenew() async {
    final plans = _center?.plans ?? const <MembershipPlan>[];
    if (plans.isEmpty) {
      _toast('暂无可用套餐');
      return;
    }
    var enabled = _center?.autoRenew.enabled ?? false;
    var planId = _center?.autoRenew.planId ?? plans.first.id;
    if (!plans.any((plan) => plan.id == planId)) planId = plans.first.id;
    final saved = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('自动续费预约'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('到期前生成待支付订单'),
                subtitle: const Text('不会自动扣款，需手动确认支付'),
                value: enabled,
                onChanged: (value) =>
                    setDialogState(() => enabled = value),
              ),
              DropdownButtonFormField<String>(
                initialValue: planId,
                decoration: const InputDecoration(labelText: '续费套餐'),
                items: [
                  for (final plan in plans)
                    DropdownMenuItem(value: plan.id, child: Text(plan.name)),
                ],
                onChanged: (value) =>
                    setDialogState(() => planId = value ?? planId),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text('保存'),
            ),
          ],
        ),
      ),
    );
    if (saved != true || !mounted) return;
    setState(() => _autoRenewSaving = true);
    try {
      await ref.read(membershipRepositoryProvider).setAutoRenew(
        enabled: enabled,
        planId: planId,
      );
      await _load();
      _toast(enabled ? '自动续费预约已开启' : '自动续费预约已关闭');
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _autoRenewSaving = false);
    }
  }

  Widget _autoRenewCard(BuildContext context) {
    final autoRenew = _center?.autoRenew;
    if (autoRenew == null || !autoRenew.canEdit) {
      return const SizedBox.shrink();
    }
    final subtitle = autoRenew.enabled
        ? '已开启 · ${autoRenew.planName.isEmpty ? '未选套餐' : autoRenew.planName}（到期前生成待支付订单）'
        : '开启后到期前 3 天生成待支付续费订单，不会自动扣款';
    return Card(
      child: ListTile(
        leading: const Icon(Icons.autorenew),
        title: const Text('自动续费预约'),
        subtitle: Text(subtitle),
        trailing: _autoRenewSaving
            ? const SizedBox(
                width: 18,
                height: 18,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : const Icon(Icons.chevron_right),
        onTap: _autoRenewSaving ? null : _configureAutoRenew,
      ),
    );
  }

  Widget _sharingCard(BuildContext context) {
    final sharing = _center?.sharing;
    if (sharing == null || (!sharing.canEdit && !sharing.shared)) {
      return const SizedBox.shrink();
    }
    return Card(
      child: SwitchListTile(
        secondary: const Icon(Icons.family_restroom),
        title: const Text('家庭会员共享'),
        subtitle: Text(
          sharing.canEdit
              ? '家庭成员可共享你的会员权益（付费专题等）'
              : '共享由家庭创建者控制',
        ),
        value: sharing.shared,
        onChanged: sharing.canEdit && !_sharingSaving ? _toggleSharing : null,
      ),
    );
  }

  Widget _statusCard(BuildContext context) {
    final membership = _center?.membership;
    final active = membership?.active == true;
    final expiring = active && (membership?.daysRemaining ?? 99) <= 7;
    final color = expiring
        ? const Color(0xFF9A5B00)
        : active
        ? const Color(0xFF1B8A6B)
        : const Color(0xFF8B8B8B);
    final title = expiring
        ? '会员即将到期'
        : active
        ? '会员有效'
        : membership?.status == 'expired'
        ? '会员已到期'
        : '尚未开通会员';
    final subtitle = active
        ? '${membership!.planName} · 有效期至 ${formatDate(membership.expiresAt)}'
              '（${expiring ? '仅剩' : '剩余'} ${membership.daysRemaining} 天）'
              '${membership.shared ? ' · 家庭共享' : ''}'
        : membership?.status == 'expired'
        ? '${membership!.planName} · 到期时间 ${formatDate(membership.expiresAt)}'
        : '选择下方套餐，使用模拟支付立即开通。';
    return Card(
      color: expiring
          ? const Color(0xFFFFF4E5)
          : active
          ? const Color(0xFFE7F8EF)
          : null,
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
    final invoice = _invoices
        .where((item) => item.orderId == order.id)
        .firstOrNull;
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Icon(
          paid ? Icons.check_circle_outline : Icons.receipt_long_outlined,
          color: paid ? const Color(0xFF1B8A6B) : null,
        ),
        title: Text('${order.planName} · ${formatYuan(order.amountCents)}'),
        subtitle: Text(
          '${formatDateTime(order.createdAt)} · ${order.orderNo}\n$label'
          '${invoice == null ? '' : ' · 发票${invoice.status == 'issued' ? '已开具' : invoice.status == 'rejected' ? '已驳回' : '申请中'}'}'
          '$reason',
        ),
        isThreeLine: true,
        trailing: paid
            ? invoice == null
                  ? TextButton(
                      onPressed: () => _applyInvoice(order),
                      child: const Text('申请发票'),
                    )
                  : null
            : null,
      ),
    );
  }
}
