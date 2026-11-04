import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/battle_repository.dart';
import 'package:openquizgo_parent/data/daily_repository.dart';
import 'package:openquizgo_parent/features/battle/battle_page.dart';

BattleQuestion _question() => BattleQuestion(
  id: 'q1',
  stem: '2+3=?',
  type: 'choice',
  options: [
    DailyOption(id: 'a', text: '5'),
    DailyOption(id: 'b', text: '6'),
  ],
);

BattleState _state({
  String status = 'child_turn',
  int childScore = 0,
  int parentScore = 0,
  bool finished = false,
}) => BattleState(
  id: 'b1',
  status: status,
  questionCount: 1,
  questions: [_question()],
  childScore: childScore,
  parentScore: parentScore,
  result: finished
      ? BattleResult(winner: 'child', childScore: 1, parentScore: 0)
      : null,
  answers: finished
      ? [
          BattleAnswer(
            side: 'child',
            questionId: 'q1',
            correct: true,
            correctAnswer: const ['a'],
            explain: '三加二等于五',
          ),
          BattleAnswer(
            side: 'parent',
            questionId: 'q1',
            correct: false,
            correctAnswer: const ['a'],
            explain: '三加二等于五',
          ),
        ]
      : const [],
);

class _FakeBattleRepository implements BattleRepository {
  int childSubmits = 0;
  int parentSubmits = 0;

  @override
  Future<BattleState> create(String childId, {int questionCount = 5}) async =>
      _state();

  @override
  Future<BattleState> get(String childId, String battleId) async => _state();

  @override
  Future<BattleState> submit(
    String childId,
    String battleId,
    String side,
    Map<String, String> answers,
  ) async {
    if (side == 'child') {
      childSubmits++;
      return _state(status: 'parent_turn', childScore: 1);
    }
    parentSubmits++;
    return _state(
      status: 'finished',
      childScore: 1,
      parentScore: 0,
      finished: true,
    );
  }
}

void main() {
  testWidgets('亲子对战轮流答题流程', (tester) async {
    final repository = _FakeBattleRepository();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [battleRepositoryProvider.overrideWithValue(repository)],
        child: MaterialApp(
          home: BattlePage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
          ),
        ),
      ),
    );

    expect(find.text('亲子对战'), findsOneWidget);
    expect(find.text('开始对战'), findsOneWidget);

    await tester.tap(find.text('开始对战'));
    await tester.pumpAndSettle();
    expect(find.text('2+3=?'), findsOneWidget);
    expect(find.text('孩子答题'), findsOneWidget);

    await tester.tap(find.text('a. 5'));
    await tester.pump();
    await tester.tap(find.text('提交答案'));
    await tester.pumpAndSettle();
    expect(repository.childSubmits, 1);
    expect(find.textContaining('把手机交给家长'), findsOneWidget);

    await tester.tap(find.text('我是家长，开始答题'));
    await tester.pumpAndSettle();
    expect(find.text('家长答题'), findsOneWidget);

    await tester.tap(find.text('b. 6'));
    await tester.pump();
    await tester.tap(find.text('提交答案'));
    await tester.pumpAndSettle();
    expect(repository.parentSubmits, 1);
    expect(find.text('孩子获胜'), findsOneWidget);
    expect(find.text('孩子 1 : 0 家长'), findsOneWidget);
    expect(find.textContaining('三加二等于五'), findsOneWidget);
  });
}
