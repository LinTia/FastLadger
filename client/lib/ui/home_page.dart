import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';
import 'book_detail_page.dart';

/// 首页：账本列表（骨架期最小可用；CRUD 页面在 5.2 补齐）。
class HomePage extends StatefulWidget {
  final LedgerApi ledger;
  const HomePage({super.key, required this.ledger});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  List<dynamic>? _books;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final books = await widget.ledger.books();
      if (mounted) setState(() => _books = books);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = '加载失败，请下拉重试');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('FastLedger')),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _buildBody(),
      ),
    );
  }

  Widget _buildBody() {
    if (_error != null) {
      return ListView(children: [
        Padding(
            padding: const EdgeInsets.all(24),
            child: Center(child: Text(_error!))),
      ]);
    }
    final books = _books;
    if (books == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (books.isEmpty) {
      return ListView(children: const [
        Padding(
            padding: EdgeInsets.all(24),
            child: Center(child: Text('还没有账本，去 Web 端或服务端创建一个吧'))),
      ]);
    }
    return ListView.builder(
      itemCount: books.length,
      itemBuilder: (context, i) {
        final book = books[i] as Map<String, dynamic>;
        return ListTile(
          title: Text('${book['name'] ?? ''}'),
          subtitle: Text('ID ${book['id'] ?? ''} · ${book['bookType'] ?? ''}'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () {
            Navigator.of(context).push(MaterialPageRoute(
              builder: (_) => BookDetailPage(
                ledger: widget.ledger,
                bookId: (book['id'] as num).toInt(),
                bookName: '${book['name'] ?? ''}',
              ),
            ));
          },
        );
      },
    );
  }
}
