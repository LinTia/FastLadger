import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/api/ledger_api.dart';
import 'package:fastledger_client/core/app_settings.dart';
import 'package:fastledger_client/core/local_store.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:fastledger_client/ui/settings_page.dart';

import '../helpers.dart';

Widget wrap(Widget child) => MaterialApp(home: Scaffold(body: child));

void main() {
  group('SettingsPage', () {
    testWidgets('渲染个人资料（用户名只读 + 邮箱）', (tester) async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (
            200,
            okJson({'id': 7, 'username': 'tester', 'email': 'a@b.com'})
          );
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: AppSettings(store: InMemoryLocalStore()),
        onLogout: () {},
      )));
      await tester.pumpAndSettle();
      expect(find.text('tester'), findsOneWidget);
      expect(find.text('a@b.com'), findsOneWidget);
      expect(find.text('用户名是登录标识，不可修改'), findsNothing); // Tooltip 惰性渲染
    });

    testWidgets('通知开关切换走 AppSettings 并持久化', (tester) async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (200, okJson({'id': 7, 'username': 'tester', 'email': ''}));
        }
        return null;
      });
      final store = InMemoryLocalStore();
      final settings = AppSettings(store: store);
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: settings,
        onLogout: () {},
      )));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(SwitchListTile));
      await tester.pumpAndSettle();
      expect(settings.notifyEnabled, isTrue);
      expect(await store.getString('fl_notify_enabled'), '1');
    });

    testWidgets('主题切换生效（深色）', (tester) async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (200, okJson({'id': 7, 'username': 'tester', 'email': ''}));
        }
        return null;
      });
      final settings = AppSettings(store: InMemoryLocalStore());
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: settings,
        onLogout: () {},
      )));
      await tester.pumpAndSettle();

      // PAT 区块拉长了页面，"深色"可能滚出视口——ensureVisible 精确滚到位
      // （scrollUntilVisible 依赖惯性动画，测试环境易滚过头）
      await tester.ensureVisible(find.text('深色'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('深色'));
      await tester.pumpAndSettle();
      expect(settings.themeMode, ThemeMode.dark);
    });

    testWidgets('修改密码：两次新密码不一致时不发请求', (tester) async {
      var passwordCalls = 0;
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (200, okJson({'id': 7, 'username': 'tester', 'email': ''}));
        }
        if (method == 'POST' && path == '/api/v1/users/me/password') {
          passwordCalls++;
          return (200, okJson({'revokedSessions': 1}));
        }
        return null;
      });
      var loggedOut = false;
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: AppSettings(store: InMemoryLocalStore()),
        onLogout: () => loggedOut = true,
      )));
      await tester.pumpAndSettle();

      await tester.tap(find.text('修改密码'));
      await tester.pumpAndSettle();
      await tester.enterText(find.widgetWithText(TextField, '当前密码'), 'password123');
      await tester.enterText(find.widgetWithText(TextField, '新密码（至少 8 位）'), 'aaaa12345');
      await tester.enterText(find.widgetWithText(TextField, '确认新密码'), 'bbbb12345');
      await tester.tap(find.text('修改').last);
      await tester.pumpAndSettle();

      expect(passwordCalls, 0, reason: '两次输入不一致不得发请求');
      expect(find.text('两次输入的新密码不一致'), findsOneWidget);
      expect(loggedOut, isFalse);
    });

    testWidgets('修改密码成功后回调 onLogout（会话已被服务端撤销）', (tester) async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (200, okJson({'id': 7, 'username': 'tester', 'email': ''}));
        }
        if (method == 'POST' && path == '/api/v1/users/me/password') {
          return (200, okJson({'revokedSessions': 2, 'message': '密码已修改'}));
        }
        return null;
      });
      var loggedOut = false;
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: AppSettings(store: InMemoryLocalStore()),
        onLogout: () => loggedOut = true,
      )));
      await tester.pumpAndSettle();

      await tester.tap(find.text('修改密码'));
      await tester.pumpAndSettle();
      await tester.enterText(find.widgetWithText(TextField, '当前密码'), 'password123');
      await tester.enterText(find.widgetWithText(TextField, '新密码（至少 8 位）'), 'aaaa12345');
      await tester.enterText(find.widgetWithText(TextField, '确认新密码'), 'aaaa12345');
      await tester.tap(find.text('修改').last);
      await tester.pumpAndSettle();

      expect(loggedOut, isTrue);
      expect(jsonEncode({}), isNotEmpty); // helpers 引用保持
    });

    // ---- 长期 API Token（WBS 6.6 客户端侧）----

    testWidgets('PAT 区块：列表渲染（状态/备注/最近使用）', (tester) async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (200, okJson({'id': 7, 'username': 'tester', 'email': ''}));
        }
        if (method == 'GET' && path == '/api/v1/users/me/tokens') {
          return (200, okJson([
            {
              'id': 1, 'name': '快捷指令', 'createdAt': 1793580000,
              'expiresAt': 0, 'revokedAt': 0, 'lastUsedAt': 1793581000,
            },
            {
              'id': 2, 'name': 'CI', 'createdAt': 1793500000,
              'expiresAt': 0, 'revokedAt': 1793550000, 'lastUsedAt': 0,
            },
          ]));
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: AppSettings(store: InMemoryLocalStore()),
        onLogout: () {},
      )));
      await tester.pumpAndSettle();
      expect(find.text('快捷指令'), findsOneWidget);
      expect(find.text('CI'), findsOneWidget);
      expect(find.text('已撤销'), findsOneWidget);
      expect(find.textContaining('最近使用 从未'), findsOneWidget); // CI 的 Token 从未使用
    });

    testWidgets('PAT 生成：明文一次性展示 + 复制按钮，随后刷新列表', (tester) async {
      var createCalls = 0;
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (200, okJson({'id': 7, 'username': 'tester', 'email': ''}));
        }
        if (method == 'POST' && path == '/api/v1/users/me/tokens') {
          createCalls++;
          return (200, okJson({'token': 'flt_abc123', 'item': {'id': 9, 'name': 'x'}}));
        }
        if (method == 'GET' && path == '/api/v1/users/me/tokens') {
          return (200, okJson([
            {'id': 9, 'name': 'x', 'createdAt': 1, 'expiresAt': 0, 'revokedAt': 0, 'lastUsedAt': 0},
          ]));
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: AppSettings(store: InMemoryLocalStore()),
        onLogout: () {},
      )));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('生成新 Token'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), '快捷指令');
      await tester.tap(find.text('生成'));
      await tester.pumpAndSettle();
      expect(createCalls, 1);
      // 明文一次性展示 + 警示语
      expect(find.textContaining('flt_abc123'), findsOneWidget);
      expect(find.text('仅此一次展示，关闭后无法再查看。'), findsOneWidget);
      expect(find.text('复制'), findsOneWidget);
      await tester.tap(find.text('完成'));
      await tester.pumpAndSettle();
      // 列表刷新出现新条目
      expect(find.text('x'), findsOneWidget);
    });

    testWidgets('PAT 撤销：确认对话框 → DELETE → 列表刷新', (tester) async {
      var revokeCalls = 0;
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (200, okJson({'id': 7, 'username': 'tester', 'email': ''}));
        }
        if (method == 'GET' && path == '/api/v1/users/me/tokens') {
          return (200, okJson([
            {'id': 5, 'name': '旧 Token', 'createdAt': 1, 'expiresAt': 0, 'revokedAt': 0, 'lastUsedAt': 0},
          ]));
        }
        if (method == 'DELETE' && path == '/api/v1/users/me/tokens/5') {
          revokeCalls++;
          return (200, okJson({'revoked': true}));
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await tester.pumpWidget(wrap(SettingsPage(
        auth: auth,
        settings: AppSettings(store: InMemoryLocalStore()),
        onLogout: () {},
      )));
      await tester.pumpAndSettle();

      await tester.tap(find.text('撤销'));
      await tester.pumpAndSettle();
      // 确认对话框出现，尚未删除
      expect(find.textContaining('此操作不可恢复'), findsOneWidget);
      expect(revokeCalls, 0);
      await tester.tap(find.text('撤销').last);
      await tester.pumpAndSettle();
      expect(revokeCalls, 1);
      expect(find.text('Token 已撤销'), findsOneWidget);
    });
  });
}
