import '../features/auth/login_screen.dart';
import '../features/shell/home_shell.dart';
import 'api/api_client.dart';
import 'api/models.dart';
import 'auth/auth_controller.dart';
import 'auth/auth_repository.dart';
import '../data/family_repository.dart';
import '../data/device_repository.dart';
import '../data/rule_repository.dart';
import 'storage/token_store.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/legacy.dart';
import 'package:go_router/go_router.dart';

final tokenStoreProvider = Provider<TokenStore>((ref) => SecureTokenStore());

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(ref.watch(tokenStoreProvider));
});

final familyRepositoryProvider = Provider<FamilyRepository>((ref) {
  return FamilyRepository(ref.watch(apiClientProvider));
});

final childrenProvider = FutureProvider<List<Child>>((ref) {
  return ref.watch(familyRepositoryProvider).listChildren();
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

final ruleProvider = FutureProvider.family<RuleWithVersion, String>(
  (ref, childId) => ref.watch(ruleRepositoryProvider).getRule(childId),
);

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
