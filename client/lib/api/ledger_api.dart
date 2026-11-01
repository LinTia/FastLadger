import 'api_client.dart';

/// 认证 API（WBS 5.1：能登录）。
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
}
