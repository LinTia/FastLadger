import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/api/ledger_api.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:fastledger_client/ui/tx_form_page.dart';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

class RecordingHttp extends http.BaseClient {
  final List<http.Request> requests = [];
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    requests.add(request as http.Request);
    return http.StreamedResponse(
        Stream.value(utf8.encode(jsonEncode({
          'code': 0,
          'msg': 'ok',
          'data': {'transactionIds': [11], 'version': 1}
        }))),
        200,
        headers: {'content-type': 'application/json'});
  }
}

const accounts = [
  {'id': 5, 'name': '现金', 'category': 1, 'currency': 'CNY'},
  {'id': 6, 'name': '储蓄卡', 'category': 2, 'currency': 'CNY'},
];

const categories = [
  {'id': 11, 'name': '餐饮', 'catType': 2},
  {'id': 12, 'name': '工资', 'catType': 1},
];

Widget _wrap(LedgerApi api, {Map<String, dynamic>? tx}) => MaterialApp(
      home: Scaffold(
        body: TxFormPage(
            ledger: api,
            bookId: 1,
            accounts: accounts,
            categories: categories,
            tx: tx),
      ),
    );

void main() {
  group('TxFormPage（记一笔/编辑）', () {
    testWidgets('支出表单：选账户/分类 + 输入金额 → POST 整数分', (tester) async {
      final http_ = RecordingHttp();
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t',
          tokens: InMemoryTokenStore(),
          httpClient: http_));
      await tester.pumpWidget(_wrap(api));

      await tester.enterText(find.byType(TextField).first, '12.34');
      await tester.tap(find.text('账户').last); // 打开下拉
      await tester.pumpAndSettle();
      await tester.tap(find.text('现金').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('分类').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('餐饮').last);
      await tester.pumpAndSettle();

      await tester.tap(find.widgetWithText(FilledButton, '保存'));
      await tester.pumpAndSettle();

      final body =
          jsonDecode(http_.requests.single.body) as Map<String, dynamic>;
      expect(body['txType'], 2);
      expect(body['amount'], 1234); // 整数分，禁浮点
      expect(body['accountId'], 5);
      expect(body['categoryId'], 11);
      expect(body['clientUuid'], isNotEmpty);
      expect(body.containsKey('baseVersion'), isFalse); // 创建不带乐观锁
    });

    testWidgets('编辑模式：类型只读、携带 baseVersion', (tester) async {
      final http_ = RecordingHttp();
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t',
          tokens: InMemoryTokenStore(),
          httpClient: http_));
      final tx = {
        'id': 9,
        'txType': 2,
        'accountId': 5,
        'categoryId': 11,
        'amount': 1234,
        'comment': '旧备注',
        'txTime': 1761900000,
        'syncVersion': 3,
      };
      await tester.pumpWidget(_wrap(api, tx: tx));

      // 类型分段按钮禁用
      final seg = tester.widget<SegmentedButton<int>>(
          find.byType(SegmentedButton<int>));
      expect(seg.onSelectionChanged, isNull);
      // 预填金额与备注
      expect(find.text('12.34'), findsOneWidget);
      expect(find.text('旧备注'), findsOneWidget);

      await tester.enterText(find.byType(TextField).first, '43.21');
      await tester.tap(find.widgetWithText(FilledButton, '保存'));
      await tester.pumpAndSettle();

      final body =
          jsonDecode(http_.requests.single.body) as Map<String, dynamic>;
      expect(body['baseVersion'], 3); // 乐观锁
      expect(body['amount'], 4321);
      expect(http_.requests.single.method, 'PUT');
    });

    testWidgets('非法金额提交显示错误且不发请求', (tester) async {
      final http_ = RecordingHttp();
      final api = LedgerApi(ApiClient(
          baseUrl: 'http://t',
          tokens: InMemoryTokenStore(),
          httpClient: http_));
      await tester.pumpWidget(_wrap(api));

      await tester.enterText(find.byType(TextField).first, '1.234');
      await tester.tap(find.widgetWithText(FilledButton, '保存'));
      await tester.pumpAndSettle();
      expect(find.text('金额非法（正数，最多两位小数）'), findsOneWidget);
      expect(http_.requests, isEmpty);
    });
  });
}
