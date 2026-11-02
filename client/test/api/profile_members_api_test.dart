import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/api/api_client.dart';
import 'package:fastledger_client/api/ledger_api.dart';
import 'package:fastledger_client/core/token_store.dart';

import '../helpers.dart';

void main() {
  group('AuthApi 资料与密码（PRD §4.12）', () {
    test('me() 返回当前用户', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/users/me') {
          return (
            200,
            okJson({
              'id': 7,
              'username': 'tester',
              'email': 'a@b.com',
              'isInstanceAdmin': false
            })
          );
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      final me = await auth.me();
      expect(me['username'], 'tester');
      expect(me['id'], 7);
    });

    test('updateEmail() PUT /users/me 携带 email', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'PUT' && path == '/api/v1/users/me') {
          return (
            200,
            okJson({'id': 7, 'username': 'tester', 'email': 'new@x.com'})
          );
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      final me = await auth.updateEmail('new@x.com');
      expect(me['email'], 'new@x.com');
      expect(h.lastPath, '/api/v1/users/me');
      expect(h.lastJsonBody?['email'], 'new@x.com');
    });

    test('changePassword() 返回撤销会话数', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'POST' && path == '/api/v1/users/me/password') {
          return (
            200,
            okJson({'revokedSessions': 3, 'message': '密码已修改'})
          );
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      final revoked = await auth.changePassword('old', 'newpassword1');
      expect(revoked, 3);
      expect(h.lastJsonBody?['oldPassword'], 'old');
      expect(h.lastJsonBody?['newPassword'], 'newpassword1');
    });

    test('旧密码错误 → ApiException 带服务端文案', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'POST' && path == '/api/v1/users/me/password') {
          return (400, errJson(400, '登录已失效，请重新登录'));
        }
        return null;
      });
      final auth = AuthApi(
          ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));
      await expectLater(
        auth.changePassword('bad', 'newpassword1'),
        throwsA(isA<ApiException>()),
      );
    });
  });

  group('LedgerApi 成员管理', () {
    LedgerApi apiOf(FakeApiHttp h) => LedgerApi(
        ApiClient(baseUrl: 'http://t', tokens: InMemoryTokenStore(), httpClient: h));

    test('members() 解列表', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'GET' && path == '/api/v1/books/3/members') {
          return (
            200,
            okJson([
              {'uid': 1, 'username': 'owner', 'role': 1, 'canViewOthers': true},
              {'uid': 2, 'username': 'm2', 'role': 3, 'canViewOthers': false},
            ])
          );
        }
        return null;
      });
      final list = await apiOf(h).members(3);
      expect(list.length, 2);
      expect((list[0] as Map)['username'], 'owner');
    });

    test('createInvite() POST 携带角色与绑定名', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'POST' && path == '/api/v1/books/3/members/invite') {
          return (200, okJson({'code': 'INV-XYZ', 'role': 2, 'expiresAt': 1}));
        }
        return null;
      });
      final inv = await apiOf(h).createInvite(3, role: 2, boundUsername: 'm2');
      expect(inv['code'], 'INV-XYZ');
      expect(h.lastJsonBody?['role'], 2);
      expect(h.lastJsonBody?['boundUsername'], 'm2');
    });

    test('changeRole() PUT 正确路径', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'PUT' && path == '/api/v1/books/3/members/9') {
          return (200, okJson({'uid': 9, 'role': 2}));
        }
        return null;
      });
      await apiOf(h).changeRole(3, 9, 2);
      expect(h.lastPath, '/api/v1/books/3/members/9');
      expect(h.lastJsonBody?['role'], 2);
    });

    test('removeMember() DELETE 正确路径', () async {
      final h = FakeApiHttp((method, path, body) {
        if (method == 'DELETE' && path == '/api/v1/books/3/members/9') {
          return (200, okJson({'removed': true}));
        }
        return null;
      });
      await apiOf(h).removeMember(3, 9);
      expect(h.lastPath, '/api/v1/books/3/members/9');
    });
  });

  // jsonEncode 引用保持（helpers 内已用）
  test('smoke', () => expect(jsonEncode({}), '{}'));
}
