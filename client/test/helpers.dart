import 'dart:convert';

import 'package:http/http.dart' as http;

/// 测试用假 HTTP 客户端：按 (method, path 正则) 分发预设响应，
/// 并记录最近一次请求供断言（路径/请求体）。
class FakeApiHttp extends http.BaseClient {
  /// handler 返回 (status, body)；返回 null → 404。
  final (int, String)? Function(String method, String path, String? body)
      handler;
  final List<http.BaseRequest> requests = [];

  FakeApiHttp(this.handler);

  (int, String)? dispatch(http.BaseRequest request) {
    final path = request.url.toString().replaceFirst(
        RegExp('^https?://[^/]+'), '');
    final body = request is http.Request ? request.body : null;
    return handler(request.method, path, body);
  }

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    requests.add(request);
    final (status, body) =
        dispatch(request) ?? (404, jsonEncode({'code': 404, 'msg': 'not found'}));
    return http.StreamedResponse(Stream.value(utf8.encode(body)), status,
        headers: {'content-type': 'application/json'});
  }

  /// 最近一次 JSON 请求体（非 body 请求返回 null）
  Map<String, dynamic>? get lastJsonBody {
    for (final r in requests.reversed) {
      if (r is http.Request && r.body.isNotEmpty) {
        return jsonDecode(r.body) as Map<String, dynamic>;
      }
    }
    return null;
  }

  String? get lastPath {
    if (requests.isEmpty) return null;
    return requests.last.url.toString().replaceFirst(RegExp('^https?://[^/]+'), '');
  }
}

/// 标准成功信封
String okJson(Object? data) => jsonEncode({'code': 0, 'msg': 'ok', 'data': data});

/// 标准错误信封
String errJson(int code, String msg) =>
    jsonEncode({'code': code, 'msg': msg});
