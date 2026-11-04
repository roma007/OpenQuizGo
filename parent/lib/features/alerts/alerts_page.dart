import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../data/alert_repository.dart';

class AlertsPage extends ConsumerStatefulWidget {
  const AlertsPage({super.key});

  @override
  ConsumerState<AlertsPage> createState() => _AlertsPageState();
}

class _AlertsPageState extends ConsumerState<AlertsPage> {
  bool _marking = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _markRead());
  }

  Future<void> _markRead() async {
    if (_marking) return;
    _marking = true;
    try {
      await ref.read(alertRepositoryProvider).markAllRead();
      ref.invalidate(alertsProvider);
    } catch (_) {
      // 已读失败不影响列表展示。
    } finally {
      _marking = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final alerts = ref.watch(alertsProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('消息中心')),
      body: ContentFrame(
        maxWidth: 1000,
        child: alerts.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('加载告警失败：$error'),
                const SizedBox(height: 12),
                OutlinedButton(
                  onPressed: () => ref.invalidate(alertsProvider),
                  child: const Text('重试'),
                ),
              ],
            ),
          ),
          data: (data) {
            if (data.items.isEmpty) {
              return Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      Icons.notifications_none,
                      size: 64,
                      color: Theme.of(context).colorScheme.primary,
                    ),
                    const SizedBox(height: 16),
                    const Text('暂无告警'),
                    const SizedBox(height: 8),
                    const Text('权限异常、卸载尝试等事件会显示在这里'),
                  ],
                ),
              );
            }
            return RefreshIndicator(
              onRefresh: () async => ref.invalidate(alertsProvider),
              child: ListView.separated(
                physics: const AlwaysScrollableScrollPhysics(),
                padding: const EdgeInsets.all(16),
                itemCount: data.items.length,
                separatorBuilder: (_, _) => const SizedBox(height: 12),
                itemBuilder: (context, index) =>
                    _AlertCard(alert: data.items[index]),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _AlertCard extends StatelessWidget {
  const _AlertCard({required this.alert});

  final AlertItem alert;

  @override
  Widget build(BuildContext context) {
    final time = alert.reportedAt == null
        ? ''
        : DateFormat('yyyy-MM-dd HH:mm').format(alert.reportedAt!.toLocal());
    return Card(
      child: ListTile(
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 20,
          vertical: 10,
        ),
        leading: CircleAvatar(
          backgroundColor: alert.read
              ? Theme.of(context).colorScheme.surfaceContainerHighest
              : Colors.orange.withValues(alpha: 0.18),
          child: Icon(
            Icons.warning_amber_rounded,
            color: alert.read ? Colors.grey : Colors.orange,
          ),
        ),
        title: Row(
          children: [
            Expanded(
              child: Text(
                alertTitle(alert.type, alert.detail),
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: alert.read ? FontWeight.normal : FontWeight.bold,
                ),
              ),
            ),
            if (!alert.read)
              Container(
                width: 8,
                height: 8,
                decoration: const BoxDecoration(
                  color: Colors.orange,
                  shape: BoxShape.circle,
                ),
              ),
          ],
        ),
        subtitle: Text(
          '${alert.childName.isEmpty ? '设备' : alert.childName}'
          '${alert.deviceId.isEmpty ? '' : ' · ${alert.deviceId}'}'
          '${time.isEmpty ? '' : ' · $time'}',
        ),
      ),
    );
  }
}
