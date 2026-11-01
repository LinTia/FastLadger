/// 登录令牌的安全存储抽象（WBS 5.1）。
///
/// 骨架期提供两个实现：
/// - [InMemoryTokenStore]：测试与 Web 首期使用；
/// - 生产端 Android/iOS 应接 flutter_secure_storage（Keychain/Keystore），
///   Windows 可接 DPAPI——接入点即本抽象，替换实现无需改动业务层。
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
