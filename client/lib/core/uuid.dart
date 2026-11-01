import 'dart:math';

final Random _random = Random.secure();

/// RFC 4122 v4 UUID，用于生成交易操作的 clientUuid（幂等键）。
/// 零依赖实现：避免为骨架引入额外 pub 依赖；如后续需要更强保证可替换 package:uuid。
String uuidV4() {
  final bytes = List<int>.generate(16, (_) => _random.nextInt(256));
  bytes[6] = (bytes[6] & 0x0F) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3F) | 0x80; // variant 10
  String hex(int b) => b.toRadixString(16).padLeft(2, '0');
  final h = bytes.map(hex).join();
  return '${h.substring(0, 8)}-${h.substring(8, 12)}-'
      '${h.substring(12, 16)}-${h.substring(16, 20)}-${h.substring(20)}';
}
