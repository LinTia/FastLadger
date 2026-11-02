import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';
import '../core/app_settings.dart';

/// 设置页（PRD §4.12「我的 / 设置」）。
///
/// 内容：个人资料（用户名只读 / 邮箱编辑）、账号安全（修改密码——成功后
/// 服务端撤销全部会话并回登录页）、通知开关与主题切换（本地偏好，经
/// [AppSettings] 持久化）、退出登录。
///
/// 状态管理：页面内 StatefulWidget + setState（项目既有风格）；主题/通知
/// 经构造器注入的 [AppSettings]（ChangeNotifier）上抛到根应用生效。
class SettingsPage extends StatefulWidget {
  final AuthApi auth;
  final AppSettings settings;
  final VoidCallback onLogout;

  const SettingsPage({
    super.key,
    required this.auth,
    required this.settings,
    required this.onLogout,
  });

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  Map<String, dynamic>? _me;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
    _loadPats();
  }

  Future<void> _load() async {
    try {
      final me = await widget.auth.me();
      if (mounted) setState(() => _me = me);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = '加载失败，请下拉重试');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('设置')),
      body: _error != null
          ? ListView(children: [
              Padding(
                  padding: const EdgeInsets.all(24),
                  child: Center(child: Text(_error!))),
            ])
          : ListView(
              padding: const EdgeInsets.symmetric(vertical: 8),
              children: [
                _sectionTitle('个人资料'),
                _profileCard(),
                _sectionTitle('账号安全'),
                _securityCard(),
                _sectionTitle('长期 API Token'),
                _patCard(),
                _sectionTitle('偏好'),
                _preferenceCard(),
                _sectionTitle(''),
                _logoutCard(),
              ],
            ),
    );
  }

  Widget _sectionTitle(String text) => text.isEmpty
      ? const SizedBox(height: 8)
      : Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
          child: Text(text,
              style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: Theme.of(context).colorScheme.primary)),
        );

  Widget _profileCard() {
    final me = _me;
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12),
      child: Column(children: [
        ListTile(
          leading: const Icon(Icons.person),
          title: const Text('用户名'),
          subtitle: Text(me?['username'] as String? ?? '…'),
          trailing: Tooltip(
            message: '用户名是登录标识，不可修改',
            child: Icon(Icons.lock_outline,
                size: 18, color: Theme.of(context).colorScheme.outline),
          ),
        ),
        ListTile(
          leading: const Icon(Icons.email_outlined),
          title: const Text('邮箱'),
          subtitle: Text(
              (me?['email'] as String?)?.isEmpty ?? true ? '未设置' : me!['email'] as String),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => _editEmail(),
        ),
      ]),
    );
  }

  Future<void> _editEmail() async {
    final ctrl = TextEditingController(text: _me?['email'] as String? ?? '');
    final saved = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('修改邮箱'),
        content: TextField(
          controller: ctrl,
          keyboardType: TextInputType.emailAddress,
          decoration: const InputDecoration(
              labelText: '邮箱', hintText: 'me@example.com'),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true), child: const Text('保存')),
        ],
      ),
    );
    if (saved != true) return;
    try {
      final me = await widget.auth.updateEmail(ctrl.text.trim());
      if (mounted) {
        setState(() => _me = me);
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('邮箱已更新')));
      }
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Widget _securityCard() => Card(
        margin: const EdgeInsets.symmetric(horizontal: 12),
        child: ListTile(
          leading: const Icon(Icons.password),
          title: const Text('修改密码'),
          subtitle: const Text('修改后所有设备将下线，需要重新登录'),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => _changePassword(),
        ),
      );

  Future<void> _changePassword() async {
    final oldCtrl = TextEditingController();
    final newCtrl = TextEditingController();
    final confirmCtrl = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('修改密码'),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(
            controller: oldCtrl,
            obscureText: true,
            decoration: const InputDecoration(labelText: '当前密码'),
          ),
          TextField(
            controller: newCtrl,
            obscureText: true,
            decoration: const InputDecoration(labelText: '新密码（至少 8 位）'),
          ),
          TextField(
            controller: confirmCtrl,
            obscureText: true,
            decoration: const InputDecoration(labelText: '确认新密码'),
          ),
        ]),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true), child: const Text('修改')),
        ],
      ),
    );
    if (ok != true) return;
    if (newCtrl.text != confirmCtrl.text) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('两次输入的新密码不一致')));
      }
      return;
    }
    try {
      await widget.auth.changePassword(oldCtrl.text, newCtrl.text);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('密码已修改，所有设备已下线，请重新登录')));
      // 改密撤销了当前会话：本地令牌已失效，回登录页
      widget.onLogout();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  // ---- 长期 API Token（WBS 6.6；交互对照 Web 端设置页 [1.53]）----
  //
  // 安全口径：明文**仅生成响应出现一次**，本页展示后不落任何存储（含
  // secure storage——长期凭证只存服务端哈希）；撤销立即生效。

  List<dynamic>? _pats;
  bool _patLoading = false;

  Future<void> _loadPats() async {
    if (mounted) setState(() => _patLoading = true);
    try {
      final list = await widget.auth.listTokens();
      if (mounted) setState(() => _pats = list);
    } on ApiException {
      // PAT 列表失败不阻塞设置页其余部分（如服务端版本较旧未启用 6.6）
      if (mounted) setState(() => _pats = const []);
    } finally {
      if (mounted) setState(() => _patLoading = false);
    }
  }

  Widget _patCard() => Card(
        margin: const EdgeInsets.symmetric(horizontal: 12),
        child: Column(children: [
          ListTile(
            leading: const Icon(Icons.vpn_key_outlined),
            title: const Text('长期 API Token'),
            subtitle: const Text('开发/API 用——此 Token 拥有你的账户完全访问权限，请勿泄露'),
            trailing: IconButton(
              icon: const Icon(Icons.add),
              tooltip: '生成新 Token',
              onPressed: () => _createPat(),
            ),
          ),
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 16),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Text('明文只在生成时显示一次，关闭后无法再查看。',
                  style: TextStyle(fontSize: 12)),
            ),
          ),
          _patList(),
        ]),
      );

  Widget _patList() {
    if (_patLoading && _pats == null) {
      return const Padding(
          padding: EdgeInsets.all(16),
          child: Center(child: CircularProgressIndicator()));
    }
    final pats = _pats;
    if (pats == null || pats.isEmpty) {
      return const Padding(
        padding: EdgeInsets.all(16),
        child: Align(
            alignment: Alignment.centerLeft,
            child: Text('还没有长期 Token。', style: TextStyle(fontSize: 13))),
      );
    }
    return Column(
      children: [
        for (final it in pats.cast<Map<String, dynamic>>())
          ListTile(
            dense: true,
            title: Text(it['name'] as String? ?? '—'),
            subtitle: Text(
              '创建 ${_fmtTime(it['createdAt'] as num?)} · '
              '最近使用 ${it['lastUsedAt'] == 0 ? '从未' : _fmtTime(it['lastUsedAt'] as num?)}',
              style: const TextStyle(fontSize: 12),
            ),
            trailing: (it['revokedAt'] as num? ?? 0) > 0
                ? const Text('已撤销',
                    style: TextStyle(fontSize: 12))
                : TextButton(
                    child: const Text('撤销'),
                    onPressed: () =>
                        _revokePat((it['id'] as num).toInt()),
                  ),
          ),
      ],
    );
  }

  String _fmtTime(num? unixSec) {
    if (unixSec == null || unixSec == 0) return '—';
    final d = DateTime.fromMillisecondsSinceEpoch(unixSec.toInt() * 1000);
    String p(int n) => n.toString().padLeft(2, '0');
    return '${d.year}-${p(d.month)}-${p(d.day)} ${p(d.hour)}:${p(d.minute)}';
  }

  Future<void> _createPat() async {
    final nameCtrl = TextEditingController();
    final go = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('生成长期 API Token'),
        content: TextField(
          controller: nameCtrl,
          decoration: const InputDecoration(
              labelText: '用途备注', hintText: '如：快捷指令'),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true), child: const Text('生成')),
        ],
      ),
    );
    if (go != true) return;
    try {
      final data = await widget.auth.createToken(nameCtrl.text.trim());
      if (!mounted) return;
      final plaintext = data['token'] as String? ?? '';
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('请立即复制保存'),
          content: Column(mainAxisSize: MainAxisSize.min, children: [
            SelectableText(plaintext,
                style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600)),
            const SizedBox(height: 8),
            const Text('仅此一次展示，关闭后无法再查看。',
                style: TextStyle(fontSize: 12, color: Colors.redAccent)),
          ]),
          actions: [
            TextButton(
              onPressed: () async {
                await Clipboard.setData(ClipboardData(text: plaintext));
                if (ctx.mounted) {
                  ScaffoldMessenger.of(ctx).showSnackBar(
                      const SnackBar(content: Text('Token 已复制')));
                }
              },
              child: const Text('复制'),
            ),
            FilledButton(
                onPressed: () => Navigator.pop(ctx), child: const Text('完成')),
          ],
        ),
      );
      _loadPats();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Future<void> _revokePat(int id) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('撤销 Token'),
        content: const Text('撤销立即生效，使用该 Token 的客户端/快捷指令将无法再访问。此操作不可恢复。'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              style: FilledButton.styleFrom(
                  backgroundColor: Theme.of(ctx).colorScheme.error),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('撤销')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.auth.revokeToken(id);
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('Token 已撤销')));
      }
      _loadPats();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Widget _preferenceCard() => Card(
        margin: const EdgeInsets.symmetric(horizontal: 12),
        child: Column(children: [
          SwitchListTile(
            secondary: const Icon(Icons.notifications_outlined),
            title: const Text('本地通知'),
            subtitle: const Text('客户端内提醒（推送通道为 v0.3 规划，当前为本地偏好）'),
            value: widget.settings.notifyEnabled,
            onChanged: (v) => widget.settings.setNotifyEnabled(v),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 4, 16, 12),
            child: Row(children: [
              const Icon(Icons.palette_outlined, size: 22),
              const SizedBox(width: 16),
              const Expanded(child: Text('主题')),
              SegmentedButton<ThemeMode>(
                segments: const [
                  ButtonSegment(value: ThemeMode.system, label: Text('系统')),
                  ButtonSegment(value: ThemeMode.light, label: Text('浅色')),
                  ButtonSegment(value: ThemeMode.dark, label: Text('深色')),
                ],
                selected: {widget.settings.themeMode},
                showSelectedIcon: false,
                onSelectionChanged: (sel) =>
                    widget.settings.setThemeMode(sel.first),
              ),
            ]),
          ),
        ]),
      );

  Widget _logoutCard() => Card(
        margin: const EdgeInsets.symmetric(horizontal: 12),
        child: ListTile(
          leading: const Icon(Icons.logout, color: Colors.redAccent),
          title: const Text('退出登录',
              style: TextStyle(color: Colors.redAccent)),
          subtitle: const Text('本地数据会保留'),
          onTap: () async {
            final ok = await showDialog<bool>(
              context: context,
              builder: (ctx) => AlertDialog(
                title: const Text('退出登录'),
                content: const Text('退出后需要重新登录。本地数据会保留。'),
                actions: [
                  TextButton(
                      onPressed: () => Navigator.pop(ctx, false),
                      child: const Text('取消')),
                  FilledButton(
                      style: FilledButton.styleFrom(
                          backgroundColor: Colors.redAccent),
                      onPressed: () => Navigator.pop(ctx, true),
                      child: const Text('退出')),
                ],
              ),
            );
            if (ok == true && mounted) widget.onLogout();
          },
        ),
      );
}
