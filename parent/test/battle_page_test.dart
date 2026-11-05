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
  String mode = 'same_device',
  String joinCode = '',
  DateTime? joinCodeExpiresAt,
}) => BattleState(
  id: 'b1',
  status: status,
  questionCount: 1,
  questions: [_question()],
  childScore: childScore,
  parentScore: parentScore,
  mode: mode,
  joinCode: joinCode,
  joinCodeExpiresAt: joinCodeExpiresAt,
  result: finished
      ? BattleResult(
          winner: childScore > parentScore
              ? 'child'
              : parentScore > childScore
              ? 'parent'
              : 'draw',
          childScore: childScore,
          parentScore: parentScore,
        )
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
  _FakeBattleRepository({this.roomExpiresAt});

  final DateTime? roomExpiresAt;
  int childSubmits = 0;
  int parentSubmits = 0;
  int getCalls = 0;
  BattleState current = _state();

  @override
  Future<BattleState> create(
    String childId, {
    int questionCount = 5,
    String mode = 'same_device',
  }) async {
    current = mode == 'room'
        ? _state(
            status: 'waiting',
            mode: 'room',
            joinCode: 'ABC123',
            joinCodeExpiresAt: roomExpiresAt,
          )
        : _state();
    return current;
  }

  @override
  Future<BattleState> join(String childId, String code) async {
    current = _state(mode: 'room', joinCode: code);
    return current;
  }

  @override
  Future<BattleState> get(String childId, String battleId) async {
    getCalls++;
    if (battleId == 'b0') {
      return _state(
        status: 'finished',
        childScore: 5,
        parentScore: 5,
        finished: true,
        mode: 'room',
        joinCode: 'ABC123',
      );
    }
    return current;
  }

  @override
  Future<List<BattleSummary>> history(String childId, {int limit = 20}) async =>
      [
        BattleSummary(
          id: 'b0',
          mode: 'room',
          status: 'finished',
          questionCount: 5,
          childScore: 5,
          parentScore: 5,
          createdAt: DateTime(2026, 11, 5, 9, 30),
          finishedAt: DateTime(2026, 11, 5, 9, 32),
        ),
      ];

  @override
  Future<BattleState> submit(
    String childId,
    String battleId,
    String side,
    Map<String, String> answers,
  ) async {
    if (side == 'child') {
      childSubmits++;
      current = _state(
        status: 'parent_turn',
        childScore: 1,
        mode: current.mode,
        joinCode: current.joinCode,
      );
      return current;
    }
    parentSubmits++;
    current = _state(
      status: 'finished',
      childScore: 1,
      parentScore: 0,
      finished: true,
      mode: current.mode,
      joinCode: current.joinCode,
    );
    return current;
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

  testWidgets('创建双设备房间显示房间码', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          battleRepositoryProvider.overrideWithValue(_FakeBattleRepository()),
        ],
        child: MaterialApp(
          home: BattlePage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
          ),
        ),
      ),
    );
    await tester.tap(find.text('双设备房间'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.text('房间已创建'), findsOneWidget);
    expect(find.text('ABC123'), findsOneWidget);
  });

  testWidgets('输入房间码加入后孩子答题', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          battleRepositoryProvider.overrideWithValue(_FakeBattleRepository()),
        ],
        child: MaterialApp(
          home: BattlePage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
          ),
        ),
      ),
    );
    await tester.tap(find.text('输入房间码加入'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, 'abc123');
    await tester.tap(find.text('加入'));
    await tester.pumpAndSettle();
    expect(find.text('孩子答题'), findsOneWidget);
    expect(find.text('2+3=?'), findsOneWidget);
  });

  testWidgets('入口展示对战记录', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          battleRepositoryProvider.overrideWithValue(_FakeBattleRepository()),
        ],
        child: MaterialApp(
          home: BattlePage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('对战记录'), findsOneWidget);
    expect(find.text('孩子 5 : 5 家长'), findsOneWidget);
    expect(find.text('平局'), findsOneWidget);
  });

  testWidgets('等待页显示房间码倒计时并在过期后停止轮询', (tester) async {
    final start = DateTime(2026, 11, 5, 10);
    var now = start;
    final repository = _FakeBattleRepository(
      roomExpiresAt: start.add(const Duration(seconds: 3)),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [battleRepositoryProvider.overrideWithValue(repository)],
        child: MaterialApp(
          home: BattlePage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
            now: () => now,
          ),
        ),
      ),
    );

    await tester.tap(find.text('双设备房间'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.text('房间码剩余 00:03'), findsOneWidget);

    now = start.add(const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
    expect(find.text('房间码剩余 00:02'), findsOneWidget);

    now = start.add(const Duration(seconds: 3));
    await tester.pump(const Duration(seconds: 2));
    expect(find.text('房间码已过期，请重新创建'), findsOneWidget);

    final callsAfterExpiry = repository.getCalls;
    now = start.add(const Duration(seconds: 20));
    await tester.pump(const Duration(seconds: 5));
    expect(repository.getCalls, callsAfterExpiry);
  });

  testWidgets('点击对战记录进入详情', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          battleRepositoryProvider.overrideWithValue(_FakeBattleRepository()),
        ],
        child: MaterialApp(
          home: BattlePage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('孩子 5 : 5 家长'));
    await tester.pumpAndSettle();
    expect(find.textContaining('对战详情'), findsOneWidget);
    expect(find.text('平局'), findsOneWidget);
    expect(find.text('孩子 5 : 5 家长'), findsOneWidget);
  });
}
