import '../core/api/api_client.dart';

class DailyOption {
  DailyOption({required this.id, required this.text});

  final String id;
  final String text;

  factory DailyOption.fromJson(Map<String, dynamic> json) => DailyOption(
    id: json['id'] as String? ?? '',
    text: json['text'] as String? ?? '',
  );
}

class DailyQuestion {
  DailyQuestion({
    required this.id,
    required this.stem,
    required this.type,
    required this.options,
  });

  final String id;
  final String stem;
  final String type;
  final List<DailyOption> options;

  factory DailyQuestion.fromJson(Map<String, dynamic> json) {
    final rawOptions = json['options'];
    return DailyQuestion(
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

class DailyState {
  DailyState({
    required this.date,
    required this.question,
    required this.answered,
    required this.correct,
    required this.correctAnswer,
    required this.explain,
    required this.points,
    required this.streak,
    required this.totalPoints,
  });

  final String date;
  final DailyQuestion? question;
  final bool answered;
  final bool? correct;
  final List<String> correctAnswer;
  final String explain;
  final int points;
  final int streak;
  final int totalPoints;

  factory DailyState.fromJson(Map<String, dynamic> json) {
    final rawQuestion = json['question'];
    final rawCorrectAnswer = json['correct_answer'];
    return DailyState(
      date: json['date'] as String? ?? '',
      question: rawQuestion is Map<String, dynamic>
          ? DailyQuestion.fromJson(rawQuestion)
          : null,
      answered: json['answered'] as bool? ?? false,
      correct: json['correct'] as bool?,
      correctAnswer: rawCorrectAnswer is List
          ? rawCorrectAnswer.map((e) => '$e').toList(growable: false)
          : const [],
      explain: json['explain'] as String? ?? '',
      points: (json['points'] as num?)?.toInt() ?? 0,
      streak: (json['streak'] as num?)?.toInt() ?? 0,
      totalPoints: (json['total_points'] as num?)?.toInt() ?? 0,
    );
  }
}

class DailyRepository {
  DailyRepository(this._api);

  final ApiClient _api;

  Future<DailyState> get(String childId) async {
    final json = await _api.getJson('/v1/family/children/$childId/daily');
    return DailyState.fromJson(json);
  }

  Future<DailyState> answer(String childId, String optionId) async {
    final json = await _api.postJson(
      '/v1/family/children/$childId/daily/answers',
      {
        'answer': [optionId],
      },
    );
    return DailyState.fromJson(json);
  }
}
