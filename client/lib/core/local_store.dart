/// 本地 KV 缓存抽象（WBS 5.1：能登录、能缓存）。
///
/// 骨架期提供 [InMemoryLocalStore]；结构化数据（账本/账户/分类/交易列表）
/// 的持久化应在 5.2 接入 sqflite（Android）与 drift/sqlite3（Windows），
/// 接入点即本抽象与 [LocalStore.json] 的序列化约定。
abstract class LocalStore {
  Future<String?> getString(String key);
  Future<void> setString(String key, String value);
  Future<void> remove(String key);
}

class InMemoryLocalStore implements LocalStore {
  final Map<String, String> _data = {};

  @override
  Future<String?> getString(String key) async => _data[key];

  @override
  Future<void> setString(String key, String value) async => _data[key] = value;

  @override
  Future<void> remove(String key) async => _data.remove(key);
}
