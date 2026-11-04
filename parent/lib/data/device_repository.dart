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
}

String platformLabel(String platform) => switch (platform) {
  'tv' => '电视',
  'phone' => '手机',
  'pad' => '平板',
  _ => platform,
};
