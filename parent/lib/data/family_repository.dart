import '../core/api/api_client.dart';
import '../core/api/models.dart';

class ChildDraft {
  const ChildDraft({
    required this.nickname,
    required this.stage,
    this.birthYear,
    this.birthMonth,
    this.gradeTag,
  });

  final String nickname;
  final String stage;
  final int? birthYear;
  final int? birthMonth;
  final String? gradeTag;

  Map<String, dynamic> toJson() => {
    'nickname': nickname,
    'stage': stage,
    if (birthYear != null) 'birth_year': birthYear,
    if (birthMonth != null) 'birth_month': birthMonth,
    if (gradeTag != null && gradeTag!.isNotEmpty) 'grade_tag': gradeTag,
  };
}

class FamilyRepository {
  FamilyRepository(this._api);

  final ApiClient _api;

  Future<List<Child>> listChildren() async {
    final data = await _api.getList('/v1/family/children');
    return data
        .whereType<Map<String, dynamic>>()
        .map(Child.fromJson)
        .toList(growable: false);
  }

  Future<Child> createChild(ChildDraft draft) async {
    final json = await _api.postJson('/v1/family/children', draft.toJson());
    return Child.fromJson(json);
  }

  Future<Child> updateChild(String id, ChildDraft draft) async {
    final json = await _api.putJson('/v1/family/children/$id', draft.toJson());
    return Child.fromJson(json);
  }

  Future<void> setPin({
    required String currentPin,
    required String newPin,
  }) async {
    await _api.postJson('/v1/family/pin', {
      'current_pin': currentPin,
      'new_pin': newPin,
    });
  }

  Future<List<FamilyMember>> listMembers() async {
    final data = await _api.getList('/v1/family/members');
    return data
        .whereType<Map<String, dynamic>>()
        .map(FamilyMember.fromJson)
        .toList(growable: false);
  }

  Future<InviteCode> inviteMember() async {
    final json = await _api.postJson('/v1/family/members/invite', {});
    return InviteCode.fromJson(json);
  }

  Future<void> joinFamily(String code) async {
    await _api.postJson('/v1/family/members/join', {'code': code});
  }
}

class FamilyMember {
  FamilyMember({
    required this.accountId,
    required this.phone,
    required this.role,
    this.joinedAt,
  });

  final String accountId;
  final String phone;
  final String role;
  final DateTime? joinedAt;

  factory FamilyMember.fromJson(Map<String, dynamic> json) => FamilyMember(
    accountId: json['account_id'] as String? ?? '',
    phone: json['phone'] as String? ?? '',
    role: json['role'] as String? ?? 'member',
    joinedAt: DateTime.tryParse(json['joined_at'] as String? ?? ''),
  );
}

class InviteCode {
  InviteCode({required this.code, this.expiresAt});

  final String code;
  final DateTime? expiresAt;

  factory InviteCode.fromJson(Map<String, dynamic> json) => InviteCode(
    code: json['code'] as String? ?? '',
    expiresAt: DateTime.tryParse(json['expires_at'] as String? ?? ''),
  );
}

String stageLabel(String stage) => switch (stage) {
  'preschool' => '学龄前',
  'low' => '小学低年级',
  'mid' => '小学中年级',
  'high' => '小学高年级',
  _ => '未设置',
};
