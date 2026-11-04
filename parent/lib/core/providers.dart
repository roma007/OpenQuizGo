import '../features/auth/login_screen.dart';
import '../features/shell/home_shell.dart';
import 'api/api_client.dart';
import 'cache/cache_store.dart';
import 'api/models.dart';
import 'auth/auth_controller.dart';
import 'auth/auth_repository.dart';
import '../data/family_repository.dart';
import '../data/device_repository.dart';
import '../data/rule_repository.dart';
import '../data/report_repository.dart';
import '../data/alert_repository.dart';
import 'storage/token_store.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/legacy.dart';
import 'package:go_router/go_router.dart';

final tokenStoreProvider = Provider<TokenStore>((ref) => SecureTokenStore());

final cacheStoreProvider = Provider<CacheStore>((ref) => CacheStore());

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(ref.watch(tokenStoreProvider));
});

final familyRepositoryProvider = Provider<FamilyRepository>((ref) {
  return FamilyRepository(ref.watch(apiClientProvider));
});

final childrenProvider = FutureProvider<Cached<List<Child>>>((ref) async {
  final repository = ref.watch(familyRepositoryProvider);
  final cache = ref.watch(cacheStoreProvider);
  try {
    final children = await repository.listChildren();
    await cache.write(
      'children',
      children.map((child) => child.toJson()).toList(),
    );
    return Cached.fresh(children);
  } on ApiException {
    final entry = await cache.read('children');
    final raw = entry?.data;
    if (raw is List) {
      final children = raw
          .map((item) => Child.fromJson(Map<String, dynamic>.from(item as Map)))
          .toList(growable: false);
      return Cached.stale(children, entry!.cachedAt);
    }
    rethrow;
  }
});

final membersProvider = FutureProvider<List<FamilyMember>>((ref) {
  return ref.watch(familyRepositoryProvider).listMembers();
});

final deviceRepositoryProvider = Provider<DeviceRepository>((ref) {
  return DeviceRepository(ref.watch(apiClientProvider));
});

final devicesProvider = FutureProvider<List<DeviceInfo>>((ref) {
  return ref.watch(deviceRepositoryProvider).listDevices();
});

final ruleRepositoryProvider = Provider<RuleRepository>((ref) {
  return RuleRepository(ref.watch(apiClientProvider));
});

final ruleProvider = FutureProvider.family<Cached<RuleWithVersion>, String>((
  ref,
  childId,
) async {
  final repository = ref.watch(ruleRepositoryProvider);
  final cache = ref.watch(cacheStoreProvider);
  final key = 'rule:$childId';
  try {
    final result = await repository.getRule(childId);
    await cache.write(key, {
      'rule': result.rule.toJson(),
      'version': result.version,
    });
    return Cached.fresh(result);
  } on ApiException {
    final entry = await cache.read(key);
    final raw = entry?.data;
    if (raw is Map) {
      final map = Map<String, dynamic>.from(raw);
      return Cached.stale(
        RuleWithVersion(
          rule: Rule.fromJson(Map<String, dynamic>.from(map['rule'] as Map)),
          version: (map['version'] as num?)?.toInt() ?? 0,
        ),
        entry!.cachedAt,
      );
    }
    rethrow;
  }
});

final alertRepositoryProvider = Provider<AlertRepository>((ref) {
  return AlertRepository(ref.watch(apiClientProvider));
});

final alertsProvider = FutureProvider<AlertList>((ref) {
  return ref.watch(alertRepositoryProvider).list();
});

final reportRepositoryProvider = Provider<ReportRepository>((ref) {
  return ReportRepository(ref.watch(apiClientProvider));
});

final reportsProvider =
    FutureProvider.family<Cached<List<GrowthReport>>, ReportQuery>((
      ref,
      query,
    ) async {
      final repository = ref.watch(reportRepositoryProvider);
      final cache = ref.watch(cacheStoreProvider);
      final key = 'reports:${query.childId}:${query.period}';
      try {
        final reports = await repository.listReports(query);
        await cache.write(
          key,
          reports.map((report) => report.toJson()).toList(),
        );
        return Cached.fresh(reports);
      } on ApiException {
        final entry = await cache.read(key);
        final raw = entry?.data;
        if (raw is List) {
          final reports = raw
              .map(
                (item) => GrowthReport.fromJson(
                  Map<String, dynamic>.from(item as Map),
                ),
              )
              .toList(growable: false);
          return Cached.stale(reports, entry!.cachedAt);
        }
        rethrow;
      }
    });

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(
    ref.watch(apiClientProvider),
    ref.watch(tokenStoreProvider),
  );
});

final authControllerProvider = ChangeNotifierProvider<AuthController>((ref) {
  final controller = AuthController(ref.watch(tokenStoreProvider));
  controller.restore();
  return controller;
});

final routerProvider = Provider<GoRouter>((ref) {
  final auth = ref.watch(authControllerProvider);
  return GoRouter(
    initialLocation: '/home',
    refreshListenable: auth,
    redirect: (context, state) {
      if (!auth.ready) return null;
      final loggingIn = state.matchedLocation == '/login';
      if (!auth.signedIn && !loggingIn) return '/login';
      if (auth.signedIn && loggingIn) return '/home';
      return null;
    },
    routes: [
      GoRoute(path: '/login', builder: (_, _) => const LoginScreen()),
      GoRoute(path: '/home', builder: (_, _) => const HomeShell()),
    ],
  );
});
