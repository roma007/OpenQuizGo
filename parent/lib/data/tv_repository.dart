import '../core/api/api_client.dart';

/// 081：单台电视的观看配置，逐项独立生效。
///
/// 做题开关与每日上限互相独立：关掉做题仍会到点硬停，关掉上限仍会弹题。
class TvConfig {
  TvConfig({
    required this.quizEnabled,
    required this.limitEnabled,
    required this.unlockMinutes,
    required this.dailyLimit,
    required this.subjects,
    required this.passThreshold,
    required this.quizScope,
    required this.hardStopGrace,
    required this.version,
  });

  final bool quizEnabled;
  final bool limitEnabled;
  final int unlockMinutes;
  final int dailyLimit;
  final List<String> subjects;
  final double passThreshold;
  final String quizScope;
  final int hardStopGrace;
  final int version;

  factory TvConfig.fromJson(Map<String, dynamic> json) => TvConfig(
    quizEnabled: json['quiz_enabled'] as bool? ?? true,
    limitEnabled: json['limit_enabled'] as bool? ?? true,
    unlockMinutes: (json['unlock_minutes'] as num?)?.toInt() ?? 15,
    dailyLimit: (json['daily_limit'] as num?)?.toInt() ?? 60,
    subjects:
        (json['subjects'] as List<dynamic>?)?.whereType<String>().toList(
          growable: false,
        ) ??
        const <String>[],
    passThreshold: (json['pass_threshold'] as num?)?.toDouble() ?? 3,
    quizScope: json['quiz_scope'] as String? ?? 'random',
    hardStopGrace: (json['hard_stop_grace'] as num?)?.toInt() ?? 10,
    version: (json['version'] as num?)?.toInt() ?? 0,
  );

  Map<String, dynamic> toJson() => {
    'quiz_enabled': quizEnabled,
    'limit_enabled': limitEnabled,
    'unlock_minutes': unlockMinutes,
    'daily_limit': dailyLimit,
    'subjects': subjects,
    'pass_threshold': passThreshold,
    'quiz_scope': quizScope,
    'hard_stop_grace': hardStopGrace,
    'version': version,
  };

  TvConfig copyWith({
    bool? quizEnabled,
    bool? limitEnabled,
    int? unlockMinutes,
    int? dailyLimit,
    List<String>? subjects,
    double? passThreshold,
    String? quizScope,
    int? hardStopGrace,
  }) => TvConfig(
    quizEnabled: quizEnabled ?? this.quizEnabled,
    limitEnabled: limitEnabled ?? this.limitEnabled,
    unlockMinutes: unlockMinutes ?? this.unlockMinutes,
    dailyLimit: dailyLimit ?? this.dailyLimit,
    subjects: subjects ?? this.subjects,
    passThreshold: passThreshold ?? this.passThreshold,
    quizScope: quizScope ?? this.quizScope,
    hardStopGrace: hardStopGrace ?? this.hardStopGrace,
    version: version,
  );
}

/// 081：家庭下的一台电视（登录后自动出现在家长端，无需激活码）。
class TvDevice {
  TvDevice({
    required this.id,
    required this.name,
    required this.deviceId,
    required this.childId,
    required this.childName,
    required this.gradeTag,
    required this.online,
    required this.lastSeenAt,
    required this.usedMinutes,
    required this.config,
  });

  final String id;
  final String name;
  final String deviceId;
  final String childId;
  final String childName;
  final String gradeTag;
  final bool online;
  final DateTime? lastSeenAt;
  final int usedMinutes;
  final TvConfig config;

  bool get hasChild => childName.trim().isNotEmpty;

  factory TvDevice.fromJson(Map<String, dynamic> json) => TvDevice(
    id: json['id'] as String,
    name: json['name'] as String? ?? '电视',
    deviceId: json['device_id'] as String? ?? '',
    childId: json['child_id'] as String? ?? '',
    childName: json['child_name'] as String? ?? '',
    gradeTag: json['grade_tag'] as String? ?? '',
    online: json['online'] as bool? ?? false,
    lastSeenAt: DateTime.tryParse(json['last_seen_at'] as String? ?? ''),
    usedMinutes: (json['used_minutes'] as num?)?.toInt() ?? 0,
    config: TvConfig.fromJson(
      (json['config'] as Map?)?.cast<String, dynamic>() ?? const {},
    ),
  );
}

class UnlockGrant {
  UnlockGrant({
    required this.grantId,
    required this.minutes,
    required this.remainingSeconds,
  });

  final String grantId;
  final int minutes;
  final int remainingSeconds;

  factory UnlockGrant.fromJson(Map<String, dynamic> json) => UnlockGrant(
    grantId: json['grant_id'] as String? ?? '',
    minutes: (json['minutes'] as num?)?.toInt() ?? 0,
    remainingSeconds: (json['remaining_seconds'] as num?)?.toInt() ?? 0,
  );
}

class TvRepository {
  TvRepository(this._api);

  final ApiClient _api;

  Future<List<TvDevice>> listTVs() async {
    final data = await _api.getList('/v1/family/tvs');
    return data
        .whereType<Map<String, dynamic>>()
        .map(TvDevice.fromJson)
        .toList(growable: false);
  }

  Future<TvDevice> getTV(String deviceId) async {
    final json = await _api.getJson('/v1/family/tvs/$deviceId');
    return TvDevice.fromJson(json);
  }

  Future<TvConfig> getConfig(String deviceId) async {
    final json = await _api.getJson('/v1/family/tvs/$deviceId/config');
    return TvConfig.fromJson(json);
  }

  /// 保存配置需要家长 PIN；[config.version] 参与乐观锁校验。
  Future<TvConfig> updateConfig({
    required String deviceId,
    required String pin,
    required TvConfig config,
  }) async {
    final json = await _api.putJson('/v1/family/tvs/$deviceId/config', {
      'pin': pin,
      'config': config.toJson(),
    });
    return TvConfig.fromJson(json);
  }

  /// 指定孩子（昵称 + 年级）；年级每年 9 月 1 日由服务端自动升级。
  Future<TvDevice> assignChild({
    required String deviceId,
    required String nickname,
    String? gradeTag,
  }) async {
    final json = await _api.putJson('/v1/family/tvs/$deviceId/child', {
      'nickname': nickname,
      'grade_tag': gradeTag ?? '',
    });
    return TvDevice.fromJson(json);
  }

  Future<UnlockGrant> unlock({
    required String deviceId,
    required String pin,
    int? minutes,
  }) async {
    final json = await _api.postJson('/v1/family/tvs/$deviceId/unlock', {
      'pin': pin,
      'minutes': ?minutes,
    });
    return UnlockGrant.fromJson(json);
  }

  Future<void> unbind(String deviceId) async {
    await _api.delete('/v1/family/tvs/$deviceId');
  }
}

const subjectLabels = <String, String>{
  'chinese': '语文',
  'math': '数学',
  'english': '英语',
  'science': '科学',
  'fun': '趣味',
};

const gradeLabels = <String, String>{
  'g1': '一年级',
  'g2': '二年级',
  'g3': '三年级',
  'g4': '四年级',
  'g5': '五年级',
  'g6': '六年级',
  'g7': '七年级',
  'g8': '八年级',
  'g9': '九年级',
};

String gradeLabel(String tag) => gradeLabels[tag] ?? '未设置年级';

String subjectLabel(String subject) => subjectLabels[subject] ?? subject;

String scopeLabel(String scope) => switch (scope) {
  'grade' => '按孩子年级出题',
  _ => '全库随机出题',
};
