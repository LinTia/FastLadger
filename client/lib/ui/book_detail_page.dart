import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';
import '../core/money.dart';
import '../core/uuid.dart';
import '../hotkey/hotkey_service.dart';
import '../ocr/ocr_api.dart';
import 'members_page.dart';
import '../ocr/ocr_flow.dart';
import 'accounts_page.dart';
import 'categories_page.dart';
import 'tx_form_page.dart';

const _txTypeName = {1: '收入', 2: '支出', 3: '转账'};

/// 账本详情：交易列表 + 记一笔/编辑/删除 + 账户/分类管理入口（WBS 5.2）。
class BookDetailPage extends StatefulWidget {
  final LedgerApi ledger;
  final int bookId;
  final String bookName;
  const BookDetailPage({
    super.key,
    required this.ledger,
    required this.bookId,
    required this.bookName,
  });

  @override
  State<BookDetailPage> createState() => _BookDetailPageState();
}

class _BookDetailPageState extends State<BookDetailPage> {
  List<dynamic>? _txs;
  List<dynamic> _accounts = const [];
  List<dynamic> _categories = const [];
  String? _error;
  HotkeyService? _hotkeys;

  @override
  void initState() {
    super.initState();
    _load();
    _registerHotkey();
  }

  /// WBS 5.3：Windows 全局热键（默认 Ctrl+Alt+F）→ 截图 → OCR → 确认弹窗。
  /// OCR 与账本共用同一 ApiClient（token 一致）。
  Future<void> _registerHotkey() async {
    final ocr = OcrApi(widget.ledger.client);
    final hk = createWindowsHotkeyService(
        onHotkey: () => runOcrFlow(
              context,
              ocr: ocr,
              ledger: widget.ledger,
              bookId: widget.bookId,
              accounts: _accounts,
              categories: _categories,
              screenshot: () => _hotkeys!.screenshot(),
            ));
    if (hk == null) return;
    final ok = await hk.register();
    if (ok && mounted) setState(() => _hotkeys = hk);
  }

  @override
  void dispose() {
    _hotkeys?.unregister();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final results = await Future.wait([
        widget.ledger.transactions(widget.bookId),
        widget.ledger.accounts(widget.bookId),
        widget.ledger.categories(widget.bookId),
      ]);
      if (mounted) {
        setState(() {
          _txs = results[0];
          _accounts = results[1];
          _categories = results[2];
          _error = null;
        });
      }
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = '加载失败，请下拉重试');
    }
  }

  String _accountName(int? id) {
    for (final a in _accounts) {
      if ((a as Map<String, dynamic>)['id'] == id) return '${a['name']}';
    }
    return '#$id';
  }

  String _categoryName(int? id) {
    for (final c in _categories) {
      if ((c as Map<String, dynamic>)['id'] == id) return '${c['name']}';
    }
    return '—';
  }

  Future<void> _openForm([Map<String, dynamic>? tx]) async {
    final saved = await Navigator.of(context).push<bool>(MaterialPageRoute(
      builder: (_) => TxFormPage(
        ledger: widget.ledger,
        bookId: widget.bookId,
        accounts: _accounts,
        categories: _categories,
        tx: tx,
      ),
    ));
    if (saved == true) _load();
  }

  Future<void> _deleteTx(Map<String, dynamic> tx) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除交易'),
        content: const Text('整笔删除（转账含对侧），余额自动回滚。确定？'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('删除')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await widget.ledger.deleteTransaction(
          widget.bookId, (tx['id'] as num).toInt(), uuidV4());
      await _load();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.bookName),
        actions: [
          PopupMenuButton<String>(
            onSelected: (key) async {
              if (key == 'members') {
                await Navigator.of(context).push(MaterialPageRoute(
                  builder: (_) => MembersPage(
                    ledger: widget.ledger,
                    bookId: widget.bookId,
                    bookName: widget.bookName,
                  ),
                ));
                return;
              }
              await Navigator.of(context).push(MaterialPageRoute(
                builder: (_) => key == 'accounts'
                    ? AccountsPage(ledger: widget.ledger, bookId: widget.bookId)
                    : CategoriesPage(
                        ledger: widget.ledger, bookId: widget.bookId),
              ));
              _load();
            },
            itemBuilder: (_) => const [
              PopupMenuItem(value: 'accounts', child: Text('账户管理')),
              PopupMenuItem(value: 'categories', child: Text('分类管理')),
              PopupMenuItem(value: 'members', child: Text('家庭成员')),
            ],
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _openForm(),
        child: const Icon(Icons.add),
      ),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _buildList(),
      ),
    );
  }

  Widget _buildList() {
    if (_error != null) {
      return ListView(children: [
        Padding(
            padding: const EdgeInsets.all(24),
            child: Center(child: Text(_error!))),
      ]);
    }
    final txs = _txs;
    if (txs == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (txs.isEmpty) {
      return ListView(children: const [
        Padding(
            padding: EdgeInsets.all(24),
            child: Center(child: Text('还没有交易，点右下角 + 记一笔'))),
      ]);
    }
    return ListView.separated(
      itemCount: txs.length,
      separatorBuilder: (_, _) => const Divider(height: 1),
      itemBuilder: (context, i) {
        final tx = txs[i] as Map<String, dynamic>;
        final type = (tx['txType'] as num?)?.toInt() ?? 0;
        final cents = (tx['amount'] as num?)?.toInt() ?? 0;
        final sign = type == 1 ? '+' : type == 2 ? '-' : '';
        final subtitle = type == 3
            ? '${_accountName((tx['accountId'] as num?)?.toInt())} → '
                '${_accountName((tx['toAccountId'] as num?)?.toInt())}'
            : '${_accountName((tx['accountId'] as num?)?.toInt())}'
                ' · ${_categoryName((tx['categoryId'] as num?)?.toInt())}';
        final comment = (tx['comment'] ?? '') as String;
        return ListTile(
          title: Text('$sign¥${centsToYuan(cents)}'
              '${comment.isEmpty ? '' : '  $comment'}'),
          subtitle: Text('${_txTypeName[type] ?? type} · $subtitle'),
          trailing: Row(mainAxisSize: MainAxisSize.min, children: [
            IconButton(
                icon: const Icon(Icons.edit, size: 20),
                onPressed: () => _openForm(tx)),
            IconButton(
                icon: const Icon(Icons.delete_outline, size: 20),
                onPressed: () => _deleteTx(tx)),
          ]),
        );
      },
    );
  }
}
