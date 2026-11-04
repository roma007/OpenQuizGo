import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/models.dart';
import '../../core/providers.dart';
import '../../data/family_repository.dart';
import 'child_edit_page.dart';

class ChildPage extends ConsumerWidget {
  const ChildPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final children = ref.watch(childrenProvider);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  '孩子档案',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
              ),
              FilledButton.icon(
                onPressed: () => _openEditor(context, ref),
                icon: const Icon(Icons.add),
                label: const Text('新增孩子'),
              ),
            ],
          ),
        ),
        Expanded(
          child: children.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => _ErrorView(
              message: error.toString(),
              onRetry: () => ref.invalidate(childrenProvider),
            ),
            data: (items) {
              if (items.isEmpty) {
                return _EmptyView(onCreate: () => _openEditor(context, ref));
              }
              return RefreshIndicator(
                onRefresh: () async => ref.invalidate(childrenProvider),
                child: ListView.separated(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: const EdgeInsets.all(16),
                  itemCount: items.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 12),
                  itemBuilder: (context, index) {
                    final child = items[index];
                    return Card(
                      child: ListTile(
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 20,
                          vertical: 10,
                        ),
                        leading: CircleAvatar(
                          radius: 26,
                          child: Text(
                            child.nickname.isEmpty
                                ? '?'
                                : child.nickname.substring(0, 1),
                            style: const TextStyle(fontSize: 20),
                          ),
                        ),
                        title: Text(
                          child.nickname,
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        subtitle: Text(_subtitle(child)),
                        trailing: const Icon(Icons.chevron_right),
                        onTap: () => _openEditor(context, ref, child: child),
                      ),
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

  String _subtitle(Child child) {
    final birthday = (child.birthYear != null)
        ? '${child.birthYear}年${child.birthMonth ?? 1}月'
        : '生日未设置';
    return '${stageLabel(child.stage)} · $birthday';
  }

  Future<void> _openEditor(
    BuildContext context,
    WidgetRef ref, {
    Child? child,
  }) async {
    final saved = await Navigator.of(context).push<bool>(
      MaterialPageRoute(builder: (_) => ChildEditPage(child: child)),
    );
    if (saved == true) ref.invalidate(childrenProvider);
  }
}

class _EmptyView extends StatelessWidget {
  const _EmptyView({required this.onCreate});

  final VoidCallback onCreate;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.child_care,
            size: 64,
            color: Theme.of(context).colorScheme.primary,
          ),
          const SizedBox(height: 16),
          const Text('还没有孩子档案'),
          const SizedBox(height: 8),
          const Text('创建后即可绑定电视、配置规则'),
          const SizedBox(height: 20),
          FilledButton(onPressed: onCreate, child: const Text('创建第一个孩子')),
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
