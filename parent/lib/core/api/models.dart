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

  Map<String, dynamic> toJson() => {
    'id': id,
    'nickname': nickname,
    'stage': stage,
    if (birthYear != null) 'birth_year': birthYear,
    if (birthMonth != null) 'birth_month': birthMonth,
    if (gradeTag != null) 'grade_tag': gradeTag,
  };

  factory Child.fromJson(Map<String, dynamic> json) => Child(
    id: json['id'] as String,
    nickname: json['nickname'] as String? ?? '孩子',
    stage: json['stage'] as String? ?? 'low',
    birthYear: (json['birth_year'] as num?)?.toInt(),
    birthMonth: (json['birth_month'] as num?)?.toInt(),
    gradeTag: json['grade_tag'] as String?,
  );
}
