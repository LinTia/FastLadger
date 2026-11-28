# T22 批次 1 第 3 版（`fd4a9a7`）交付级独立复核

> 复核角色：第三 AI（不参与 WB 交付撰写），只读复核 + 独立报告。
> 被审分支：`work/r0-t22-batch1`
> 被审 SHA：`fd4a9a77d17685379df80cf0cc2d137ad4fa8974`（2026-11-27 17:38）
> 对照基线：`3999ceef76bd5e4f0e2f416d3287284613c97179`
> 计划基线：`0033a1899878c2b832578cbfbb11ab858c2f8649`（`docs/r0-t22-execution-plan.md`，本轮仍未改）
> 上一轮复核报告：`79f147dba566799d4a6ff08cb516425c61fbef16`（`docs/r0-t22-b1-wb-delivery-recheck-v2.md`，分支 `review/t22-w1-w8-recheck-v2`）
> 本报告分支：`review/t22-w1-w8-recheck-v3`
> 复核日期：2026-11-28

---

## 0. 摘要结论

**整体判定：PARTIAL。W1 可签收；W8 暂不签收；不建议签收本次交付。**

| 项 | 判定 | 一句话依据 |
|---|---|---|
| W1（13×8 矩阵骨架） | **PASS** | 13 行 × 10 列、三计数 104 = 17 + 51 + 36、`缺` 与 §3 预填一致，四条验收项全部实测通过 |
| W8（offline / conflict 边界） | **PARTIAL** | offline 12 格三要素与页 5 冲突载体链成立；「逐页 409 语义」有 3 处实质性错误/遗漏（页 12、页 1、页 5） |
| T22 Batch 1（W1+W8 合并交付） | **PARTIAL** | W8 未达交付级，且 §9 哈希自述与 §4.2 行号两处可核性缺陷未清 |
| 是否建议签收 | **否** | 修正 A-1…A-5 五项（只改文档，不需复采、不需重跑统计基线）后可签收 |

**计数未变，且经我独立复算**：`104 = 有 17 + 缺 51 + N/A 36`；适用分母 `96 = 17 + 51 + 28`；三档齐备 `0`；§7.6 逐格比对 `改判 5 / 一致 99`；V-10 脚本 `PASS（未通过 0 项）`。**本轮没有任何统计基线漂移。**

**R0 仍 BLOCK**。本批只改两份 Markdown，不触碰第 3 条门禁，不得据此宣布 T22 完成、第 3 条 PASS 或 R0 PASS。

---

## 1. 提交范围与边界合规

```
$ git log --format='%h %p %ad %s' --date=iso 3999cee..fd4a9a7
fd4a9a7 2363cdc 2026-11-27 17:38:07 +0800 docs(r0): pairs/README 哈希自述改为如实描述（不重签校验清单）
2363cdc 3999cee 2026-11-27 17:37:35 +0800 docs(r0): 修正 T22 批次1 复核 v2 的 6 项 P2 与可安全修正的 P3

$ git diff --name-status 3999cee fd4a9a7
M  docs/r0-screenshots/pairs/README.md
M  docs/r0-status-matrix-13x8.md

$ git diff --shortstat 3999cee fd4a9a7
 2 files changed, 86 insertions(+), 50 deletions(-)

$ git log --merges --oneline 3999cee..fd4a9a7
（空）
```

判定：**边界合规**。两个提交、两个 Markdown；`server/`、`tools/`、`prototype/`、CI、PNG、`SHA256SUMS*.txt`、fixture、`controls.json` 零改动；无合并、无 amend（`fd4a9a7` 的父提交是 `2363cdc`，`2363cdc` 的父提交是 `3999cee`）。

一条需要记录的事实：远端 `main`（`b4309db`）**不是** `fd4a9a7` 的祖先（`git merge-base --is-ancestor b4309db fd4a9a7` → `rc=1`）。这说明本分支与 main 并行推进，不构成本轮违规；但将来合并时必然要做文档冲突检查，不能假定本分支已包含 main 的最新文档。

---

## 2. N-11 至 N-16 逐项核对

### 2.1 汇总

| 编号 | 要求 | 实测证据 | 判定 |
|---|---|---|---|
| N-11 | runtime 02/04 骨架条数改为 DOM 实测 4 | `dom-snapshots.json`：`01`=4、`02`=4、`03`=4、`04`=4、`05`=0；§6 三行均写 4 | **已修** |
| N-12 | 页 13 邮箱 409 补 `PUT /users/me`，说明 Token 增删不产生该 409 | `app.js:2206` 确认 `api("PUT", "/users/me", { email: … })`；`ErrEmailTaken` 全仓唯一返回点 `profile.go:53`（`UpdateMe`） | **已修** |
| N-13 | 页 8 写调用补 `DELETE /instruments/:id`，注明仅 DELETE 产生 409 | `app.js:1412` 确认 DELETE；`ErrInstrumentInUse` 全仓唯一返回点 `invest.go:192`（`DeleteInstrument`） | **已修** |
| N-14 | 逐页 409 补全，标题改「非穷举」 | 标题与完整性声明已改（§5.5:409、:427）；但新增 §5.5.1 有 **3 处错误/遗漏**（见 §4 A-1/A-2/A-3） | **未修完** |
| N-15 | 编辑已删交易 404 未登记 | 缺口第 4 条已登记（§5.5 实现缺口）；代码链逐行成立（见 2.3） | **已修（措辞需微调）** |
| N-16 | 43 张基准由 mtime 改为入库提交 | 43/43 张页面级 PNG 均由 `575eae3` 入库；`575eae3` 是 `0472484` 的祖先（`rc=0`） | **已修** |

### 2.2 N-11：骨架条数

```
$ python3 -c "import json; d=json.load(open('docs/r0-screenshots/runtime/dom-snapshots.json')); …"
01-pending-390x844.png skelCount= 4 ariaBusy= 1
02-rejected-network-390x844.png skelCount= 4 ariaBusy= 1
03-rejected-503-390x844.png skelCount= 4 ariaBusy= 1
04-pending-1280x800.png skelCount= 4 ariaBusy= 1
05-recovered-390x844.png skelCount= 0 ariaBusy= 0
```

§6 第 5/6/8 行分别写「4 条骨架」「4 条骨架残留」「4 条骨架（第 4 条较短）」，与 DOM 快照一致。N-11 真修，不是文字替换。

### 2.3 N-15：已删交易 404 的证据链

```
$ grep -n "loadTxRowByState" server/internal/service/tx_update.go
111:	row, err := loadTxRowByState(sess, bookID, txID, true)

$ sed -n '1044,1057p' server/internal/service/tx.go
func loadTxRowByState(sess *xorm.Session, bookID, txID int64, expectLive bool) (*model.Transaction, error) {
	...
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = ?", txID, bookID, deleted).Get(&r)
	...
	if !has {
		return nil, ErrTransactionNotFound
	}
	...
}
```

`expectLive=true` → SQL 条件 `deleted = 0`；目标已删则 `!has` → `ErrTransactionNotFound`（`tx.go:48–49`，HTTP 404）。与之对照，`ErrEntityDeleted`（`tx.go:54–56`）在 `common.go:51–53` 映射为 409「目标已被删除」。PRD:593 的原文是「旧离线数据回流……目标已删则返回明确错误（409 + `ENTITY_DELETED`）」。

三点评价：

1. **代码证据充分**，404 的判定不是推测，`tx_update.go:111` + `tx.go:1044–1057` + `common.go:73` 三处可逐行验证。
2. **缺口定性成立但需收窄**：PRD:593 的字面场景是「离线队列回流」，不是「交互式编辑」。文档把二者合并为「与 PRD 的 409 口径相反」，结论方向对，但应写明「交互式编辑同属『不静默复活』原则，非该条字面场景」。
3. **「实现缺口」与「设计选择」需分开**：`tx.go:1001–1002` 对**删除/恢复回执重放**明确写了「状态不符 → 404，与幂等未命中同口径」，这是有意的设计选择；而**更新路径**（`tx_update.go:111`）没有这类注释。文档应写成「更新路径的 404 无注释依据，且与 `tx.go:54–56` 的冲突定义不一致」，而不是暗示全仓都在违反 PRD。

### 2.4 N-16：`575eae3` 的基准推断是否严谨

```
$ for f in $(find docs/r0-screenshots -name '*.png' -not -path '*/pairs/*' -not -path '*/runtime/*'); do
    git log --diff-filter=A --format='%h' -- "$f" | tail -1; done | sort | uniq -c
     43 575eae3

$ git log -1 --format='%h A=%ad C=%cd' --date=iso 575eae3
575eae3 A=2026-11-04 17:52:25 +0800 C=2026-11-04 17:52:25 +0800
$ git log -1 --format='%h A=%ad C=%cd' --date=iso 0472484
0472484 A=2026-11-05 18:18:00 +0800 C=2026-11-05 18:18:00 +0800

$ git merge-base --is-ancestor 575eae3 0472484; echo rc=$?
rc=0
```

推断链成立且比「提交时间比较」更强：`575eae3` 不仅是时间上早一天，还是 `0472484` 的**祖先**，两张图的采集时间必然早于其入库提交，入库提交又早于 `0472484`。文档同时明确写了「精确代码 SHA 不可得」「mtime 不作依据」，没有把入库提交冒充代码基准。**N-16 严谨，予以认可。**

唯一可再加一句的是：`575eae3` 是**文档提交**，其树内 `server/` 内容才是隐含代码基准；若将来要精确到 SHA，只能重采，不能从该提交反推。

### 2.5 N-14：新增 §5.5.1 的逐条体检

§5.5.1 共 8 条 bullet。逐条与 `service` 实际错误树比对：

| bullet | 声明 | 实测 | 判定 |
|---|---|---|---|
| 页 4 | `common.go:51`、`:62`；`tx.go:469`、`:492` | `:51` = `ErrEntityDeleted` case、`:62` = `ErrIncompleteReceipt` case；`tx.go:469/:492` 确为账户/分类已删 | 正确 |
| 页 5 | `common.go:62`、`:49`；`tx_update.go:107/:163/:206/:230/:233` | 五个行号逐一命中 `ErrIncompleteReceipt`/`ErrCreateConflict` | 正确但不完整（见 A-3） |
| 页 6 | `common.go:100`、`:102` | `:100` = `ErrAccountInUse`、`:102` = `ErrAccountHasChildren` | 正确 |
| 页 7 | `common.go:105`、`:107` | `:105` = `ErrCategoryInUse`、`:107` = `ErrCategoryHasChildren` | 正确 |
| 页 8 | `common.go:80` | `:80` = `case ErrInstrumentInUse` | 正确 |
| 页 12 | `PUT`/`DELETE` → `common.go:51`、`member.go:234` | **两条路径都不调用 `requireActiveBookTx`**；实际 409 是 `ErrConcurrentConflict` | **错误（A-1）** |
| 页 13 | `common.go:90` | `:90` = `ErrEmailTaken` | 正确 |
| 页 1/2/3/10/11 | 「无写路径，无其他 409」 | 页 1 有 `POST /auth/login`（同一文件 §5.5 已列）与可选 `POST /auth/register` | **错误（A-2）** |

**结论：6 条正确、2 条错误；另有 1 处应列未列（页 5，A-3）。**

---

## 3. N-17 至 N-25 逐项核对

| 编号 | 要求 | 实测 | 判定 |
|---|---|---|---|
| N-17 | `557e4d2` 不再标成代码基准 | §4 E3/E15 写「文档 HEAD `557e4d2`；对应代码 SHA 未独立验证」；`runtime/README:11` 原文即「历史文档 HEAD」 | **已修** |
| N-18 | 合计行移出表格；待冻结写全第三数 | §1 表后三行散文（`:86–88`）；§2.1 写「18/50/36 或 17/51/36」，两式均 = 104 | **已修** |
| N-19 | 641 断点解释 | `style.css:327` `max-width:640`；`:334` `max-width:768 and min-width:641`；641 落在后者，解释正确 | **已修** |
| N-20 | 函数区间收窄 | `refillCategories()` 确为 `560–571`（`:572` 起 `syncFormFields`）；但 §4.2 另两处行号仍偏（A-5） | **部分修复** |
| N-21 | G4 转录去掉 `233` | §5.2 G4 输出只剩 `98`/`234`/`235` 三行，与实测一致 | **已修** |
| N-22 | §10 N-03 落点改 §5.4 | §10 表 N-03 行落点已为「§5.4；§5.5」 | **已修** |
| N-23 | 页 3 七行逐行写理由 | §3.2 第 8–14 行各写「整页无 Web 路由（R01），该状态同样不适用」，无「同上」 | **已修** |
| N-24 | README 行数自述 | `SHA256SUMS.txt` 实测 109 行；README:182 写「109 行 = 108 条证据 + 本文件」 | **已修** |
| N-25 | 规则 3 成员数分侧 | §0.3 规则 3 写「原型 2 名，实现侧仅 owner 1 名」 | **已修** |

### 3.1 N-20 / A-5：`§4.2` 与 README 的行号偏差（遗留项）

```
$ git show 0472484 --unified=0 -- server/web/dist/app.js | grep '^@@'
@@ -385,0 +386 @@ async function renderTxns(bookId) {
@@ -399,4 +400,21 @@ async function renderTxns(bookId) {
@@ -535 +552,0 @@ function openTxForm(bookId, onDone) {
@@ -541,0 +559,13 @@ function openTxForm(bookId, onDone) {
@@ -545,0 +576 @@ function openTxForm(bookId, onDone) {
@@ -568 +599 @@ function openTxForm(bookId, onDone) {
@@ -1678 +1709 @@ async function renderMembers(bookId) {
@@ -1684 +1715,4 @@ async function renderMembers(bookId) {
@@ -1710 +1744 @@ async function renderMembers(bookId) {
```

按新文件行号，实际改动点是 `386`、`400–420`、`552`、`559–571`、`576`、`599`、`1709`、`1715–1718`、`1744`。

文档写的是：

- §4.2:252 与 README:28 —— `renderTxns`（`app.js:383`、`:397–423`）；实际 `:386`、`:400–420`。
- §4.2:252 —— `openTxForm`「hunk `:550–578`」；实际最后一处 hunk 在 `:599`，区间应为 `:552–599`（或枚举 `552 / 559–571 / 576 / 599`）。
- §4.2:252 —— `renderMembers`（`:1706–1741`）；实际 `:1709`、`:1715–1718`、`:1744`。

**来源判定（对我自己上一轮的更正）**：这组行号在 `3999cee` 就已存在（`git show 3999cee:docs/r0-status-matrix-13x8.md` 同一行文本），**不是本轮新引入**，是我上一轮复核漏检。本轮仍未被 N-20 的收窄动作覆盖，因此列为待修项，但不计入「修订引入的新问题」。

结论层面不受影响：`renderTxns`、`openTxForm`、`renderMembers` 确实都被 `0472484` 改动，方向正确；错的是可核对的坐标。

---

## 4. 新发现（按严重度）

### A-1【P1 · W8】页 12 的 409 归属两处反向错误，且漏 `ErrConcurrentConflict`

**位置**：

- `docs/r0-status-matrix-13x8.md:424`（§5.5 第 12 行）
- `docs/r0-status-matrix-13x8.md:197`（§3.2 第 33 行，R08-12）
- `docs/r0-status-matrix-13x8.md:436`（§5.5.1 页 12 bullet）

**文档写法**：

> 页 12 `PUT` / `DELETE members/:uid`：`common.go:51`（账本已删/不可用，`member.go:234`）。

**实际代码**：

```
$ grep -n "requireActiveBookTx" server/internal/service/member.go
200:	if err := requireActiveBookTx(sess, bookID); err != nil {     # List（GET）
224:func requireActiveBookTx(...)                                  # 定义，:234 在函数体内
377:	if err := requireActiveBookTx(sess, bookID); err != nil {     # CreateInvite（POST）
459:			if aerr := requireActiveBookTx(sess, bookID); aerr != nil {  # RevokeInvite，错误被转成 ErrNotMember
834:	if err := requireActiveBookTx(sess, inv.BookId); err != nil { # JoinViaInvite
```

`ChangeRole`（`:240`）与 `Remove`（`:299`）**都没有调用** `requireActiveBookTx`。它们的 409 来自 0 行更新：

```
$ sed -n '284p;332p' server/internal/service/member.go
		return fmt.Errorf("%w：目标成员已不存在或已被移除", ErrConcurrentConflict)
		return fmt.Errorf("%w：目标成员已不存在或已被移除", ErrConcurrentConflict)
```

`ErrConcurrentConflict` 在 `common.go:64–65` 映射为 409「数据已被并发修改，请重试」。语义是**目标成员已被移除**，不是**账本已删**。

**另一处反证**：§5.5 第 12 行把 409 来源写成 `common.go:115`（`ErrBookFull`，成员上限 20）。但 `ErrBookFull` 的唯一产生路径是兑换邀请：

```
$ grep -rn "inviteReasonBookFull" server/internal/service/member.go
590:	case inviteReasonBookFull:        # inviteRejectError → ErrBookFull
893:			inviteReasonBookFull, ...   # JoinViaInvite 成员数 >= 20
```

对应路由是 `router.go:128` 的 `POST /books/:id/members/join`（未认证组），而 Web 前端**从未调用**该接口：

```
$ grep -n "join" server/web/dist/app.js
1086:    const q = new URLSearchParams({ format, fields: fields.join(",") });
```

`members/join` 零命中。因此 `ErrBookFull` 对页 12 列出的四个写调用（`:1689`、`:1761`、`:1771`、`:1786`）**全部不可达**。

**风险**：读者按表排障会把「改角色/移除成员失败」误判为「账本已被删除」，实际应查「目标成员是否已被他人移除」；而真正的「账本已删」409 只在签发邀请时出现。方向完全相反。

**修复要求**（只改文档）：把三条落点统一改为下列四项，不再保留 `common.go:115` 作为页 12 的来源：

| 写调用 | 可达 409 | 代码链 | 语义 |
|---|---|---|---|
| `POST members/invite` `:1689` | `ErrEntityDeleted` | `member.go:377` → `:234` → `common.go:51/53` | 账本已删/不可用 |
| `PUT members/:uid` `:1761`/`:1771` | `ErrConcurrentConflict` | `member.go:284` → `common.go:64/65` | 目标成员已被移除 |
| `DELETE members/:uid` `:1786` | `ErrConcurrentConflict` | `member.go:332` → `common.go:64/65` | 目标成员已被移除 |
| `POST members/invite/:inviteId/revoke`（Web 未调用） | 无 409 | `member.go:459` 把 `requireActiveBookTx` 错误转 `ErrNotMember` → 403 | 统一表现为无权限 |
| `POST members/join`（Web 未调用） | `ErrBookFull` | `member.go:886–892` → `:593` → `common.go:117` | 成员上限 20 |

**附带观察（不在本批范围，建议另行登记）**：`app.js:1698` 的提示文案写「对方在登录页选择『加入账本』并输入邀请码即可」，但登录页没有「加入账本」入口，`app.js` 也没有 `members/join` 调用。这是产品文案承诺了未实现的流程，属于实现缺口，不应由本批文档修正，但应在 W9/收口清单中留痕。

### A-2【P1 · W8】页 1 被写成「无写路径」，与同一文件的 §5.5 自相矛盾，并漏注册 409

**位置**：`docs/r0-status-matrix-13x8.md:438`

**文档写法**：

> 页 1/2/3/10/11 无写路径，无其他 409。

**同一文件 §5.5 第 1 行（`:413`）却写**：

> 页 1 登录：`POST /auth/login` `:193`、`:209`

**实际代码**：

```
$ sed -n '192p;224,228p' server/web/dist/app.js
        await api("POST", "/auth/register", { username: u.value, password: p.value });
  const toggle = document.createElement("a");
  toggle.textContent = isRegister ? "已有账号？返回登录" : "没有账号？注册新账号";
  toggle.addEventListener("click", (e) => { e.preventDefault(); renderLogin(isRegister ? "login" : "register"); });

$ grep -n "auth/register" server/internal/api/router.go
124:		v1.POST("/auth/register", ah.register)

$ sed -n '109,110p' server/internal/api/common.go
	case errors.Is(err, service.ErrUserExists):
		fail(c, http.StatusConflict, "用户名已存在")
```

即：登录页内可通过切换进入注册模式，注册会调用 `POST /auth/register`；用户名重复时 `auth.go:239` 返回 `ErrUserExists`，API 层映射为 **409**。

**限定条件**：注册受 `EBK_ENABLE_REGISTER` 控制，默认关闭（`config_infra_test.go:242`：`legacyEnvBool("EBK_ENABLE_REGISTER", false)`）。因此它是**条件可达**，不是默认可达。

**风险**：把「页 1 无写路径」写成事实，会让后续复核者跳过整个登录页的 409 检查；而这一页恰好存在一个真的 409（用户名已存在），只是默认关闭。

**修复要求**：把 `:438` 改为分列表述，例如：

> - 页 2/3/10/11 无写路径，无其他 409。
> - 页 1 有 `POST /auth/login`（`:193`、`:209`；失败以 401/锁定为主）与**条件可达**的 `POST /auth/register`（`:192`，需 `EBK_ENABLE_REGISTER=true`；用户名重复 → `ErrUserExists` → `common.go:109–110` 409）。注册模式不在原型范围内，故 conflict 列仍判 `N/A R08-01`，但不得表述为「无写路径」。

### A-3【P1 · W8】页 5 写调用漏 `DELETE :2054`，§5.5.1 漏两个可达 409

**位置**：

- `docs/r0-status-matrix-13x8.md:417`（§5.5 第 5 行，写调用只列 `PUT /transactions/:id :664`）
- `docs/r0-status-matrix-13x8.md:432`（§5.5.1 页 5 bullet）

**实际代码**：交易详情页 `renderTxDetail`（`app.js:2008–2102`）带删除按钮：

```
$ sed -n '2045,2056p' server/web/dist/app.js
  delBtn.onclick = async () => {
    …
      await api("DELETE", "/books/" + bookId + "/transactions/" + tx.id + "?clientUuid=" + encodeURIComponent(uuid()));
```

删除与更新都会触发转账整笔校验，从而产生两个 409：

```
$ grep -n "ErrTxDirectionMissing\|ErrTransferGroupIncomplete" server/internal/service/tx.go
864:			return 0, false, fmt.Errorf("%w：转账记录 %d 缺少方向标记，…", ErrTxDirectionMissing, r.Id)
1077/1083/1087/1100:  ErrTransferGroupIncomplete, …

$ sed -n '120,122p' server/internal/service/tx_update.go
	group, err := collectTransferGroup(sess, bookID, row, true)
	if err != nil {
		return nil, err
```

`common.go:54`（`ErrTxDirectionMissing`）与 `:59`（`ErrTransferGroupIncomplete`）都是 409，前者文案「方向标记异常」，后者「转账配对不完整」。二者在 §5.5 与 §5.5.1 中**均未登记**。

**风险**：页 5 是 13 页里唯一「有版本冲突载体」的页，也是 409 分支最多的页；漏掉这两个分支会让「逐页 409 语义」在最重要的一页上不完整。且 §5.5 的写调用列缺 DELETE，会让读者误以为详情页只能编辑不能删除。

**修复要求**：

1. §5.5 第 5 行写调用补 `DELETE /transactions/:id` `:2054`（删除按钮，`renderTxDetail`）。
2. §5.5.1 页 5 bullet 补 `common.go:54`（DELETE → `tx.go:864`）、`common.go:59`（DELETE → `tx.go:1077/1083/1087/1100`；UPDATE → `tx_update.go:120`）。
3. 若担心表格过宽，可在 §5.5.1 增加一行小表，按「方法 / 错误 / 代码链 / 语义」四列列出，不再挤进一句话。

### A-4【P2 · 文档】§9 第 11 条登记的 README 哈希，在本提交内已经失效

**位置**：`docs/r0-status-matrix-13x8.md:710`

**文档写法**：

> 原 `c329854f…`，实测现为 `eda61aee70a6d8a51bf2bce558e9b00a7e6869f2b862492af95079f4b051539c`

**实测**：

```
$ for c in 3999cee 2363cdc fd4a9a7; do printf "%s " $c; git show $c:docs/r0-screenshots/pairs/README.md | sha256sum; done
3999cee  eda61aee70a6d8a51bf2bce558e9b00a7e6869f2b862492af95079f4b051539c
2363cdc  6ecb6789932756ecd5dce8088ace69bda3e7765881a5d155b6fc93f32bfe71ed
fd4a9a7  987409085f3af142935c265a131cfcf20271a0d66f11beee7ec4445f517bf30f
```

`eda61aee…` 是 **`3999cee` 版本**的 README 哈希。`2363cdc` 修改 README 后变为 `6ecb6789…`，`fd4a9a7` 再改后变为 `98740908…`。也就是说：**记录该值的提交 `2363cdc` 自身就已使它失效，`fd4a9a7` 又失效一次**。

文档已经写了「README 无法内嵌自身最终哈希（自指）」，但紧接着又给了一个具体值，形成自指陷阱。这是本轮新引入的问题（`3999cee` 时该值还是对的）。

**风险**：复核者按 §9 抄下 `eda61aee…` 去比对，必然对不上，进而怀疑整份归档校验的可信度。

**修复要求**（二选一）：

- 方案 A（推荐）：删除具体值，改为「更正前（`3999cee`）为 `eda61aee…`；更正后请用 `git show <commit>:docs/r0-screenshots/pairs/README.md | sha256sum` 取得」。
- 方案 B：保留值但显式标注「此为 `3999cee` 版本，非本提交版本」，并紧接上述命令。

### A-5【P2 · 文档】§4.2 / README 的 `renderTxns`、`openTxForm`、`renderMembers` 行号偏差

见 §3.1。虽为 `3999cee` 遗留（非本轮引入），但它是 F-05「代码基准」列的可核性基础，且 N-20 本轮只修了 `refillCategories()` 一处，未覆盖同行的另两处。修复要求：

- `renderTxns`：`:383` → `:386`；`:397–423` → `:400–420`（§4.2:252、README:28 两处同步）。
- `openTxForm`：`hunk :550–578` → `:552–599`，或枚举 `552 / 559–571 / 576 / 599`。
- `renderMembers`：`:1706–1741` → `:1709`、`:1715–1718`、`:1744`。

### A-6【P3 · 文档】README:31 的复现命令在 Windows 工作树可能得到 CRLF 哈希

README:31 教读者用 `sha256sum docs/r0-screenshots/pairs/README.md` 取实测值；但同一文件末尾已说明：`core.autocrlf=true` 的 clone 会把 `.md` 检出为 CRLF，文本哈希据此不同。两条说明互相冲突——按前者做，得到的不是 git blob 的哈希。

**修复要求**：把复现命令改为 `git show <commit>:docs/r0-screenshots/pairs/README.md | sha256sum`，与 A-4 的方案 A 合并处理。

---

## 5. 计数与验收脚本复跑（真实输出）

```
$ awk '/^## 1\./,/^## 2\./' matrix.md | awk -F'|' '…'
行=13 有=17 缺=51 N/A=36 合计=104 未分类=0

$ awk '/^## 3\./,/^## 4\./' matrix.md | grep -c '^| [0-9]'
36

$ awk '/^## 4\./,/^## 5\./' matrix.md | grep -c '^| E[0-9]'
17
$ awk -F'|' '/^\| E[0-9]+ \|/ {gsub(/[^0-9]/,"",$6); s+=$6} END {print "图数合计=" s}' matrix.md
图数合计=56

$ awk '/^## 2\./,/^## 3\./' matrix.md | awk -F'|' '…'
有格=17 三档齐备=0 缺390=0 缺641=17 缺1280=1

$ awk '/^## 1\./,/^## 2\./' matrix.md | awk -F'|' '…（排除页 3）'
适用格=96 其中有=17 缺=51 N/A=28

$ paste plan.txt matrix.txt | awk '{for(i=1;i<=8;i++) if($i!=$(i+8)) …}'
  第2行 第1列 M→Y
  第2行 第2列 M→Y
  第5行 第8列 NA→M
  第9行 第1列 M→Y
  第10行 第1列 M→Y
改判格=5 一致格=99

$ python3 /tmp/v10_check.py matrix.md
通过项：
  OK  §1 行数=13
  OK  总格=104（有 17 / 缺 51 / N/A 36）
  OK  N/A 台账闭合 36/36
  OK  E 台账一一对应 17 格
  OK  E 台账路径全部可解析
未通过项：
结论：PASS（未通过 0 项）
```

V-10 是**真 PASS**，不是采信自报；但它只覆盖结构闭合与 5 个字面口径，**覆盖不到 A-1/A-2/A-3 的语义错误**，因此不能作为签收依据。这一点与上一轮一致。

---

## 6. 验收边界复核

| 边界 | 实测 | 判定 |
|---|---|---|
| 只改文档 | `git diff --name-status 3999cee fd4a9a7` → 仅 `pairs/README.md`、`r0-status-matrix-13x8.md` | 合规 |
| 不改业务代码 / CSS / HTML | `server/`、`tools/`、`prototype/`、`.github/` 零改动 | 合规 |
| 不采图 / 不造数 / 不起服务 | 无新增 PNG；无 manifest 变更；无运行时产物 | 合规 |
| 不执行 W2–W7 / W9 | 计划文件仍为 `0033a18` 版本，未改 | 合规 |
| 不处理 P1–P6 | 无新增采集脚本、无新裁决 | 合规 |
| 不改 main / 不合并 | 无 merge commit；`main` 不在本分支 | 合规 |
| 统计基线不动 | 104 / 17 / 51 / 36 / 96 / 0 全部与 `3999cee` 一致 | 合规 |
| 不擅自 PASS 待冻结格 | §2.1 五格仍为 `缺`/`N/A R04`，未改判 | 合规 |

**签收边界是否仍然成立：成立。** 本批没有被用来夹带代码、图片或统计变更；问题全部落在文档表述的准确性上。

---

## 7. `pairs/README.md` 哈希未重签的处理意见（回答重点 8）

**现状**：

```
$ sed -n '108p' docs/r0-screenshots/pairs/SHA256SUMS.txt
c329854fa3dbe55267ee816528d86329481721e7c18f51a2a925f3e38e93a28b *README.md

$ grep -v 'README.md' docs/r0-screenshots/pairs/SHA256SUMS.txt > /tmp/sums.txt
$ wc -l < /tmp/sums.txt
108
$ sha256sum -c /tmp/sums.txt --quiet; echo rc=$?
rc=0
```

**意见：处理方式可以接受，但必须补两件事。**

可以接受的理由：

1. `SHA256SUMS*.txt` 是**历史归档**，记录的是采集当时 104 张 PNG + JSON 的完整性；为了让一份说明文档的措辞变化去重签 108 条，反而会破坏「归档 = 当时状态」的语义。
2. 剔除 README 一行后 108 条全部 `rc=0`，证明**图片与元数据本体未被改动**，这是本轮最需要证明的事。
3. README 与 §9 都已写明「不重签」及其原因，读者不会被静默误导。

必须补的两件事：

1. **A-4 的具体哈希必须改**：现在写的是一个已经失效的值。要么删值留命令，要么显式标注版本。否则「如实描述」这四个字站不住。
2. **后续补一次归档校验**：建议在下次复采（W2 落地）时新增 `SHA256SUMS-5th-round.txt`，其中包含当次 README 的哈希；或在 `pairs/` 下增加一个 `README.sha256` 边车文件，每次修改 README 时同步重算。**不要**改写既有的 `SHA256SUMS.txt` / `SHA256SUMS-4th-round.txt`，也不要通过增删行来「修复」它们。

另建议：README 的复现命令统一用 `git show <commit>:<path> | sha256sum`（A-6），避免 Windows CRLF 造成「同一文件两个哈希」的争议。

---

## 8. 逐条修复要求（只改文档）

| 编号 | 优先级 | 文件:行 | 要求 |
|---|---|---|---|
| R-1 | P1 | matrix:424、:197、:436 | 按 A-1 的四行表重写页 12 的 409 归属；删除 `common.go:115` 作为页 12 来源；补 `ErrConcurrentConflict`（`member.go:284`/`:332`） |
| R-2 | P1 | matrix:438 | 按 A-2 拆成「页 2/3/10/11 无写」+「页 1 登录/条件注册」两段；补 `common.go:109–110` |
| R-3 | P1 | matrix:417、:432 | 按 A-3 补 `DELETE :2054`；补 `common.go:54`、`:59` 及对应代码链 |
| R-4 | P2 | matrix:710 | 按 A-4 删除或标注 `eda61aee…`，并给 `git show …` 管道 `sha256sum` 的复现命令 |
| R-5 | P2 | matrix:252、README:28、README:31 | 按 A-5/A-6 修正行号与 hunk 区间；复现命令改 `git show` |

**验收方式**：修完后只需重跑 §5 的六段脚本（计数不变）+ 对 R-1…R-3 的目标行做一次 `sed -n` 回读 + 对每个新引用的行号做 `grep -n` 命中检查。**不需要复采、不需要起服务、不需要动统计基线。**

---

## 9. 未解决问题（本轮不处理，仅登记）

1. **5 格待 Owner 冻结**：投资 normal、首页 permission、导出 empty、手动记账 empty、成员 empty（§2.1）。任一格改判都会移动计数，必须一次冻结。
2. **P1–P6 全部未决**：`capture-states.js`、13/12 页口径、多角色造数、投资数据路径、像素 diff 是否硬性、`style.css:331` 范围。
3. **641 视口零覆盖**：17 个 `有` 格全部缺 641；`find -name '*641*'` = 0。
4. **E4/E12 基准过期未复采**：E4 图直接显示 `0472484` 已修的「支出类型下分类=工资」；E12 受邀请入口门控改动影响。
5. **43 张页面级 PNG 无 manifest**：精确代码 SHA 不可得，只能到入库提交粒度。
6. **409 响应无 `errorCode`**：`fail()` 只回 `{code,msg}`（`common.go:22–24`），与 `落地方案:933–939`、`:946` 的机器可判枚举要求不符；无缺口编号。
7. **已删交易返回 404 而非 409**：与 `tx.go:54–56`、`PRD:593` 口径不一致；缺口编号待 Owner 指定。
8. **加入账本流程 Web 未实现**：`app.js:1698` 文案承诺「登录页选择加入账本」，但无入口、无 `members/join` 调用；`ErrBookFull` 因此对 Web 不可达（A-1 附带观察）。
9. **R0 仍 BLOCK**：上述任一项都不得因本批文档修订被关闭。

---

## 10. 诚实清单

1. **未运行任何测试**：未跑 C 套件、未起服务、未做浏览器实测；所有结论来自静态代码与文件核对。
2. **未逐张打开图片**：本轮未新增视觉判断，A-1…A-3 全部是代码/路由/调用点事实，不依赖看图。
3. **未穷举全部 409 分支**：我核对了 `common.go` 的全部 15 个 `http.StatusConflict` 映射，并逐页对照 §5.5/§5.5.1；但不能保证 13 页之外（如回收站、交易总表）没有其他可达 409——这些不属于本批矩阵对象。
4. **行号口径**：全部行号基于 `fd4a9a7` 的 LF 检出；Windows 工作树若为 CRLF，行号不变，但 `sha256sum` 会变（见 A-6）。
5. **A-5 是我上一轮的漏检**：该行号偏差在 `3999cee` 已存在，不是 DB 本轮引入。我保留这一说明，避免把旧账算到本轮。
6. **未修改被审分支**：本报告只新增 `docs/r0-t22-b1-wb-delivery-recheck-v3.md`；未改矩阵、未改 README、未改代码、未合并、未动 main。
7. **本地 main 落后远端**：本地 `main` = `dadc9a3`，远端 `main` = `b4309db`；祖先判断使用远端 SHA。

---

## 附录：本轮独立复核命令清单（可复现）

```bash
cd fastledger-r0
git fetch origin work/r0-t22-batch1 review/t22-w1-w8-recheck-v2
git checkout -b review/t22-w1-w8-recheck-v3 fd4a9a7

git log --format='%h %p %ad %s' --date=iso 3999cee..fd4a9a7
git diff --name-status 3999cee fd4a9a7
git diff --name-only 3999cee fd4a9a7 -- server/ tools/ prototype/ .github/
git log --merges --oneline 3999cee..fd4a9a7
git merge-base --is-ancestor b4309db fd4a9a7; echo rc=$?

# 计数（§7.1–7.8 原样重跑）
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '…'
awk '/^## 3\./,/^## 4\./' docs/r0-status-matrix-13x8.md | grep -c '^| [0-9]'
awk -F'|' '/^\| E[0-9]+ \|/ {…}' docs/r0-status-matrix-13x8.md
python3 /tmp/v10_check.py docs/r0-status-matrix-13x8.md

# 409 事实
grep -n "http.StatusConflict" server/internal/api/common.go
grep -n "requireActiveBookTx" server/internal/service/member.go
sed -n '284p;332p;377p' server/internal/service/member.go
grep -rn "inviteReasonBookFull" server/internal/service/member.go
grep -n "join" server/web/dist/app.js
grep -n "auth/register" server/internal/api/router.go
sed -n '109,110p' server/internal/api/common.go
sed -n '2045,2056p' server/web/dist/app.js
grep -n "ErrTxDirectionMissing\|ErrTransferGroupIncomplete" server/internal/service/tx.go
sed -n '120,122p' server/internal/service/tx_update.go

# 基准与哈希
for f in $(find docs/r0-screenshots -name '*.png' -not -path '*/pairs/*' -not -path '*/runtime/*'); do
  git log --diff-filter=A --format='%h' -- "$f" | tail -1; done | sort | uniq -c
git log -1 --format='%h A=%ad C=%cd' --date=iso 575eae3
git log -1 --format='%h A=%ad C=%cd' --date=iso 0472484
git merge-base --is-ancestor 575eae3 0472484; echo rc=$?
for c in 3999cee 2363cdc fd4a9a7; do
  git show $c:docs/r0-screenshots/pairs/README.md | sha256sum; done
sed -n '108p' docs/r0-screenshots/pairs/SHA256SUMS.txt
grep -v 'README.md' docs/r0-screenshots/pairs/SHA256SUMS.txt > /tmp/sums.txt
sha256sum -c /tmp/sums.txt --quiet; echo rc=$?
```
