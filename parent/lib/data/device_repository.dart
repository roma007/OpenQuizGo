import '../core/api/api_client.dart';
import '../core/api/models.dart';

class DeviceRepository {
  DeviceRepository(this._api);

  final ApiClient _api;

  Future<List<DeviceInfo>> listDevices() async {
    final data = await _api.getList('/v1/family/devices');
    return data
        .whereType<Map<String, dynamic>>()
        .map(DeviceInfo.fromJson)
        .toList(growable: false);
  }

  Future<void> activate({required String code, required String childId}) async {
    await _api.postJson('/v1/family/devices/activate', {
      'code': code,
      'child_id': childId,
    });
  }

  Future<void> unbind(String bindId) async {
    await _api.delete('/v1/family/devices/$bindId');
  }

  Future<UnlockGrant> unlock({
    required String bindId,
    required String pin,
    required int minutes,
  }) async {
    final json = await _api.postJson('/v1/family/devices/$bindId/unlock', {
      'pin': pin,
      'minutes': minutes,
    });
    return UnlockGrant.fromJson(json);
  }
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

String platformLabel(String platform) => switch (platform) {
  'tv' => '电视',
  'phone' => '手机',
  'pad' => '平板',
  _ => platform,
};
