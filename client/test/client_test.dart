import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/api/ledger_api.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:fastledger_client/main.dart';
import 'package:fastledger_client/ui/login_page.dart';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

class ScriptedHttp extends http.BaseClient {
  final http.BaseResponse Function(http.BaseRequest request) responder;
  final List<http.BaseRequest> requests = [];
  ScriptedHttp(this.responder);

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    requests.add(request);
    final r = responder(request);
    if (r is http.StreamedResponse) return r;
    final resp = r as http.Response;
    return http.StreamedResponse(
        Stream.value(resp.bodyBytes), resp.statusCode,
        headers: resp.headers);
  }
}

http.Response jsonResp(int status, Map<String, dynamic> body) =>
    http.Response(jsonEncode(body), status,
        headers: {'content-type': 'application/json'});

void main() {
  group('LedgerApi', () {
    test('createTransaction 发送 clientUuid 与整数分金额', () async {
      final h = ScriptedHttp(
          (req) => jsonResp(200, {'code': 0, 'msg': 'ok', 'data': null}));
      final tokens = InMemoryTokenStore();
      final c = ApiClient(baseUrl: 'http://t', tokens: tokens, httpClient: h);
      final api = LedgerApi(c);
      await api.createTransaction(
        bookId: 1,
        clientUuid: 'u-1',
        txType: 2,
        accountId: 5,
        amountCents: 1234,
        txTimeUnix: 1761990000,
        comment: '午餐',
      );
      final body = jsonDecode((h.requests.single as http.Request).body)
          as Map<String, dynamic>;
      expect(body['clientUuid'], 'u-1');
      expect(body['amount'], 1234);
      expect(body['txType'], 2);
      expect(body['txTime'], 1761990000);
    });

    test('updateTransaction 携带 baseVersion（乐观锁）', () async {
      final h = ScriptedHttp(
          (req) => jsonResp(200, {'code': 0, 'msg': 'ok', 'data': null}));
      final c = ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h);
      await LedgerApi(c).updateTransaction(
        bookId: 1,
        txId: 9,
        clientUuid: 'u-2',
        txType: 2,
        accountId: 5,
        amountCents: 4321,
        txTimeUnix: 1761990000,
        baseVersion: 3,
      );
      expect(h.requests.single.url.path, '/api/v1/books/1/transactions/9');
      final body = jsonDecode((h.requests.single as http.Request).body)
          as Map<String, dynamic>;
      expect(body['baseVersion'], 3);
    });

    test('deleteTransaction 走 DELETE + clientUuid query', () async {
      final h = ScriptedHttp(
          (req) => jsonResp(200, {'code': 0, 'msg': 'ok', 'data': null}));
      final c = ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h);
      await LedgerApi(c).deleteTransaction(1, 9, 'u-3');
      expect(h.requests.single.method, 'DELETE');
      expect(h.requests.single.url.queryParameters['clientUuid'], 'u-3');
    });

    test('books 空列表：null → 空（防 null.length 崩溃）', () async {
      final h = ScriptedHttp(
          (req) => jsonResp(200, {'code': 0, 'msg': 'ok', 'data': null}));
      final c = ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h);
      expect(await LedgerApi(c).books(), isEmpty);
    });
  });

  group('登录页 widget', () {
    testWidgets('空输入提示错误；成功登录回调 onLoggedIn', (tester) async {
      final h = ScriptedHttp((req) => jsonResp(200, {
            'code': 0,
            'msg': 'ok',
            'data': {'accessToken': 'tok-xyz'}
          }));
      final c = ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h);
      var loggedIn = false;
      await tester.pumpWidget(MaterialApp(
          home: LoginPage(
              auth: AuthApi(c), onLoggedIn: () => loggedIn = true)));

      // 空输入 → 本地校验错误
      await tester.tap(find.byType(FilledButton));
      await tester.pump();
      expect(find.text('请输入用户名与密码'), findsOneWidget);
      expect(loggedIn, isFalse);

      // 输入后提交 → 成功回调
      await tester.enterText(
          find.widgetWithText(TextField, '用户名'), 'alice');
      await tester.enterText(
          find.widgetWithText(TextField, '密码'), 'password123');
      await tester.tap(find.byType(FilledButton));
      await tester.pumpAndSettle();
      expect(loggedIn, isTrue);
      final body = jsonDecode(
          (h.requests.single as http.Request).body) as Map<String, dynamic>;
      expect(body['username'], 'alice');
    });

    testWidgets('FastLedgerApp 可渲染（冒烟）', (tester) async {
      // 注入内存 TokenStore：真插件依赖平台通道，测试环境不可用
      await tester.pumpWidget(FastLedgerApp(tokenStore: InMemoryTokenStore()));
      await tester.pumpAndSettle();
      expect(find.byType(LoginPage), findsOneWidget);
    });
  });
}
