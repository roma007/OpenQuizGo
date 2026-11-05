import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/battle_repository.dart';
import 'package:openquizgo_parent/data/daily_repository.dart';
import 'package:openquizgo_parent/features/battle/battle_record_page.dart';

BattleState _finishedState() => BattleState(
  id: 'b1',
  status: 'finished',
  questionCount: 1,
  questions: [
    BattleQuestion(
      id: 'q1',
      stem: '2+3=?',
      type: 'choice',
      options: [
        DailyOption(id: 'a', text: '5'),
        DailyOption(id: 'b', text: '6'),
      ],
    ),
  ],
  childScore: 1,
  parentScore: 0,
  mode: 'room',
  joinCode: '',
  result: BattleResult(winner: 'child', childScore: 1, parentScore: 0),
  answers: [
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
  ],
);

class _FakeBattleRepository implements BattleRepository {
  @override
  Stream<void> events(String childId, String battleId) =>
      const Stream<void>.empty();

  @override
  Future<BattleState> get(String childId, String battleId) async =>
      _finishedState();

  @override
  Future<BattleState> create(
    String childId, {
    int questionCount = 5,
    String mode = 'same_device',
  }) async => throw UnimplementedError();

  @override
  Future<BattleState> join(String childId, String code) async =>
      throw UnimplementedError();

  @override
  Future<List<BattleSummary>> history(String childId, {int limit = 20}) async =>
      const [];

  @override
  Future<List<BattleRankingEntry>> ranking({String period = 'all'}) async =>
      const [];

  @override
  Future<BattleState> submit(
    String childId,
    String battleId,
    String side,
    Map<String, String> answers,
  ) async => throw UnimplementedError();
}

void main() {
  testWidgets('对战详情展示比分与逐题回顾', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          battleRepositoryProvider.overrideWithValue(_FakeBattleRepository()),
        ],
        child: MaterialApp(
          home: BattleRecordPage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
            battleId: 'b1',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('孩子获胜'), findsOneWidget);
    expect(find.text('孩子 1 : 0 家长'), findsOneWidget);
    expect(find.text('2+3=?'), findsOneWidget);
    expect(find.text('孩子 答对'), findsOneWidget);
    expect(find.text('家长 答错'), findsOneWidget);
    expect(find.text('正确答案：a'), findsOneWidget);
    expect(find.text('解析：三加二等于五'), findsOneWidget);
  });
}
