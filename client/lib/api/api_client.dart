import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;

import '../core/token_store.dart';

/// FastLedger 服务端 API 信封：{code, msg, data}。
class ApiEnvelope {
  final int code;
  final String msg;
  final dynamic data;
  ApiEnvelope(this.code, this.msg, this.data);
}

/// 服务端/网络错误。status 为 HTTP 状态码（网络异常时为 0）。
class ApiException implements Exception {
  final int status;
  final String message;
  ApiException(this.status, this.message);
  @override
  String toString() => 'ApiException($status, $message)';
}

/// HTTP API 客户端（WBS 5.1 网络层）。
///
/// - 统一携带 `Authorization: Bearer`；
/// - 统一解 {code,msg,data} 信封，HTTP 2xx 且 code==0 才算成功；
/// - 401 时回调 [onUnauthorized]（清会话回登录页由上层决定）。
class ApiClient {
  final String baseUrl;
  final http.Client _http;
  final TokenStore tokens;

  /// 401 统一出口（令牌失效/被撤销——服务端 v7 起强校验 jti）。
  void Function()? onUnauthorized;

  ApiClient({
    required this.baseUrl,
    required this.tokens,
    http.Client? httpClient,
  }) : _http = httpClient ?? http.Client();

  Uri _uri(String path, [Map<String, String>? query]) {
    final base = baseUrl.endsWith('/')
        ? baseUrl.substring(0, baseUrl.length - 1)
        : baseUrl;
    return Uri.parse('$base$path').replace(queryParameters: query);
  }

  Future<dynamic> get(String path, {Map<String, String>? query}) async {
    final env = await _send('GET', _uri(path, query));
    return env.data;
  }

  Future<dynamic> post(String path, Map<String, dynamic> body) async {
    final env = await _send('POST', _uri(path), body: body);
    return env.data;
  }

  Future<dynamic> put(String path, Map<String, dynamic> body) async {
    final env = await _send('PUT', _uri(path), body: body);
    return env.data;
  }

  Future<dynamic> delete(String path, {Map<String, String>? query}) async {
    final env = await _send('DELETE', _uri(path, query));
    return env.data;
  }

  /// multipart 文件上传（OCR recognize 用）。返回解信封后的 data。
  Future<dynamic> postMultipart(
      String path, String fileField, Uint8List bytes, String filename) async {
    final token = await tokens.read();
    var request = http.MultipartRequest('POST', _uri(path));
    if (token != null && token.isNotEmpty) {
      request.headers['Authorization'] = 'Bearer $token';
    }
    request.files.add(
        http.MultipartFile.fromBytes(fileField, bytes, filename: filename));
    final http.StreamedResponse streamed;
    try {
      streamed = await _http.send(request);
    } catch (_) {
      throw ApiException(0, '网络不可用，请检查服务器地址与网络');
    }
    final text = await streamed.stream.bytesToString();
    ApiEnvelope env;
    try {
      final decoded = jsonDecode(text) as Map<String, dynamic>;
      env = ApiEnvelope(
        (decoded['code'] as num?)?.toInt() ?? -1,
        (decoded['msg'] as String?) ?? '',
        decoded['data'],
      );
    } catch (_) {
      throw ApiException(streamed.statusCode, '响应解析失败（非 JSON 信封）');
    }
    if (streamed.statusCode == 401) {
      await tokens.clear();
      onUnauthorized?.call();
      throw ApiException(401, env.msg.isEmpty ? '登录已过期，请重新登录' : env.msg);
    }
    if (streamed.statusCode < 200 || streamed.statusCode >= 300 || env.code != 0) {
      throw ApiException(streamed.statusCode,
          env.msg.isEmpty ? '请求失败（HTTP ${streamed.statusCode}）' : env.msg);
    }
    return env.data;
  }

  Future<ApiEnvelope> _send(String method, Uri uri,
      {Map<String, dynamic>? body}) async {
    final token = await tokens.read();
    final request = http.Request(method, uri);
    request.headers['Content-Type'] = 'application/json';
    if (token != null && token.isNotEmpty) {
      request.headers['Authorization'] = 'Bearer $token';
    }
    if (body != null) {
      request.body = jsonEncode(body);
    }
    final http.StreamedResponse streamed;
    try {
      streamed = await _http.send(request);
    } catch (_) {
      throw ApiException(0, '网络不可用，请检查服务器地址与网络');
    }
    final text = await streamed.stream.bytesToString();
    ApiEnvelope env;
    try {
      final decoded = jsonDecode(text) as Map<String, dynamic>;
      env = ApiEnvelope(
        (decoded['code'] as num?)?.toInt() ?? -1,
        (decoded['msg'] as String?) ?? '',
        decoded['data'],
      );
    } catch (_) {
      throw ApiException(streamed.statusCode, '响应解析失败（非 JSON 信封）');
    }
    if (streamed.statusCode == 401) {
      await tokens.clear();
      onUnauthorized?.call();
      throw ApiException(401, env.msg.isEmpty ? '登录已过期，请重新登录' : env.msg);
    }
    if (streamed.statusCode < 200 || streamed.statusCode >= 300 || env.code != 0) {
      throw ApiException(streamed.statusCode,
          env.msg.isEmpty ? '请求失败（HTTP ${streamed.statusCode}）' : env.msg);
    }
    return env;
  }
}
