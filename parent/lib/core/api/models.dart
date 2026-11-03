class LoginResult {
  LoginResult({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresIn,
  });

  final String accessToken;
  final String refreshToken;
  final int expiresIn;

  factory LoginResult.fromJson(Map<String, dynamic> json) => LoginResult(
    accessToken: json['access_token'] as String,
    refreshToken: json['refresh_token'] as String,
    expiresIn: (json['expires_in'] as num?)?.toInt() ?? 0,
  );
}

class Child {
  Child({
    required this.id,
    required this.nickname,
    required this.stage,
    this.birthYear,
    this.birthMonth,
    this.gradeTag,
  });

  final String id;
  final String nickname;
  final String stage;
  final int? birthYear;
  final int? birthMonth;
  final String? gradeTag;

  factory Child.fromJson(Map<String, dynamic> json) => Child(
    id: json['id'] as String,
    nickname: json['nickname'] as String? ?? '孩子',
    stage: json['stage'] as String? ?? 'low',
    birthYear: (json['birth_year'] as num?)?.toInt(),
    birthMonth: (json['birth_month'] as num?)?.toInt(),
    gradeTag: json['grade_tag'] as String?,
  );
}

class DeviceInfo {
  DeviceInfo({
    required this.id,
    required this.deviceId,
    required this.platform,
    required this.childId,
    required this.childName,
    this.boundAt,
  });

  final String id;
  final String deviceId;
  final String platform;
  final String childId;
  final String childName;
  final DateTime? boundAt;

  factory DeviceInfo.fromJson(Map<String, dynamic> json) => DeviceInfo(
    id: json['id'] as String,
    deviceId: json['device_id'] as String? ?? '',
    platform: json['platform'] as String? ?? 'tv',
    childId: json['child_id'] as String? ?? '',
    childName: json['child_name'] as String? ?? '',
    boundAt: DateTime.tryParse(json['bound_at'] as String? ?? ''),
  );
}
