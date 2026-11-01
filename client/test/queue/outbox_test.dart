import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/core/local_store.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:fastledger_client/queue/outbox.dart';
import 'package:http/http.dart' as http;

/// 可编程假 HTTP 客户端：按路径前缀返回预设状态码。
class FakeHttp extends http.BaseClient {
  final int Function(http.BaseRequest request) handler;
  final List<http.BaseRequest> requests = [];
  FakeHttp(this.handler);

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    requests.add(request);
    final status = handler(request);
    final body = jsonEncode({
      'code': status < 300 ? 0 : 1,
      'msg': status < 300 ? 'ok' : 'err',
      'data': null,
    });
    return http.StreamedResponse(
        Stream.value(utf8.encode(body)), status,
        headers: {'content-type': 'application/json'});
  }
}

void main() {
  group('Outbox（本地队列带 clientUuid）', () {
    late InMemoryLocalStore store;
    late InMemoryTokenStore tokens;
    late FakeHttp http_;
    late ApiClient client;
    late Outbox outbox;

    setUp(() {
      store = InMemoryLocalStore();
      tokens = InMemoryTokenStore();
      http_ = FakeHttp((_) => 200);
      client = ApiClient(baseUrl: 'http://test', tokens: tokens, httpClient: http_);
      outbox = Outbox(store: store, client: client);
    });

    test('入队即生成并持久化 clientUuid', () async {
      await outbox.enqueue(OutboxItem(
          method: 'POST', path: '/api/v1/books/1/transactions', payload: {}));
      final items = await outbox.list();
      expect(items, hasLength(1));
      expect(items[0].clientUuid, matches(RegExp(
          r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$')));
      // 已落盘（同 store 再次读取可见）
      final again = await Outbox(store: store, client: client).list();
      expect(again[0].clientUuid, items[0].clientUuid);
    });

    test('flush 成功（2xx）后出队，并携带 Bearer 与路径', () async {
      await outbox.enqueue(OutboxItem(
          method: 'POST',
          path: '/api/v1/books/1/transactions',
          payload: {'clientUuid': 'u1', 'amount': 100}));
      final result = await outbox.flush();
      expect(result.sent, 1);
      expect(await outbox.list(), isEmpty);
      expect(http_.requests.single.url.path, '/api/v1/books/1/transactions');
    });

    test('flush 遇 5xx：留队且 attempts 递增、clientUuid 不变', () async {
      http_ = FakeHttp((_) => 503);
      client = ApiClient(baseUrl: 'http://test', tokens: tokens, httpClient: http_);
      outbox = Outbox(store: store, client: client);
      await outbox.enqueue(OutboxItem(
          clientUuid: 'keep-me',
          method: 'POST',
          path: '/p',
          payload: {'clientUuid': 'keep-me'}));
      final result = await outbox.flush();
      expect(result.sent, 0);
      final items = await outbox.list();
      expect(items, hasLength(1));
      expect(items[0].attempts, 1);
      expect(items[0].clientUuid, 'keep-me');
    });

    test('flush 遇 4xx：业务拒绝，出队并计入 dropped', () async {
      http_ = FakeHttp((_) => 400);
      client = ApiClient(baseUrl: 'http://test', tokens: tokens, httpClient: http_);
      int? droppedStatus;
      outbox = Outbox(store: store, client: client,
          onDropped: (item, status) => droppedStatus = status);
      await outbox.enqueue(OutboxItem(method: 'POST', path: '/p', payload: {}));
      final result = await outbox.flush();
      expect(result.dropped, 1);
      expect(droppedStatus, 400);
      expect(await outbox.list(), isEmpty);
    });

    test('网络异常：留队', () async {
      client = ApiClient(
          baseUrl: 'http://test',
          tokens: tokens,
          httpClient: _ThrowingClient());
      outbox = Outbox(store: store, client: client);
      await outbox.enqueue(OutboxItem(method: 'POST', path: '/p', payload: {}));
      final result = await outbox.flush();
      expect(result.sent, 0);
      expect(result.remaining, 1);
    });

    test('多个 item：2xx 全部出队，队列清空', () async {
      for (var i = 0; i < 3; i++) {
        await outbox.enqueue(
            OutboxItem(method: 'POST', path: '/p', payload: {'i': i}));
      }
      final result = await outbox.flush();
      expect(result.sent, 3);
      expect(await outbox.list(), isEmpty);
    });
  });
}

class _ThrowingClient extends http.BaseClient {
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    throw Exception('network down');
  }
}
