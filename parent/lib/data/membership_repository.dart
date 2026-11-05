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

class MembershipSharing {
  const MembershipSharing({required this.shared, required this.canEdit});

  final bool shared;
  final bool canEdit;

  factory MembershipSharing.fromJson(Map<String, dynamic> json) =>
      MembershipSharing(
        shared: json['shared'] as bool? ?? false,
        canEdit: json['can_edit'] as bool? ?? false,
      );
}

class MembershipInfo {
  MembershipInfo({
    required this.active,
    this.shared = false,
    required this.status,
    required this.planName,
    required this.expiresAt,
    required this.daysRemaining,
    required this.entitlements,
  });

  final bool active;
  final bool shared;
  final String status;
  final String planName;
  final DateTime? expiresAt;
  final int daysRemaining;
  final List<String> entitlements;

  factory MembershipInfo.fromJson(Map<String, dynamic> json) => MembershipInfo(
    active: json['active'] as bool? ?? false,
    shared: json['shared'] as bool? ?? false,
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

class MembershipAutoRenew {
  const MembershipAutoRenew({
    required this.enabled,
    required this.planId,
    required this.planName,
    required this.canEdit,
  });

  final bool enabled;
  final String planId;
  final String planName;
  final bool canEdit;

  factory MembershipAutoRenew.fromJson(Map<String, dynamic> json) =>
      MembershipAutoRenew(
        enabled: json['enabled'] as bool? ?? false,
        planId: json['plan_id'] as String? ?? '',
        planName: json['plan_name'] as String? ?? '',
        canEdit: json['can_edit'] as bool? ?? true,
      );
}

class MembershipCenter {
  MembershipCenter({
    required this.membership,
    required this.plans,
    this.sharing = const MembershipSharing(shared: false, canEdit: false),
    this.autoRenew = const MembershipAutoRenew(
      enabled: false,
      planId: '',
      planName: '',
      canEdit: true,
    ),
  });

  final MembershipInfo? membership;
  final List<MembershipPlan> plans;
  final MembershipSharing sharing;
  final MembershipAutoRenew autoRenew;

  factory MembershipCenter.fromJson(Map<String, dynamic> json) {
    final rawMembership = json['membership'];
    final rawPlans = json['plans'];
    final rawSharing = json['sharing'];
    return MembershipCenter(
      membership: rawMembership is Map<String, dynamic>
          ? MembershipInfo.fromJson(rawMembership)
          : null,
      sharing: rawSharing is Map<String, dynamic>
          ? MembershipSharing.fromJson(rawSharing)
          : const MembershipSharing(shared: false, canEdit: false),
      autoRenew: json['auto_renew'] is Map<String, dynamic>
          ? MembershipAutoRenew.fromJson(
              json['auto_renew'] as Map<String, dynamic>,
            )
          : const MembershipAutoRenew(
              enabled: false,
              planId: '',
              planName: '',
              canEdit: true,
            ),
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
    this.originalAmountCents = 0,
    this.discountCents = 0,
    this.couponCode = '',
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
  final int originalAmountCents;
  final int discountCents;
  final String couponCode;
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
        originalAmountCents:
            (json['original_amount_cents'] as num?)?.toInt() ?? 0,
        discountCents: (json['discount_cents'] as num?)?.toInt() ?? 0,
        couponCode: json['coupon_code'] as String? ?? '',
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

class MembershipInvoice {
  MembershipInvoice({
    required this.id,
    required this.orderId,
    required this.titleType,
    required this.title,
    required this.taxNo,
    required this.email,
    required this.amountCents,
    required this.status,
    required this.remark,
    required this.createdAt,
    required this.issuedAt,
  });

  final String id;
  final String orderId;
  final String titleType;
  final String title;
  final String taxNo;
  final String email;
  final int amountCents;
  final String status;
  final String remark;
  final DateTime? createdAt;
  final DateTime? issuedAt;

  factory MembershipInvoice.fromJson(Map<String, dynamic> json) =>
      MembershipInvoice(
        id: json['id'] as String? ?? '',
        orderId: json['order_id'] as String? ?? '',
        titleType: json['title_type'] as String? ?? 'personal',
        title: json['title'] as String? ?? '',
        taxNo: json['tax_no'] as String? ?? '',
        email: json['email'] as String? ?? '',
        amountCents: (json['amount_cents'] as num?)?.toInt() ?? 0,
        status: json['status'] as String? ?? 'requested',
        remark: json['remark'] as String? ?? '',
        createdAt: json['created_at'] is String
            ? DateTime.tryParse(json['created_at'] as String)
            : null,
        issuedAt: json['issued_at'] is String
            ? DateTime.tryParse(json['issued_at'] as String)
            : null,
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

  Future<List<MembershipInvoice>> invoices() async {
    final json = await _api.getJson('/v1/family/membership/invoices');
    final raw = json['items'];
    return raw is List
        ? raw
              .whereType<Map<String, dynamic>>()
              .map(MembershipInvoice.fromJson)
              .toList(growable: false)
        : const [];
  }

  Future<MembershipInvoice> applyInvoice(
    String orderId, {
    required String titleType,
    required String title,
    String taxNo = '',
    String email = '',
  }) async {
    final json = await _api.postJson(
      '/v1/family/membership/orders/$orderId/invoice',
      {
        'title_type': titleType,
        'title': title,
        'tax_no': taxNo,
        'email': email,
      },
    );
    return MembershipInvoice.fromJson(json);
  }

  Future<MembershipAutoRenew> autoRenew() async {
    final json = await _api.getJson('/v1/family/membership/auto-renew');
    return MembershipAutoRenew.fromJson(json);
  }

  Future<MembershipAutoRenew> setAutoRenew({
    required bool enabled,
    required String planId,
  }) async {
    final json = await _api.putJson('/v1/family/membership/auto-renew', {
      'enabled': enabled,
      'plan_id': planId,
    });
    return MembershipAutoRenew.fromJson(json);
  }

  Future<MembershipSharing> sharing() async {
    final json = await _api.getJson('/v1/family/membership/sharing');
    return MembershipSharing.fromJson(json);
  }

  Future<MembershipSharing> setSharing(bool shared) async {
    final json = await _api.putJson('/v1/family/membership/sharing', {
      'shared': shared,
    });
    return MembershipSharing.fromJson(json);
  }

  Future<MembershipOrder> createOrder(
    String planId, {
    String couponCode = '',
  }) async {
    final json = await _api.postJson('/v1/family/membership/orders', {
      'plan_id': planId,
      if (couponCode.trim().isNotEmpty) 'coupon_code': couponCode.trim(),
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
