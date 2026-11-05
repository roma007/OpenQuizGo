import '../core/api/api_client.dart';
import 'daily_repository.dart';

class BattleQuestion {
  BattleQuestion({
    required this.id,
    required this.stem,
    required this.type,
    required this.options,
  });

  final String id;
  final String stem;
  final String type;
  final List<DailyOption> options;

  factory BattleQuestion.fromJson(Map<String, dynamic> json) {
    final rawOptions = json['options'];
    return BattleQuestion(
      id: json['id'] as String? ?? '',
      stem: json['stem'] as String? ?? '',
      type: json['type'] as String? ?? 'choice',
      options: rawOptions is List
          ? rawOptions
                .whereType<Map<String, dynamic>>()
                .map(DailyOption.fromJson)
                .toList(growable: false)
          : const [],
    );
  }
}

class BattleResult {
  BattleResult({
    required this.winner,
    required this.childScore,
    required this.parentScore,
  });

  final String winner;
  final int childScore;
  final int parentScore;

  factory BattleResult.fromJson(Map<String, dynamic> json) => BattleResult(
    winner: json['winner'] as String? ?? 'draw',
    childScore: (json['child_score'] as num?)?.toInt() ?? 0,
    parentScore: (json['parent_score'] as num?)?.toInt() ?? 0,
  );
}

class BattleAnswer {
  BattleAnswer({
    required this.side,
    required this.questionId,
    required this.correct,
    required this.correctAnswer,
    required this.explain,
  });

  final String side;
  final String questionId;
  final bool correct;
  final List<String> correctAnswer;
  final String explain;

  factory BattleAnswer.fromJson(Map<String, dynamic> json) {
    final raw = json['correct_answer'];
    return BattleAnswer(
      side: json['side'] as String? ?? '',
      questionId: json['question_id'] as String? ?? '',
      correct: json['correct'] as bool? ?? false,
      correctAnswer: raw is List
          ? raw.map((e) => '$e').toList(growable: false)
          : const [],
      explain: json['explain'] as String? ?? '',
    );
  }
}

class BattleState {
  BattleState({
    required this.id,
    required this.status,
    required this.questionCount,
    required this.questions,
    required this.childScore,
    required this.parentScore,
    required this.result,
    required this.answers,
    required this.mode,
    required this.joinCode,
  });

  final String id;
  final String status;
  final int questionCount;
  final List<BattleQuestion> questions;
  final int childScore;
  final int parentScore;
  final BattleResult? result;
  final List<BattleAnswer> answers;
  final String mode;
  final String joinCode;

  bool get finished => status == 'finished';
  bool get childTurn => status == 'child_turn';

  BattleAnswer? answerFor(String side, String questionId) {
    for (final answer in answers) {
      if (answer.side == side && answer.questionId == questionId) {
        return answer;
      }
    }
    return null;
  }

  factory BattleState.fromJson(Map<String, dynamic> json) {
    final rawQuestions = json['questions'];
    final rawAnswers = json['answers'];
    final rawResult = json['result'];
    return BattleState(
      id: json['id'] as String? ?? '',
      status: json['status'] as String? ?? 'child_turn',
      questionCount: (json['question_count'] as num?)?.toInt() ?? 0,
      questions: rawQuestions is List
          ? rawQuestions
                .whereType<Map<String, dynamic>>()
                .map(BattleQuestion.fromJson)
                .toList(growable: false)
          : const [],
      childScore: (json['child_score'] as num?)?.toInt() ?? 0,
      parentScore: (json['parent_score'] as num?)?.toInt() ?? 0,
      result: rawResult is Map<String, dynamic>
          ? BattleResult.fromJson(rawResult)
          : null,
      answers: rawAnswers is List
          ? rawAnswers
                .whereType<Map<String, dynamic>>()
                .map(BattleAnswer.fromJson)
                .toList(growable: false)
          : const [],
      mode: json['mode'] as String? ?? 'same_device',
      joinCode: json['join_code'] as String? ?? '',
    );
  }
}

class BattleRepository {
  BattleRepository(this._api);

  final ApiClient _api;

  Future<BattleState> create(
    String childId, {
    int questionCount = 5,
    String mode = 'same_device',
  }) async {
    final json = await _api.postJson('/v1/family/children/$childId/battles', {
      'question_count': questionCount,
      'mode': mode,
    });
    return BattleState.fromJson(json);
  }

  Future<BattleState> join(String childId, String code) async {
    final json = await _api.postJson(
      '/v1/family/children/$childId/battles/join',
      {'code': code},
    );
    return BattleState.fromJson(json);
  }

  Future<BattleState> get(String childId, String battleId) async {
    final json = await _api.getJson(
      '/v1/family/children/$childId/battles/$battleId',
    );
    return BattleState.fromJson(json);
  }

  Future<BattleState> submit(
    String childId,
    String battleId,
    String side,
    Map<String, String> answers,
  ) async {
    final payload = answers.entries
        .map(
          (entry) => {
            'question_id': entry.key,
            'answer': [entry.value],
          },
        )
        .toList(growable: false);
    final json = await _api.postJson(
      '/v1/family/children/$childId/battles/$battleId/answers',
      {'side': side, 'answers': payload},
    );
    return BattleState.fromJson(json);
  }
}

String describeWinner(BattleResult result) {
  if (result.winner == 'child') return '孩子获胜';
  if (result.winner == 'parent') return '家长获胜';
  return '平局';
}
