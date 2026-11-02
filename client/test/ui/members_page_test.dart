import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/api/ledger_api.dart';
import 'package:fastledger_client/core/token_store.dart';
import 'package:fastledger_client/ui/members_page.dart';

import '../helpers.dart';

Widget wrap(Widget child) => MaterialApp(home: Scaffold(body: child));

MembersPage pageOf(FakeApiHttp h) => MembersPage(
      ledger: LedgerApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h)),
      bookId: 3,
      bookName: '家账',
    );

void main() {
  group('MembersPage', () {
    testWidgets('渲染成员列表（角色徽标 + 可见性）', (tester) async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/books/3/members') {
          return (
            200,
            okJson([
              {'uid': 1, 'username': 'owner', 'role': 1, 'canViewOthers': true},
              {'uid': 2, 'username': '小明', 'role': 3, 'canViewOthers': false},
            ])
          );
        }
        return null;
      });
      await tester.pumpWidget(wrap(pageOf(h)));
      await tester.pumpAndSettle();
      expect(find.text('owner'), findsOneWidget);
      expect(find.text('小明'), findsOneWidget);
      expect(find.text('所有者'), findsOneWidget);
      expect(find.text('普通成员'), findsOneWidget);
      expect(find.text('已开启账本可见'), findsOneWidget);
      expect(find.text('已关闭账本可见'), findsOneWidget);
    });

    testWidgets('成员详情弹层展示基础信息并可设为管理员', (tester) async {
      var roleCalls = <String, int>{};
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/books/3/members') {
          return (
            200,
            okJson([
              {'uid': 1, 'username': 'owner', 'role': 1, 'canViewOthers': true},
              {'uid': 2, 'username': '小明', 'role': 3, 'canViewOthers': false},
            ])
          );
        }
        final m = RegExp(r'^/api/v1/books/3/members/(\d+)$').firstMatch(path);
        if (method == 'PUT' && m != null) {
          roleCalls[m.group(1)!] = 2; // 用例只发 role=2（设为管理员）
          return (200, okJson({'uid': 2, 'role': 2}));
        }
        return null;
      });
      await tester.pumpWidget(wrap(pageOf(h)));
      await tester.pumpAndSettle();

      await tester.tap(find.text('小明'));
      await tester.pumpAndSettle();
      expect(find.text('账本可见'), findsOneWidget);
      expect(find.text('设为管理员'), findsOneWidget);
      expect(find.text('移除成员'), findsOneWidget);

      await tester.tap(find.text('设为管理员'));
      await tester.pumpAndSettle();
      expect(roleCalls.containsKey('2'), isTrue, reason: '应向 /members/2 发 PUT');
      expect(find.text('已设为管理员'), findsOneWidget);
    });

    testWidgets('邀请成员 → 邀请码一次性展示', (tester) async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/books/3/members') {
          return (200, okJson([
            {'uid': 1, 'username': 'owner', 'role': 1, 'canViewOthers': true},
          ]));
        }
        if (method == 'POST' && path == '/api/v1/books/3/members/invite') {
          return (200, okJson({'code': 'INV-1234', 'role': 3, 'expiresAt': 99}));
        }
        return null;
      });
      await tester.pumpWidget(wrap(pageOf(h)));
      await tester.pumpAndSettle();

      await tester.tap(find.text('邀请成员'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('生成邀请码'));
      await tester.pumpAndSettle();
      expect(find.text('邀请码已生成'), findsOneWidget);
      expect(find.textContaining('INV-1234'), findsOneWidget);
      expect(find.text('复制邀请码'), findsOneWidget);
    });

    testWidgets('移除成员需确认', (tester) async {
      var removeCalls = 0;
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/books/3/members') {
          return (200, okJson([
            {'uid': 2, 'username': '小明', 'role': 3, 'canViewOthers': false},
          ]));
        }
        if (method == 'DELETE' && path == '/api/v1/books/3/members/2') {
          removeCalls++;
          return (200, okJson({'removed': true}));
        }
        return null;
      });
      await tester.pumpWidget(wrap(pageOf(h)));
      await tester.pumpAndSettle();

      await tester.tap(find.text('小明'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('移除成员'));
      await tester.pumpAndSettle();
      // 确认对话框出现，尚未删除
      expect(find.text('确认移除'), findsOneWidget);
      expect(removeCalls, 0);
      await tester.tap(find.text('确认移除'));
      await tester.pumpAndSettle();
      expect(removeCalls, 1);
      expect(find.text('成员已移除'), findsOneWidget);
    });
  });
}
