import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';

/// 登录页（WBS 5.1 验收：能登录）。
class LoginPage extends StatefulWidget {
  final AuthApi auth;
  final VoidCallback onLoggedIn;
  const LoginPage({super.key, required this.auth, required this.onLoggedIn});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  bool _busy = false;
  String? _error;

  Future<void> _submit() async {
    final u = _username.text.trim();
    final p = _password.text;
    if (u.isEmpty || p.isEmpty) {
      setState(() => _error = '请输入用户名与密码');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.auth.login(u, p);
      widget.onLoggedIn();
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = '登录失败，请稍后重试');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('FastLedger 登录')),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 360),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: _username,
                  decoration: const InputDecoration(
                      labelText: '用户名', border: OutlineInputBorder()),
                  autofillHints: const [AutofillHints.username],
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _password,
                  obscureText: true,
                  decoration: const InputDecoration(
                      labelText: '密码', border: OutlineInputBorder()),
                  autofillHints: const [AutofillHints.password],
                  onSubmitted: (_) => _submit(),
                ),
                const SizedBox(height: 16),
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: Text(_error!,
                        style: TextStyle(
                            color: Theme.of(context).colorScheme.error)),
                  ),
                FilledButton(
                  onPressed: _busy ? null : _submit,
                  child: _busy
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('登录'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
