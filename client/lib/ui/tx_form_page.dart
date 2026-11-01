import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';
import '../core/money.dart';
import '../core/uuid.dart';

String _fmtDate(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

const kTxTypeExpense = 2;
const kTxTypeIncome = 1;
const kTxTypeTransfer = 3;

/// 记一笔 / 编辑交易表单（WBS 5.2）。
///
/// 编辑模式（tx != null）：类型只读（服务端契约：类型不可改，400 引导删除重建），
/// 提交携带 baseVersion 乐观锁；409 时提示刷新重试。
class TxFormPage extends StatefulWidget {
  final LedgerApi ledger;
  final int bookId;
  final List<dynamic> accounts;
  final List<dynamic> categories;
  final Map<String, dynamic>? tx; // 编辑模式传入

  const TxFormPage({
    super.key,
    required this.ledger,
    required this.bookId,
    required this.accounts,
    required this.categories,
    this.tx,
  });

  @override
  State<TxFormPage> createState() => _TxFormPageState();
}

class _TxFormPageState extends State<TxFormPage> {
  late int _txType;
  late final TextEditingController _amount;
  late final TextEditingController _comment;
  int? _accountId;
  int? _toAccountId;
  int? _categoryId;
  late DateTime _date;
  bool _busy = false;
  String? _error;

  bool get _isEdit => widget.tx != null;

  @override
  void initState() {
    super.initState();
    final tx = widget.tx;
    _txType = tx == null ? kTxTypeExpense : (tx['txType'] as num).toInt();
    _amount = TextEditingController(
        text: tx == null ? '' : centsToYuan((tx['amount'] as num).toInt()));
    _comment = TextEditingController(text: (tx?['comment'] ?? '') as String);
    _date = tx == null
        ? DateTime.now()
        : DateTime.fromMillisecondsSinceEpoch(
            (tx['txTime'] as num).toInt() * 1000);
    if (tx != null) {
      _accountId = (tx['accountId'] as num?)?.toInt();
      _toAccountId = (tx['toAccountId'] as num?)?.toInt();
      _categoryId = (tx['categoryId'] as num?)?.toInt();
    }
  }

  @override
  void dispose() {
    _amount.dispose();
    _comment.dispose();
    super.dispose();
  }

  List<dynamic> get _typeCategories => widget.categories
      .where((c) => (c as Map<String, dynamic>)['catType'] == _txType)
      .toList();

  Future<void> _pickDate() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _date,
      firstDate: DateTime(2000),
      lastDate: DateTime(2100),
    );
    if (picked != null) setState(() => _date = picked);
  }

  Future<void> _submit() async {
    final cents = parseYuanToCents(_amount.text);
    if (cents == null) {
      setState(() => _error = '金额非法（正数，最多两位小数）');
      return;
    }
    if (_accountId == null) {
      setState(() => _error = '请选择账户');
      return;
    }
    if (_txType == kTxTypeTransfer && _toAccountId == null) {
      setState(() => _error = '请选择转入账户');
      return;
    }
    if (_txType != kTxTypeTransfer && _categoryId == null) {
      setState(() => _error = '请选择分类');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    final txTimeUnix = DateTime(_date.year, _date.month, _date.day)
        .millisecondsSinceEpoch ~/
        1000;
    try {
      if (_isEdit) {
        final tx = widget.tx!;
        await widget.ledger.updateTransaction(
          bookId: widget.bookId,
          txId: (tx['id'] as num).toInt(),
          clientUuid: uuidV4(),
          txType: _txType,
          accountId: _accountId!,
          amountCents: cents,
          categoryId: _categoryId,
          toAccountId: _toAccountId,
          txTimeUnix: txTimeUnix,
          comment: _comment.text.trim(),
          baseVersion: (tx['syncVersion'] as num?)?.toInt() ?? 0,
        );
      } else {
        await widget.ledger.createTransaction(
          bookId: widget.bookId,
          clientUuid: uuidV4(),
          txType: _txType,
          accountId: _accountId!,
          amountCents: cents,
          categoryId: _categoryId,
          toAccountId: _toAccountId,
          txTimeUnix: txTimeUnix,
          comment: _comment.text.trim(),
        );
      }
      if (mounted) Navigator.of(context).pop(true);
    } on ApiException catch (e) {
      setState(() => _error = e.status == 409
          ? '交易已被他人修改（版本冲突），请返回刷新后重试'
          : e.message);
    } catch (_) {
      setState(() => _error = '保存失败，请稍后重试');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(_isEdit ? '编辑交易' : '记一笔')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          SegmentedButton<int>(
            segments: const [
              ButtonSegment(value: kTxTypeExpense, label: Text('支出')),
              ButtonSegment(value: kTxTypeIncome, label: Text('收入')),
              ButtonSegment(value: kTxTypeTransfer, label: Text('转账')),
            ],
            selected: {_txType},
            onSelectionChanged: _isEdit
                ? null // 类型不可改（服务端契约）
                : (s) => setState(() {
                      _txType = s.first;
                      _categoryId = null;
                    }),
          ),
          if (_isEdit)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Text('类型不可修改（如需变更请删除后重记）',
                  style: Theme.of(context).textTheme.bodySmall),
            ),
          const SizedBox(height: 12),
          TextField(
            controller: _amount,
            keyboardType:
                const TextInputType.numberWithOptions(decimal: true),
            decoration: const InputDecoration(
                labelText: '金额（元）', border: OutlineInputBorder()),
          ),
          const SizedBox(height: 12),
          DropdownButtonFormField<int>(
            initialValue: _accountId,
            decoration:
                const InputDecoration(labelText: '账户', border: OutlineInputBorder()),
            items: widget.accounts
                .map((a) => DropdownMenuItem(
                    value: (a as Map<String, dynamic>)['id'] as int?,
                    child: Text('${a['name']}')))
                .toList(),
            onChanged: (v) => setState(() => _accountId = v),
          ),
          if (_txType == kTxTypeTransfer) ...[
            const SizedBox(height: 12),
            DropdownButtonFormField<int>(
              initialValue: _toAccountId,
              decoration: const InputDecoration(
                  labelText: '转入账户', border: OutlineInputBorder()),
              items: widget.accounts
                  .map((a) => DropdownMenuItem(
                      value: (a as Map<String, dynamic>)['id'] as int?,
                      child: Text('${a['name']}')))
                  .toList(),
              onChanged: (v) => setState(() => _toAccountId = v),
            ),
          ] else ...[
            const SizedBox(height: 12),
            DropdownButtonFormField<int>(
              initialValue: _categoryId,
              decoration:
                  const InputDecoration(labelText: '分类', border: OutlineInputBorder()),
              items: _typeCategories
                  .map((c) => DropdownMenuItem(
                      value: (c as Map<String, dynamic>)['id'] as int?,
                      child: Text('${c['name']}')))
                  .toList(),
              onChanged: (v) => setState(() => _categoryId = v),
            ),
          ],
          const SizedBox(height: 12),
          Row(children: [
            Expanded(child: Text('日期：${_fmtDate(_date)}')),
            TextButton(onPressed: _pickDate, child: const Text('选择日期')),
          ]),
          TextField(
            controller: _comment,
            maxLength: 255,
            decoration: const InputDecoration(
                labelText: '备注（可选）', border: OutlineInputBorder()),
          ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text(_error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error)),
            ),
          const SizedBox(height: 8),
          FilledButton(
            onPressed: _busy ? null : _submit,
            child: Text(_busy ? '保存中…' : '保存'),
          ),
        ],
      ),
    );
  }
}
