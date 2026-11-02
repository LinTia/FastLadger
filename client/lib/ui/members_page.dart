import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/ledger_api.dart';

/// 家庭成员页（阶段 3 / PRD §7，客户端侧补齐）。
///
/// 内容：成员列表（角色徽标 + 可见性状态）、邀请成员（邀请码一次性展示 +
/// 复制）、成员详情（底部弹层：基础信息 + 设为管理员/降级为成员/移除成员）。
///
/// 权限口径由服务端把关：写操作（邀请/角色/移除）仅 owner/admin 可用，
/// 403 时以 SnackBar 如实展示服务端文案。
class MembersPage extends StatefulWidget {
  final LedgerApi ledger;
  final int bookId;
  final String bookName;

  const MembersPage({
    super.key,
    required this.ledger,
    required this.bookId,
    required this.bookName,
  });

  @override
  State<MembersPage> createState() => _MembersPageState();
}

class _MembersPageState extends State<MembersPage> {
  List<dynamic>? _members;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await widget.ledger.members(widget.bookId);
      if (mounted) setState(() => _members = list);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = '加载失败，请下拉重试');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('家庭成员 · ${widget.bookName}')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _invite,
        icon: const Icon(Icons.person_add_alt),
        label: const Text('邀请成员'),
      ),
      body: _error != null
          ? ListView(children: [
              Padding(
                  padding: const EdgeInsets.all(24),
                  child: Center(child: Text(_error!))),
            ])
          : RefreshIndicator(
              onRefresh: _load,
              child: _buildBody(),
            ),
    );
  }

  Widget _buildBody() {
    final members = _members;
    if (members == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (members.isEmpty) {
      return ListView(children: const [
        Padding(
            padding: EdgeInsets.all(24),
            child: Center(child: Text('暂无成员记录'))),
      ]);
    }
    return ListView.separated(
      itemCount: members.length,
      separatorBuilder: (_, _) => const Divider(height: 1),
      itemBuilder: (context, i) {
        final m = members[i] as Map<String, dynamic>;
        return ListTile(
          leading: CircleAvatar(
            child: Text(
              (m['username'] as String? ?? '?').characters.first.toUpperCase(),
            ),
          ),
          title: Text(m['username'] as String? ?? ''),
          subtitle: Text(m['canViewOthers'] == true ? '已开启账本可见' : '已关闭账本可见'),
          trailing: _roleChip(m['role'] as num? ?? 3),
          onTap: () => _showMemberDetail(m),
        );
      },
    );
  }

  Widget _roleChip(num role) {
    final isOwner = role == 1;
    final isAdmin = role == 2;
    final label = isOwner ? '所有者' : (isAdmin ? '管理员' : '普通成员');
    return Chip(
      label: Text(label, style: const TextStyle(fontSize: 12)),
      visualDensity: VisualDensity.compact,
      backgroundColor: isOwner
          ? Colors.teal.shade50
          : isAdmin
              ? Colors.blue.shade50
              : null,
    );
  }

  /// 成员详情（底部弹层，移动端适配：内容自适应高度 + 安全区 padding）。
  Future<void> _showMemberDetail(Map<String, dynamic> m) async {
    final role = (m['role'] as num? ?? 3).toInt();
    await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(m['username'] as String? ?? '',
                style: Theme.of(ctx).textTheme.titleLarge),
            const SizedBox(height: 8),
            _detailRow('角色', role == 1 ? '所有者' : (role == 2 ? '管理员' : '普通成员')),
            _detailRow('账本可见', m['canViewOthers'] == true ? '已开启' : '已关闭'),
            _detailRow('实例管理员',
                m['isInstanceAdmin'] == true ? '是' : '否'),
            _detailRow('用户 ID', '${m['uid'] ?? ''}'),
            const SizedBox(height: 12),
            if (role == 3)
              FilledButton.tonal(
                onPressed: () => Navigator.pop(ctx, 'promote'),
                child: const Text('设为管理员'),
              ),
            if (role == 2) ...[
              FilledButton.tonal(
                onPressed: () => Navigator.pop(ctx, 'demote'),
                child: const Text('降级为成员'),
              ),
            ],
            if (role != 1) ...[
              const SizedBox(height: 8),
              FilledButton(
                style: FilledButton.styleFrom(
                    backgroundColor: Theme.of(ctx).colorScheme.error,
                    foregroundColor: Theme.of(ctx).colorScheme.onError),
                onPressed: () => Navigator.pop(ctx, 'remove'),
                child: const Text('移除成员'),
              ),
            ],
          ]),
        ),
      ),
    ).then((action) {
      if (action == null) return;
      final uid = (m['uid'] as num).toInt();
      if (action == 'promote' || action == 'demote') {
        _changeRole(uid, action == 'promote' ? 2 : 3);
      } else if (action == 'remove') {
        _remove(m['username'] as String? ?? '', uid);
      }
    });
  }

  Widget _detailRow(String label, String value) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(children: [
          SizedBox(
              width: 88,
              child: Text(label,
                  style: TextStyle(color: Theme.of(context).hintColor))),
          Expanded(child: Text(value)),
        ]),
      );

  Future<void> _changeRole(int uid, int role) async {
    try {
      await widget.ledger.changeRole(widget.bookId, uid, role);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text(role == 2 ? '已设为管理员' : '已降级为成员')));
      }
      _load();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Future<void> _remove(String username, int uid) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('移除成员'),
        content: Text('移除后该成员将无法再访问家庭账本「$username」。'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              style: FilledButton.styleFrom(
                  backgroundColor: Theme.of(ctx).colorScheme.error),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('确认移除')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.ledger.removeMember(widget.bookId, uid);
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('成员已移除')));
      }
      _load();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Future<void> _invite() async {
    final roleSel = ValueNotifier<int>(3);
    final boundCtrl = TextEditingController();
    final invited = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('邀请成员'),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          ValueListenableBuilder<int>(
            valueListenable: roleSel,
            builder: (_, role, _) => DropdownButtonFormField<int>(
              initialValue: role,
              decoration: const InputDecoration(labelText: '角色'),
              items: const [
                DropdownMenuItem(value: 3, child: Text('普通成员')),
                DropdownMenuItem(value: 2, child: Text('管理员')),
              ],
              onChanged: (v) => roleSel.value = v ?? 3,
            ),
          ),
          TextField(
            controller: boundCtrl,
            decoration: const InputDecoration(
                labelText: '绑定用户名（可选）',
                hintText: '留空 = 任何人凭码加入'),
          ),
        ]),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true), child: const Text('生成邀请码')),
        ],
      ),
    );
    if (invited != true) return;
    try {
      final inv = await widget.ledger
          .createInvite(widget.bookId, role: roleSel.value, boundUsername: boundCtrl.text.trim());
      if (!mounted) return;
      final code = inv['code'] as String? ?? '';
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('邀请码已生成'),
          content: Column(mainAxisSize: MainAxisSize.min, children: [
            SelectableText(code,
                style: const TextStyle(
                    fontSize: 20, fontWeight: FontWeight.w700, letterSpacing: 2)),
            const SizedBox(height: 8),
            const Text('请复制给对方，在其登录页选择「加入账本」输入即可。',
                style: TextStyle(fontSize: 13)),
          ]),
          actions: [
            TextButton(
              onPressed: () async {
                await Clipboard.setData(ClipboardData(text: code));
                if (ctx.mounted) {
                  ScaffoldMessenger.of(ctx)
                      .showSnackBar(const SnackBar(content: Text('邀请码已复制')));
                }
              },
              child: const Text('复制邀请码'),
            ),
            FilledButton(
                onPressed: () => Navigator.pop(ctx), child: const Text('完成')),
          ],
        ),
      );
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }
}
