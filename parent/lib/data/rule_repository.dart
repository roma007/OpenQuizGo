import '../core/api/api_client.dart';

class TimeWindow {
  TimeWindow({required this.start, required this.end});

  final String start;
  final String end;

  Map<String, dynamic> toJson() => {'start': start, 'end': end};

  factory TimeWindow.fromJson(Map<String, dynamic> json) => TimeWindow(
    start: json['start'] as String? ?? '07:00',
    end: json['end'] as String? ?? '21:00',
  );
}

class Rule {
  Rule({
    required this.unlockMinutes,
    required this.passThreshold,
    required this.subjectWeight,
    required this.funRatioCap,
    required this.funEnabled,
    required this.weightPreset,
    required this.timeWindow,
    required this.dailyLimit,
    required this.offlineFallbackMin,
    required this.scopeMode,
    required this.allowedApps,
    required this.timezone,
  });

  int unlockMinutes;
  double passThreshold;
  Map<String, double> subjectWeight;
  int funRatioCap;
  bool funEnabled;
  String weightPreset;
  List<TimeWindow> timeWindow;
  int dailyLimit;
  int offlineFallbackMin;
  String scopeMode;
  List<String> allowedApps;
  String timezone;

  factory Rule.fromJson(Map<String, dynamic> json) {
    final weights = <String, double>{};
    final rawWeights = json['subject_weight'];
    if (rawWeights is Map) {
      rawWeights.forEach((key, value) {
        weights['$key'] = (value as num?)?.toDouble() ?? 0;
      });
    }
    final windows = <TimeWindow>[];
    final rawWindow = json['time_window'];
    if (rawWindow is Map && rawWindow['allow'] is List) {
      for (final item in rawWindow['allow'] as List) {
        if (item is Map<String, dynamic>) {
          windows.add(TimeWindow.fromJson(item));
        }
      }
    }
    return Rule(
      unlockMinutes: (json['unlock_minutes'] as num?)?.toInt() ?? 15,
      passThreshold: (json['pass_threshold'] as num?)?.toDouble() ?? 3,
      subjectWeight: weights,
      funRatioCap: (json['fun_ratio_cap'] as num?)?.toInt() ?? 2,
      funEnabled: json['fun_enabled'] as bool? ?? true,
      weightPreset: json['weight_preset'] as String? ?? 'balanced',
      timeWindow: windows,
      dailyLimit: (json['daily_limit'] as num?)?.toInt() ?? 60,
      offlineFallbackMin: (json['offline_fallback_min'] as num?)?.toInt() ?? 10,
      scopeMode: json['scope_mode'] as String? ?? 'media',
      allowedApps:
          (json['allowed_apps'] as List?)
              ?.map((e) => '$e')
              .toList(growable: false) ??
          const [],
      timezone: json['timezone'] as String? ?? 'Asia/Shanghai',
    );
  }

  Map<String, dynamic> toJson() => {
    'unlock_minutes': unlockMinutes,
    'pass_threshold': passThreshold,
    'subject_weight': subjectWeight,
    'fun_ratio_cap': funRatioCap,
    'fun_enabled': funEnabled,
    'weight_preset': weightPreset,
    'time_window': {
      'allow': [for (final w in timeWindow) w.toJson()],
    },
    'daily_limit': dailyLimit,
    'offline_fallback_min': offlineFallbackMin,
    'scope_mode': scopeMode,
    'allowed_apps': allowedApps,
    'timezone': timezone,
  };

  Rule copy() => Rule.fromJson(toJson());
}

class RuleWithVersion {
  RuleWithVersion({required this.rule, required this.version});

  final Rule rule;
  final int version;
}

class RulePreview {
  RulePreview({
    required this.questionCount,
    required this.maxScore,
    required this.funSlots,
    required this.quota,
    required this.warnings,
  });

  final int questionCount;
  final double maxScore;
  final int funSlots;
  final Map<String, int> quota;
  final List<String> warnings;

  factory RulePreview.fromJson(Map<String, dynamic> json) {
    final quota = <String, int>{};
    final raw = json['quota'];
    if (raw is Map) {
      raw.forEach(
        (key, value) => quota['$key'] = (value as num?)?.toInt() ?? 0,
      );
    }
    return RulePreview(
      questionCount: (json['n'] as num?)?.toInt() ?? 0,
      maxScore: (json['max_score'] as num?)?.toDouble() ?? 0,
      funSlots: (json['fun_slots'] as num?)?.toInt() ?? 0,
      quota: quota,
      warnings:
          (json['warnings'] as List?)?.map((e) => '$e').toList() ?? const [],
    );
  }
}

class RuleRepository {
  RuleRepository(this._api);

  final ApiClient _api;

  Future<RuleWithVersion> getRule(String childId) async {
    final json = await _api.getJson('/v1/family/children/$childId/rule');
    return RuleWithVersion(
      rule: Rule.fromJson(json['rule'] as Map<String, dynamic>),
      version: (json['version'] as num?)?.toInt() ?? 0,
    );
  }

  Future<RulePreview> preview(String childId, Rule rule) async {
    final json = await _api.postJson(
      '/v1/family/children/$childId/rule/preview',
      {'rule': rule.toJson()},
    );
    return RulePreview.fromJson(json);
  }

  Future<void> update(String childId, String pin, Rule rule) async {
    await _api.putJson('/v1/family/children/$childId/rule', {
      'pin': pin,
      'rule': rule.toJson(),
    });
  }
}

const subjectLabels = <String, String>{
  'math': '数学',
  'chinese': '语文',
  'english': '英语',
  'science': '科学',
  'logic': '逻辑',
  'safety': '安全',
  'fun': '趣味',
};

const weightPresets = <String, Map<String, double>>{
  'balanced': {
    'math': 1.5,
    'chinese': 1.5,
    'english': 1.5,
    'science': 1,
    'logic': 1,
    'safety': 1,
    'fun': 0.5,
  },
  'math': {
    'math': 2,
    'chinese': 1.2,
    'english': 1.2,
    'science': 1,
    'logic': 1,
    'safety': 1,
    'fun': 0.5,
  },
  'literacy': {
    'math': 1.2,
    'chinese': 1.2,
    'english': 1,
    'science': 2,
    'logic': 2,
    'safety': 2,
    'fun': 0.5,
  },
};

const presetLabels = <String, String>{
  'balanced': '均衡',
  'math': '数学优先',
  'literacy': '阅读/科学优先',
  'custom': '自定义',
};

const scopeLabels = <String, String>{
  'media': '仅媒体应用',
  'custom': '指定应用',
  'all_apps': '全部应用',
};
