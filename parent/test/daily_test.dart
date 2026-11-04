import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openquizgo_parent/core/api/models.dart';
import 'package:openquizgo_parent/core/providers.dart';
import 'package:openquizgo_parent/data/daily_repository.dart';
import 'package:openquizgo_parent/features/daily/daily_quiz_page.dart';

DailyState _state({bool answered = false}) => DailyState(
  date: '2026-11-04',
  question: DailyQuestion(
    id: 'q1',
    stem: '2+3=?',
    type: 'choice',
    options: [
      DailyOption(id: 'a', text: '5'),
      DailyOption(id: 'b', text: '6'),
    ],
  ),
  answered: answered,
  correct: answered ? true : null,
  correctAnswer: answered ? const ['a'] : const [],
  explain: '',
  points: answered ? 10 : 0,
  streak: 1,
  totalPoints: answered ? 10 : 0,
);

class _FakeDailyRepository implements DailyRepository {
  _FakeDailyRepository(this.state);

  DailyState state;

  @override
  Future<DailyState> get(String childId) async => state;

  @override
  Future<DailyState> answer(String childId, String optionId) async {
    state = _state(answered: true);
    return state;
  }
}

void main() {
  testWidgets('每日一题页面渲染选项', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          dailyRepositoryProvider.overrideWithValue(
            _FakeDailyRepository(_state()),
          ),
        ],
        child: MaterialApp(
          home: DailyQuizPage(
            child: Child(id: 'c1', nickname: '小明', stage: 'low'),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('2+3=?'), findsOneWidget);
    expect(find.textContaining('a. 5'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, '提交答案'), findsOneWidget);
  });
}
