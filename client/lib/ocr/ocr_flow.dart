import 'dart:io' show Platform;
import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../api/ledger_api.dart';
import '../hotkey/hotkey_service.dart';
import '../parse/receipt_parser.dart';
import '../ui/tx_confirm_dialog.dart';
import 'ocr_api.dart';

/// 快捷键 → 截图 → OCR → 确认弹窗 的编排（WBS 5.3）。
///
/// 链路与"绝不自动入账"回退契约（OCR 双模式契约）：
/// 1. 端侧离线 parse（零上传）；候选为空时
/// 2. 服务端 recognize（multipart 上传）→ 服务端 parse；候选仍为空或 503/429 时
/// 3. 弹"未识别到交易，请手动记账"提示。
///
/// 任何环节失败都不会写交易；入账只能经 [TxConfirmDialog] 用户点击完成。
Future<void> runOcrFlow(
  BuildContext context, {
  required OcrApi ocr,
  required LedgerApi ledger,
  required int bookId,
  required List<dynamic> accounts,
  required List<dynamic> categories,
  required Future<Uint8List> Function() screenshot,
}) async {
  Uint8List image;
  try {
    image = await screenshot();
  } catch (e) {
    if (!context.mounted) return;
    return _manualFallback(context, '截图失败：$e');
  }
  List<ReceiptCandidate> candidates;
  try {
    final lines = await ocr.recognize(image);
    // 1) 端侧离线优先
    candidates = OcrApi.parseOfflineLines(lines);
    // 2) 端侧无候选 → 服务端规则 parse
    if (candidates.isEmpty) {
      final text = lines.map((l) => '${l['text'] ?? ''}').join('\n');
      final remote = await ocr.parseText(text);
      candidates = remote
          .map((c) => ReceiptCandidate(
                amountCents: (c['amountCents'] as num).toInt(),
                direction: '${c['direction']}',
                occurredDate: '${c['occurredDate'] ?? ''}',
                description: '${c['description'] ?? ''}',
                confidence: (c['confidence'] as num?)?.toDouble() ?? 0,
              ))
          .toList();
    }
  } catch (e) {
    // 503/429/网络异常 → 友好回退手动
    if (!context.mounted) return;
    return _manualFallback(context, '识别失败：$e');
  }
  if (!context.mounted) return;
  // 3) 确认弹窗（空候选 → 手动记账提示；绝不自动入账）
  await TxConfirmDialog.show(
    context,
    ledger: ledger,
    bookId: bookId,
    accounts: accounts,
    categories: categories,
    candidates: candidates,
  );
}

Future<void> _manualFallback(BuildContext context, String message) async {
  if (!context.mounted) return;
  await showDialog<void>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('OCR 未完成'),
      content: Text('$message。\n（识别失败绝不自动入账，请手动记账）'),
      actions: [
        FilledButton(
            onPressed: () => Navigator.pop(ctx), child: const Text('好的')),
      ],
    ),
  );
}

/// 创建 Windows 平台热键服务（非 Windows 返回 null）。
HotkeyService? createWindowsHotkeyService(
    {required void Function() onHotkey}) {
  if (!Platform.isWindows) return null;
  return HotkeyService(onHotkey: onHotkey);
}
