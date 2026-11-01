import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';

/// 分类管理（浏览/新增/删除，WBS 5.2）。
class CategoriesPage extends StatefulWidget {
  final LedgerApi ledger;
  final int bookId;
  const CategoriesPage({super.key, required this.ledger, required this.bookId});

  @override
  State<CategoriesPage> createState() => _CategoriesPageState();
}

class _CategoriesPageState extends State<CategoriesPage> {
  List<dynamic>? _categories;
  String? _error;

  static const _catTypeNames = {1: '收入', 2: '支出'};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await widget.ledger.categories(widget.bookId);
      if (mounted) setState(() => _categories = list);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    }
  }

  Future<void> _create() async {
    final controller = TextEditingController();
    var catType = 2;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDlg) => AlertDialog(
          title: const Text('新增分类'),
          content: Column(mainAxisSize: MainAxisSize.min, children: [
            TextField(
              controller: controller,
              decoration: const InputDecoration(labelText: '分类名'),
            ),
            const SizedBox(height: 8),
            DropdownButtonFormField<int>(
              initialValue: catType,
              decoration: const InputDecoration(labelText: '收支类型'),
              items: _catTypeNames.entries
                  .map((e) => DropdownMenuItem(
                      value: e.key, child: Text(e.value)))
                  .toList(),
              onChanged: (v) => setDlg(() => catType = v ?? 2),
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
      await widget.ledger.createCategory(
          bookId: widget.bookId, name: name, catType: catType);
      await _load();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Future<void> _delete(Map<String, dynamic> category) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除分类'),
        content: Text('确定删除「${category['name']}」？'),
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
      await widget.ledger.deleteCategory(
          widget.bookId, (category['id'] as num).toInt());
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
    final categories = _categories;
    return Scaffold(
      appBar: AppBar(title: const Text('分类管理'), actions: [
        IconButton(icon: const Icon(Icons.add), onPressed: _create),
      ]),
      body: _error != null
          ? Center(child: Text(_error!))
          : categories == null
              ? const Center(child: CircularProgressIndicator())
              : categories.isEmpty
                  ? const Center(child: Text('暂无分类'))
                  : ListView.builder(
                      itemCount: categories.length,
                      itemBuilder: (context, i) {
                        final c = categories[i] as Map<String, dynamic>;
                        return ListTile(
                          title: Text('${c['name']}'),
                          subtitle: Text(_catTypeNames[
                                      (c['catType'] as num?)?.toInt() ?? 0] ??
                                  '${c['catType']}'),
                          trailing: IconButton(
                            icon: const Icon(Icons.delete_outline, size: 20),
                            onPressed: () => _delete(c),
                          ),
                        );
                      },
                    ),
    );
  }
}
