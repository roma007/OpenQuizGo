import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/models.dart';
import '../../core/providers.dart';
import '../../data/family_repository.dart';
import '../battle/battle_page.dart';
import '../daily/daily_quiz_page.dart';
import '../recommendation/recommendations_page.dart';
import '../rule/rule_page.dart';
import 'child_edit_page.dart';

class ChildPage extends ConsumerWidget {
  const ChildPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final children = ref.watch(childrenProvider);
    return children.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => _ErrorView(
        message: error.toString(),
        onRetry: () => ref.invalidate(childrenProvider),
      ),
      data: (cached) {
        final items = cached.data;
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
            if (cached.stale) _OfflineBanner(cachedAt: cached.cachedAt),
            Expanded(
              child: items.isEmpty
                  ? _EmptyView(onCreate: () => _openEditor(context, ref))
                  : LayoutBuilder(
                      builder: (context, constraints) {
                        final columns = constraints.maxWidth >= 1400
                            ? 3
                            : constraints.maxWidth >= 900
                            ? 2
                            : 1;
                        final cards = [
                          for (final child in items)
                            _ChildCard(
                              child: child,
                              subtitle: _subtitle(child),
                              onEdit: () =>
                                  _openEditor(context, ref, child: child),
                              onRule: () => _openRule(context, child),
                              onDaily: () => _openDaily(context, child),
                              onBattle: () => _openBattle(context, child),
                              onRecommendations: () =>
                                  _openRecommendations(context, child),
                            ),
                        ];
                        return RefreshIndicator(
                          onRefresh: () async =>
                              ref.invalidate(childrenProvider),
                          child: columns == 1
                              ? ListView.separated(
                                  physics:
                                      const AlwaysScrollableScrollPhysics(),
                                  padding: const EdgeInsets.all(16),
                                  itemCount: cards.length,
                                  separatorBuilder: (_, _) =>
                                      const SizedBox(height: 12),
                                  itemBuilder: (_, index) => cards[index],
                                )
                              : GridView.builder(
                                  physics:
                                      const AlwaysScrollableScrollPhysics(),
                                  padding: const EdgeInsets.all(16),
                                  gridDelegate:
                                      SliverGridDelegateWithFixedCrossAxisCount(
                                        crossAxisCount: columns,
                                        mainAxisSpacing: 12,
                                        crossAxisSpacing: 12,
                                        mainAxisExtent: 168,
                                      ),
                                  itemCount: cards.length,
                                  itemBuilder: (_, index) => cards[index],
                                ),
                        );
                      },
                    ),
            ),
          ],
        );
      },
    );
  }

  String _subtitle(Child child) {
    final birthday = (child.birthYear != null)
        ? '${child.birthYear}年${child.birthMonth ?? 1}月'
        : '生日未设置';
    return '${stageLabel(child.stage)} · $birthday';
  }

  void _openRecommendations(BuildContext context, Child child) {
    Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => RecommendationsPage(child: child)),
    );
  }

  void _openBattle(BuildContext context, Child child) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (_) => BattlePage(child: child)));
  }

  void _openDaily(BuildContext context, Child child) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (_) => DailyQuizPage(child: child)));
  }

  void _openRule(BuildContext context, Child child) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (_) => RulePage(child: child)));
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

class _ChildCard extends StatelessWidget {
  const _ChildCard({
    required this.child,
    required this.subtitle,
    required this.onEdit,
    required this.onRule,
    required this.onDaily,
    required this.onBattle,
    required this.onRecommendations,
  });

  final Child child;
  final String subtitle;
  final VoidCallback onEdit;
  final VoidCallback onRule;
  final VoidCallback onDaily;
  final VoidCallback onBattle;
  final VoidCallback onRecommendations;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    return Card(
      margin: EdgeInsets.zero,
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onEdit,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 12, 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  CircleAvatar(
                    radius: 24,
                    child: Text(
                      child.nickname.isEmpty
                          ? '?'
                          : child.nickname.substring(0, 1),
                      style: const TextStyle(fontSize: 19),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          child.nickname,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: theme.textTheme.titleMedium,
                        ),
                        const SizedBox(height: 4),
                        Text(
                          subtitle,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: colors.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 4),
                  Icon(Icons.chevron_right, color: colors.onSurfaceVariant),
                ],
              ),
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: _ChildAction(
                      icon: Icons.tips_and_updates_outlined,
                      label: '推荐',
                      tooltip: '练习推荐',
                      onTap: onRecommendations,
                    ),
                  ),
                  Expanded(
                    child: _ChildAction(
                      icon: Icons.sports_esports_outlined,
                      label: '对战',
                      tooltip: '亲子对战',
                      onTap: onBattle,
                    ),
                  ),
                  Expanded(
                    child: _ChildAction(
                      icon: Icons.quiz_outlined,
                      label: '每日',
                      tooltip: '每日一题',
                      onTap: onDaily,
                    ),
                  ),
                  Expanded(
                    child: _ChildAction(
                      icon: Icons.tune,
                      label: '规则',
                      tooltip: '观看规则',
                      onTap: onRule,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ChildAction extends StatelessWidget {
  const _ChildAction({
    required this.icon,
    required this.label,
    required this.tooltip,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final String tooltip;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Tooltip(
      message: tooltip,
      child: InkWell(
        borderRadius: BorderRadius.circular(10),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 6),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 20, color: theme.colorScheme.primary),
              const SizedBox(height: 2),
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.labelSmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ),
    );
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

class _OfflineBanner extends StatelessWidget {
  const _OfflineBanner({this.cachedAt});

  final DateTime? cachedAt;

  @override
  Widget build(BuildContext context) {
    final time = cachedAt == null
        ? ''
        : ' · 最后同步 ${cachedAt!.month}-${cachedAt!.day} '
              '${cachedAt!.hour.toString().padLeft(2, '0')}:'
              '${cachedAt!.minute.toString().padLeft(2, '0')}';
    return Container(
      width: double.infinity,
      color: Colors.orange.withValues(alpha: 0.12),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Text(
        '离线数据$time',
        style: const TextStyle(color: Colors.orange, fontSize: 13),
      ),
    );
  }
}
