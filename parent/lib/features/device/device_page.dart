import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/api/api_client.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../core/storage/pending_unlock_store.dart';
import '../../data/tv_repository.dart';
import 'tv_settings_page.dart';

/// 081：电视登录后自动出现在这里；激活码绑定已取消。
class DevicePage extends ConsumerWidget {
  const DevicePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final tvs = ref.watch(tvsProvider);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '已登录设备',
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                    const SizedBox(height: 2),
                    Text(
                      '在电视或平板上用家长手机号登录后，会自动出现在这里',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
              IconButton(
                tooltip: '刷新',
                onPressed: () => ref.invalidate(tvsProvider),
                icon: const Icon(Icons.refresh),
              ),
            ],
          ),
        ),
        Expanded(
          child: tvs.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => _ErrorView(
              message: error.toString(),
              onRetry: () => ref.invalidate(tvsProvider),
            ),
            data: (items) {
              if (items.isEmpty) {
                return _EmptyView(onRefresh: () => ref.invalidate(tvsProvider));
              }
              return LayoutBuilder(
                builder: (context, constraints) {
                  final columns = gridColumns(constraints.maxWidth);
                  final cards = [
                    for (final tv in items) _tvCard(context, ref, tv),
                  ];
                  return RefreshIndicator(
                    onRefresh: () async => ref.invalidate(tvsProvider),
                    child: columns == 1
                        ? ListView.separated(
                            physics: const AlwaysScrollableScrollPhysics(),
                            padding: const EdgeInsets.all(16),
                            itemCount: cards.length,
                            separatorBuilder: (_, _) =>
                                const SizedBox(height: 12),
                            itemBuilder: (_, index) => cards[index],
                          )
                        : GridView.builder(
                            physics: const AlwaysScrollableScrollPhysics(),
                            padding: const EdgeInsets.all(16),
                            gridDelegate:
                                SliverGridDelegateWithFixedCrossAxisCount(
                                  crossAxisCount: columns,
                                  mainAxisSpacing: 12,
                                  crossAxisSpacing: 12,
                                  mainAxisExtent: 188,
                                ),
                            itemCount: cards.length,
                            itemBuilder: (_, index) => cards[index],
                          ),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _tvCard(BuildContext context, WidgetRef ref, TvDevice tv) {
    final pending = ref
        .watch(pendingUnlockProvider(tv.id))
        .when(
          data: (value) => value,
          loading: () => null,
          error: (_, _) => null,
        );
    return _TvCard(
      tv: tv,
      pending: pending,
      onSettings: () => _openSettings(context, ref, tv),
      onUnlock: () => _unlock(context, ref, tv),
      onSendPending: () => _sendPending(context, ref, tv),
      onUnbind: () => _confirmUnbind(context, ref, tv),
    );
  }

  Future<void> _openSettings(
    BuildContext context,
    WidgetRef ref,
    TvDevice tv,
  ) async {
    final changed = await Navigator.of(
      context,
    ).push<bool>(MaterialPageRoute(builder: (_) => TvSettingsPage(device: tv)));
    if (changed == true) {
      ref.invalidate(tvsProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('配置已保存，设备将在 60 秒内生效')));
      }
    }
  }

  Future<void> _unlock(BuildContext context, WidgetRef ref, TvDevice tv) async {
    final input = await showDialog<_UnlockInput>(
      context: context,
      builder: (_) => _UnlockDialog(tv: tv),
    );
    if (input == null) return;
    try {
      final grant = await ref
          .read(tvRepositoryProvider)
          .unlock(deviceId: tv.id, pin: input.pin, minutes: input.minutes);
      ref.invalidate(tvsProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('已放行 ${grant.minutes} 分钟，设备将在 10 秒内恢复')),
        );
      }
    } on ApiException catch (error) {
      if (error.statusCode == null) {
        await ref.read(pendingUnlockStoreProvider).save(tv.id, input.minutes);
        ref.invalidate(pendingUnlockProvider(tv.id));
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('网络不可用，已暂存放行请求，10 分钟内可在卡片补发')),
          );
        }
      } else if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
      }
    }
  }

  Future<void> _sendPending(
    BuildContext context,
    WidgetRef ref,
    TvDevice tv,
  ) async {
    final store = ref.read(pendingUnlockStoreProvider);
    final pending = await store.read(tv.id);
    if (pending == null) {
      ref.invalidate(pendingUnlockProvider(tv.id));
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('待补发放行已过期')));
      }
      return;
    }
    if (!context.mounted) return;
    final pin = await _askPin(context, title: '补发放行（${pending.minutes} 分钟）');
    if (pin == null) return;
    final latest = await store.read(tv.id);
    if (latest == null) {
      ref.invalidate(pendingUnlockProvider(tv.id));
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('待补发放行已过期')));
      }
      return;
    }
    try {
      final grant = await ref
          .read(tvRepositoryProvider)
          .unlock(deviceId: tv.id, pin: pin, minutes: latest.minutes);
      await store.clear(tv.id);
      ref.invalidate(pendingUnlockProvider(tv.id));
      ref.invalidate(tvsProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('已补发放行 ${grant.minutes} 分钟')));
      }
    } on ApiException catch (error) {
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
      }
    }
  }

  Future<String?> _askPin(BuildContext context, {required String title}) async {
    final controller = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          autofocus: true,
          obscureText: true,
          keyboardType: TextInputType.number,
          maxLength: 6,
          decoration: const InputDecoration(
            labelText: '家长 PIN',
            counterText: '',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: const Text('补发'),
          ),
        ],
      ),
    );
    controller.dispose();
    return result;
  }

  Future<void> _confirmUnbind(
    BuildContext context,
    WidgetRef ref,
    TvDevice tv,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('解绑设备'),
        content: Text('解绑后「${tv.name}」将立即停止守护，需要在设备上重新登录才能继续使用，确定解绑？'),
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
      await ref.read(tvRepositoryProvider).unbind(tv.id);
      ref.invalidate(tvsProvider);
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

class _TvCard extends StatelessWidget {
  const _TvCard({
    required this.tv,
    required this.pending,
    required this.onSettings,
    required this.onUnlock,
    required this.onSendPending,
    required this.onUnbind,
  });

  final TvDevice tv;
  final PendingUnlock? pending;
  final VoidCallback onSettings;
  final VoidCallback onUnlock;
  final VoidCallback onSendPending;
  final VoidCallback onUnbind;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final limit = tv.config.limitEnabled ? tv.config.dailyLimit : 0;
    final ratio = limit <= 0 ? 0.0 : (tv.usedMinutes / limit).clamp(0.0, 1.0);
    final lastSeen = tv.lastSeenAt;
    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                CircleAvatar(
                  radius: 22,
                  child: Icon(tv.isPad ? Icons.tablet_android : Icons.tv),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Flexible(
                            child: Text(
                              tv.name,
                              style: theme.textTheme.titleMedium,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          const SizedBox(width: 6),
                          Text(
                            tv.typeLabel,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: theme.colorScheme.primary,
                            ),
                          ),
                        ],
                      ),
                      Text(
                        tv.hasChild
                            ? '${tv.childName} · ${gradeLabel(tv.gradeTag)}'
                            : '未指定孩子 · ${scopeLabel(tv.config.quizScope)}',
                        style: theme.textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
                if (pending != null)
                  IconButton(
                    tooltip: '补发放行',
                    onPressed: onSendPending,
                    icon: const Icon(Icons.schedule_send),
                  ),
                IconButton(
                  tooltip: '设置',
                  onPressed: onSettings,
                  icon: const Icon(Icons.tune),
                ),
                IconButton(
                  tooltip: '临时放行',
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
            const SizedBox(height: 10),
            Wrap(
              spacing: 12,
              runSpacing: 4,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                _chip(
                  context,
                  tv.online ? '在线' : '离线',
                  color: tv.online ? Colors.green : Colors.grey,
                ),
                Text(
                  lastSeen == null
                      ? '尚未上报状态'
                      : '最后在线 ${DateFormat('MM-dd HH:mm').format(lastSeen.toLocal())}',
                  style: theme.textTheme.bodySmall,
                ),
                _chip(
                  context,
                  tv.config.quizEnabled
                      ? '每 ${tv.config.unlockMinutes} 分钟答题'
                      : '做题已关闭',
                  color: tv.config.quizEnabled ? Colors.blue : Colors.grey,
                ),
              ],
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                Expanded(
                  child: Text(
                    limit <= 0
                        ? '今日已看 ${tv.usedMinutes} 分钟 · 不限制时长'
                        : '今日已看 ${tv.usedMinutes} / $limit 分钟',
                    style: theme.textTheme.bodySmall,
                  ),
                ),
                if (pending != null)
                  Text(
                    '待补发放行 ${pending!.minutes} 分钟',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: Colors.orange,
                    ),
                  ),
              ],
            ),
            if (limit > 0) ...[
              const SizedBox(height: 6),
              LinearProgressIndicator(value: ratio),
            ],
          ],
        ),
      ),
    );
  }

  Widget _chip(BuildContext context, String text, {required Color color}) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Text(
        text,
        style: Theme.of(context).textTheme.bodySmall?.copyWith(color: color),
      ),
    );
  }
}

class _EmptyView extends StatelessWidget {
  const _EmptyView({required this.onRefresh});

  final VoidCallback onRefresh;

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
          const Text('还没有设备登录'),
          const SizedBox(height: 8),
          const Text('在电视或平板上打开守护端，用家长手机号收验证码登录后会自动出现'),
          const SizedBox(height: 20),
          OutlinedButton(onPressed: onRefresh, child: const Text('刷新')),
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
  const _UnlockDialog({required this.tv});

  final TvDevice tv;

  @override
  State<_UnlockDialog> createState() => _UnlockDialogState();
}

class _UnlockDialogState extends State<_UnlockDialog> {
  final _pin = TextEditingController();
  late int _minutes = widget.tv.config.hardStopGrace;

  @override
  void dispose() {
    _pin.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final options = {5, 10, 15, 20, 30, 60, _minutes}.toList()..sort();
    return AlertDialog(
      title: const Text('临时放行'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            '为「${widget.tv.name}」放行一段观看时间；'
            '放行期间即使今日额度已用完也不会硬停。',
          ),
          const SizedBox(height: 16),
          DropdownButtonFormField<int>(
            initialValue: _minutes,
            decoration: const InputDecoration(labelText: '放行时长'),
            items: [
              for (final value in options)
                DropdownMenuItem(value: value, child: Text('$value 分钟')),
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
          child: const Text('放行'),
        ),
      ],
    );
  }
}
