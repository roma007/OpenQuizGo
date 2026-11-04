import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/models.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../data/daily_repository.dart';

class DailyQuizPage extends ConsumerStatefulWidget {
  const DailyQuizPage({super.key, required this.child});

  final Child child;

  @override
  ConsumerState<DailyQuizPage> createState() => _DailyQuizPageState();
}

class _DailyQuizPageState extends ConsumerState<DailyQuizPage> {
  String? _selected;
  bool _submitting = false;

  Future<void> _submit() async {
    final option = _selected;
    if (option == null || _submitting) return;
    setState(() => _submitting = true);
    try {
      await ref.read(dailyRepositoryProvider).answer(widget.child.id, option);
      ref.invalidate(dailyProvider(widget.child.id));
      setState(() => _selected = null);
    } on ApiException catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.message)));
      }
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final daily = ref.watch(dailyProvider(widget.child.id));
    return Scaffold(
      appBar: AppBar(title: Text('${widget.child.nickname} · 每日一题')),
      body: ContentFrame(
        maxWidth: 800,
        child: daily.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('加载每日一题失败：$error'),
                const SizedBox(height: 12),
                OutlinedButton(
                  onPressed: () =>
                      ref.invalidate(dailyProvider(widget.child.id)),
                  child: const Text('重试'),
                ),
              ],
            ),
          ),
          data: (state) => _DailyBody(
            state: state,
            selected: _selected,
            submitting: _submitting,
            onSelect: (id) => setState(() => _selected = id),
            onSubmit: _submit,
          ),
        ),
      ),
    );
  }
}

class _DailyBody extends StatelessWidget {
  const _DailyBody({
    required this.state,
    required this.selected,
    required this.submitting,
    required this.onSelect,
    required this.onSubmit,
  });

  final DailyState state;
  final String? selected;
  final bool submitting;
  final ValueChanged<String> onSelect;
  final VoidCallback onSubmit;

  @override
  Widget build(BuildContext context) {
    final question = state.question;
    if (question == null) {
      return const Center(child: Text('题库暂无可用题目，请稍后再试'));
    }
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                state.date,
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
            Text(
              '连续 ${state.streak} 天 · 总积分 ${state.totalPoints}',
              style: const TextStyle(color: Color(0xFF1B8A6B)),
            ),
          ],
        ),
        const SizedBox(height: 24),
        Text(
          question.stem,
          style: Theme.of(
            context,
          ).textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 24),
        if (!state.answered)
          for (final option in question.options)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: OutlinedButton(
                onPressed: submitting ? null : () => onSelect(option.id),
                style: OutlinedButton.styleFrom(
                  minimumSize: const Size.fromHeight(64),
                  backgroundColor: selected == option.id
                      ? Theme.of(context).colorScheme.primaryContainer
                      : null,
                  alignment: Alignment.centerLeft,
                  padding: const EdgeInsets.symmetric(horizontal: 24),
                ),
                child: Text(
                  '${option.id}. ${option.text}',
                  style: const TextStyle(fontSize: 22),
                ),
              ),
            ),
        if (!state.answered) ...[
          const SizedBox(height: 12),
          FilledButton(
            onPressed: selected == null || submitting ? null : onSubmit,
            child: submitting
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('提交答案'),
          ),
        ] else
          _ResultCard(state: state),
      ],
    );
  }
}

class _ResultCard extends StatelessWidget {
  const _ResultCard({required this.state});

  final DailyState state;

  @override
  Widget build(BuildContext context) {
    final correct = state.correct == true;
    return Card(
      color: correct ? const Color(0xFFE7F8EF) : const Color(0xFFFFF2F2),
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              correct ? '回答正确 +${state.points} 分' : '回答错误',
              style: TextStyle(
                fontSize: 28,
                fontWeight: FontWeight.bold,
                color: correct
                    ? const Color(0xFF1B8A6B)
                    : const Color(0xFFD84343),
              ),
            ),
            if (state.correctAnswer.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text('正确答案：${state.correctAnswer.join('、')}'),
            ],
            if (state.explain.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text('解析：${state.explain}'),
            ],
            const SizedBox(height: 16),
            const Text('明天再来挑战吧！'),
          ],
        ),
      ),
    );
  }
}
