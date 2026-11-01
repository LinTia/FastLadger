import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/api/ledger_api.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:fastledger_client/ocr/ocr_api.dart';
import 'package:fastledger_client/ocr/ocr_flow.dart';
import 'package:fastledger_client/parse/receipt_parser.dart';
import 'package:fastledger_client/ui/tx_confirm_dialog.dart';
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
    return http.StreamedResponse(Stream.value(resp.bodyBytes), resp.statusCode,
        headers: resp.headers);
  }
}

http.Response jsonResp(int status, Map<String, dynamic> body) =>
    http.Response(jsonEncode(body), status,
        headers: {'content-type': 'application/json'});

const accounts = [
  {'id': 5, 'name': '现金', 'category': 1, 'currency': 'CNY'},
];
const categories = [
  {'id': 11, 'name': '餐饮', 'catType': 2},
];

void main() {
  group('OcrApi.parseOfflineLines（端侧规则引擎，零上传）', () {
    test('OCR 行文本 → 候选', () {
      final got = OcrApi.parseOfflineLines([
        {'text': '支出'},
        {'text': '¥45.00'},
        {'text': '2026-11-01'},
      ]);
      expect(got, hasLength(1));
      expect(got[0].amountCents, 4500);
      expect(got[0].direction, 'expense');
    });
  });

  group('TxConfirmDialog（绝不自动入账）', () {
    testWidgets('空候选 → 仅提示手动记账，无网络请求', (tester) async {
      final h = ScriptedHttp(
          (req) => fail('空候选不应发起任何请求'));
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(MaterialApp(home: Builder(builder: (context) {
        return TextButton(
          onPressed: () => TxConfirmDialog.show(context,
              ledger: api,
              bookId: 1,
              accounts: accounts,
              categories: categories,
              candidates: const []),
          child: const Text('go'),
        );
      })));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();
      expect(find.text('未识别到交易'), findsOneWidget);
      expect(h.requests, isEmpty);
    });

    testWidgets('有候选：选账户 → 点入账 → POST 交易（含幂等 clientUuid）',
        (tester) async {
      final h = ScriptedHttp(
          (req) => jsonResp(200, {'code': 0, 'msg': 'ok', 'data': null}));
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));

      final candidates = [
        ReceiptCandidate(
            amountCents: 4500,
            direction: 'expense',
            occurredDate: '2026-11-01',
            description: '测试商户'),
      ];
      await tester.pumpWidget(MaterialApp(home: Builder(builder: (context) {
        return TextButton(
          onPressed: () => TxConfirmDialog.show(context,
              ledger: api,
              bookId: 1,
              accounts: accounts,
              categories: categories,
              candidates: candidates),
          child: const Text('go'),
        );
      })));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();

      // 未选账户先点入账 → 错误提示，不请求
      await tester.tap(find.widgetWithText(FilledButton, '入账'));
      await tester.pumpAndSettle();
      expect(find.text('请选择入账账户'), findsOneWidget);
      expect(h.requests, isEmpty);

      // 选账户 → 入账
      await tester.tap(find.text('入账账户').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('现金').last);
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, '入账'));
      await tester.pumpAndSettle();

      expect(h.requests, hasLength(1));
      final body =
          jsonDecode((h.requests.single as http.Request).body)
              as Map<String, dynamic>;
      expect(body['amount'], 4500);
      expect(body['txType'], 2);
      expect(body['clientUuid'], isNotEmpty);
    });

    testWidgets('取消：有候选也不入账', (tester) async {
      final h = ScriptedHttp((req) => fail('取消后不应发请求'));
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      final candidates = [
        ReceiptCandidate(
            amountCents: 4500, direction: 'expense', occurredDate: '', description: ''),
      ];
      await tester.pumpWidget(MaterialApp(home: Builder(builder: (context) {
        return TextButton(
          onPressed: () => TxConfirmDialog.show(context,
              ledger: api,
              bookId: 1,
              accounts: accounts,
              categories: categories,
              candidates: candidates),
          child: const Text('go'),
        );
      })));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();
      expect(h.requests, isEmpty);
    });
  });

  group('runOcrFlow 编排', () {
    testWidgets('识别异常 → 手动回退提示，不抛异常不入账', (tester) async {
      final h = ScriptedHttp(
          (req) => jsonResp(503, {'code': 1, 'msg': 'OCR 服务暂时不可用'}));
      final ocr = OcrApi(ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));

      await tester.pumpWidget(MaterialApp(home: Builder(builder: (context) {
        return TextButton(
          onPressed: () => runOcrFlow(
            context,
            ocr: ocr,
            ledger: api,
            bookId: 1,
            accounts: accounts,
            categories: categories,
            screenshot: () async => Uint8List.fromList([0x89, 0x50, 0x4E, 0x47]),
          ),
          child: const Text('go'),
        );
      })));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();
      expect(find.text('OCR 未完成'), findsOneWidget);
    });

    testWidgets('端侧离线命中 → 直接出确认弹窗（服务端仅 recognize 被调用）',
        (tester) async {
      final h = ScriptedHttp((req) {
        if (req.url.path.endsWith('/ocr/recognize')) {
          return jsonResp(200, {
            'code': 0,
            'msg': 'ok',
            'data': {
              'engine': 'server',
              'lines': [
                {'text': '支出', 'confidence': 0.99},
                {'text': '¥30.00', 'confidence': 0.98},
                {'text': '2026-11-01', 'confidence': 0.97},
              ]
            }
          });
        }
        return fail('离线命中后不应再调服务端 parse');
      });
      final ocr = OcrApi(ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));

      await tester.pumpWidget(MaterialApp(home: Builder(builder: (context) {
        return TextButton(
          onPressed: () => runOcrFlow(
            context,
            ocr: ocr,
            ledger: api,
            bookId: 1,
            accounts: accounts,
            categories: categories,
            screenshot: () async => Uint8List.fromList([0x89, 0x50]),
          ),
          child: const Text('go'),
        );
      })));
      await tester.tap(find.text('go'));
      await tester.pumpAndSettle();
      // 确认弹窗出现且显示候选金额（¥30.00 → 3000 分）
      expect(find.text('确认入账'), findsOneWidget);
      expect(find.text('支出 ¥30.00'), findsOneWidget);
    });
  });
}
