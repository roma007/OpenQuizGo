import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/api/api_client.dart';
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
                      onUnlock: () => _unlock(context, ref, device),
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

  Future<void> _unlock(
    BuildContext context,
    WidgetRef ref,
    DeviceInfo device,
  ) async {
    final input = await showDialog<_UnlockInput>(
      context: context,
      builder: (_) => _UnlockDialog(childName: device.childName),
    );
    if (input == null) return;
    try {
      final grant = await ref
          .read(deviceRepositoryProvider)
          .unlock(bindId: device.id, pin: input.pin, minutes: input.minutes);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('已解锁 ${grant.minutes} 分钟，电视将在 10 秒内恢复')),
        );
      }
    } on ApiException catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
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
  const _DeviceCard({
    required this.device,
    required this.onUnlock,
    required this.onUnbind,
  });

  final DeviceInfo device;
  final VoidCallback onUnlock;
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
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            IconButton(
              tooltip: '远程解锁',
              onPressed: onUnlock,
              icon: const Icon(Icons.lock_open),
            ),
            IconButton(
              tooltip: '解绑',
              onPressed: onUnbind,
              icon: const Icon(Icons.link_off),
            ),
          ],
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

class _UnlockInput {
  _UnlockInput({required this.pin, required this.minutes});

  final String pin;
  final int minutes;
}

class _UnlockDialog extends StatefulWidget {
  const _UnlockDialog({required this.childName});

  final String childName;

  @override
  State<_UnlockDialog> createState() => _UnlockDialogState();
}

class _UnlockDialogState extends State<_UnlockDialog> {
  final _pin = TextEditingController();
  int _minutes = 10;

  @override
  void dispose() {
    _pin.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('远程解锁'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text('为「${widget.childName}」签发一段观看时间，电视阻挡页会自动放行。'),
          const SizedBox(height: 16),
          DropdownButtonFormField<int>(
            initialValue: _minutes,
            decoration: const InputDecoration(labelText: '解锁时长'),
            items: const [
              DropdownMenuItem(value: 5, child: Text('5 分钟')),
              DropdownMenuItem(value: 10, child: Text('10 分钟')),
              DropdownMenuItem(value: 20, child: Text('20 分钟')),
              DropdownMenuItem(value: 30, child: Text('30 分钟')),
              DropdownMenuItem(value: 60, child: Text('60 分钟')),
            ],
            onChanged: (value) => setState(() => _minutes = value ?? 10),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _pin,
            autofocus: true,
            obscureText: true,
            keyboardType: TextInputType.number,
            maxLength: 6,
            decoration: const InputDecoration(
              labelText: '家长 PIN',
              counterText: '',
            ),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed: () {
            final pin = _pin.text.trim();
            if (pin.isEmpty) return;
            Navigator.pop(context, _UnlockInput(pin: pin, minutes: _minutes));
          },
          child: const Text('解锁'),
        ),
      ],
    );
  }
}
