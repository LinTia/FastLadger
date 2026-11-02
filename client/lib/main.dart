import 'package:flutter/material.dart';

import 'api/api_client.dart';
import 'api/ledger_api.dart';
import 'core/app_settings.dart';
import 'core/local_store.dart';
import 'core/token_store.dart';
import 'ui/home_page.dart';
import 'ui/login_page.dart';

/// FastLedger 客户端（WBS 5.1 工程骨架）。
///
/// 依赖注入方式组装：ApiClient → AuthApi/LedgerApi → 页面；
/// TokenStore 抽象使测试可注入内存实现，生产接 secure storage。
/// 全局设置（主题/通知）经 [AppSettings]（ChangeNotifier）上根生效。
void main() => runApp(const FastLedgerApp());

class FastLedgerApp extends StatefulWidget {
  const FastLedgerApp({super.key});

  @override
  State<FastLedgerApp> createState() => _FastLedgerAppState();
}

class _FastLedgerAppState extends State<FastLedgerApp> {
  late final AppSettings _settings;

  @override
  void initState() {
    super.initState();
    _settings = AppSettings(store: InMemoryLocalStore());
    _settings.load();
  }

  @override
  void dispose() {
    _settings.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: _settings,
      builder: (context, _) => MaterialApp(
        title: 'FastLedger',
        theme: ThemeData(colorSchemeSeed: Colors.teal, useMaterial3: true),
        darkTheme: ThemeData(
            colorSchemeSeed: Colors.teal,
            brightness: Brightness.dark,
            useMaterial3: true),
        themeMode: _settings.themeMode,
        home: RootGate(settings: _settings),
      ),
    );
  }
}

/// 根路由：持有会话状态（有 token → 首页；无 token/401 → 登录页）。
class RootGate extends StatefulWidget {
  final AppSettings settings;
  const RootGate({super.key, required this.settings});

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
        ? HomePage(
            ledger: _ledger,
            auth: _auth,
            settings: widget.settings,
            onLogout: () => setState(() => _loggedIn = false),
          )
        : LoginPage(auth: _auth, onLoggedIn: () => setState(() => _loggedIn = true));
  }
}
