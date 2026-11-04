import '../core/api/api_client.dart';

class AlertItem {
  AlertItem({
    required this.id,
    required this.type,
    required this.detail,
    required this.reportedAt,
    required this.childName,
    required this.deviceId,
    required this.read,
  });

  final int id;
  final String type;
  final Map<String, dynamic> detail;
  final DateTime? reportedAt;
  final String childName;
  final String deviceId;
  final bool read;

  factory AlertItem.fromJson(Map<String, dynamic> json) => AlertItem(
    id: (json['id'] as num?)?.toInt() ?? 0,
    type: json['type'] as String? ?? 'other',
    detail: (json['detail'] as Map?)?.cast<String, dynamic>() ?? const {},
    reportedAt: DateTime.tryParse(json['reported_at'] as String? ?? ''),
    childName: json['child_name'] as String? ?? '',
    deviceId: json['device_id'] as String? ?? '',
    read: json['read'] as bool? ?? false,
  );
}

class AlertList {
  AlertList({required this.items, required this.unread});

  final List<AlertItem> items;
  final int unread;

  factory AlertList.fromJson(Map<String, dynamic> json) {
    final raw = json['items'];
    return AlertList(
      items: raw is List
          ? raw
                .whereType<Map<String, dynamic>>()
                .map(AlertItem.fromJson)
                .toList(growable: false)
          : const [],
      unread: (json['unread'] as num?)?.toInt() ?? 0,
    );
  }
}

class AlertRepository {
  AlertRepository(this._api);

  final ApiClient _api;

  Future<AlertList> list({int limit = 50}) async {
    final json = await _api.getJson(
      '/v1/family/alerts',
      query: {'limit': limit},
    );
    return AlertList.fromJson(json);
  }

  Future<void> markAllRead() async {
    await _api.postJson('/v1/family/alerts/read', {});
  }
}

String alertTitle(String type, Map<String, dynamic> detail) {
  switch (type) {
    case 'permission_revoked':
      final permission = detail['permission'] as String?;
      return switch (permission) {
        'accessibility' => '无障碍服务被关闭',
        'usage_stats' => '使用情况访问被关闭',
        'notifications' => '通知权限被关闭',
        _ => '权限被关闭',
      };
    case 'uninstall_attempt':
      return '疑似卸载尝试';
    case 'package_tampered':
      return '应用完整性异常';
    case 'clock_skew':
      return '设备时间异常';
    default:
      return '设备异常';
  }
}
