import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/models.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';

const _subjectLabels = <String, String>{
  'math': '数学',
  'chinese': '语文',
  'english': '英语',
  'science': '科学',
  'logic': '逻辑',
  'safety': '安全',
  'fun': '趣味',
};

class RecommendationsPage extends ConsumerWidget {
  const RecommendationsPage({super.key, required this.child});

  final Child child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final data = ref.watch(recommendationsProvider(child.id));
    return Scaffold(
      appBar: AppBar(title: Text('${child.nickname} 的练习推荐')),
      body: ContentFrame(
        maxWidth: 760,
        child: data.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('加载推荐失败：$error'),
                const SizedBox(height: 12),
                OutlinedButton(
                  onPressed: () =>
                      ref.invalidate(recommendationsProvider(child.id)),
                  child: const Text('重试'),
                ),
              ],
            ),
          ),
          data: (result) => ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Card(
                color: Theme.of(context).colorScheme.primaryContainer,
                child: ListTile(
                  leading: const Icon(Icons.tips_and_updates_outlined),
                  title: const Text('建议练习难度'),
                  subtitle: Text('${result.suggestedDifficulty} 级（1 最易，5 最难）'),
                ),
              ),
              const SizedBox(height: 16),
              Text('薄弱学科', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 8),
              if (result.weakSubjects.isEmpty)
                const Card(
                  child: ListTile(
                    title: Text('答题样本还不够'),
                    subtitle: Text('完成更多题目后，会按学科正确率推荐练习方向。'),
                  ),
                )
              else
                for (final subject in result.weakSubjects)
                  Card(
                    child: ListTile(
                      title: Text(
                        _subjectLabels[subject.subject] ?? subject.subject,
                      ),
                      subtitle: Text(
                        '作答 ${subject.answered} 题 · 正确 ${subject.correct} 题 · '
                        '正确率 ${(subject.accuracy * 100).toStringAsFixed(0)}%',
                      ),
                      trailing: SizedBox(
                        width: 120,
                        child: LinearProgressIndicator(
                          value: subject.accuracy,
                          minHeight: 8,
                        ),
                      ),
                    ),
                  ),
              const SizedBox(height: 16),
              Text('推荐专题', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 8),
              if (result.topics.isEmpty)
                const Card(
                  child: ListTile(
                    title: Text('暂无匹配专题'),
                    subtitle: Text('题库专题上线后会在这里推荐。'),
                  ),
                )
              else
                for (final topic in result.topics)
                  Card(
                    child: ListTile(
                      leading: const Icon(Icons.menu_book_outlined),
                      title: Text(topic.name),
                      subtitle: Text(topic.reason),
                    ),
                  ),
            ],
          ),
        ),
      ),
    );
  }
}
