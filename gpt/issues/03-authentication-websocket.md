# [审查] 认证与实时通道：账本授权、可见性、JWT 配置/撤销与并发清理

审查基线：`main@b8741465afa98a89ed99796c9e2c4a841b15a361`。按 `workbuddy/第三方审阅者导读.md` 完成前轮去重与权威文档对齐。

本仓库尚无正式后端工程。以下是**设计/教学代码/契约问题**，并非声称已经验证线上漏洞。SQL/算法反例与静态结论各自标注验证边界；不对原型生产安全作推断。

本 Issue 包含同一修复域的 R05, R08, R09, R18；处理状态均先为 **待确认**，请逐项更新 CHANGELOG 为“已采纳（含位置）/未采纳（含理由）”，并保留被证伪结论的纠正记录。

处理清单：

- [ ] R05：WebSocket 订阅信任客户端账本 ID，广播无法执行成员可见性
- [ ] R08：JWT 签发仍使用硬编码密钥，未对接配置中的密钥
- [ ] R09：JWT 认证路径未执行令牌撤销与账号失效检查
- [ ] R18：WS 订阅 map 并发读写，断连清理路径不完整

### R05 · P1 · WebSocket 订阅信任客户端账本 ID，广播无法执行成员可见性

验证方式：源码直接确认；未运行真实 WS 服务。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §14.1 L1012–L1016](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1012-L1016)
- [docs/FastLedger-新手开发完整指南.md · §14.1 L1037–L1047](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1037-L1047)
- [docs/FastLedger-新手开发完整指南.md · §14.2 L1054–L1059](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1054-L1059)
- [docs/FastLedger-新手开发完整指南.md · §14.3 L1065–L1088](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1065-L1088)
- [docs/FastLedger-产品需求文档-PRD.md · §7.3 L414–L417](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L414-L417)

原文片段：

```text
for _, b := range sub.Books { client.BookIds[b] = true }
if c.BookIds[bookId] { ... c.Send <- msg ... }
```

复现/核查步骤：

1. 源码核对：订阅消息 books 的每个值被直接写入 BookIds，中间没有调用 GetMemberRole。
2. 即使连接层先完成 JWT 登录和初始成员验证，仍核对同账本两名成员关闭互看后的广播分支。

实际结果：处理器接受任意客户端提供的账本 ID。Client 只保存 BookIds/Send，没有 uid/角色；广播把完整交易发给全部订阅者，无法依据 creator_uid 或账本可见性筛选。JWT 连接鉴权本身不能解决账本授权或关闭互看后的隐私隔离。

预期结果：每次订阅都由服务端验证当前成员资格；交易详情推送和 pull/list/export 使用相同可见性规则；移除成员或变更可见性后实时撤销访问。

修复建议：建议改后规则：连接绑定认证用户，订阅仅接收授权账本；广播前按接收者权限过滤，或仅发送不含敏感数据的失效通知，客户端再经授权接口拉取。连接存续期间处理成员移除、权限变化与令牌失效。关闭互看时不向无权成员发送金额、账户、备注等信息。

验证边界：按完整 Client/subscribe/BroadcastToBook 路径静态核对。没有把任意线上环境的可利用性当作已验证事实。

与前轮关系：前轮声称 WS 鉴权已补全；本次给出指南中遗漏成员授权与可见性过滤的具体路径。。

### R08 · P1 · JWT 签发仍使用硬编码密钥，未对接配置中的密钥

验证方式：源码直接确认。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §9.1 L663–L665](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L663-L665)
- [docs/FastLedger-新手开发完整指南.md · §10.1 L744–L744](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L744-L744)
- [docs/FastLedger-新手开发完整指南.md · §10.1 L778–L784](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L778-L784)
- [docs/FastLedger-新手开发完整指南.md · §10.2 L801–L812](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L801-L812)
- [docs/FastLedger-落地方案.md · §8.1 L1132–L1134](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L1132-L1134)

原文片段：

```text
var jwtSecret = []byte("改成你自己的密钥")
return token.SignedString(jwtSecret)
func JWTAuth(secret string) gin.HandlerFunc
```

复现/核查步骤：

1. 对照配置 jwt.secret / EBK_JWT_SECRET 与 Login 签发变量来源。
2. 在实现联调时给 JWTAuth 传配置密钥，检查 Login 返回 token 的签名密钥是否相同。

实际结果：签发函数确定使用包级字面量，与传入中间件的密钥没有数据流关系。若中间件读配置，合法登录 token 无法通过；若也照抄公开字面量，签名密钥可预测。第二句为明确条件结果，不声称生产正在使用该值。

预期结果：签发、验证、刷新均使用同一份经校验的配置；空值或示例占位值应拒绝启动。

修复建议：建议改后规则：把密钥随配置注入 AuthService 与 JWTAuth，移除源码中的示例 fallback；启动校验密钥强度与占位值。密钥轮换/令牌失效行为另行明确。

验证边界：静态数据流确认；未生成或尝试任何真实用户凭证。

与前轮关系：前轮已声称改为环境变量；该结论没有传播到可照敲指南。。

### R09 · P1 · JWT 认证路径未执行令牌撤销与账号失效检查

验证方式：完整认证函数源码核对。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-产品需求文档-PRD.md · §4.1 L186–L189](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L186-L189)
- [docs/FastLedger-产品需求文档-PRD.md · §8.2 L432–L435](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-产品需求文档-PRD.md#L432-L435)
- [docs/FastLedger-落地方案.md · §4.5 L710–L716](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-落地方案.md#L710-L716)
- [docs/FastLedger-新手开发完整指南.md · §10.1 L766–L784](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L766-L784)
- [docs/FastLedger-新手开发完整指南.md · §10.2 L808–L819](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L808-L819)

原文片段：

```text
jwt.ParseWithClaims(tokenStr, claims, ... )
c.Set("uid", int64(claims["uid"].(float64)))
c.Next()
```

复现/核查步骤：

1. 逐行跟踪 JWTAuth 从 ParseWithClaims 到 c.Next 的路径，并核对 Login claims。
2. 实现联调应覆盖：有效 token 登录后，撤销该会话/禁用或删除用户，再请求受保护接口。

实际结果：所示中间件只验签/标准 claims，不查询 auth_tokens.revoked 或用户状态；Login 也没有会话 ID/令牌记录。仅修改撤销表不会影响此鉴权函数。uid 还通过无检查类型断言读取，异常结构的已签名 claims 会 panic，而不是返回 401。

预期结果：PRD 承诺的可撤销会话应在失效后拒绝访问，WS 同样失效；无效身份字段稳定返回 401。

修复建议：建议改后规则：采用可查询/撤销的会话标识（如 jti）或不透明令牌哈希，把用户状态和会话状态校验接入共同认证层；安全解析必需 uid、exp，限定允许签名算法与用途。刷新流程不能复活已撤销会话。保留已有 token_hash 设计，不要求 JWT 本身改为明文落库。

验证边界：这是已确认的教学认证路径缺口；因为服务未实现，撤销接口到后端的完整联调尚未执行。

与前轮关系：新增发现。

### R18 · P2 · WS 订阅 map 并发读写，断连清理路径不完整

验证方式：并发控制流静态确认。处理状态：待确认（待维护者决定是否采纳；不代表事实未经核查）。

证据位置：

- [docs/FastLedger-新手开发完整指南.md · §14.1 L1037–L1046](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1037-L1046)
- [docs/FastLedger-新手开发完整指南.md · §14.3 L1074–L1097](https://github.com/LinTia/FastLadger/blob/b8741465afa98a89ed99796c9e2c4a841b15a361/docs/FastLedger-新手开发完整指南.md#L1074-L1097)

原文片段：

```text
client.BookIds[b] = true
if c.BookIds[bookId] { ... }
for msg := range client.Send { ... }
```

复现/核查步骤：

1. 让同一连接的读 goroutine 更新订阅，同时另一业务 goroutine 执行 BroadcastToBook 读取 BookIds。
2. 核对只发生读断连时：Unregister 是否关闭/取消 Send 读循环；写失败时是否统一 Unregister 和关闭连接。

实际结果：订阅写 map 未取得 Hub 的锁，广播读 map 时的 RLock 无法保护该写，形成数据竞争。读错误路径只 Unregister，不关闭或取消写循环，Send 也没有关闭路径；写错误返回又没有统一注销/Close。可留下阻塞的 handler/连接资源。

预期结果：订阅状态所有访问共享同一同步机制；连接任一方向结束都取消另一方向并恰好清理一次。

修复建议：建议改后规则：经 Hub 的受锁方法更新订阅，或使用每 Client 的锁/单 goroutine 所有权；读写循环共用取消上下文，统一 defer unregister/close。关闭 Send 与广播发送也需用同一所有权协议避免向已关闭 channel 写；增加读写 deadline、心跳和消息大小限制。用 go test -race 与反复连接/断开资源测试验证。

验证边界：根据 Go 并发规则与所示访问路径静态确认；没有声称运行过 race detector 或观测过生产崩溃。

与前轮关系：新增发现。

维护者复核说明：复现脚本及完整报告由本轮审阅交付；不直接替换业务方案。对已有修复的补充项已注明具体旧代码残留位置，避免把“新增机制存在”误认为“所有写路径均已闭合”。

官方参考：[Go 未保护 map 并发读写](https://go.dev/doc/articles/race_detector#Typical_Data_Races)。本轮未运行 Go race detector。
