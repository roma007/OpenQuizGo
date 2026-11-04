import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/api/models.dart';
import '../../core/providers.dart';
import '../../data/device_repository.dart';
import 'bind_device_page.dart';

class DevicePage extends ConsumerWidget {
  const DevicePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final devices = ref.watch(devicesProvider);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  '电视设备',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
              ),
              FilledButton.icon(
                onPressed: () => _bind(context, ref),
                icon: const Icon(Icons.add_link),
                label: const Text('绑定电视'),
              ),
            ],
          ),
        ),
        Expanded(
          child: devices.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => _ErrorView(
              message: error.toString(),
              onRetry: () => ref.invalidate(devicesProvider),
            ),
            data: (items) {
              if (items.isEmpty) {
                return _EmptyView(onBind: () => _bind(context, ref));
              }
              return RefreshIndicator(
                onRefresh: () async => ref.invalidate(devicesProvider),
                child: ListView.separated(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: const EdgeInsets.all(16),
                  itemCount: items.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 12),
                  itemBuilder: (context, index) {
                    final device = items[index];
                    return _DeviceCard(
                      device: device,
                      onUnbind: () => _confirmUnbind(context, ref, device),
                    );
                  },
                ),
              );
            },
          ),
        ),
      ],
    );
  }

  Future<void> _bind(BuildContext context, WidgetRef ref) async {
    final bound = await Navigator.of(
      context,
    ).push<bool>(MaterialPageRoute(builder: (_) => const BindDevicePage()));
    if (bound == true) {
      ref.invalidate(devicesProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('设备绑定成功')));
      }
    }
  }

  Future<void> _confirmUnbind(
    BuildContext context,
    WidgetRef ref,
    DeviceInfo device,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('解绑设备'),
        content: Text('解绑后「${device.childName}」将无法继续在该电视上使用答题解锁，确定解绑？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('解绑'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(deviceRepositoryProvider).unbind(device.id);
      ref.invalidate(devicesProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('已解绑')));
      }
    } catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('$error')));
      }
    }
  }
}

class _DeviceCard extends StatelessWidget {
  const _DeviceCard({required this.device, required this.onUnbind});

  final DeviceInfo device;
  final VoidCallback onUnbind;

  @override
  Widget build(BuildContext context) {
    final boundAt = device.boundAt;
    return Card(
      child: ListTile(
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 20,
          vertical: 10,
        ),
        leading: CircleAvatar(
          radius: 26,
          child: Icon(device.platform == 'tv' ? Icons.tv : Icons.devices_other),
        ),
        title: Text(
          device.childName,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        subtitle: Text(
          '${platformLabel(device.platform)} · '
          '${boundAt == null ? "绑定时间未知" : "绑定于 ${DateFormat("yyyy-MM-dd HH:mm").format(boundAt.toLocal())}"}',
        ),
        trailing: IconButton(
          tooltip: '解绑',
          onPressed: onUnbind,
          icon: const Icon(Icons.link_off),
        ),
      ),
    );
  }
}

class _EmptyView extends StatelessWidget {
  const _EmptyView({required this.onBind});

  final VoidCallback onBind;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.tv_off,
            size: 64,
            color: Theme.of(context).colorScheme.primary,
          ),
          const SizedBox(height: 16),
          const Text('还没有绑定电视'),
          const SizedBox(height: 8),
          const Text('在电视上打开守护端，输入显示的 6 位激活码'),
          const SizedBox(height: 20),
          FilledButton(onPressed: onBind, child: const Text('绑定电视')),
        ],
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.cloud_off, size: 56, color: Colors.orange),
          const SizedBox(height: 12),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 16),
          OutlinedButton(onPressed: onRetry, child: const Text('重试')),
        ],
      ),
    );
  }
}
