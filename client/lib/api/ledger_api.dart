import 'api_client.dart';

/// 认证 API（WBS 5.1：能登录；5.x 扩展：个人资料与账号安全，PRD §4.12）。
class AuthApi {
  final ApiClient client;
  AuthApi(this.client);

  /// 登录成功返回 accessToken 并写入 TokenStore。
  Future<String> login(String username, String password) async {
    final data = await client.post('/api/v1/auth/login', {
      'username': username,
      'password': password,
    }) as Map<String, dynamic>;
    final token = data['accessToken'] as String?;
    if (token == null || token.isEmpty) {
      throw ApiException(0, '登录响应缺少 accessToken');
    }
    await client.tokens.save(token);
    return token;
  }

  /// 当前用户（uid/username/email/isInstanceAdmin）。
  Future<Map<String, dynamic>> me() async =>
      await client.get('/api/v1/users/me') as Map<String, dynamic>;

  /// 更新邮箱（用户名为登录标识，服务端不开放修改）。
  Future<Map<String, dynamic>> updateEmail(String email) async =>
      await client.put('/api/v1/users/me', {'email': email})
          as Map<String, dynamic>;

  /// 修改密码。成功后服务端撤销全部会话（含当前）——客户端应清理本地凭证
  /// 并回到登录页。返回撤销的会话数。
  Future<int> changePassword(String oldPassword, String newPassword) async {
    final data = await client.post('/api/v1/users/me/password', {
      'oldPassword': oldPassword,
      'newPassword': newPassword,
    }) as Map<String, dynamic>;
    return (data['revokedSessions'] as num?)?.toInt() ?? 0;
  }
}

/// 账本 / 账户 / 分类 / 交易 API（WBS 5.2 CRUD 的客户端侧）。
class LedgerApi {
  final ApiClient client;
  LedgerApi(this.client);

  Future<List<dynamic>> books() async =>
      (await client.get('/api/v1/books') as List?) ?? const [];

  Future<List<dynamic>> accounts(int bookId) async =>
      (await client.get('/api/v1/books/$bookId/accounts') as List?) ?? const [];

  Future<List<dynamic>> categories(int bookId) async =>
      (await client.get('/api/v1/books/$bookId/categories') as List?) ??
      const [];

  /// 交易列表（分页参数透传）。
  Future<List<dynamic>> transactions(int bookId,
      {int page = 1, int pageSize = 50}) async {
    final data = await client.get('/api/v1/books/$bookId/transactions',
        query: {'page': '$page', 'pageSize': '$pageSize'}) as Map<String, dynamic>?;
    final items = data?['items'] as List?;
    return items ?? const [];
  }

  /// 记一笔（单笔收/支或转账）。[clientUuid] 为幂等键，重试不产生重复入账。
  Future<dynamic> createTransaction({
    required int bookId,
    required String clientUuid,
    required int txType,
    required int accountId,
    required int amountCents,
    int? categoryId,
    int? toAccountId,
    required int txTimeUnix,
    String comment = '',
  }) {
    return client.post('/api/v1/books/$bookId/transactions', {
      'clientUuid': clientUuid,
      'txType': txType,
      'accountId': accountId,
      'amount': amountCents,
      'categoryId': ?categoryId,
      'toAccountId': ?toAccountId,
      'txTime': txTimeUnix,
      'comment': comment,
    });
  }

  /// 编辑交易（PUT 全量替换；类型不可改；[baseVersion] 乐观锁，冲突 409）。
  Future<dynamic> updateTransaction({
    required int bookId,
    required int txId,
    required String clientUuid,
    required int txType,
    required int accountId,
    required int amountCents,
    int? categoryId,
    int? toAccountId,
    required int txTimeUnix,
    String comment = '',
    required int baseVersion,
  }) {
    return client.put('/api/v1/books/$bookId/transactions/$txId', {
      'clientUuid': clientUuid,
      'txType': txType,
      'accountId': accountId,
      'amount': amountCents,
      'categoryId': ?categoryId,
      'toAccountId': ?toAccountId,
      'txTime': txTimeUnix,
      'comment': comment,
      'baseVersion': baseVersion,
    });
  }

  /// 删除交易（整笔，含转账对侧；clientUuid 幂等）。
  Future<dynamic> deleteTransaction(int bookId, int txId, String clientUuid) {
    return client.delete(
      '/api/v1/books/$bookId/transactions/$txId',
      query: {'clientUuid': clientUuid},
    );
  }

  // ---- 账户 CRUD（WBS 5.2）----

  Future<dynamic> createAccount({
    required int bookId,
    required String name,
    required int category,
    String currency = 'CNY',
  }) {
    return client.post('/api/v1/books/$bookId/accounts',
        {'name': name, 'category': category, 'currency': currency});
  }

  Future<dynamic> updateAccount({
    required int bookId,
    required int accountId,
    String? name,
    int? parentId,
  }) {
    return client.put('/api/v1/books/$bookId/accounts/$accountId', {
      'name': ?name,
      'parentId': ?parentId,
    });
  }

  Future<dynamic> deleteAccount(int bookId, int accountId) {
    return client.delete('/api/v1/books/$bookId/accounts/$accountId');
  }

  // ---- 分类 CRUD（WBS 5.2）----

  Future<dynamic> createCategory({
    required int bookId,
    required String name,
    required int catType,
    int parentId = 0,
  }) {
    return client.post('/api/v1/books/$bookId/categories',
        {'name': name, 'catType': catType, 'parentId': parentId});
  }

  Future<dynamic> updateCategory({
    required int bookId,
    required int categoryId,
    String? name,
  }) {
    return client.put('/api/v1/books/$bookId/categories/$categoryId',
        {'name': ?name});
  }

  Future<dynamic> deleteCategory(int bookId, int categoryId) {
    return client.delete('/api/v1/books/$bookId/categories/$categoryId');
  }

  // ---- 家庭成员（阶段 3 / PRD §7，客户端侧补齐）----

  /// 成员列表（uid/username/role/canViewOthers/isInstanceAdmin）。
  Future<List<dynamic>> members(int bookId) async =>
      (await client.get('/api/v1/books/$bookId/members') as List?) ?? const [];

  /// 生成邀请码（返回 {code, role, expiresAt}——邀请码只此一次展示）。
  Future<Map<String, dynamic>> createInvite(int bookId,
      {required int role, String boundUsername = ''}) async {
    return await client.post('/api/v1/books/$bookId/members/invite', {
      'role': role,
      'boundUsername': boundUsername,
    }) as Map<String, dynamic>;
  }

  /// 变更成员角色（2=管理员，3=普通成员；owner 不可变更）。
  Future<dynamic> changeRole(int bookId, int uid, int role) {
    return client.put('/api/v1/books/$bookId/members/$uid', {'role': role});
  }

  /// 移除成员（移除后其令牌对该账本即失去访问权）。
  Future<dynamic> removeMember(int bookId, int uid) {
    return client.delete('/api/v1/books/$bookId/members/$uid');
  }
}
