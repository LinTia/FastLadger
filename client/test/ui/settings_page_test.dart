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
  });
}
