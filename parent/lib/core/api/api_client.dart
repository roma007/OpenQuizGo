import 'dart:convert';

import 'package:dio/dio.dart';

import '../config.dart';
import '../storage/token_store.dart';

class ApiException implements Exception {
  ApiException(this.message, {this.code, this.statusCode});

  final String message;
  final String? code;
  final int? statusCode;

  @override
  String toString() => message;
}

/// 统一 HTTP 客户端：附加令牌、错误码转中文、预留 401 刷新入口。
class ApiClient {
  ApiClient(this._tokenStore, {Dio? dio, String? baseUrl})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              baseUrl: baseUrl ?? AppConfig.apiBaseUrl,
              connectTimeout: const Duration(seconds: 10),
              receiveTimeout: const Duration(seconds: 20),
              headers: {'Content-Type': 'application/json'},
            ),
          ) {
    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          if (options.extra['noAuth'] != true) {
            final token = await _tokenStore.accessToken();
            if (token != null && token.isNotEmpty) {
              options.headers['Authorization'] = 'Bearer $token';
            }
          }
          handler.next(options);
        },
        onError: (error, handler) async {
          final request = error.requestOptions;
          final unauthorized = error.response?.statusCode == 401;
          if (unauthorized &&
              request.extra['noAuth'] != true &&
              request.extra['retried'] != true) {
            final refreshed = await _refreshTokens();
            if (refreshed) {
              try {
                final response = await _dio.fetch<dynamic>(
                  request.copyWith(extra: {...request.extra, 'retried': true}),
                );
                handler.resolve(response);
                return;
              } catch (_) {
                // 落到下方的原始错误。
              }
            }
          }
          handler.next(error);
        },
      ),
    );
  }

  final Dio _dio;
  final TokenStore _tokenStore;
  Future<bool>? _refreshing;

  late final Dio _refreshDio = Dio(
    BaseOptions(
      baseUrl: _dio.options.baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 20),
      headers: {'Content-Type': 'application/json'},
    ),
  );

  Dio get raw => _dio;

  Future<bool> _refreshTokens() {
    return _refreshing ??= _doRefresh().whenComplete(() => _refreshing = null);
  }

  Future<bool> _doRefresh() async {
    final refresh = await _tokenStore.refreshToken();
    if (refresh == null || refresh.isEmpty) return false;
    try {
      final response = await _refreshDio.post<Map<String, dynamic>>(
        'v1/auth/refresh',
        data: {'refresh_token': refresh},
      );
      final body = response.data;
      final access = body?['access_token'] as String?;
      if (access == null || access.isEmpty) return false;
      final nextRefresh = body?['refresh_token'] as String? ?? refresh;
      await _tokenStore.save(access: access, refresh: nextRefresh);
      return true;
    } catch (_) {
      await _tokenStore.clear();
      return false;
    }
  }

  /// 读取 text/event-stream，逐条产出 data: 内容。
  Stream<String> eventStream(String path) async* {
    final response = await _dio.get<ResponseBody>(
      path,
      options: Options(
        responseType: ResponseType.stream,
        receiveTimeout: const Duration(minutes: 30),
        headers: {'Accept': 'text/event-stream'},
      ),
    );
    final stream = response.data?.stream;
    if (stream == null) return;
    final lines = stream
        .cast<List<int>>()
        .transform(utf8.decoder)
        .transform(const LineSplitter());
    await for (final line in lines) {
      if (line.startsWith('data:')) {
        yield line.substring(5).trim();
      }
    }
  }

  Future<Map<String, dynamic>> getJson(
    String path, {
    Map<String, dynamic>? query,
  }) async {
    return _request(
      () => _dio.get<Map<String, dynamic>>(path, queryParameters: query),
    );
  }

  Future<Map<String, dynamic>> postJson(
    String path,
    Map<String, dynamic> body, {
    bool noAuth = false,
  }) async {
    return _request(
      () => _dio.post<Map<String, dynamic>>(
        path,
        data: body,
        options: Options(extra: {'noAuth': noAuth}),
      ),
    );
  }

  Future<List<dynamic>> getList(
    String path, {
    Map<String, dynamic>? query,
  }) async {
    try {
      final response = await _dio.get<List<dynamic>>(
        path,
        queryParameters: query,
      );
      return response.data ?? <dynamic>[];
    } on DioException catch (error) {
      throw _mapError(error);
    }
  }

  Future<Map<String, dynamic>> putJson(
    String path,
    Map<String, dynamic> body,
  ) async {
    return _request(() => _dio.put<Map<String, dynamic>>(path, data: body));
  }

  Future<void> delete(String path) async {
    await _request(() => _dio.delete<Map<String, dynamic>>(path));
  }

  Future<Map<String, dynamic>> _request(
    Future<Response<Map<String, dynamic>>> Function() call,
  ) async {
    try {
      final response = await call();
      return response.data ?? <String, dynamic>{};
    } on DioException catch (error) {
      throw _mapError(error);
    }
  }

  ApiException _mapError(DioException error) {
    final data = error.response?.data;
    if (data is Map && data['message'] is String) {
      return ApiException(
        data['message'] as String,
        code: data['code'] as String?,
        statusCode: error.response?.statusCode,
      );
    }
    if (error.type == DioExceptionType.connectionTimeout ||
        error.type == DioExceptionType.receiveTimeout ||
        error.type == DioExceptionType.connectionError) {
      return ApiException('网络连接失败，请检查网络后重试');
    }
    return ApiException(
      '请求失败（${error.response?.statusCode ?? '未知'}）',
      statusCode: error.response?.statusCode,
    );
  }
}
