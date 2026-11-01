import 'dart:typed_data';

import '../api/api_client.dart';
import '../parse/receipt_parser.dart';

/// OCR 双模式客户端侧（WBS 5.3 / 5.7 契约）。
///
/// - [recognize]：截图字节 → 服务端 POST /api/v1/ocr/recognize（multipart，
///   MIME 由服务端魔数嗅探，客户端头不可信）→ lines；
/// - [parseText]：OCR 文本 → POST /api/v1/ocr/parse → 服务端规则候选（可能为空）；
/// - [parseOfflineLines]：端侧离线规则引擎（receipt_parser，零上传）；
/// - **候选绝不自动入账**——入账确认由 TxConfirmDialog 的用户点击完成；
///   503（服务未配置/下游异常）→ 友好回退手动记账。
class OcrApi {
  final ApiClient client;
  OcrApi(this.client);

  static const _maxImageBytes = 10 * 1024 * 1024;

  Future<List<Map<String, dynamic>>> recognize(Uint8List image) async {
    if (image.isEmpty || image.length > _maxImageBytes) {
      throw ApiException(0, '截图为空或超过 10 MiB');
    }
    final data = await client.postMultipart(
        '/api/v1/ocr/recognize', 'image', image, 'screenshot.png') as Map?;
    final lines = data?['lines'];
    if (lines is! List) return const [];
    return lines
        .whereType<Map>()
        .map((e) => e.cast<String, dynamic>())
        .toList();
  }

  /// 服务端规则解析（文本兜底链路第二步）。
  Future<List<Map<String, dynamic>>> parseText(String text) async {
    final data = await client.post('/api/v1/ocr/parse', {'text': text})
        as Map<String, dynamic>?;
    final candidates = data?['candidates'];
    if (candidates is! List) return const [];
    return candidates
        .whereType<Map>()
        .map((e) => e.cast<String, dynamic>())
        .toList();
  }

  /// 截图 OCR 行 → 端侧规则引擎（离线，零上传）→ 候选。
  static List<ReceiptCandidate> parseOfflineLines(
      List<Map<String, dynamic>> lines) {
    final text = lines.map((l) => '${l['text'] ?? ''}').join('\n');
    return parseReceiptText(text);
  }
}
