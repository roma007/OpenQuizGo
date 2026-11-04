import '../core/api/api_client.dart';

class MembershipPlan {
  MembershipPlan({
    required this.id,
    required this.code,
    required this.name,
    required this.description,
    required this.durationDays,
    required this.priceCents,
    required this.features,
    required this.entitlementCodes,
    required this.status,
    required this.sort,
  });

  final String id;
  final String code;
  final String name;
  final String description;
  final int durationDays;
  final int priceCents;
  final List<String> features;
  final List<String> entitlementCodes;
  final String status;
  final int sort;

  factory MembershipPlan.fromJson(Map<String, dynamic> json) {
    final raw = json['features'];
    final rawCodes = json['entitlement_codes'];
    return MembershipPlan(
      id: json['id'] as String? ?? '',
      code: json['code'] as String? ?? '',
      name: json['name'] as String? ?? '',
      description: json['description'] as String? ?? '',
      durationDays: (json['duration_days'] as num?)?.toInt() ?? 0,
      priceCents: (json['price_cents'] as num?)?.toInt() ?? 0,
      features: raw is List
          ? raw.map((e) => '$e').toList(growable: false)
          : const [],
      entitlementCodes: rawCodes is List
          ? rawCodes.map((e) => '$e').toList(growable: false)
          : const [],
      status: json['status'] as String? ?? 'draft',
      sort: (json['sort'] as num?)?.toInt() ?? 0,
    );
  }
}

class MembershipInfo {
  MembershipInfo({
    required this.active,
    required this.status,
    required this.planName,
    required this.expiresAt,
    required this.daysRemaining,
    required this.entitlements,
  });

  final bool active;
  final String status;
  final String planName;
  final DateTime? expiresAt;
  final int daysRemaining;
  final List<String> entitlements;

  factory MembershipInfo.fromJson(Map<String, dynamic> json) => MembershipInfo(
    active: json['active'] as bool? ?? false,
    status: json['status'] as String? ?? 'none',
    planName: json['plan_name'] as String? ?? '',
    expiresAt: json['expires_at'] is String
        ? DateTime.tryParse(json['expires_at'] as String)
        : null,
    daysRemaining: (json['days_remaining'] as num?)?.toInt() ?? 0,
    entitlements: json['entitlements'] is List
        ? (json['entitlements'] as List)
              .map((e) => '$e')
              .toList(growable: false)
        : const [],
  );
}

class MembershipCenter {
  MembershipCenter({required this.membership, required this.plans});

  final MembershipInfo? membership;
  final List<MembershipPlan> plans;

  factory MembershipCenter.fromJson(Map<String, dynamic> json) {
    final rawMembership = json['membership'];
    final rawPlans = json['plans'];
    return MembershipCenter(
      membership: rawMembership is Map<String, dynamic>
          ? MembershipInfo.fromJson(rawMembership)
          : null,
      plans: rawPlans is List
          ? rawPlans
                .whereType<Map<String, dynamic>>()
                .map(MembershipPlan.fromJson)
                .toList(growable: false)
          : const [],
    );
  }
}

class MembershipOrder {
  MembershipOrder({
    required this.id,
    required this.orderNo,
    required this.planName,
    required this.amountCents,
    required this.durationDays,
    required this.status,
    required this.createdAt,
    required this.paidAt,
    this.refundReason = '',
  });

  final String id;
  final String orderNo;
  final String planName;
  final int amountCents;
  final int durationDays;
  final String status;
  final DateTime? createdAt;
  final DateTime? paidAt;
  final String refundReason;

  factory MembershipOrder.fromJson(Map<String, dynamic> json) =>
      MembershipOrder(
        id: json['id'] as String? ?? '',
        orderNo: json['order_no'] as String? ?? '',
        planName: json['plan_name'] as String? ?? '',
        amountCents: (json['amount_cents'] as num?)?.toInt() ?? 0,
        durationDays: (json['duration_days'] as num?)?.toInt() ?? 0,
        status: json['status'] as String? ?? 'pending',
        createdAt: json['created_at'] is String
            ? DateTime.tryParse(json['created_at'] as String)
            : null,
        paidAt: json['paid_at'] is String
            ? DateTime.tryParse(json['paid_at'] as String)
            : null,
        refundReason: json['refund_reason'] as String? ?? '',
      );
}

class MembershipPayResult {
  MembershipPayResult({required this.order, required this.membership});

  final MembershipOrder order;
  final MembershipInfo membership;

  factory MembershipPayResult.fromJson(Map<String, dynamic> json) =>
      MembershipPayResult(
        order: MembershipOrder.fromJson(
          json['order'] as Map<String, dynamic>? ?? const {},
        ),
        membership: MembershipInfo.fromJson(
          json['membership'] as Map<String, dynamic>? ?? const {},
        ),
      );
}

class MembershipRepository {
  MembershipRepository(this._api);

  final ApiClient _api;

  Future<MembershipCenter> center() async {
    final json = await _api.getJson('/v1/family/membership');
    return MembershipCenter.fromJson(json);
  }

  Future<List<MembershipOrder>> orders() async {
    final json = await _api.getJson('/v1/family/membership/orders');
    final raw = json['items'];
    return raw is List
        ? raw
              .whereType<Map<String, dynamic>>()
              .map(MembershipOrder.fromJson)
              .toList(growable: false)
        : const [];
  }

  Future<MembershipOrder> createOrder(String planId) async {
    final json = await _api.postJson('/v1/family/membership/orders', {
      'plan_id': planId,
    });
    return MembershipOrder.fromJson(json);
  }

  Future<MembershipPayResult> payOrder(String orderId) async {
    final json = await _api.postJson(
      '/v1/family/membership/orders/$orderId/pay',
      const {},
    );
    return MembershipPayResult.fromJson(json);
  }
}

String formatYuan(int cents) => '¥${(cents / 100).toStringAsFixed(2)}';

String formatDate(DateTime? value) {
  if (value == null) return '—';
  final local = value.toLocal();
  final month = local.month.toString().padLeft(2, '0');
  final day = local.day.toString().padLeft(2, '0');
  return '${local.year}-$month-$day';
}

String formatDateTime(DateTime? value) {
  if (value == null) return '—';
  final local = value.toLocal();
  final hour = local.hour.toString().padLeft(2, '0');
  final minute = local.minute.toString().padLeft(2, '0');
  return '${formatDate(local)} $hour:$minute';
}
