import 'package:flutter/material.dart';

import '../api/ledger_api.dart';
import '../core/money.dart';
import '../core/uuid.dart';
import '../parse/receipt_parser.dart';
import 'tx_form_page.dart' show kTxTypeExpense, kTxTypeIncome;

/// OCR 候选确认弹窗（WBS 5.3 验收：快捷键出确认弹窗；**绝不自动入账**）。
///
/// 流程：截图 → recognize → parse（端侧离线优先，失败回退服务端 parse，
/// 再失败回退手动）→ 本弹窗展示候选 → 用户选择账户并点击"入账"才写交易。
class TxConfirmDialog extends StatefulWidget {
  final LedgerApi ledger;
  final int bookId;
  final List<dynamic> accounts;
  final List<dynamic> categories;
  final List<ReceiptCandidate> candidates;

  const TxConfirmDialog({
    super.key,
    required this.ledger,
    required this.bookId,
    required this.accounts,
    required this.categories,
    required this.candidates,
  });

  /// 便捷入口：无候选 → 提示走手动（返回 false）。
  static Future<bool> show(
    BuildContext context, {
    required LedgerApi ledger,
    required int bookId,
    required List<dynamic> accounts,
    required List<dynamic> categories,
    required List<ReceiptCandidate> candidates,
  }) async {
    if (candidates.isEmpty) {
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('未识别到交易'),
          content:
              const Text('无法从截图中确认金额/方向/日期，请手动记账。\n（识别失败绝不自动入账）'),
          actions: [
            FilledButton(
                onPressed: () => Navigator.pop(ctx), child: const Text('好的')),
          ],
        ),
      );
      return false;
    }
    final ok = await showDialog<bool>(
      context: context,
      builder: (_) => TxConfirmDialog(
        ledger: ledger,
        bookId: bookId,
        accounts: accounts,
        categories: categories,
        candidates: candidates,
      ),
    );
    return ok ?? false;
  }

  @override
  State<TxConfirmDialog> createState() => _TxConfirmDialogState();
}

class _TxConfirmDialogState extends State<TxConfirmDialog> {
  late final List<bool> _checked;
  int? _accountId;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _checked = List.filled(widget.candidates.length, true);
  }

  static const _dirName = {'income': '收入', 'expense': '支出'};

  Future<void> _confirm() async {
    if (_accountId == null) {
      setState(() => _error = '请选择入账账户');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      for (var i = 0; i < widget.candidates.length; i++) {
        if (!_checked[i]) continue;
        final c = widget.candidates[i];
        final txType = c.direction == 'income'
            ? kTxTypeIncome
            : kTxTypeExpense; // OCR 候选不产生转账
        final dateParts = c.occurredDate.split('-');
        final txTimeUnix = dateParts.length == 3
            ? DateTime(int.parse(dateParts[0]), int.parse(dateParts[1]),
                    int.parse(dateParts[2]))
                .millisecondsSinceEpoch ~/
                1000
            : DateTime.now().millisecondsSinceEpoch ~/ 1000;
        await widget.ledger.createTransaction(
          bookId: widget.bookId,
          clientUuid: uuidV4(),
          txType: txType,
          accountId: _accountId!,
          amountCents: c.amountCents,
          categoryId: _firstCategoryId(txType),
          txTimeUnix: txTimeUnix,
          comment: c.description.isEmpty ? 'OCR 入账' : c.description,
        );
      }
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      setState(() => _error = '入账失败：$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  int? _firstCategoryId(int txType) {
    for (final c in widget.categories) {
      if ((c as Map<String, dynamic>)['catType'] == txType) {
        return c['id'] as int?;
      }
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('确认入账'),
      content: SizedBox(
        width: 420,
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          ...List.generate(widget.candidates.length, (i) {
            final c = widget.candidates[i];
            return CheckboxListTile(
              value: _checked[i],
              controlAffinity: ListTileControlAffinity.leading,
              title: Text(
                  '${_dirName[c.direction] ?? c.direction} ¥${centsToYuan(c.amountCents)}'),
              subtitle: Text(
                  '${c.occurredDate.isEmpty ? '日期未识别' : c.occurredDate} · ${c.description}'),
              onChanged: (v) => setState(() => _checked[i] = v ?? false),
            );
          }),
          const SizedBox(height: 8),
          DropdownButtonFormField<int>(
            initialValue: _accountId,
            decoration:
                const InputDecoration(labelText: '入账账户', border: OutlineInputBorder()),
            items: widget.accounts
                .map((a) => DropdownMenuItem(
                    value: (a as Map<String, dynamic>)['id'] as int?,
                    child: Text('${a['name']}')))
                .toList(),
            onChanged: (v) => setState(() => _accountId = v),
          ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(_error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ),
        ]),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消')),
        FilledButton(
            onPressed: _busy ? null : _confirm,
            child: Text(_busy ? '入账中…' : '入账')),
      ],
    );
  }
}
