import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fastledger_client/core/secure_token_store.dart';
import 'package:fastledger_client/core/token_store.dart';

/// FlutterSecureStorage 替身：覆写三个方法，行为同内存 KV，
/// 并记录调用（验证 SecureTokenStore 的 key 与语义）。
/// 签名照抄 11.x（required key + 全平台 options，覆写必须参数兼容）。
class FakeSecureStorage extends FlutterSecureStorage {
  final Map<String, String> data = {};
  int writes = 0;
  int deletes = 0;

  @override
  Future<void> write({
    required String key,
    required String? value,
    AndroidOptions? aOptions,
    AppleOptions? iOptions,
    LinuxOptions? lOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
    WebOptions? webOptions,
  }) async {
    data[key] = value ?? '';
    writes++;
  }

  @override
  Future<String?> read({
    required String key,
    AndroidOptions? aOptions,
    AppleOptions? iOptions,
    LinuxOptions? lOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
    WebOptions? webOptions,
  }) async =>
      data[key];

  @override
  Future<void> delete({
    required String key,
    AndroidOptions? aOptions,
    AppleOptions? iOptions,
    LinuxOptions? lOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
    WebOptions? webOptions,
  }) async {
    data.remove(key);
    deletes++;
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('SecureTokenStore（生产实现，flutter_secure_storage 11.x）', () {
    test('实现 TokenStore 抽象（接缝契约）', () {
      final store = SecureTokenStore(FakeSecureStorage());
      expect(store, isA<TokenStore>());
    });

    test('save → read 往返，key 固定 fl_token', () async {
      final fake = FakeSecureStorage();
      final store = SecureTokenStore(fake);
      await store.save('jwt-abc');
      expect(fake.data['fl_token'], 'jwt-abc');
      expect(fake.writes, 1);
      expect(await store.read(), 'jwt-abc');
    });

    test('clear 删除条目（401 下线/退出登录语义）', () async {
      final fake = FakeSecureStorage();
      final store = SecureTokenStore(fake);
      await store.save('jwt-abc');
      await store.clear();
      expect(fake.data.containsKey('fl_token'), isFalse);
      expect(fake.deletes, 1);
      expect(await store.read(), isNull);
    });

    test('未保存时 read 返回 null（首启进登录页）', () async {
      final store = SecureTokenStore(FakeSecureStorage());
      expect(await store.read(), isNull);
    });
  });
}
