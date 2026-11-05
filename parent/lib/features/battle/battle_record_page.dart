import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/api/models.dart';
import '../../core/layout/adaptive.dart';
import '../../core/providers.dart';
import '../../data/battle_repository.dart';

class BattleRecordPage extends ConsumerStatefulWidget {
  const BattleRecordPage({
    super.key,
    required this.child,
    required this.battleId,
  });

  final Child child;
  final String battleId;

  @override
  ConsumerState<BattleRecordPage> createState() => _BattleRecordPageState();
}

class _BattleRecordPageState extends ConsumerState<BattleRecordPage> {
  late Future<BattleState> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<BattleState> _load() {
    return ref
        .read(battleRepositoryProvider)
        .get(widget.child.id, widget.battleId);
  }

  void _retry() {
    setState(() => _future = _load());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('${widget.child.nickname} · 对战详情')),
      body: ContentFrame(
        maxWidth: 900,
        child: FutureBuilder<BattleState>(
          future: _future,
          builder: (context, snapshot) {
            if (snapshot.connectionState != ConnectionState.done) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              final error = snapshot.error;
              return Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      '加载对战详情失败：'
                      '${error is ApiException ? error.message : error}',
                    ),
                    const SizedBox(height: 12),
                    OutlinedButton(onPressed: _retry, child: const Text('重试')),
                  ],
                ),
              );
            }
            return _detail(context, snapshot.data!);
          },
        ),
      ),
    );
  }

  Widget _detail(BuildContext context, BattleState state) {
    final result = state.result;
    final childScore = result?.childScore ?? state.childScore;
    final parentScore = result?.parentScore ?? state.parentScore;
    final winner =
        result?.winner ??
        (childScore > parentScore
            ? 'child'
            : parentScore > childScore
            ? 'parent'
            : 'draw');
    final color = winner == 'child'
        ? const Color(0xFF1B8A6B)
        : winner == 'parent'
        ? const Color(0xFF2F6FDE)
        : const Color(0xFF8A6D1B);
    final mode = state.mode == 'room' ? '双设备' : '同设备';
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Card(
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Column(
              children: [
                Icon(
                  winner == 'draw'
                      ? Icons.handshake_outlined
                      : Icons.emoji_events_outlined,
                  size: 56,
                  color: color,
                ),
                const SizedBox(height: 8),
                Text(
                  state.finished
                      ? describeWinner(
                          BattleResult(
                            winner: winner,
                            childScore: childScore,
                            parentScore: parentScore,
                          ),
                        )
                      : '对局尚未结束',
                  style: TextStyle(
                    fontSize: 28,
                    fontWeight: FontWeight.bold,
                    color: color,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  '孩子 $childScore : $parentScore 家长',
                  style: const TextStyle(fontSize: 20),
                ),
                const SizedBox(height: 4),
                Text(
                  '$mode · ${_statusLabel(state.status)}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        if (!state.finished)
          const Card(
            child: Padding(
              padding: EdgeInsets.all(20),
              child: Text('对局尚未结束，暂无逐题回顾。'),
            ),
          )
        else if (state.answers.isEmpty)
          const Card(
            child: Padding(padding: EdgeInsets.all(20), child: Text('暂无逐题数据。')),
          )
        else
          for (var i = 0; i < state.questions.length; i++)
            _reviewCard(context, state, state.questions[i], i),
      ],
    );
  }

  String _statusLabel(String status) {
    return switch (status) {
      'finished' => '已完成',
      'waiting' => '等待加入',
      'child_turn' => '孩子答题中',
      'parent_turn' => '家长答题中',
      _ => status,
    };
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
