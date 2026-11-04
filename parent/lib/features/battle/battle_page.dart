import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/models.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../data/battle_repository.dart';

class BattlePage extends ConsumerStatefulWidget {
  const BattlePage({super.key, required this.child});

  final Child child;

  @override
  ConsumerState<BattlePage> createState() => _BattlePageState();
}

class _BattlePageState extends ConsumerState<BattlePage> {
  BattleState? _state;
  bool _starting = false;
  bool _submitting = false;
  bool _handover = false;
  int _index = 0;
  final Map<String, String> _selections = {};

  Future<void> _start() async {
    setState(() => _starting = true);
    try {
      final state = await ref
          .read(battleRepositoryProvider)
          .create(widget.child.id);
      setState(() {
        _state = state;
        _index = 0;
        _selections.clear();
        _handover = false;
      });
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _starting = false);
    }
  }

  Future<void> _submit() async {
    final state = _state;
    if (state == null || _submitting) return;
    final missing = state.questions
        .where((question) => !_selections.containsKey(question.id))
        .length;
    if (missing > 0) {
      _toast('还有 $missing 道题未作答');
      return;
    }
    setState(() => _submitting = true);
    try {
      final next = await ref
          .read(battleRepositoryProvider)
          .submit(
            widget.child.id,
            state.id,
            state.childTurn ? 'child' : 'parent',
            _selections,
          );
      setState(() {
        _state = next;
        _index = 0;
        _selections.clear();
        _handover = false;
      });
    } on ApiException catch (error) {
      _toast(error.message);
    } finally {
      if (mounted) setState(() => _submitting = false);
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
      appBar: AppBar(title: Text('${widget.child.nickname} · 亲子对战')),
      body: ContentFrame(
        maxWidth: 900,
        child: AnimatedSwitcher(
          duration: const Duration(milliseconds: 200),
          child: _body(context),
        ),
      ),
    );
  }

  Widget _body(BuildContext context) {
    final state = _state;
    if (state == null) return _intro(context);
    if (state.finished) return _result(context, state);
    if (!state.childTurn && !_handover) {
      return _handoverView(context, state);
    }
    return _questionView(context, state);
  }

  Widget _intro(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.sports_esports_outlined, size: 72),
            const SizedBox(height: 16),
            Text(
              '亲子对战',
              style: Theme.of(
                context,
              ).textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 12),
            const Text(
              '同一部手机轮流答题：孩子先答，答完交给家长，最后看谁得分高。',
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            FilledButton.icon(
              onPressed: _starting ? null : _start,
              icon: _starting
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.play_arrow),
              label: const Text('开始对战'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _handoverView(BuildContext context, BattleState state) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.swap_horiz, size: 72),
            const SizedBox(height: 16),
            Text(
              '孩子答完啦！',
              style: Theme.of(
                context,
              ).textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 12),
            Text(
              '孩子得分 ${state.childScore} / ${state.questionCount}\n请把手机交给家长。',
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 18),
            ),
            const SizedBox(height: 24),
            FilledButton(
              onPressed: () => setState(() => _handover = true),
              child: const Text('我是家长，开始答题'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _questionView(BuildContext context, BattleState state) {
    final question = state.questions[_index];
    final selected = _selections[question.id];
    final isLast = _index == state.questions.length - 1;
    final sideLabel = state.childTurn ? '孩子答题' : '家长答题';
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Row(
          children: [
            Chip(label: Text(sideLabel)),
            const Spacer(),
            Text('第 ${_index + 1} / ${state.questions.length} 题'),
          ],
        ),
        const SizedBox(height: 8),
        LinearProgressIndicator(value: (_index + 1) / state.questions.length),
        const SizedBox(height: 24),
        Text(
          question.stem,
          style: Theme.of(
            context,
          ).textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 24),
        for (final option in question.options)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: OutlinedButton(
              onPressed: () => setState(() {
                _selections[question.id] = option.id;
              }),
              style: OutlinedButton.styleFrom(
                minimumSize: const Size(0, 64),
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
        const SizedBox(height: 16),
        Row(
          children: [
            if (_index > 0)
              OutlinedButton(
                onPressed: () => setState(() => _index--),
                child: const Text('上一题'),
              ),
            const Spacer(),
            if (!isLast)
              FilledButton(
                onPressed: selected == null
                    ? null
                    : () => setState(() => _index++),
                child: const Text('下一题'),
              )
            else
              FilledButton(
                onPressed: _submitting ? null : _submit,
                child: _submitting
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('提交答案'),
              ),
          ],
        ),
      ],
    );
  }

  Widget _result(BuildContext context, BattleState state) {
    final result = state.result!;
    final color = result.winner == 'child'
        ? const Color(0xFF1B8A6B)
        : result.winner == 'parent'
        ? const Color(0xFF2F6FDE)
        : const Color(0xFF8A6D1B);
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Center(
          child: Column(
            children: [
              Icon(
                result.winner == 'draw'
                    ? Icons.handshake_outlined
                    : Icons.emoji_events_outlined,
                size: 72,
                color: color,
              ),
              const SizedBox(height: 8),
              Text(
                describeWinner(result),
                style: TextStyle(
                  fontSize: 32,
                  fontWeight: FontWeight.bold,
                  color: color,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                '孩子 ${result.childScore} : ${result.parentScore} 家长',
                style: const TextStyle(fontSize: 22),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        for (var i = 0; i < state.questions.length; i++)
          _reviewCard(context, state, state.questions[i], i),
        const SizedBox(height: 8),
        FilledButton.icon(
          onPressed: _starting ? null : _start,
          icon: const Icon(Icons.replay),
          label: const Text('再来一局'),
        ),
      ],
    );
  }

  Widget _reviewCard(
    BuildContext context,
    BattleState state,
    BattleQuestion question,
    int index,
  ) {
    final child = state.answerFor('child', question.id);
    final parent = state.answerFor('parent', question.id);
    final correct = child?.correctAnswer ?? parent?.correctAnswer ?? const [];
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '第 ${index + 1} 题',
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 4),
            Text(
              question.stem,
              style: const TextStyle(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              children: [
                _answerChip('孩子', child?.correct),
                _answerChip('家长', parent?.correct),
              ],
            ),
            if (correct.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text('正确答案：${correct.join('、')}'),
            ],
            if (parent?.explain.isNotEmpty == true) ...[
              const SizedBox(height: 4),
              Text('解析：${parent!.explain}'),
            ],
          ],
        ),
      ),
    );
  }

  Widget _answerChip(String label, bool? correct) {
    if (correct == null) {
      return Chip(label: Text('$label 未作答'));
    }
    return Chip(
      avatar: Icon(
        correct ? Icons.check_circle : Icons.cancel,
        size: 18,
        color: correct ? const Color(0xFF1B8A6B) : const Color(0xFFD84343),
      ),
      label: Text('$label ${correct ? '答对' : '答错'}'),
    );
  }
}
