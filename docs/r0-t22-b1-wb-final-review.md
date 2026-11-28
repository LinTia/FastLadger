# T22 批次 1 · WB 正式独立交付复核报告

| 项 | 值 |
|---|---|
| 复核对象 | `work/r0-t22-batch1` @ `fd4a9a77d17685379df80cf0cc2d137ad4fa8974` |
| 对比基线 | `3999ceef76bd5e4f0e2f416d3287284613c97179` |
| 对照报告（仅参考，不采信结论） | `docs/r0-t22-b1-wb-delivery-recheck-v3.md` @ `21cdd2e8ea239a47fb3dfac397027a612eb79504` |
| 复核分支 | `review/t22-b1-wb-final` |
| 复核方式 | 只读：`git ls-remote`、`git show`、`git diff`、逐行读代码、复跑统计脚本 |
| 复核日期 | 2026-11-28 |

---

## 0. 结论摘要

| 项 | 判定 | 一句话依据 |
|---|---|---|
| W1 矩阵骨架 | **PASS** | 13 行 × 10 列、`104 = 17 + 51 + 36`、适用格 96、齐备 0、缺 51 与计划预填一致（改判 5 / 一致 99） |
| W8 offline/conflict 边界 | **PARTIAL** | offline 12 格与页 5 冲突载体成立；逐页 409 语义有页 12 归属错误（3 个落点）、页 1 陈述错误、页 5 漏 DELETE 及 4 个可达 409 |
| T22 Batch 1 | **PARTIAL** | W1 达交付级，W8 未达；修正 4 项 P2 文档问题后可签收，不需复采、不需重跑统计、不需改代码 |
| 是否建议签收 | **否（暂不签收）** | A-1、A-2、A-3、A-4 为 P2 事实错误；A-5、A-6 为 P3，建议同批修 |
| R0 状态 | **仍 BLOCK** | 本批只改 W1/W8 文档，不改变第 3 条门禁；不得因本批修订宣布 T22 完成或 R0 通过 |

**分级口径**（本报告自定，与第三 AI 的 P1 分级不同）：

- **P1** = 需返工、复采、改代码或重跑测试才能关闭。
- **P2** = 事实错误或证据链不成立，只改文档即可关闭，但修前不得签收。
- **P3** = 精度、复现性、表述问题，不阻断签收。

本批**没有 P1**：六项问题全部只涉及 `docs/` 下两份 Markdown，证据图、代码、统计基线均不受影响。这与第三 AI 把 A-1/A-2/A-3 定为 P1 不同，理由见 §11。

---

## 1. 复核对象与边界

### 1.1 SHA 回读

```bash
$ git ls-remote origin refs/heads/work/r0-t22-batch1 refs/heads/review/t22-w1-w8-recheck-v3 refs/heads/main
b4309dbd481423321f6740dfad5099ad037b0e10	refs/heads/main
21cdd2e8ea239a47fb3dfac397027a612eb79504	refs/heads/review/t22-w1-w8-recheck-v3
fd4a9a77d17685379df80cf0cc2d137ad4fa8974	refs/heads/work/r0-t22-batch1
```

目标 SHA 与题述一致；本地复核分支从 `fd4a9a7` 直接创建，未改动被审分支。

### 1.2 提交链与变更范围

```bash
$ git log --format='%h %p %ad %s' --date=iso 3999cee..fd4a9a7
fd4a9a7 2363cdc 2026-11-27 17:38:07 +0800 docs(r0): pairs/README 哈希自述改为如实描述（不重签校验清单）
2363cdc 3999cee 2026-11-27 17:37:35 +0800 docs(r0): 修正 T22 批次1 复核 v2 的 6 项 P2 与可安全修正的 P3

$ git diff --shortstat 3999cee..fd4a9a7
 2 files changed, 86 insertions(+), 50 deletions(-)

$ git diff --name-status 3999cee..fd4a9a7
M	docs/r0-screenshots/pairs/README.md
M	docs/r0-status-matrix-13x8.md
```

- 仅两份文档变更，`server/`、`tools/`、`.github/`、全部 PNG 零改动。
- 无 merge commit；`fd4a9a7^ = 2363cdc`、`fd4a9a7^^ = 3999cee`，非 amend、非 force。
- `docs/r0-t22-execution-plan.md` 自 `0033a18` 起零改动。

### 1.3 main 状态说明

本地 `main` 为 `dadc9a3`（落后 `origin/main` 33 个提交），远端 `main` 为 `b4309db`；`dadc9a3` 是 `b4309db` 的祖先。本次复核未触碰 `main`，该差异为既有状态。

---

## 2. A-1：页 12 成员管理的 409 归属

**判定：真实缺陷，P2。三处落点全部错误，且错误方向相反。**

### 2.1 文档位置

- `docs/r0-status-matrix-13x8.md:424`（§5.5 第 12 行）：409 来源写 `common.go:115`，语义写「成员上限 20（容量类）」。
- `docs/r0-status-matrix-13x8.md:197`（§3.2 第 33 行，R08-12）：重复同一说法。
- `docs/r0-status-matrix-13x8.md:436`（§5.5.1 页 12 bullet）：「`PUT` / `DELETE members/:uid`：`common.go:51`（账本已删/不可用，`member.go:234`）」。

### 2.2 实际代码

**(a) `PUT`/`DELETE` 不调用 `requireActiveBookTx`：**

```bash
$ grep -n "requireActiveBookTx" server/internal/service/member.go
200:	if err := requireActiveBookTx(sess, bookID); err != nil {   # List（GET）
377:	if err := requireActiveBookTx(sess, bookID); err != nil {   # CreateInvite（POST）
459:			if aerr := requireActiveBookTx(sess, bookID); aerr != nil {  # RevokeInvite
834:	if err := requireActiveBookTx(sess, inv.BookId); err != nil {    # JoinViaInvite
```

（`#` 后为本报告注释，不是命令输出。）

`ChangeRole`（`member.go:240`）与 `Remove`（`member.go:299`）都不在列。

**(b) 两者的 409 来自 0 行更新：**

```go
// member.go:284（ChangeRole）
return fmt.Errorf("%w：目标成员已不存在或已被移除", ErrConcurrentConflict)
// member.go:332（Remove）
return fmt.Errorf("%w：目标成员已不存在或已被移除", ErrConcurrentConflict)
```

`common.go:64–65` 把 `ErrConcurrentConflict` 映射为 409「数据已被并发修改，请重试」。

**(c) 账本已删时实际是 403，不是 409：**

```go
// guard.go:37–45
SELECT m.role FROM book_members m
 INNER JOIN books b ON b.id = m.book_id AND b.deleted = 0
 WHERE m.book_id = ? AND m.uid = ? AND m.deleted = 0
...
if !has { return 0, ErrNotMember }
```

`common.go:43–46` 把 `ErrNotMember` 映射为 **403 无权限**。源码注释亦明确：

```go
// member.go:454–455
// ① 账本已删除 —— 与 ChangeRole / Remove 保持同一口径：
//    由 roleOfTx 的 JOIN 天然拦下，对外仍是 ErrNotMember。
```

**(d) `ErrBookFull` 只由兑换邀请产生，Web 不调用该接口：**

```bash
$ awk 'NR>=353 && NR<=440' server/internal/service/member.go | grep -c "ErrBookFull"
0
$ grep -n "inviteReasonBookFull" server/internal/service/member.go
590:	case inviteReasonBookFull:     # inviteRejectError → ErrBookFull → common.go:115–117
891:		inviteReasonBookFull, ...    # JoinViaInvite 成员数达到上限
$ grep -n "members/join" server/internal/api/router.go
128:	v1.POST("/books/:id/members/join", bh.join)
$ grep -n "join" server/web/dist/app.js
1086:    const q = new URLSearchParams({ format, fields: fields.join(",") });
```

`members/join` 在前端零调用（`:1086` 是 `Array.join`）。

### 2.3 正确的归属表

| 写调用 | 可达 409 / 403 | 代码链 | 语义 |
|---|---|---|---|
| `POST members/invite` `app.js:1689` | 409 `ErrEntityDeleted` | `member.go:377` → `:234` → `common.go:51–53` | 账本已删/不可用 |
| `PUT members/:uid` `app.js:1761`、`:1771` | 409 `ErrConcurrentConflict` | `member.go:284` → `common.go:64–65` | 目标成员已被移除 |
| `DELETE members/:uid` `app.js:1786` | 409 `ErrConcurrentConflict` | `member.go:332` → `common.go:64–65` | 目标成员已被移除 |
| 同上两条，若**账本**已删 | **403 `ErrNotMember`** | `guard.go:39` JOIN → `:45` → `common.go:43–46` | 无权限，不暴露账本状态 |
| `POST members/invite/:inviteId/revoke`（Web 未调用） | 403 | `member.go:449–462` | 统一表现为无权限 |
| `POST members/join`（Web 未调用） | 409 `ErrBookFull` | `member.go:591–593` → `common.go:115–117` | 成员上限 20 |

### 2.4 影响

- **不影响统计与判定**：页 12 的 conflict 仍为 `N/A R08-12`，理由「四条写调用均不传版本」独立成立；`17/51/36` 不变。
- **影响排障方向**：按现文档，改角色/移除成员失败会被读成「账本已删」；实际应查「目标成员是否已被他人移除」，而账本已删只会得到 403。方向完全相反。
- **影响证据可信度**：同一页三个落点重复同一错误，且 `common.go:115` 对 Web 的四条写调用全部不可达。

### 2.5 修复要求

1. `:424` 的 409 来源删除 `common.go:115`，改为按 §2.3 分行列出（至少区分 invite 与 PUT/DELETE）。
2. `:197` 的 R08-12 理由句同步改写，不得再写「成员上限 20」作为页 12 的 409 来源。
3. `:436` 的页 12 bullet 拆成「invite → 409 `common.go:51–53`」与「PUT/DELETE → 409 `common.go:64–65`，账本已删则 403 `common.go:43–46`」两段。
4. 验收：三处均不再出现「PUT/DELETE → `common.go:51`」或「页 12 → `common.go:115`」的因果；`grep -n "common.go:115" docs/r0-status-matrix-13x8.md` 的剩余命中只能是 join/未调用说明。

---

## 3. A-2：页 1 的「无写路径」

**判定：真实缺陷，P2。结论（conflict=N/A）正确，但支撑句是错的，且与同文件自相矛盾。**

### 3.1 文档位置

- `docs/r0-status-matrix-13x8.md:438`：「页 1/2/3/10/11 无写路径，无其他 409。」
- 同一文件 `:413` 却写：「页 1 登录：`POST /auth/login` `:193`、`:209`」。

### 3.2 实际代码

```js
// app.js:192
await api("POST", "/auth/register", { username: u.value, password: p.value });
// app.js:224
toggle.textContent = isRegister ? "已有账号？返回登录" : "没有账号？注册新账号";
```

```go
// router.go:124
v1.POST("/auth/register", ah.register)
// auth_handler.go:51–53
if !h.enableRegister {
    fail(c, http.StatusForbidden, "已关闭公开注册，请使用邀请码加入")
    return
}
// config.go:147
enableRegister, err := envBool("EBK_ENABLE_REGISTER", false)
// auth.go:239
return nil, ErrUserExists
```

`common.go:109–110` 把 `ErrUserExists` 映射为 **409「用户名已存在」**。

即：注册是**条件可达**——默认 `EBK_ENABLE_REGISTER=false` 时返回 403；开启后用户名重复返回 409。它不是「无写路径」；登录本身也是 POST 调用（不是只读 GET）。

### 3.3 影响

- 不影响页 1 的 conflict 判定：登录/注册都不参与实体版本比对，`N/A R08-01` 成立。
- 但「无写路径」是事实错误，会让后续复核者跳过整页 409 检查；该页恰有一个真实 409（用户名已存在）。

### 3.4 修复要求

把 `:438` 拆成两条：

- 页 2/3/10/11 无写路径，无其他 409。
- 页 1 有 `POST /auth/login`（`:193`、`:209`）与条件可达的 `POST /auth/register`（`:192`，需 `EBK_ENABLE_REGISTER=true`；重复用户名 → `common.go:109–110` 409）；conflict 仍判 `N/A R08-01`。

验收：`grep -n "页 1/2/3/10/11 无写路径" docs/r0-status-matrix-13x8.md` 零命中。

---

## 4. A-3：页 5 交易详情的 DELETE 写调用与 409

**判定：真实缺陷，P2。写调用列漏一整条删除路径；该路径的 409 全部未登记。**

### 4.1 文档位置

- `docs/r0-status-matrix-13x8.md:417`（§5.5 第 5 行）：写调用只列 `PUT /transactions/:id` `:664`。
- `docs/r0-status-matrix-13x8.md:432`（§5.5.1 页 5 bullet）：只写 `PUT` 的 `common.go:62`、`:49`。

### 4.2 实际代码

```js
// app.js:2054（renderTxDetail，函数体 2008–2102）
await api("DELETE", "/books/" + bookId + "/transactions/" + tx.id + "?clientUuid=" + encodeURIComponent(uuid()));
```

```go
// tx.go:748–750
func (s *TxService) Delete(...) { ver, _, err := s.mutateVisibility(..., true, ...) }
// tx.go:836
return 0, false, ErrIncompleteReceipt
// tx.go:844
group, err := collectTransferGroup(sess, bookID, row, wantDeleted)
// tx.go:864
return 0, false, fmt.Errorf("%w：转账记录 %d 缺少方向标记，...", ErrTxDirectionMissing, r.Id)
// tx.go:976（replayMutateReceipt）
return 0, ErrCreateConflict
```

对应 409：

| 错误 | 映射 | DELETE 触发点 | 文档是否登记 |
|---|---|---|---|
| `ErrCreateConflict` | `common.go:49` | `tx.go:976`（同 clientUuid 不同内容） | 只登记为 PUT 的 `:49`，未说明 DELETE 同样可达 |
| `ErrIncompleteReceipt` | `common.go:62` | `tx.go:836` | 同上 |
| `ErrTxDirectionMissing` | `common.go:54–58` | `tx.go:864` | **完全未登记** |
| `ErrTransferGroupIncomplete` | `common.go:59–61` | `tx.go:844` → `tx.go:1077/1083/1087/1100/1120…` | **完全未登记** |

### 4.3 影响

- 页 5 是 13 页中唯一有版本冲突载体的页，也是 409 分支最多的页；写调用列漏 DELETE，会让读者误以为详情页只能编辑不能删除。
- `§5.5.1` 的标题是「其他可达 409」，但页 5 只按 `PUT` 列了两项；DELETE 另有四项（49/62/54/59），其中 54/59 完全未登记，49/62 只被写成 PUT 的来源。
- 判定不变：页 5 conflict 仍为 `缺（有版本冲突载体）`。

### 4.4 修复要求

1. `:417` 写调用列补 `DELETE /transactions/:id` `:2054`。
2. `:432` 拆成 PUT 与 DELETE 两行：PUT 保留 `:49`、`:62`；DELETE 补 `:49`（`tx.go:976`）、`:62`（`tx.go:836`）、`:54`（`tx.go:864`）、`:59`（`tx.go:844`）。
3. 验收：页 5 行内同时出现 `PUT` 与 `DELETE`；`§5.5.1` 页 5 段出现 `common.go:54` 与 `:59`。

---

## 5. A-4：README 修改后的 SHA256SUMS 失效记录

**判定：真实缺陷，P2（低影响）；归档处理策略本身合理，但登记值不能写「实测现为」。**

### 5.1 三个提交的 README 实际哈希（LF blob）

```bash
$ for c in 3999cee 2363cdc fd4a9a7; do printf "%s  " $c; git show $c:docs/r0-screenshots/pairs/README.md | sha256sum; done
3999cee  eda61aee70a6d8a51bf2bce558e9b00a7e6869f2b862492af95079f4b051539c
2363cdc  6ecb6789932756ecd5dce8088ace69bda3e7765881a5d155b6fc93f32bfe71ed
fd4a9a7  987409085f3af142935c265a131cfcf20271a0d66f11beee7ec4445f517bf30f
```

### 5.2 文档记录

- `matrix:710`（§9 第 11 条）：「原 `c329854f…`，**实测现为** `eda61aee70a6…`」。
- `SHA256SUMS.txt:108` = `c329854fa3dbe55267ee816528d86329481721e7c18f51a2a925f3e38e93a28b *README.md`。
- `SHA256SUMS-4th-round.txt:108` = `43123890… *README.md`。

### 5.3 问题

1. `eda61aee` 是 **`3999cee` 版本**的哈希，不是 `fd4a9a7`；在 `:710` 所在提交 `2363cdc` 里，README 已经是 `6ecb6789`。也就是说，**写入这句话的同一提交内，值就已失效**。
2. 当前 HEAD 的正确值是 `98740908…`。
3. 该句是全文件唯一声称「实测现为」的地方，读者会把它当成 HEAD 值复现，必然失败。

### 5.4 归档处理是否合理

**合理，应保留：**

- 未改写 `SHA256SUMS*.txt`（历史归档不重签），符合「不改既有证据」的边界。
- 剔除 README 一行后 108 条全部通过：

```bash
$ grep -v 'README.md' docs/r0-screenshots/pairs/SHA256SUMS.txt > /tmp/sums_now.txt
$ wc -l < /tmp/sums_now.txt
108
$ (cd docs/r0-screenshots/pairs && sha256sum -c /tmp/sums_now.txt --quiet); echo rc=$?
rc=0
```

- 清单构成：104 PNG + 3 JSON + 1 TXT = 108 条证据，加 README 自身 1 条 = 109 行。

**但必须补两点：**

1. `:710` 删除「实测现为 `eda61aee…`」，改为「`3999cee` 版为 `eda61aee…`；`2363cdc` 版为 `6ecb6789…`；`fd4a9a7` 版为 `98740908…`；复现命令：`git show <commit>:docs/r0-screenshots/pairs/README.md | sha256sum`」。
2. 说明这是「声明文件自指」问题，不是图片变化；下次复采新增 `SHA256SUMS-5th-round.txt`，不改写既有两份。

### 5.5 是否需要后续单独补归档校验

**需要，但不属本批。** 建议在 W2 复采时新增第五轮清单，并在 README 中固定「历史清单只增不改」的规则；本轮不重签是正确选择。

---

## 6. A-5：文档中的代码行号

**判定：真实但不阻断，P3。行号落在正确函数内，但不是精确改动行；README 一处是本轮新增。**

### 6.1 实际改动行

```bash
$ git show 0472484 --unified=0 -- server/web/dist/app.js | grep "^@@"
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

即：`renderTxns` 改动行为 `386`、`400–420`；`openTxForm` 为 `559–571`、`576`、`599`；`renderMembers` 为 `1709`、`1715–1718`、`1744`。

### 6.2 文档写法与偏差

| 位置 | 文档写法 | 实际 | 偏差 |
|---|---|---|---|
| `matrix:252`、`README:28` | `renderTxns` `:383`、`:397–423` | `:386`、`:400–420` | 起止各偏 3 行，覆盖到改动但非精确行 |
| `matrix:252` | `openTxForm` hunk `:550–578` | `:559–571`、`:576`、`:599` | 漏 `:599`（`请选择账户与分类` 校验） |
| `matrix:252` | `renderMembers` `:1706–1741` | `:1709`、`:1715–1718`、`:1744` | 覆盖角色门控，漏 `:1744`（`myUid`） |

三处都落在对应函数内（`renderTxns` 起于 `:337`、`openTxForm` 起于 `:548`、`renderMembers` 起于 `:1667`），因此**不是「指向别的函数」**，而是区间不精确。

### 6.3 来源判定

- 该写法在 `3999cee` 的矩阵中已存在（该版 `:253`），属**历史遗留**，非本轮引入。
- `README:28` 在 `3999cee` 的 README 中不存在（该文件当时无 `:383` 字样），是本轮 `2363cdc` 新增。**因此第三 AI「该偏差在 3999cee 已存在」只对矩阵成立，对 README 不成立。**

### 6.4 修复要求

1. 统一改为精确行：`386`、`400–420`；`559–571`、`576`、`599`；`1709`、`1715–1718`、`1744`。
2. 或保留「hunk 区间」写法，但必须标注「hunk 为 diff 上下文窗口，改动行为 …」，并补 `:599`。
3. 验收：`sed -n '386p' app.js` 为 `view.append(card)`；`sed -n '599p'` 含 `请选择账户与分类`；`sed -n '1744p'` 含 `myUid`。

---

## 7. A-6：Windows CRLF 下的 README 哈希复现命令

**判定：真实，非阻断，P3。**

### 7.1 文档写法

`README:31`：「实测值请用 `sha256sum docs/r0-screenshots/pairs/README.md` 取得。」
`README:184`：「`SHA256SUMS.txt` 基于 LF 工作树计算。Windows 上 `core.autocrlf=true` 的 clone 会把 `*.json` / `*.md` / `*.txt` 检出为 CRLF，文本文件哈希据此不同。」

两条自相矛盾：前者让读者对工作树取哈希，后者承认工作树可能是 CRLF。

### 7.2 实测

```bash
$ git config --get core.autocrlf
true
$ cat .gitattributes
cat: .gitattributes: No such file or directory

$ git show fd4a9a7:docs/r0-screenshots/pairs/README.md | sha256sum
987409085f3af142935c265a131cfcf20271a0d66f11beee7ec4445f517bf30f
$ sha256sum docs/r0-screenshots/pairs/README.md
57b12d4d9d9c87bb54e3cdb453263f249b53cb292f62e5443b8edfc2629d1a69
```

| 版本 | 字节 | 换行 | SHA-256 |
|---|---|---|---|
| Git blob | 13,301 | 184 LF | `98740908…` |
| 工作树 | 13,485 | 184 CRLF | `57b12d4d…` |

同一文件两个哈希，差 184 字节，全部来自 CRLF。

### 7.3 影响

- PNG 不受影响，`sha256sum -c` 的 108 条仍通过。
- 只有 README 自身的哈希无法按文档命令复现；这正好是 A-4 涉及的唯一文件。
- 不影响任何格判定与统计。

### 7.4 修复要求

把 `README:31` 的命令改为：

```bash
git show HEAD:docs/r0-screenshots/pairs/README.md | sha256sum
```

并注明「输出为 LF blob 哈希；工作树若为 CRLF，`sha256sum <文件>` 会得到另一个值」。

验收：在 `core.autocrlf=true` 的工作树执行上述命令，结果与文档登记值一致。

---

## 8. N-11 至 N-25 复核

| 编号 | 要求 | 实际状态 | 证据 |
|---|---|---|---|
| N-11 | runtime 02/04 骨架条数改 4 | **已修** | `matrix:96/478/479/481` 均写 4 + `skelCount=4`；`dom-snapshots.json` 01–04 全为 4，05 为 0 |
| N-12 | 页 13 邮箱 409 挂到 `PUT /users/me` | **已修** | `matrix:425` 已补 `:2206`；`§3.2` 第 36 行（`:200`）已改；`ErrEmailTaken` 唯一返回点 `profile.go:53` |
| N-13 | 页 8 补 `DELETE /instruments/:id` | **已修** | `matrix:420` 已补 `:1412` 并注「仅 DELETE 标的时」；`ErrInstrumentInUse` 唯一返回点 `invest.go:192` |
| N-14 | 逐页 409 补齐或声明非穷举 | **部分修** | 表头（`:409`）已改「主要分支，非穷举」，新增 §5.5.1；但页 12 归属错误（A-1）、页 5 DELETE 缺失（A-3）、页 1 陈述错误（A-2） |
| N-15 | 登记编辑已删交易 404 | **已修** | `matrix:448` 事实与链路正确：`tx_update.go:111` → `tx.go:1044–1057` → `common.go:72–73` 404 |
| N-16 | 43 张基准由 mtime 改入库提交 | **已修** | `matrix:212–228/255/705` 全部改 `575eae3`；43/43 PNG 均由 `575eae3` 入库，且 `575eae3` 是 `0472484` 祖先（`merge-base rc=0`） |
| N-17 | E3/E15 的 `557e4d2` 标注为文档 HEAD | **已修** | `matrix:214/226`：「文档 HEAD `557e4d2`；对应代码 SHA 未独立验证」 |
| N-18 | 合计行移出表格、补第三数 | **已修** | `matrix:87` 脚注「104 = 17 + 51 + 36」；`§2.1` 写「18/50/36 或 17/51/36」 |
| N-19 | 641 断点解释 | **已修** | `matrix:128`：「641 落在 `style.css:334` 的 641–768 带内」 |
| N-20 | 函数区间收窄 | **部分修** | `README:27`、`matrix:252` 已把 `refillCategories()` 收窄为 `560–571`；但 `openTxForm` 的 hunk 仍写 `550–578`，漏 `:599`（A-5） |
| N-21 | G4 删除 `233` 注释行 | **已修** | `matrix:300–306` 实际输出仅 `98/234/235`，并说明中文注释不含 ASCII `sync` |
| N-22 | §10 落点 `§2.4` → `§5.4` | **已修** | `matrix:734` 已改 `§5.4`；全文无 `§2.4` 引用 |
| N-23 | 页 3 七行「同上」逐行写 | **已修** | `matrix:172–178` 每行写明「整页无 Web 路由…同样不适用」 |
| N-24 | README 行数 108 → 109 | **已修** | `README:182`：「109 行 = 108 条证据 + 本文件」；实测 `wc -l` = 109 |
| N-25 | 规则 3 区分原型/实现成员数 | **已修** | `matrix:41`：「原型 2 名，实现侧仅 owner 1 名」；`dataset.counts.family=2` 标明为原型侧 |

**汇总：13 项已修（含 N-15），2 项部分修（N-14、N-20）。**

关于 N-15 的语境建议：`PRD:593` 的原文场景是「旧离线数据回流」，而 `tx.go:54–59` 的设计注释是通用表述（「已删是冲突，必须与目标不存在 404 区分」）。文档写「与设计注释/PRD 的 409 口径相反」成立，但建议注明「PRD 该条针对离线回流；更新路径的直接依据是设计注释」，避免被读成 PRD 对所有编辑路径的强制要求。这是表述收窄，不是事实错误。

---

## 9. 矩阵统计复跑

```bash
# §1 三计数
行=13 有=17 缺=51 N/A=36 合计=104 未分类=0

# §1 列数（NF=12，即 10 个逻辑列）
12

# §2 齐备度
有格=17 三档齐备=0 缺390=0 缺641=17 缺1280=1

# 适用分母（剔除页 3）
适用格=96 其中有=17 缺=51 N/A=28

# §3 N/A 行
36

# §4 证据行 / 图数
17
图数合计=56
```

与计划 §3 预填逐格比对：

```bash
改判格=5 一致格=99
 第2行 第1列 M→Y（首页 normal）
 第2行 第2列 M→Y（首页 loading）
 第5行 第8列 NA→M（交易详情 conflict）
 第9行 第1列 M→Y（预算 normal）
 第10行 第1列 M→Y（报表 normal）
```

**结论：统计基线完全保持 `104 = 有 17 + 缺 51 + N/A 36`、适用 96、三档齐备 0。** 六项 A 问题均未移动任何计数。

---

## 10. W1 / W8 / Batch 1 判定

### 10.1 W1：PASS

计划验收四条逐条核对：

| 验收项 | 要求 | 实测 |
|---|---|---|
| 13 行齐备 | 13 页 | 13 行 |
| 每行 10 列 | 2 定位 + 8 状态 | `NF=12`（首尾空字段）= 10 逻辑列 |
| 三计数之和 = 104 | 有 + 缺 + N/A | 17 + 51 + 36 = 104 |
| 缺格与 §3 预填一致 | 逐格 | 改判 5 / 一致 99；5 格均有 §1.1 依据 |

补充：36 条 N/A 逐格有理由（R01 × 8、R02/R03/R04/R05/R06 各 1、R07 × 12、R08 11 码各 1）；17 条证据行、56 张图，张数等式 `4+4+2+4+4+6+2+2+2+2+2+2+6+4+2+4+4 = 56` 成立。

**W1 达交付级。**

### 10.2 W8：PARTIAL

**成立的部分：**

- offline 12 格全部 `N/A R07/R0-E-15`，三要素（无载体举证 / 产品有要求 / 缺口未闭环）齐全。
- 前端 0 载体复跑一致：

```text
WebSocket 0 / ws:// 0 / wss:// 0 / sync/pull 0 / sync/push 0
navigator.onLine 0 / serviceWorker 0 / EventSource 0 / BroadcastChannel 0
If-Match 0 / ETag 0 / pendingOps 0 / conflict 0
sync 总命中 61，其中 async 57，非 async 仅 4（:556/:572/:578 的 syncFormFields + :655 的 tx.syncVersion）
```

- 后端确有路由（`router.go:133` WS、`:234–235` sync pull/push），文档未写成「Web 已接通」。
- 页 5 冲突链完整：`app.js:655 baseVersion` → `:664 PUT` → `tx_update.go:130–135` 比对 → `common.go:64` 409 → `app.js:670` 提示。
- 两列合计 26 = 24（12 页 × 2）+ 2（页 3），`N/A 25 + 缺 1` 恒等。

**不成立的部分：**

- §5.5 / §3.2 / §5.5.1 页 12 的 409 归属（A-1）三处错误；
- §5.5.1 页 1「无写路径」（A-2）；
- §5.5 页 5 漏 DELETE 及 4 个可达 409（A-3）。

W8 的核心交付是「边界书面化 + 24 格 N/A + grep 留证」，这部分成立；但文档把「逐页 409 语义」作为支撑证据一并交付，该子表存在事实错误，故不能判 PASS。

**补充说明**：计划 W8 的字面验收写「offline/conflict 列全部为 N/A」，实际页 5 conflict 判 `缺`。这是 F-06 要求的修正（有版本冲突载体就不能判 N/A），属**改进而非违规**，不应反过来扣分。

### 10.3 T22 Batch 1：PARTIAL

| 维度 | 结果 |
|---|---|
| 边界 | 只改两份文档；未动代码、PNG、runner、fixture、CI；未合并、未动 main |
| W1 | PASS |
| W8 | PARTIAL |
| 统计 | 104/17/51/36、96、0 全部保持 |
| 提交规范 | 两个独立提交，无 amend、无 force、无 merge |
| 结论 | **PARTIAL，不建议签收** |

---

## 11. 与第三 AI 报告的分歧

第三 AI 的事实核查大部分正确，以下五点我不采纳或需要修正：

1. **分级过重**：A-1/A-2/A-3 被定为 P1。按「P1 = 需返工/复采/改代码」的口径，这三项都只需改 Markdown，应为 P2。这不改变「不签收」结论，但会影响 Owner 判断是否需要重开采集。
2. **A-1 少一层**：第三 AI 说「PUT/DELETE 的 409 是 `ErrConcurrentConflict`」，正确；但没有指出**账本已删时实际是 403**。反证：`guard.go:39` 的 `INNER JOIN books ... b.deleted = 0`、`guard.go:45` 的 `ErrNotMember`、`common.go:43–46` 的 403，以及 `member.go:454–455` 的源码注释「与 ChangeRole / Remove 保持同一口径：由 roleOfTx 的 JOIN 天然拦下，对外仍是 ErrNotMember」。因此文档写 `common.go:51` 不只是「错误码挂错」，而是「状态码也不可能是 409」。
3. **A-3 漏两项**：第三 AI 只列 `common.go:54`、`:59`。实际上 DELETE 还会返回 `common.go:49`（`tx.go:976` 回执哈希不符）与 `common.go:62`（`tx.go:836` 回执不完整）；这两项在 §5.5.1 里被写成「PUT 的可达 409」，但 DELETE 同样可达。
4. **A-5 的来源判断只对一半**：第三 AI 说「该行号偏差在 `3999cee` 已存在」。矩阵 `:253` 确实已存在；但 `README:28` 在 `3999cee` 的 README 中不存在，是本轮 `2363cdc` 新写入的。把 README 实例也算作「上轮漏检」不准确。
5. **「真正的账本已删 409 只在签发邀请时出现」不准确**：`GET /books/:id/members`（`member.go:200` → `:234`）也会返回 409；只是它是读路径，不在「写调用」列。作为排障结论应写「只在 invite 与 list 出现」。

另有一处证据可以加强：第三 AI 引用 `config_infra_test.go:242` 证明注册默认关闭；实际默认值就在 `config.go:147`（`envBool("EBK_ENABLE_REGISTER", false)`），更直接。

---

## 12. 可直接交给 DB 的修复要求与验收标准

> 全部只改 `docs/r0-status-matrix-13x8.md` 与 `docs/r0-screenshots/pairs/README.md`；不改代码、不改图、不改统计。

| 编号 | 级别 | 位置 | 动作 | 验收 |
|---|---|---|---|---|
| R-1 | P2 | `matrix:424`、`:197`、`:436` | 按 §2.3 重写页 12 的 409 归属：invite → `common.go:51–53`；PUT/DELETE → `common.go:64–65`；账本已删 → 403 `common.go:43–46`；删除 `common.go:115` 作为页 12 来源 | `grep -n "common.go:115" docs/r0-status-matrix-13x8.md` 只剩 join/未调用说明；三处均无「PUT/DELETE → 51」 |
| R-2 | P2 | `matrix:438` | 拆成「页 2/3/10/11 无写」与「页 1 登录 + 条件注册」两条，补 `common.go:109–110` | `grep -n "页 1/2/3/10/11 无写路径"` 零命中 |
| R-3 | P2 | `matrix:417`、`:432` | 页 5 写调用补 `DELETE :2054`；§5.5.1 拆 PUT/DELETE，DELETE 补 `:49`、`:62`、`:54`、`:59` | 页 5 行含 `DELETE`；§5.5.1 页 5 段含 `common.go:54` 与 `:59` |
| R-4 | P2 | `matrix:710` | 删「实测现为 `eda61aee…`」，改为三版本哈希 + `git show` 复现命令 | `grep -n "实测现为" docs/r0-status-matrix-13x8.md` 不再指向 README 哈希；命令输出 = 登记值 |
| R-5 | P3 | `matrix:252`、`README:28` | 行号改 `386`、`400–420`；`559–571`、`576`、`599`；`1709`、`1715–1718`、`1744`；或注明 hunk 为上下文窗口并补 `:599` | `sed -n '386p;599p;1744p' app.js` 分别命中 `view.append(card)`、`请选择账户与分类`、`myUid` |
| R-6 | P3 | `README:31` | 复现命令改 `git show HEAD:docs/r0-screenshots/pairs/README.md \| sha256sum`，注明 CRLF 差异 | 在 `core.autocrlf=true` 下输出与登记值一致 |

**签收条件**：R-1 至 R-4 全部完成（P2），R-5/R-6 建议同批；完成后只需复核这六个落点，**不需复采、不需重跑 C 套件、不需改统计**。

**维持不变**：`有 17 / 缺 51 / N/A 36`、适用 96、三档齐备 0、5 格待 Owner 冻结、R0 仍 BLOCK。

---

## 13. 诚实清单与边界

1. **未运行任何测试**：未跑 C 套件、未起服务、未做浏览器实测；全部结论来自静态代码与 Git 对象。
2. **未打开任何 PNG**：本轮未做视觉复核；骨架条数采信 `dom-snapshots.json` 的 `skelCount`（01–04 均为 4，05 为 0），并复跑了 W8 的 grep 计数。
3. **未修改被审分支**：`work/r0-t22-batch1` 仍为 `fd4a9a7`；未合并、未动 `main`；未执行 W2–W7/W9；未处理 P1–P6。
4. **行号口径**：全部行号基于 `fd4a9a7` 的 LF blob；Windows 工作树为 CRLF，但行号不变，仅 `sha256sum` 变化（见 A-6）。
5. **W8 的部分判断依赖文档自述**：`runtime` 五张的注入方式与时间未重新执行，仅复核 JSON 与 README 记录；这属「证据不足」而非「已证伪」，不影响本次 PARTIAL 结论。
6. **第三 AI 报告的定位**：仅用于对照；A-1 至 A-6 的每条结论均由本轮独立命令重新取得，未直接引用其证据。
7. **未做的事**：未评估 P1–P6 的取舍，未对 5 格待冻结项提出改判，未对 R0 第 3 条之外的八条门禁做任何状态变更。

---

## 附录：关键命令索引

```bash
# 目标与基线
git ls-remote origin refs/heads/work/r0-t22-batch1
git log --format='%h %p %ad %s' --date=iso 3999cee..fd4a9a7
git diff --shortstat 3999cee..fd4a9a7

# A-1
grep -n "requireActiveBookTx" server/internal/service/member.go
sed -n '284p;332p' server/internal/service/member.go
sed -n '35,47p' server/internal/service/guard.go
sed -n '43,46p;51,53p;64,65p;115,117p' server/internal/api/common.go
grep -n "members/join" server/internal/api/router.go

# A-2
sed -n '192p;224p' server/web/dist/app.js
sed -n '49,54p' server/internal/api/auth_handler.go
sed -n '147p' server/internal/config/config.go
sed -n '109,110p' server/internal/api/common.go

# A-3
sed -n '2045,2056p' server/web/dist/app.js
sed -n '748,757p;836p;844p;864p;976p' server/internal/service/tx.go
sed -n '49p;54p;59p;62p' server/internal/api/common.go

# A-4 / A-6
for c in 3999cee 2363cdc fd4a9a7; do git show $c:docs/r0-screenshots/pairs/README.md | sha256sum; done
sed -n '108p' docs/r0-screenshots/pairs/SHA256SUMS.txt
git config --get core.autocrlf

# 统计（完整脚本见 §9；核心是 §1 的 13 行 × 8 状态列累加）
awk '/^## 1\./,/^## 2\./' docs/r0-status-matrix-13x8.md | awk -F'|' '/^\| [0-9]+ \|/ {for(i=4;i<=11;i++){c=$i; sub(/^[ \t]+/,"",c); if(c ~ /^N\/A/) n++; else if(c ~ /^有/) h++; else m++}} END {printf "有=%d 缺=%d N/A=%d 合计=%d\n",h,m,n,h+m+n}'
```
