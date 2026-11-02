import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'token_store.dart';

/// 生产级令牌安全存储（WBS 5.x / PRD §4.12 设置页前置）。
///
/// 底层映射（flutter_secure_storage 11.x）：
///   Android → Keystore + EncryptedSharedPreferences；
///   iOS/macOS → Keychain；
///   Windows → DPAPI 加密文件（v9 起弃用 CredManager——其 ~2KB 条目上限
///   存不下较长的 JWT/多条凭证；本类只存一个登录 JWT，余量充足）。
///
/// 语义与 [InMemoryTokenStore] 完全一致（接口即契约）：登录态 JWT 之外
/// **不存任何业务数据**；PAT 明文按设计"只出现一次"（创建响应），本存储
/// 不持久化 PAT——长期凭证的管理与撤销都在服务端 auth_tokens 表。
class SecureTokenStore implements TokenStore {
  final FlutterSecureStorage _storage;

  /// 测试可注入替身（FlutterSecureStorage 的方法均可覆写）。
  /// Android 默认即 Keystore 参与的加密存储（v10+ 移除手动开关）。
  SecureTokenStore([FlutterSecureStorage? storage])
      : _storage = storage ?? const FlutterSecureStorage();

  static const _key = 'fl_token';

  @override
  Future<void> save(String token) =>
      _storage.write(key: _key, value: token);

  @override
  Future<String?> read() => _storage.read(key: _key);

  @override
  Future<void> clear() => _storage.delete(key: _key);
}
