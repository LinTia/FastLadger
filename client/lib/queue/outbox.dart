import 'dart:convert';

import '../api/api_client.dart';
import '../core/local_store.dart';
import '../core/uuid.dart';

/// 离线操作队列单项（WBS 5.1：本地队列带 clientUuid）。
///
/// [clientUuid] 在入队时生成并持久化——网络重试/进程重启后重放同一键，
/// 服务端按 op+clientUuid 幂等去重，绝不产生重复入账。
class OutboxItem {
  final String clientUuid;
  final String method; // POST / PUT / DELETE
  final String path; // 完整 API 路径（含 bookId 等）
  final Map<String, dynamic> payload;
  final int attempts;
  final DateTime createdAt;

  OutboxItem({
    String? clientUuid,
    required this.method,
    required this.path,
    required this.payload,
    this.attempts = 0,
    DateTime? createdAt,
  })  : clientUuid = clientUuid ?? uuidV4(),
        createdAt = createdAt ?? DateTime.now().toUtc();

  Map<String, dynamic> toJson() => {
        'clientUuid': clientUuid,
        'method': method,
        'path': path,
        'payload': payload,
        'attempts': attempts,
        'createdAt': createdAt.toIso8601String(),
      };

  static OutboxItem fromJson(Map<String, dynamic> j) => OutboxItem(
        clientUuid: j['clientUuid'] as String,
        method: j['method'] as String,
        path: j['path'] as String,
        payload: (j['payload'] as Map).cast<String, dynamic>(),
        attempts: (j['attempts'] as num?)?.toInt() ?? 0,
        createdAt: DateTime.parse(j['createdAt'] as String),
      );
}

/// flush 结果统计。
class OutboxFlushResult {
  final int sent;
  final int dropped; // 4xx 永久失败（业务错误重试无意义）
  final int remaining; // 网络/5xx 留队
  OutboxFlushResult(this.sent, this.dropped, this.remaining);
}

/// 持久化离线队列。
///
/// 存储走 [LocalStore]（骨架期内存实现；生产接 sqflite/drift 后同语义）。
/// flush 规则：
/// - 2xx        → 出队（已入账）；
/// - 4xx        → 出队并计入 dropped（业务拒绝，重试无意义；调用方可监听）；
/// - 网络错误/5xx → 留队，attempts+1，等待下次 flush。
class Outbox {
  static const storageKey = 'fastledger.outbox.v1';
  final LocalStore store;
  final ApiClient client;
  final void Function(OutboxItem item, int status)? onDropped;

  Outbox({required this.store, required this.client, this.onDropped});

  Future<void> enqueue(OutboxItem item) async {
    final items = await list();
    items.add(item);
    await _save(items);
  }

  Future<List<OutboxItem>> list() async {
    final raw = await store.getString(storageKey);
    if (raw == null || raw.isEmpty) return [];
    final decoded = jsonDecode(raw) as List;
    return decoded
        .map((e) => OutboxItem.fromJson((e as Map).cast<String, dynamic>()))
        .toList();
  }

  Future<OutboxFlushResult> flush() async {
    var items = await list();
    var sent = 0, dropped = 0;
    final keep = <OutboxItem>[];
    for (final item in items) {
      try {
        switch (item.method) {
          case 'POST':
            await client.post(item.path, item.payload);
            break;
          case 'PUT':
            await client.put(item.path, item.payload);
            break;
          case 'DELETE':
            await client.delete(item.path);
            break;
          default:
            dropped++;
            onDropped?.call(item, 0);
            continue;
        }
        sent++;
      } on ApiException catch (e) {
        if (e.status >= 400 && e.status < 500) {
          dropped++;
          onDropped?.call(item, e.status);
        } else {
          keep.add(_bump(item));
        }
        continue;
      } catch (_) {
        keep.add(_bump(item));
        continue;
      }
    }
    await _save(keep);
    return OutboxFlushResult(sent, dropped, keep.length);
  }

  OutboxItem _bump(OutboxItem item) => OutboxItem(
        clientUuid: item.clientUuid,
        method: item.method,
        path: item.path,
        payload: item.payload,
        attempts: item.attempts + 1,
        createdAt: item.createdAt,
      );

  Future<void> _save(List<OutboxItem> items) async {
    if (items.isEmpty) {
      await store.remove(storageKey);
    } else {
      await store.setString(
          storageKey, jsonEncode(items.map((e) => e.toJson()).toList()));
    }
  }
}
