import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:http/http.dart' as http;

class ScriptedHttp extends http.BaseClient {
  final int status;
  final String body;
  ScriptedHttp(this.status, this.body);
  http.BaseRequest? last;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    last = request;
    return http.StreamedResponse(Stream.value(utf8.encode(body)), status,
        headers: {'content-type': 'application/json'});
  }
}

void main() {
  group('ApiClient 信封与鉴权', () {
    test('2xx + code=0 → 返回 data', () async {
      final h = ScriptedHttp(200,
          jsonEncode({'code': 0, 'msg': 'ok', 'data': {'books': [1, 2]}}));
      final tokens = InMemoryTokenStore();
      await tokens.save('tok-123');
      final c = ApiClient(baseUrl: 'http://t', tokens: tokens, httpClient: h);
      final data = await c.get('/api/v1/books');
      expect(data, isA<Map>());
      expect(h.last!.headers['Authorization'], 'Bearer tok-123');
    });

    test('2xx + code!=0 → ApiException 带 msg', () async {
      final h = ScriptedHttp(200, jsonEncode({'code': 1, 'msg': '余额不足'}));
      final c = ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h);
      await expectLater(c.post('/x', {}),
          throwsA(isA<ApiException>().having((e) => e.message, 'msg', '余额不足')));
    });

    test('401 → 清 token 并触发 onUnauthorized', () async {
      final h = ScriptedHttp(401,
          jsonEncode({'code': 40100, 'msg': '会话已撤销或已失效'}));
      final tokens = InMemoryTokenStore();
      await tokens.save('dead-token');
      var called = false;
      final c = ApiClient(baseUrl: 'http://t', tokens: tokens, httpClient: h)
        ..onUnauthorized = () => called = true;
      await expectLater(c.get('/x'), throwsA(isA<ApiException>()));
      expect(await tokens.read(), isNull);
      expect(called, isTrue);
    });

    test('POST body 正确序列化', () async {
      final h = ScriptedHttp(200, jsonEncode({'code': 0, 'msg': 'ok'}));
      final c = ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h);
      await c.post('/api/v1/books/1/transactions',
          {'clientUuid': 'abc', 'amount': 123});
      expect((h.last as http.Request).body, jsonEncode({'clientUuid': 'abc', 'amount': 123}));
    });

    test('尾斜杠 baseURL 不会产生双斜杠路径', () async {
      final h = ScriptedHttp(200, jsonEncode({'code': 0, 'msg': 'ok'}));
      final c = ApiClient(
          baseUrl: 'http://t/', tokens: InMemoryTokenStore(), httpClient: h);
      await c.get('/api/v1/books');
      expect(h.last!.url.toString(), 'http://t/api/v1/books');
    });
  });
}
