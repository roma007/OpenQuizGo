import '../core/api/api_client.dart';

class SubjectStat {
  SubjectStat({
    required this.answered,
    required this.correct,
    required this.accuracy,
  });

  final int answered;
  final int correct;
  final double accuracy;

  Map<String, dynamic> toJson() => {
    'answered': answered,
    'correct': correct,
    'accuracy': accuracy,
  };

  factory SubjectStat.fromJson(Map<String, dynamic> json) => SubjectStat(
    answered: (json['answered'] as num?)?.toInt() ?? 0,
    correct: (json['correct'] as num?)?.toInt() ?? 0,
    accuracy: (json['accuracy'] as num?)?.toDouble() ?? 0,
  );

  static SubjectStat empty() =>
      SubjectStat(answered: 0, correct: 0, accuracy: 0);
}

class GrowthReport {
  GrowthReport({
    required this.id,
    required this.period,
    required this.periodStart,
    required this.periodEnd,
    required this.subjectStats,
    required this.funStats,
    required this.watchMinutes,
  });

  final String id;
  final String period;
  final DateTime? periodStart;
  final DateTime? periodEnd;
  final Map<String, SubjectStat> subjectStats;
  final SubjectStat funStats;
  final int watchMinutes;

  Map<String, dynamic> toJson() => {
    'id': id,
    'period': period,
    'period_start': periodStart?.toIso8601String(),
    'period_end': periodEnd?.toIso8601String(),
    'subject_stats': {
      for (final entry in subjectStats.entries) entry.key: entry.value.toJson(),
    },
    'fun_stats': funStats.toJson(),
    'watch_minutes': watchMinutes,
  };

  factory GrowthReport.fromJson(Map<String, dynamic> json) {
    final subjects = <String, SubjectStat>{};
    final rawSubjects = json['subject_stats'];
    if (rawSubjects is Map) {
      rawSubjects.forEach((key, value) {
        if (value is Map<String, dynamic>) {
          subjects['$key'] = SubjectStat.fromJson(value);
        }
      });
    }
    final fun = json['fun_stats'];
    return GrowthReport(
      id: json['id'] as String? ?? '',
      period: json['period'] as String? ?? 'week',
      periodStart: DateTime.tryParse(json['period_start'] as String? ?? ''),
      periodEnd: DateTime.tryParse(json['period_end'] as String? ?? ''),
      subjectStats: subjects,
      funStats: fun is Map<String, dynamic>
          ? SubjectStat.fromJson(fun)
          : SubjectStat.empty(),
      watchMinutes: (json['watch_minutes'] as num?)?.toInt() ?? 0,
    );
  }

  int get answered =>
      subjectStats.values.fold(0, (sum, s) => sum + s.answered) +
      funStats.answered;

  int get correct =>
      subjectStats.values.fold(0, (sum, s) => sum + s.correct) +
      funStats.correct;

  double get accuracy => answered == 0 ? 0 : correct / answered;
}

class ReportQuery {
  ReportQuery({required this.childId, required this.period});

  final String childId;
  final String period;

  @override
  bool operator ==(Object other) =>
      other is ReportQuery &&
      other.childId == childId &&
      other.period == period;

  @override
  int get hashCode => Object.hash(childId, period);
}

class WeakSubject {
  WeakSubject({
    required this.subject,
    required this.answered,
    required this.correct,
    required this.accuracy,
  });

  final String subject;
  final int answered;
  final int correct;
  final double accuracy;

  factory WeakSubject.fromJson(Map<String, dynamic> json) => WeakSubject(
    subject: json['subject'] as String? ?? '',
    answered: (json['answered'] as num?)?.toInt() ?? 0,
    correct: (json['correct'] as num?)?.toInt() ?? 0,
    accuracy: (json['accuracy'] as num?)?.toDouble() ?? 0,
  );
}

class TopicSuggestion {
  TopicSuggestion({
    required this.topicId,
    required this.name,
    required this.reason,
  });

  final String topicId;
  final String name;
  final String reason;

  factory TopicSuggestion.fromJson(Map<String, dynamic> json) =>
      TopicSuggestion(
        topicId: json['topic_id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        reason: json['reason'] as String? ?? '',
      );
}

class Recommendations {
  Recommendations({
    required this.childId,
    required this.weakSubjects,
    required this.suggestedDifficulty,
    required this.topics,
    required this.generatedAt,
  });

  final String childId;
  final List<WeakSubject> weakSubjects;
  final int suggestedDifficulty;
  final List<TopicSuggestion> topics;
  final DateTime? generatedAt;

  factory Recommendations.fromJson(Map<String, dynamic> json) {
    final subjects = json['weak_subjects'];
    final topics = json['topics'];
    return Recommendations(
      childId: json['child_id'] as String? ?? '',
      weakSubjects: subjects is List
          ? subjects
                .whereType<Map<String, dynamic>>()
                .map(WeakSubject.fromJson)
                .toList(growable: false)
          : const [],
      suggestedDifficulty:
          (json['suggested_difficulty'] as num?)?.toInt() ?? 2,
      topics: topics is List
          ? topics
                .whereType<Map<String, dynamic>>()
                .map(TopicSuggestion.fromJson)
                .toList(growable: false)
          : const [],
      generatedAt: json['generated_at'] is String
          ? DateTime.tryParse(json['generated_at'] as String)
          : null,
    );
  }
}

class ReportRepository {
  ReportRepository(this._api);

  final ApiClient _api;

  Future<Recommendations> recommendations(String childId) async {
    final json = await _api.getJson(
      '/v1/family/children/$childId/recommendations',
    );
    return Recommendations.fromJson(json);
  }

  Future<List<GrowthReport>> listReports(ReportQuery query) async {
    final json = await _api.getJson(
      '/v1/family/children/${query.childId}/reports',
      query: {'period': query.period, 'limit': 8},
    );
    final items = json['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map<String, dynamic>>()
        .map(GrowthReport.fromJson)
        .toList(growable: false);
  }
}
