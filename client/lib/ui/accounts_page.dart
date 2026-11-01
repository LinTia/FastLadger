import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';

/// 账户管理（浏览/新增/删除，WBS 5.2）。
class AccountsPage extends StatefulWidget {
  final LedgerApi ledger;
  final int bookId;
  const AccountsPage({super.key, required this.ledger, required this.bookId});

  @override
  State<AccountsPage> createState() => _AccountsPageState();
}

class _AccountsPageState extends State<AccountsPage> {
  List<dynamic>? _accounts;
  String? _error;

  static const _categoryNames = {1: '现金', 2: '活期储蓄', 4: '信用卡', 5: '消费贷'};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await widget.ledger.accounts(widget.bookId);
      if (mounted) setState(() => _accounts = list);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    }
  }

  Future<void> _create() async {
    final controller = TextEditingController();
    var category = 1;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDlg) => AlertDialog(
          title: const Text('新增账户'),
          content: Column(mainAxisSize: MainAxisSize.min, children: [
            TextField(
              controller: controller,
              decoration: const InputDecoration(labelText: '账户名'),
            ),
            const SizedBox(height: 8),
            DropdownButtonFormField<int>(
              initialValue: category,
              decoration: const InputDecoration(labelText: '类型'),
              items: _categoryNames.entries
                  .map((e) => DropdownMenuItem(
                      value: e.key, child: Text(e.value)))
                  .toList(),
              onChanged: (v) => setDlg(() => category = v ?? 1),
            ),
          ]),
          actions: [
            TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('取消')),
            FilledButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text('创建')),
          ],
        ),
      ),
    );
    final name = controller.text.trim();
    if (ok != true || name.isEmpty) return;
    try {
      await widget.ledger.createAccount(
          bookId: widget.bookId, name: name, category: category);
      await _load();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Future<void> _delete(Map<String, dynamic> account) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除账户'),
        content: Text('确定删除「${account['name']}」？'),
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
    if (ok != true) return;
    try {
      await widget.ledger.deleteAccount(
          widget.bookId, (account['id'] as num).toInt());
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
    final accounts = _accounts;
    return Scaffold(
      appBar: AppBar(title: const Text('账户管理'), actions: [
        IconButton(icon: const Icon(Icons.add), onPressed: _create),
      ]),
      body: _error != null
          ? Center(child: Text(_error!))
          : accounts == null
              ? const Center(child: CircularProgressIndicator())
              : accounts.isEmpty
                  ? const Center(child: Text('暂无账户'))
                  : ListView.builder(
                      itemCount: accounts.length,
                      itemBuilder: (context, i) {
                        final a = accounts[i] as Map<String, dynamic>;
                        final cents = (a['balance'] as num?)?.toInt() ?? 0;
                        return ListTile(
                          title: Text('${a['name']}'),
                          subtitle: Text(
                              '${_categoryNames[(a['category'] as num?)?.toInt() ?? 0] ?? a['category']}'
                              ' · ${a['currency']}'),
                          trailing: Text('¥'
                              '${(cents ~/ 100).toString()}'
                              '.${(cents % 100).abs().toString().padLeft(2, '0')}'),
                          onLongPress: () => _delete(a),
                        );
                      },
                    ),
    );
  }
}
