/// 登录令牌的安全存储抽象（WBS 5.1）。
///
/// 实现清单：
/// - [InMemoryTokenStore]：测试使用；
/// - [SecureTokenStore]（secure_token_store.dart）：生产实现——
///   flutter_secure_storage（Android Keystore / iOS Keychain / Windows DPAPI）。
///   替换实现无需改动业务层（接缝即本抽象）。
abstract class TokenStore {
  Future<void> save(String token);
  Future<String?> read();
  Future<void> clear();
}

class InMemoryTokenStore implements TokenStore {
  String? _token;

  @override
  Future<void> save(String token) async => _token = token;

  @override
  Future<String?> read() async => _token;

  @override
  Future<void> clear() async => _token = null;
}
