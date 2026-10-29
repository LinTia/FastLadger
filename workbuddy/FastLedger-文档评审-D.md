# FastLedger 独立评审报告 · D（第四个审阅者）

> 评审代号：**D**；意见编号：**D-R01 – D-R06**
> 评审基线：`a403d13`（仓库 HEAD，2026-10-30）
> 评审方法：先读 `workbuddy/多AI对齐规范.md`（颗粒度/严重度/证据标准）+ `workbuddy/第三方审阅者导读.md`（背景与已定决策），再通读根 `CHANGELOG.md` [1.0]–[1.13]、`docs/` 全部文档、`gpt/`（含 v1.4 运行验证）、`prototype/`。
> 去重声明：已核对 GPT R01–R23、C-R01–C-R10、WB 各轮结论，**本报告不重复已裁决项**；凡与历史 finding 相邻的，均显式说明边界。
> 评审重点（按 owner 指示）：**工程实现层**——Gin 接线与 400/409、真实 WS 生命周期、Flutter 源码、OCR 后端契约、端到端同步、迁移与删除/恢复验收。

---

## 0. 总体判断

| 维度 | 结论 |
|---|---|
| 产品/技术决策 | PRD §12 + §12.1 已全部决策，无遗留待确认，口径自洽，**不应再开辩论** |
| 隔离参考骨架 | 交易创建/余额/转账/持仓的核心逻辑经 GPT v1.4 真实 SQLite 验证，质量高 |
| **工程落地（本轮火力）** | 存在两类系统性缺口：① **决策/WBS 已拍板，但主文档 DDL、代码骨架、接口清单未同步**（删除恢复审计、迁移机制）；② **从"能跑的函数"到"真实接线"的健壮性细节缺失**（WS 存活、OCR 契约、Flutter 网络层、同步读路径） |
| 数量 | 6 条：**2×P1 + 4×P2**，无 P0、无 P3 |

> 核心提醒：本项目反复出现的"设计对、骨架旧 = 没修"风险（规范 §6 血泪教训）在删除/恢复/审计与迁移两处再次出现。本轮 D-R01/D-R02 即针对此。

---

## 1. 逐条意见（规范 §2 十三字段）

### D-R01 · 删除/恢复/审计已决策，但 DDL、代码骨架与接口清单整体未落地

- `id`：D-R01
- `title`：删除/恢复/审计已决策，但开发者照敲的 DDL、骨架与接口清单未落地
- `severity`：**P1**
- `location`：
  - 决策源：`docs/FastLedger-产品需求文档-PRD.md` §12.1 L577–582
  - 任务源：`docs/FastLedger-开发WBS.md` 阶段 0.4（L39）、阶段 4.3（L99）
  - 缺口：`docs/FastLedger-新手开发完整指南.md` §6.4 建表 SQL L295–472（15 张表，无 `audit_logs`）；§13 无 Delete/Restore 骨架
  - 缺口：`docs/FastLedger-落地方案.md` §4 DDL L574–766（无 `audit_logs`）；§5.1 接口表 L813–835（无回收站/恢复/审计查询接口）
- `quote`：
  - PRD L580：`3. 审计日志独立成表（append-only）：新增 audit_logs(id, book_id, actor_uid, action, entity_type, entity_id, before_json, at, source)……仓库层禁止对该表 UPDATE/DELETE`
  - PRD L578：`保留 30 天（配置项 EBK_RECYCLE_TTL_DAYS，默认 30），到期硬清除`
  - 指南建表 SQL 实际只有 15 个 `CREATE TABLE`（users…budgets），其中**没有** `audit_logs`
- `problem`：owner 已在 [1.10] 拍板回收站 30 天/审计 1 年/转账双边删除恢复/统一 `TxService.Delete/Restore`，WBS 也列了 0.4、4.3 任务；但开发者实际照敲的《新手指南》建表 SQL 没有 `audit_logs` 表、没有 `transfer_group_id`（或等价绑定）、没有 Delete/Restore 函数，《落地方案》接口清单也没有回收站列表、恢复、硬清除、审计查询接口。决策与可执行骨架脱节。
- `reproduce`：
  1. `grep -c "CREATE TABLE" docs/FastLedger-新手开发完整指南.md` → 15，且 `grep audit_logs` 在该指南/落地方案的 DDL 中无建表命中（仅 PRD/WBS 文字提及）；
  2. 在指南 §13 搜索 `func.*Delete|func.*Restore` → 无对应服务骨架；
  3. 在落地方案 §5.1 搜索 `recycle|restore|audit` → 无对应 HTTP 路由。
- `actual`：应届生照当前骨架，无法实现"删除二次确认→余额回滚→进回收站→30 天硬清除→审计可追溯"这条已决策链路；软删只有 `deleted_at`，回答不了"谁删的、删前是什么"。
- `expected`：① 建表 SQL 补 `audit_logs`（append-only，含 before_json）与回收站所需字段；② 补 `TxService.Delete/Restore` 骨架（同一事务：置 deleted_at→`VersionRepo.Next`→写 sync_version→两侧余额回滚→写审计）；③ 接口清单补回收站列表/恢复/硬清除/审计查询；④ 转账以整笔为单位（transfer_group_id 或 related_id 绑定），禁止单边。
- `impact`：会计正确性（删除须回滚余额、转账双边）与不可篡改审计（§12 决策#8）无法落地；跨文档不一致，开发到阶段 4 会发现无表可用而返工。
- `suggestion`：按 PRD L577–582 与 WBS 4.3 的字段，把 `audit_logs` DDL、Delete/Restore 骨架与对应接口补进《落地方案》《新手指南》并记 CHANGELOG；审计表在仓库层封装为只暴露 Insert，不暴露 Update/Delete。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：PRD/WBS 文字与主文档 DDL/骨架/接口的不一致可由上述 grep 复现，未发现已有等价 DDL。

---

### D-R02 · 版本化数据库迁移机制只有 CLI 名称，缺可落地设计

- `id`：D-R02
- `title`：版本化数据库迁移机制只有 CLI 名称，缺可落地设计
- `severity`：**P1**
- `location`：
  - `docs/FastLedger-落地方案.md` §环节5 目录结构 L250：`cmd/  CLI 入口（server run / db migrate / userdata）`
  - `docs/FastLedger-开发WBS.md` 阶段 0.4 L39：`迁移框架 + 首次建表 | 迁移脚本 + 全量表`
  - 全仓未出现 schema 版本表/迁移脚本规范（grep `schema_migrations|migration|up.*down` 无设计命中）
- `quote`：
  - 落地方案 L250：`cmd/            CLI 入口（server run / db migrate / userdata）`
  - WBS L39：`| 0.4 迁移框架 + 首次建表 | 迁移脚本 + 全量表 | …老库升级不得删旧回执 |`
- `problem`：自托管软件必然跨版本升级（v1→v2/v3，未来还要加投资/行情），但当前只有一个 `db migrate` 命令名和 WBS 一行任务，没有定义：① 迁移版本如何记录（schema 版本表）；② 迁移脚本如何组织与命名（up/down）；③ 是否在事务内执行、失败如何回滚；④ SQLite 对 `ALTER TABLE` 的限制（早期版本不支持 DROP/RENAME COLUMN，需"建新表→拷数据→改名"重建）；⑤ 启动时是否自动迁移还是仅手动。开发者拿到这一行无法照做。
- `reproduce`：
  1. `grep -rniE "schema_migrations|migration|up/down|ALTER TABLE" docs/FastLedger-落地方案.md docs/FastLedger-新手开发完整指南.md` → 除 CLI 名与 WBS 行外，无机制设计；
  2. 设想从已部署 v1 升级到加了新表的 v2：当前文档没有任何"老库如何升级"的步骤。
- `actual`：迁移行为未定义；若开发者临时手写 `CREATE TABLE IF NOT EXISTS`，对需要改列/加约束的升级会失败或留下不一致 schema，且无法判断当前库处于哪个版本。
- `expected`：明确一套最小迁移规范：版本表（如 `schema_migrations(version, applied_at)`）、顺序编号的迁移脚本（含 up，down 可选）、每次迁移在事务内执行并校验、SQLite 不支持的 DDL 走重建表流程、默认仅 CLI 手动迁移（启动只检查版本不自动改），并保证老库升级不删旧回执（WBS 0.4 已提）。
- `impact`：没有可靠迁移，家庭用户升级即面临数据损坏或服务无法启动，直接违背"数据不丢"目标；这是可部署性的前置硬门槛。
- `suggestion`：在《落地方案》补一节迁移机制（版本表 + 脚本规范 + SQLite 重建表范式 + 事务/回滚），《新手指南》给最小可照敲迁移器骨架，与首次建表（WBS 0.4）对齐。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：全仓无迁移机制设计，仅两处名称/任务引用，缺口可 grep 复现。

---

### D-R03 · WS 有 ping 但无 pong/读截止，半开连接无法回收

- `id`：D-R03
- `title`：WS 有 ping 但无 pong/读截止与读大小限制，半开连接无法回收
- `severity`：P2
- `location`：`docs/FastLedger-新手开发完整指南.md` §14.3 `ServeWS` L1483–1547；写循环 ping 在 L1530–1545；`upgrader.Upgrade` 在 L1495（upgrader 定义未展示）
- `quote`：
  - L1530：`ticker := time.NewTicker(30 * time.Second)`
  - L1540–1542：`case <-ticker.C: if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil { return }`
  - 全函数未出现 `SetReadDeadline` / `SetPongHandler` / `SetReadLimit`
- `problem`：写循环每 30s 发 ping 能探测"写失败"，但服务端读循环没有 `SetReadDeadline`，也没有 `SetPongHandler` 在收到 pong 后续期读截止。因此对"TCP 未断但对端已死/网络静默丢弃"的半开连接，服务端既读不到错误、也不会因 pong 超时关闭，连接与两个 goroutine 长期挂住（泄漏）。这是 R18 修复建议（`gpt/issues/03` L146："增加读写 deadline、心跳和消息大小限制"）中**已提及但当前骨架未落地**的存活检测部分；R18 本体（锁顺序/清理/Send 不关闭）已修复，本条不重复。
- `reproduce`：
  1. 建立 WS 连接后，在客户端模拟"网络静默"（不发断开、不回 pong，丢弃所有入站）；
  2. 观察服务端：ping 写出无报错（写缓冲未满），读循环 `ReadMessage` 不返回，连接不被清理；
  3. `grep SetReadDeadline|SetPongHandler|SetReadLimit` 指南 §14.3 → 无命中。
- `actual`：半开连接无法被及时回收，长时间运行后 `Hub.clients` 与 goroutine 累积，内存上涨（设备内存预算 <150MB）。
- `expected`：升级后设置 `conn.SetReadLimit(...)`、初始 `SetReadDeadline`，并 `SetPongHandler` 中重置读截止；读循环收到 pong（默认 handler）即续期，超时未收到 pong 则读截止触发错误、走统一清理。
- `impact`：长期运行的连接/goroutine 泄漏与内存占用，降低 NAS 上服务稳定性；不影响数据正确性（pull 兜底）。
- `suggestion`：在 `ServeWS` 增加读截止 + pong 续期 + 读大小限制；同时显式定义 `upgrader`（含 `CheckOrigin` 策略），避免默认行为在不同客户端入口下不一致。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：ping 已存在、pong/deadline/limit 缺失可 grep 复现；属 R18 建议未落地增量，非 R18 本体重复。

---

### D-R04 · OCR 文本兜底 handler 吞绑定错误、不校验输入且无超时/取消

- `id`：D-R04
- `title`：OCR 文本兜底 handler 吞绑定错误、不校验输入且 LLM 调用无超时/取消
- `severity`：P2
- `location`：`docs/FastLedger-新手开发完整指南.md` §15.3 `ParseText` L1571–1584；LLM 抽象 §15.2 L1562–1567
- `quote`：
  - L1575–1577：`var req struct{ Text string `json:"text"` }; _ = c.ShouldBindJSON(&req); result, err := llm.ParseBill(req.Text)`
  - L1565：`ParseBill(text string) (BillResult, error)`（不接收 context）
- `problem`：① `ShouldBindJSON` 错误被 `_` 丢弃，非法 JSON 也继续；② 不校验 `text` 是否为空或超长——空文本白调一次 LLM，超长文本可能撑爆模型上下文或产生意外费用；③ `ParseBill` 不接收 `ctx`，内部无法随客户端断开而取消，也没有超时，云端 LLM 挂起时 handler goroutine 与连接一直占用。GPT 仅处理过 R15（端侧金额正则千分位），未覆盖此后端契约，本条不重复。
- `reproduce`：
  1. 发空 body / 非法 JSON / `{"text":""}` → 代码不报错，仍进入 `ParseBill`；
  2. 发一个超长 text（如 1MB）→ 无长度拦截直接送 LLM；
  3. 让 LLM 端点永不响应 → 请求无超时，goroutine 不退出；
  4. `grep ShouldBindJSON` 指南 §15.3 确认错误被忽略。
- `actual`：弱输入与无超时会造成 LLM 费用浪费、错误信息不明确，以及 goroutine/连接泄漏。
- `expected`：显式绑定并检查错误（失败返 400）；校验 text 非空且长度在阈值内（如 ≤ 2000 字符，超限 400）；用 `ctx, _ := context.WithTimeout(c.Request.Context(), N秒)` 并把 ctx 传入 `ParseBill(ctx, text)`，超时/取消即失败返回；对 LLM 返回的结构化结果做基本合法性校验后再返回。
- `impact`：核心卖点（OCR 闭环）最薄弱的后端契约：成本可控性、健壮性与资源回收均有缺口；因最终人工确认，不直接导致错账。
- `suggestion`：把 LLM 抽象改为 `ParseBill(ctx context.Context, text string)`，handler 补绑定/空值/长度校验与超时，错误按 §13.2 映射（400/504/5xx），不外露原始错误。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：绑定错误被吞、无校验、签名无 ctx 均为原文事实，可复现。

---

### D-R05 · Flutter ApiClient 无超时、不处理 HTTP 状态码、连接不复用、409 无法区分

- `id`：D-R05
- `title`：Flutter ApiClient 无超时、不处理 HTTP 状态码、连接不复用、409 无法区分
- `severity`：P2
- `location`：`docs/FastLedger-新手开发完整指南.md` §19.3 `ApiClient` L1848–1879
- `quote`：
  - L1862：`final resp = await http.Client().send(`（每次新建 client）
  - L1870–1871：`final data = jsonDecode(await resp.stream.bytesToString()); if (data['code'] != 0) throw Exception(data['msg']);`
- `problem`：① 每次请求 `http.Client().send` 新建 client，无连接复用（无 keep-alive），且无法统一关闭；② 整个调用没有 `timeout`，弱网/对端挂起时 Future 永不返回，UI 转圈卡死；③ 只看 body 里的 `code`，不判断 HTTP 状态码——网关/反代返回非 JSON（如 Caddy 502 HTML）时 `jsonDecode` 直接抛格式异常，401（token 过期）也无法统一拦截跳登录；④ 409 冲突被笼统抛成 `Exception`，页面无法识别并落地"覆盖/放弃/另存"三选一（与 §22.3 决策脱节）。
- `reproduce`：
  1. 断网/对端不响应调用 `request(...)` → 无超时，Future 不完成；
  2. 让服务端返回 401/502 的非约定 body → `jsonDecode` 抛异常而非可处理的业务错误；
  3. 构造 409（同键不同内容）→ 捕获到的只是通用 `Exception`，无法据此分支处理；
  4. 连续多次请求，确认每次都 new 一个 `http.Client()`。
- `actual`：弱网挂起、登录失效无统一处理、冲突三选一无法在客户端落地，且连接不复用增加开销。
- `expected`：持有一个可复用、可统一 `close()` 的 `http.Client`；统一加 `timeout`；先按 HTTP 状态码分支（401 清 token 跳登录、403/409/422 转成可判定的 typed exception、5xx 可重试），再解析业务 body；为 409 暴露专门错误类型携带冲突实体，供页面三选一。
- `impact`：客户端在弱网/鉴权失效/并发冲突下的可用性与可维护性；不影响服务端数据正确性。
- `suggestion`：重构 `ApiClient`（持久 client + 超时 + 状态码映射 + typed 错误，含 409 专门类型），与 §13.2 的 HTTP 映射和 §22.3 冲突处理保持一致；本地离线队列补传复用同一通道。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：每次新建 client、无超时、仅判断 body code 均为原文事实。

---

### D-R06 · 同步 Pull 函数缺成员校验与批次上限，存在越权读与内存风险

- `id`：D-R06
- `title`：同步 Pull 函数缺成员校验与批次上限，存在越权读与内存风险
- `severity`：P2
- `location`：`docs/FastLedger-新手开发完整指南.md` §22.2 `Pull` L2073–2114（函数体 L2087–2111）
- `quote`：
  - L2087：`func Pull(bookID, afterVersion int64) (*SyncBatch, error) {`
  - L2103–2104：`sess.Where("book_id=? AND sync_version>?", bookID, afterVersion).OrderBy("sync_version ASC").Find(&batch.Transactions)`
- `problem`：① `Pull` 函数内部不做成员资格校验，直接按 bookID 读全部实体；文档（§11.2）虽提供 `GetMemberRole` 并要求"操作前中间件校验"，但该函数本身不设防——一旦某个 handler/适配路径调用前漏校验，即构成越权读取（含关闭互看的私账）。② 不校验 `afterVersion`，且无批次大小上限：新设备首次登录或 afterVersion 很旧（含负数）时，一次性把该账本全部交易/账户/分类/持仓读进内存，在 <150MB 内存预算下可能 OOM。与 GPT-R21（写路径 client_uuid 必填）不同，本条聚焦**读路径**的授权与内存，不重复。
- `reproduce`：
  1. 静态检查 `Pull` 函数体：无 `GetMemberRole`/成员查询、无 afterVersion 校验、无 limit；
  2. 以非成员 uid 经一个未额外校验的路径调用 `Pull(otherBookID, 0)` → 可返回该账本数据；
  3. 构造大量历史行后 `Pull(bookID, 0)` → 单次全量加载，内存随行数线性增长。
- `actual`：读路径授权依赖每个调用方自觉，且全量批次无内存保护，存在越权与 OOM 隐患。
- `expected`：① `Pull` 内部（或强制经一个带校验的服务方法）先核对成员资格与角色，非成员返回 403；② 校验 afterVersion（非法值按全量处理并记录）；③ 设批次上限/按版本边界分页，超限时返回 `force_full_resync` 或分块游标，保证不把同一 sync_version 拆到两批。
- `impact`：潜在越权读取（隐私）与低内存设备 OOM（可用性）；端到端同步的关键读路径缺口。
- `suggestion`：把成员校验下沉进 Pull 的服务入口（不信任 handler），加批次大小与版本边界控制，与 §12.1 缓存重建/全量重同步决策（[1.12]）衔接；读接口统一复用该校验。
- `status`：待确认
- `evidence`：（待复核 AI 填）D 自查：函数无授权/无上限为原文事实，可 grep 复现；与 GPT-R21 写路径不同。

---

## 2. 优先级与处置建议（给 owner）

| 顺序 | finding | 级别 | 建议 |
|---|---|---|---|
| 1 | D-R01 | P1 | 先补 `audit_logs` DDL + Delete/Restore 骨架 + 接口，闭合 [1.10] 决策 |
| 2 | D-R02 | P1 | 定版本化迁移规范，是可部署/可升级的前置门槛 |
| 3 | D-R04 | P2 | OCR 后端契约补校验+超时（核心卖点，改动小、收益大） |
| 4 | D-R06 | P2 | 同步读路径补授权与批次控制 |
| 5 | D-R03 | P2 | WS 存活检测，防长期运行泄漏 |
| 6 | D-R05 | P2 | Flutter 网络层重构，配合 409 三选一 |

> 全部 6 条均为 `待确认`，请 owner 按"属实的修、误判的说理由"裁决；如任一条被判定为误判或与历史 finding 重复，D 将按规范 §5 即时收回并说明。

## 3. 给其他 AI（WB / GPT / C）的协同提示

1. D-R01/D-R02 是"决策/WBS 已存在、主文档骨架未同步"，复核时请优先核对 PRD/WBS 与《落地方案》《新手指南》的 DDL、接口、函数是否一致，而非仅看决策文字。
2. D-R03 已显式标注为 R18 修复建议的未落地增量，非 R18 本体；若认定 R18 已隐含覆盖，请指出，D 可收回。
3. 本轮仅做静态评审与跨文档核对，**未新增运行时验证**；D-R01–D-R06 的真实修复仍建议在正式工程中用 `go test -race`、HTTP/WS 集成测试与恢复演练验证。
4. 已定技术决策（Go/Gin/xorm、SQLite FULL、唯一 Create、VersionRepo.Next、多账本、可见性开关等）D 全部接受，未重开。
