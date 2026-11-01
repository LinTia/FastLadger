import 'package:flutter/material.dart';

import 'api/api_client.dart';
import 'api/ledger_api.dart';
import 'core/token_store.dart';
import 'ui/home_page.dart';
import 'ui/login_page.dart';

/// FastLedger 客户端（WBS 5.1 工程骨架）。
///
/// 依赖注入方式组装：ApiClient → AuthApi/LedgerApi → 页面；
/// TokenStore 抽象使测试可注入内存实现，生产接 secure storage。
void main() => runApp(const FastLedgerApp());

class FastLedgerApp extends StatelessWidget {
  const FastLedgerApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'FastLedger',
      theme: ThemeData(colorSchemeSeed: Colors.teal, useMaterial3: true),
      home: const RootGate(),
    );
  }
}

/// 根路由：持有会话状态（有 token → 首页；无 token/401 → 登录页）。
class RootGate extends StatefulWidget {
  const RootGate({super.key});

  @override
  State<RootGate> createState() => _RootGateState();
}

class _RootGateState extends State<RootGate> {
  late final InMemoryTokenStore _tokens;
  late final ApiClient _client;
  late final AuthApi _auth;
  late final LedgerApi _ledger;
  bool _loggedIn = false;

  @override
  void initState() {
    super.initState();
    _tokens = InMemoryTokenStore();
    _client = ApiClient(
      // 生产可通过 --dart-define=FASTLEDGER_BASE_URL=... 覆盖
      baseUrl: const String.fromEnvironment(
        'FASTLEDGER_BASE_URL',
        defaultValue: 'http://127.0.0.1:8080',
      ),
      tokens: _tokens,
    );
    _client.onUnauthorized = () {
      if (mounted) setState(() => _loggedIn = false);
    };
    _auth = AuthApi(_client);
    _ledger = LedgerApi(_client);
  }

  @override
  Widget build(BuildContext context) {
    return _loggedIn
        ? HomePage(ledger: _ledger)
        : LoginPage(auth: _auth, onLoggedIn: () => setState(() => _loggedIn = true));
  }
}
