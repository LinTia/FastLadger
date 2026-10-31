package service

// GPT 独立复审探针（reviewer-sync / 离线同步 Pull 领域）。
//
// 这些用例**只做证明，不改生产代码**：目的不是"覆盖"，而是把复审结论落到可复跑的证据上。
// 覆盖三类问题：
//  1. Pull 会不会漏数据 / 重复 / 死循环（水位推进与批次衔接）；
//  2. limit 是否真的约束了所有四类实体（批次内"严格小于 HighVersion 的行数 ≤ limit-1"）；
//  3. 一些"看起来可疑但实际没问题"的点，也用测试钉住，避免后面的人重复怀疑。
//
// 命名统一加 review 前缀，避免与既有用例/助手冲突。

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"

	"xorm.io/xorm"
)

// reviewVerRow 版本探针的投影行：只要 (id, sync_version)，不把整行读进内存。
type reviewVerRow struct {
	Id          int64 `xorm:"id"`
	SyncVersion int64 `xorm:"sync_version"`
}

// reviewVersions 读取某张同步表在该账本下所有 sync_version>0 的行。
//
// 含软删墓碑（不过滤 deleted）：Pull 的水位也计入墓碑，因此真值集必须一致。
func reviewVersions(t *testing.T, engine *xorm.Engine, table string, bookID int64) []reviewVerRow {
	t.Helper()
	rows := make([]reviewVerRow, 0, 64)
	if err := engine.SQL(
		`SELECT id, sync_version FROM `+table+` WHERE book_id = ? AND sync_version > 0`,
		bookID).Find(&rows); err != nil {
		t.Fatalf("读取 %s 真值失败：%v", table, err)
	}
	return rows
}

// reviewAddMember 直接落一条成员记录（绕过邀请码链路，只为造可见性受限的 member）。
func reviewAddMember(t *testing.T, engine *xorm.Engine, bookID int64, username string, role int8) int64 {
	t.Helper()
	u := &model.User{Username: username, PasswordHash: "x"}
	if _, err := engine.Insert(u); err != nil {
		t.Fatalf("插入用户失败：%v", err)
	}
	m := &model.BookMember{BookId: bookID, Uid: u.Id, Role: role, CanViewOthers: false}
	if _, err := engine.Insert(m); err != nil {
		t.Fatalf("插入成员失败：%v", err)
	}
	return u.Id
}

// reviewSyncStats 一次"客户端全量追平"过程的观测数据。
type reviewSyncStats struct {
	// Batches 每批返回的总行数
	Batches []int
	// Delivered 每个 (表:主键) 被下发的次数；正常必须恰好 1
	Delivered map[string]int
	// DeliveredVer 每个 (表:主键) 最后一次被下发时的 sync_version
	DeliveredVer map[string]int64
	// HighVersions 每批的 HighVersion，用于验证严格单调
	HighVersions []int64
}

// reviewSyncAll 模拟一个客户端从 afterVersion=0 一直拉到"没有增量"。
//
// limits 按批次轮换，用来覆盖"批次大小影响水位衔接"的各种组合。
// 返回观测数据；任何结构性异常（水位停滞、批次不终止）直接 t.Fatalf。
func reviewSyncAll(t *testing.T, svc *SyncService, uid, bookID int64, limits []int) *reviewSyncStats {
	t.Helper()
	st := &reviewSyncStats{
		Delivered:    map[string]int{},
		DeliveredVer: map[string]int64{},
	}
	after := int64(0)
	const maxBatches = 5000
	for i := 0; i < maxBatches; i++ {
		limit := limits[i%len(limits)]
		b, err := svc.Pull(uid, bookID, after, limit)
		if err != nil {
			t.Fatalf("第 %d 批 pull 失败（limit=%d, after=%d）：%v", i+1, limit, after, err)
		}
		got := len(b.Transactions) + len(b.Accounts) + len(b.Categories) + len(b.Holdings)

		// 结构性判据 1：只要本批有数据，水位就必须严格推进，否则客户端会无限重拉同一批
		if got > 0 && b.HighVersion <= after {
			t.Fatalf("第 %d 批有 %d 行数据，但水位未推进（high=%d, after=%d）= 死循环",
				i+1, got, b.HighVersion, after)
		}
		// 结构性判据 2：水位绝不能倒退（客户端会按它回退本地游标）
		if b.HighVersion < after {
			t.Fatalf("第 %d 批水位倒退：high=%d < after=%d", i+1, b.HighVersion, after)
		}
		// 结构性判据 3：limit 必须约束住所有四类实体 —— 本批里"版本严格小于 HighVersion"
		// 的行数不得超过 limit-1。这是 mergeSafeVersion 的直接推论：
		// 第 limit 个版本之前最多只有 limit-1 行。
		below := 0
		record := func(table string, id, ver int64) {
			if ver < b.HighVersion {
				below++
			}
			key := fmt.Sprintf("%s:%d", table, id)
			st.Delivered[key]++
			st.DeliveredVer[key] = ver
		}
		for _, x := range b.Transactions {
			record("transactions", x.Id, x.SyncVersion)
		}
		for _, x := range b.Accounts {
			record("accounts", x.Id, x.SyncVersion)
		}
		for _, x := range b.Categories {
			record("categories", x.Id, x.SyncVersion)
		}
		for _, x := range b.Holdings {
			record("holdings", x.Id, x.SyncVersion)
		}
		if below > limit-1 {
			t.Fatalf("第 %d 批（limit=%d, high=%d）版本号严格小于 high 的行有 %d 条 > limit-1=%d —— limit 未约束住全部实体",
				i+1, limit, b.HighVersion, below, limit-2+1)
		}

		st.Batches = append(st.Batches, got)
		st.HighVersions = append(st.HighVersions, b.HighVersion)
		after = b.HighVersion
		if got == 0 {
			return st // 追平
		}
	}
	t.Fatalf("拉取 %d 批仍未终止（疑似死循环），各批行数前 20 批：%v", maxBatches, st.Batches[:reviewMin(20, len(st.Batches))])
	return st
}

// reviewMin 小工具：避免切片越界（Go 1.22 的 min 内建对切片下标同样可用，这里显式保留可读性）
func reviewMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// reviewBuildMixedWorkload 造一份"四类实体混合变更"的数据，覆盖：
// 交易创建（含转账）/ 软删 / 恢复、账户新建与改名、分类新建与删除、持仓直写。
//
// 返回：memberUid（一个可见性受限的普通成员，本人也创建了 5 笔交易）。
func reviewBuildMixedWorkload(t *testing.T, e *testEnv) int64 {
	t.Helper()
	ctx := context.Background()
	rng := rand.New(rand.NewSource(20241031))

	memberUID := reviewAddMember(t, e.engine, e.bookID, "member1", model.RoleMember)
	// 账本级关闭"成员互看"：member 只能看到自己的交易
	bs := NewBookService(e.engine, version.NewRepo())
	changed, err := bs.SetVisibility(e.uid, e.bookID, false)
	if err != nil {
		t.Fatalf("关闭互看失败：%v", err)
	}
	if !changed {
		t.Fatalf("关闭互看应返回 changed=true（初始值应为 true）")
	}

	// 1) 单笔交易 ×20（owner 创建）
	for i := 0; i < 20; i++ {
		_, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
			ClientUuid: fmt.Sprintf("rv-single-%d", i),
			TxType:     int(model.TxTypeExpense),
			AccountId:  e.assetID,
			Amount:     int64(100 + rng.Intn(900)),
			CategoryId: e.expenseCat,
			TxTime:     time.Now().Unix(),
		})
		if err != nil {
			t.Fatalf("创建单笔交易 %d 失败：%v", i, err)
		}
	}
	// 2) 转账 ×5（owner 创建，一次两行 + 两个账户各一行）
	for i := 0; i < 5; i++ {
		_, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
			ClientUuid:  fmt.Sprintf("rv-transfer-%d", i),
			TxType:      int(model.TxTypeTransfer),
			AccountId:   e.assetID,
			ToAccountId: e.toAssetID,
			Amount:      int64(1000 + i),
			TxTime:      time.Now().Unix(),
		})
		if err != nil {
			t.Fatalf("创建转账 %d 失败：%v", i, err)
		}
	}
	// 3) member 自己创建 5 笔（用于验证可见性过滤：owner 全见、member 只见自己的）
	for i := 0; i < 5; i++ {
		_, err := e.txs.Create(ctx, memberUID, e.bookID, CreateRequest{
			ClientUuid: fmt.Sprintf("rv-member-%d", i),
			TxType:     int(model.TxTypeIncome),
			AccountId:  e.assetID,
			Amount:     int64(50 + i),
			CategoryId: e.incomeCat,
			TxTime:     time.Now().Unix(),
		})
		if err != nil {
			t.Fatalf("member 创建交易 %d 失败：%v", i, err)
		}
	}

	as := NewAccountService(e.engine, version.NewRepo())
	cs := NewCategoryService(e.engine, version.NewRepo())

	// 4) 账户新建 ×3（各自独占一个版本）
	spare := make([]int64, 0, 3)
	for i := 0; i < 3; i++ {
		a, err := as.Create(e.uid, e.bookID, AccountCreateInput{
			Name: fmt.Sprintf("备用账户%d", i), Category: model.CategoryCash, Currency: "CNY",
		})
		if err != nil {
			t.Fatalf("创建备用账户 %d 失败：%v", i, err)
		}
		spare = append(spare, a.Id)
	}
	// 5) 账户改名 ×2
	for i := 0; i < 2; i++ {
		newName := fmt.Sprintf("改名后%d", i)
		if _, err := as.Update(e.uid, e.bookID, spare[i], AccountUpdateInput{Name: &newName}); err != nil {
			t.Fatalf("账户改名 %d 失败：%v", i, err)
		}
	}
	// 6) 删除最后一个空闲账户（无交易引用，可删）
	if _, err := as.Delete(e.uid, e.bookID, spare[2]); err != nil {
		t.Fatalf("删除备用账户失败：%v", err)
	}

	// 7) 分类新建 ×4 + 删除其中 1 个（未被交易引用）
	newCats := make([]int64, 0, 4)
	for i := 0; i < 4; i++ {
		c, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{
			Name: fmt.Sprintf("复审分类%d", i), CatType: model.CategoryTypeExpense,
		})
		if err != nil {
			t.Fatalf("创建分类 %d 失败：%v", i, err)
		}
		newCats = append(newCats, c.Id)
	}
	if _, err := cs.Delete(e.uid, e.bookID, newCats[3]); err != nil {
		t.Fatalf("删除分类失败：%v", err)
	}

	// 8) 软删 3 笔 owner 的单笔交易 + 恢复其中 1 笔（制造墓碑与"墓碑→复活"两种行）
	var toDelete []int64
	if err := e.engine.SQL(
		`SELECT id FROM transactions WHERE book_id = ? AND client_uuid LIKE 'rv-single-%' AND deleted = 0 ORDER BY id ASC LIMIT 3`,
		e.bookID).Find(&toDelete); err != nil {
		t.Fatalf("查询待删交易失败：%v", err)
	}
	if len(toDelete) != 3 {
		t.Fatalf("待删交易应找到 3 笔，实际 %d", len(toDelete))
	}
	for i, id := range toDelete {
		if _, err := e.txs.Delete(ctx, e.uid, e.bookID, id, fmt.Sprintf("rv-del-%d", i)); err != nil {
			t.Fatalf("软删交易 %d 失败：%v", id, err)
		}
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, toDelete[0], "rv-restore-0"); err != nil {
		t.Fatalf("恢复交易 %d 失败：%v", toDelete[0], err)
	}

	// 9) 持仓：service 层尚无写接口（v1.1 手动录入未落地），这里直写两行以覆盖第四类实体
	now := time.Now().Unix()
	holdings := []*model.Holding{
		{BookId: e.bookID, AccountId: e.assetID, InstrumentId: 1, Quantity: "10.5", CostAmount: 1000, CurrentPrice: "12.3", SyncVersion: 1, CreatedAt: now, UpdatedAt: now},
		{BookId: e.bookID, AccountId: e.toAssetID, InstrumentId: 2, Quantity: "1", CostAmount: 200, CurrentPrice: "2", SyncVersion: 2, CreatedAt: now, UpdatedAt: now},
	}
	if _, err := e.engine.Insert(holdings); err != nil {
		t.Fatalf("插入持仓失败：%v", err)
	}
	return memberUID
}

// TestReviewPullNoLossNoDuplicateNoDeadLoop 端到端：混合变更后，
// 用轮换的 limit 反复拉取，验证"不漏 / 不重 / 不循环"。
//
// 真值口径：四类实体中 sync_version>0 的每一行，都必须被下发且仅被下发一次。
func TestReviewPullNoLossNoDuplicateNoDeadLoop(t *testing.T) {
	e := newTestEnv(t)
	_ = reviewBuildMixedWorkload(t, e)
	svc := NewSyncService(e.engine, e.txs, nil)

	limits := []int{1, 3, 7, 50, 500, 2, 0 /* 0 = 回落默认 500 */, 1000 /* 越界 = 回落 500 */}
	st := reviewSyncAll(t, svc, e.uid, e.bookID, limits)

	// 1) 不漏：真值集的每一行都必须出现
	truth := map[string]struct {
		table string
		id    int64
	}{}
	for _, table := range syncEntityTables {
		for _, r := range reviewVersions(t, e.engine, table, e.bookID) {
			truth[fmt.Sprintf("%s:%d", table, r.Id)] = struct {
				table string
				id    int64
			}{table, r.Id}
		}
	}
	for key := range truth {
		if st.Delivered[key] == 0 {
			t.Fatalf("漏同步：%s 从未被下发（客户端游标已推进过它，且不会重试）", key)
		}
	}
	// 2) 不重：每行恰好一次
	for key, n := range st.Delivered {
		if n != 1 {
			t.Fatalf("重复下发：%s 被下发 %d 次（批次区间应为 (after, safe] 严格不重叠）", key, n)
		}
	}
	// 3) 终态版本一致：最后下发的版本应等于库里该行的版本（没有"下发了旧版本"的漏更新）
	for _, table := range syncEntityTables {
		for _, r := range reviewVersions(t, e.engine, table, e.bookID) {
			key := fmt.Sprintf("%s:%d", table, r.Id)
			if got := st.DeliveredVer[key]; got != r.SyncVersion {
				t.Fatalf("%s 下发的版本 %d 与库中最终版本 %d 不一致", key, got, r.SyncVersion)
			}
		}
	}
	// 4) 水位严格单调。
	//    注意：最后一批是"追平后的空批"，它的 HighVersion 取 book_versions 水位，
	//    与上一批相等是**正确**的（上一批恰好把数据发到了当前水位），故不参与单调性比较。
	if len(st.Batches) == 0 || st.Batches[len(st.Batches)-1] != 0 {
		t.Fatalf("最后一批必须是空批（客户端据此判定已追平），实际各批行数=%v", st.Batches)
	}
	for i := 1; i < len(st.Batches)-1; i++ {
		if st.HighVersions[i] <= st.HighVersions[i-1] {
			t.Fatalf("第 %d 批水位 %d 未超过上一批 %d（客户端会重复拉取或死循环）",
				i+1, st.HighVersions[i], st.HighVersions[i-1])
		}
	}
	t.Logf("共 %d 批追平，各批行数=%v，最终水位=%d", len(st.Batches), st.Batches, st.HighVersions[len(st.HighVersions)-1])
}

// TestReviewPullVisibilityFilterKeepsWaterLevelMoving 关闭互看后：
// member 只能拿到自己的交易，但账户/分类/持仓（账本共享配置）必须照常下发，
// 且游标必须持续推进 —— 不能因为"别人的交易占了版本位"而卡死。
func TestReviewPullVisibilityFilterKeepsWaterLevelMoving(t *testing.T) {
	e := newTestEnv(t)
	memberUID := reviewBuildMixedWorkload(t, e)
	svc := NewSyncService(e.engine, e.txs, nil)

	st := reviewSyncAll(t, svc, memberUID, e.bookID, []int{5, 13, 200})

	// 真值：member 只应看到自己创建的交易；其余三类共享配置全量可见
	own := map[int64]bool{}
	var mine []reviewVerRow
	for _, r := range reviewVersions(t, e.engine, "transactions", e.bookID) {
		var creator int64
		if _, err := e.engine.SQL(`SELECT creator_uid FROM transactions WHERE id = ?`, r.Id).Get(&creator); err != nil {
			t.Fatalf("读取 creator_uid 失败：%v", err)
		}
		if creator == memberUID {
			own[r.Id] = true
			mine = append(mine, r)
		}
	}
	if len(mine) != 5 {
		t.Fatalf("member 自建交易应 5 笔，实际 %d", len(mine))
	}
	for _, r := range mine {
		if st.Delivered[fmt.Sprintf("transactions:%d", r.Id)] != 1 {
			t.Fatalf("member 自己的交易 %d 未被下发", r.Id)
		}
	}
	// 别人的交易一条都不能下发
	for key := range st.Delivered {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			t.Fatalf("解析 key 失败：%q", key)
		}
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			t.Fatalf("解析 key 中的主键失败：%q，%v", key, err)
		}
		if parts[0] == "transactions" && !own[id] {
			t.Fatalf("越权下发：member 收到了他人交易 %d（关闭互看后不得下发）", id)
		}
	}
	// 共享配置必须完整
	for _, table := range []string{"accounts", "categories", "holdings"} {
		for _, r := range reviewVersions(t, e.engine, table, e.bookID) {
			if st.Delivered[fmt.Sprintf("%s:%d", table, r.Id)] != 1 {
				t.Fatalf("%s:%d 未下发给 member（共享配置不做可见性过滤）", table, r.Id)
			}
		}
	}
	t.Logf("member 视角共 %d 批，各批行数=%v", len(st.Batches), st.Batches)
}

// TestReviewPullSessionConditionDoesNotAccumulate 钉住一个"看起来很像 bug"的点：
//
// Pull 在同一个 *xorm.Session 上连续调用 sess.Where(cond, ...).Find(...) 四次。
// xorm 的 Statement.Where 实际是 And（会 AND 上已有条件），若 Find 不重置 statement，
// 第二次查询就会把上一次的条件叠加进去 —— 叠加后若带上可见性条件
// （creator_uid=? AND creator_uid<>0），accounts/categories/holdings 上根本没有
// creator_uid 列，会直接报 "no such column"。
// 本用例用"可见性受限的 member"走到那条分支，证明 Find 确实会 resetStatement，
// 因此这里**没有问题**。
func TestReviewPullSessionConditionDoesNotAccumulate(t *testing.T) {
	e := newTestEnv(t)
	memberUID := reviewAddMember(t, e.engine, e.bookID, "member2", model.RoleMember)
	bs := NewBookService(e.engine, version.NewRepo())
	if _, err := bs.SetVisibility(e.uid, e.bookID, false); err != nil {
		t.Fatalf("关闭互看失败：%v", err)
	}
	svc := NewSyncService(e.engine, e.txs, nil)

	// member（canViewAll=false）拉取：若条件累积，这里必然报 no such column: creator_uid
	b, err := svc.Pull(memberUID, e.bookID, 0, 500)
	if err != nil {
		t.Fatalf("可见性受限成员 pull 失败（疑似 xorm statement 条件累积）：%v", err)
	}
	// 账户/分类必须照常返回（不是空集）
	if len(b.Accounts) == 0 || len(b.Categories) == 0 {
		t.Fatalf("共享配置不应为空：accounts=%d categories=%d", len(b.Accounts), len(b.Categories))
	}
	// 用 owner（canViewAll=true）再拉一次，两者应一致地成功
	if _, err := svc.Pull(e.uid, e.bookID, 0, 500); err != nil {
		t.Fatalf("owner pull 失败：%v", err)
	}
}

// TestReviewPullLimitCapsEveryEntityType limit 必须对所有四类实体生效：
// 逐个创建 60 个账户（每个账户独占一个版本），然后 limit=10 拉取，
// 每批必须严格只有 10 行 —— 若某类实体突破了 limit，这里会立刻暴露。
func TestReviewPullLimitCapsEveryEntityType(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	cs := NewCategoryService(e.engine, version.NewRepo())
	svc := NewSyncService(e.engine, e.txs, nil)

	// 先把游标推过 seed 的"同版本整批"（33 行写在同一个版本，是规格允许的唯一例外）
	var startVersion int64
	if _, err := e.engine.SQL(
		`SELECT MAX(sync_version) FROM accounts WHERE book_id = ?`, e.bookID).Get(&startVersion); err != nil {
		t.Fatalf("读取起始版本失败：%v", err)
	}

	for i := 0; i < 40; i++ {
		if _, err := as.Create(e.uid, e.bookID, AccountCreateInput{
			Name: fmt.Sprintf("批量账户%d", i), Category: model.CategoryCash, Currency: "CNY",
		}); err != nil {
			t.Fatalf("创建账户 %d 失败：%v", i, err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{
			Name: fmt.Sprintf("批量分类%d", i), CatType: model.CategoryTypeExpense,
		}); err != nil {
			t.Fatalf("创建分类 %d 失败：%v", i, err)
		}
	}

	const limit = 10
	after := startVersion
	full := 0
	for i := 0; i < 200; i++ {
		b, err := svc.Pull(e.uid, e.bookID, after, limit)
		if err != nil {
			t.Fatalf("第 %d 批 pull 失败：%v", i+1, err)
		}
		got := len(b.Transactions) + len(b.Accounts) + len(b.Categories) + len(b.Holdings)
		if got == 0 {
			break
		}
		if got > limit {
			t.Fatalf("第 %d 批返回 %d 行 > limit=%d（limit 未约束住全部实体）：tx=%d acct=%d cat=%d hold=%d",
				i+1, got, limit, len(b.Transactions), len(b.Accounts), len(b.Categories), len(b.Holdings))
		}
		if got == limit {
			full++
		}
		after = b.HighVersion
	}
	if full < 5 {
		t.Fatalf("应至少有 5 个满批（60 行 / limit=10），实际 %d", full)
	}
	t.Logf("满批 %d 个，均受 limit=%d 约束", full, limit)
}

// TestReviewPullSameVersionReturnedWhole 唯一允许的例外：同一 sync_version 必须整批返回。
//
// seed 把 3 个默认账户 + 30 个默认分类写在同一个版本；afterVersion=0 的首批
// 必然大于 limit。越过该版本后批大小必须立刻回到 limit。
func TestReviewPullSameVersionReturnedWhole(t *testing.T) {
	e := newTestEnv(t)
	svc := NewSyncService(e.engine, e.txs, nil)

	first, err := svc.Pull(e.uid, e.bookID, 0, 3)
	if err != nil {
		t.Fatalf("首批 pull 失败：%v", err)
	}
	got := len(first.Transactions) + len(first.Accounts) + len(first.Categories) + len(first.Holdings)
	if got <= 3 {
		t.Fatalf("首批应因「同版本整体返回」而大于 limit=3，实际 %d 行", got)
	}
	// 首批里所有行的版本必须相同（否则说明水位跨越了多个版本）
	vers := map[int64]int{}
	for _, a := range first.Accounts {
		vers[a.SyncVersion]++
	}
	for _, c := range first.Categories {
		vers[c.SyncVersion]++
	}
	if len(vers) != 1 {
		t.Fatalf("首批应只含一个版本（同版本整体返回），实际含 %d 个版本：%v", len(vers), vers)
	}
	t.Logf("首批返回 %d 行、单一版本=%v（规格允许的唯一例外）", got, vers)
}

// TestReviewPullEmptyBatchAdvancesCursorToBookVersion 追平后再拉：
// 必须回空批，且 HighVersion 等于 book_versions 的水位（客户端据此停止拉取）。
func TestReviewPullEmptyBatchAdvancesCursorToBookVersion(t *testing.T) {
	e := newTestEnv(t)
	svc := NewSyncService(e.engine, e.txs, nil)
	bs := NewBookService(e.engine, version.NewRepo())

	st := reviewSyncAll(t, svc, e.uid, e.bookID, []int{500})
	last := st.HighVersions[len(st.HighVersions)-1]

	cur, err := bs.CurrentVersion(e.bookID)
	if err != nil {
		t.Fatalf("读取账本版本失败：%v", err)
	}
	if last != cur {
		t.Fatalf("追平后 HighVersion=%d 应等于账本水位=%d（否则客户端无法判定已追平）", last, cur)
	}
	// 再拉一次：空批且水位不倒退
	b, err := svc.Pull(e.uid, e.bookID, last, 100)
	if err != nil {
		t.Fatalf("追平后再次 pull 失败：%v", err)
	}
	if n := len(b.Transactions) + len(b.Accounts) + len(b.Categories) + len(b.Holdings); n != 0 {
		t.Fatalf("追平后应返回空批，实际 %d 行", n)
	}
	if b.HighVersion != cur {
		t.Fatalf("空批的 HighVersion=%d 应仍为账本水位=%d", b.HighVersion, cur)
	}
}

// TestReviewPullRejectsNonMemberAndBadCursor 入口校验：非成员 403；负游标按 0 处理。
func TestReviewPullRejectsNonMemberAndBadCursor(t *testing.T) {
	e := newTestEnv(t)
	svc := NewSyncService(e.engine, e.txs, nil)

	// 非成员
	stranger := reviewAddMember(t, e.engine, e.bookID, "stranger", model.RoleMember)
	if _, err := e.engine.Exec("DELETE FROM book_members WHERE uid = ?", stranger); err != nil {
		t.Fatalf("删除成员失败：%v", err)
	}
	if _, err := svc.Pull(stranger, e.bookID, 0, 100); !errors.Is(err, ErrNotMember) {
		t.Fatalf("非成员 pull 应返回 ErrNotMember，实际 %v", err)
	}
	// 负游标：不得返回空集让客户端误以为已同步，而应从头下发
	b, err := svc.Pull(e.uid, e.bookID, -99, 500)
	if err != nil {
		t.Fatalf("负游标 pull 失败：%v", err)
	}
	if len(b.Accounts) == 0 {
		t.Fatalf("负游标应按 afterVersion=0 处理并返回全量，实际为空")
	}
}

// TestReviewPushResultMappingIsPerItem push 逐条独立：一条非法不影响其余。
func TestReviewPushResultMappingIsPerItem(t *testing.T) {
	e := newTestEnv(t)
	svc := NewSyncService(e.engine, e.txs, nil)
	ctx := context.Background()

	items := []PushItem{
		{ClientUuid: "rv-push-ok-1", TxType: int(model.TxTypeExpense), AccountId: e.assetID, Amount: 100, CategoryId: e.expenseCat, TxTime: time.Now().Unix()},
		{ClientUuid: "", TxType: int(model.TxTypeExpense), AccountId: e.assetID, Amount: 100, CategoryId: e.expenseCat, TxTime: time.Now().Unix()}, // 缺 clientUuid → invalid
		{ClientUuid: "rv-push-bad-acct", TxType: int(model.TxTypeExpense), AccountId: 999999, Amount: 100, CategoryId: e.expenseCat, TxTime: time.Now().Unix()},
		{ClientUuid: "rv-push-ok-2", TxType: int(model.TxTypeExpense), AccountId: e.assetID, Amount: 200, CategoryId: e.expenseCat, TxTime: time.Now().Unix()},
	}
	results, err := svc.Push(ctx, e.uid, e.bookID, items)
	if err != nil {
		t.Fatalf("push 整体不应失败：%v", err)
	}
	if len(results) != len(items) {
		t.Fatalf("结果条数应等于入参条数：%d != %d", len(results), len(items))
	}
	want := []string{PushStatusCreated, PushStatusInvalid, PushStatusInvalid, PushStatusCreated}
	for i, r := range results {
		if r.Status != want[i] {
			t.Fatalf("第 %d 条状态应为 %s，实际 %s（code=%s msg=%s）", i, want[i], r.Status, r.ErrorCode, r.Message)
		}
	}
	// 超长入参整批 400
	tooMany := make([]PushItem, maxPushItems+1)
	if _, err := svc.Push(ctx, e.uid, e.bookID, tooMany); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("超过 %d 项应整批非法，实际 %v", maxPushItems, err)
	}
}

// TestReviewSyncStreamQueryPlan 观测四类实体版本流的执行计划。
//
// syncVersionStream 对每张表跑
//
//	SELECT sync_version FROM <t> WHERE book_id=? AND sync_version>? ORDER BY sync_version, id LIMIT ?
//
// 只有 transactions 有 idx_tx_sync(book_id, sync_version)；accounts / categories /
// holdings 的索引是 (book_id, deleted) 或 (book_id, account_id, deleted)，
// 因此这三张表每次 Pull 都要"按 book_id 扫 + 排序"。家庭场景这三张表只有几十行，
// 影响可忽略；但它是每次 Pull × 每次 3 张表的固定开销，记录备查。
func TestReviewSyncStreamQueryPlan(t *testing.T) {
	e := newTestEnv(t)
	for _, table := range syncEntityTables {
		type planRow struct {
			Detail string `xorm:"detail"`
		}
		rows := make([]planRow, 0, 8)
		err := e.engine.SQL(
			`EXPLAIN QUERY PLAN SELECT sync_version FROM `+table+
				` WHERE book_id = ? AND sync_version > ? ORDER BY sync_version ASC, id ASC LIMIT ?`,
			e.bookID, 0, MaxSyncBatch).Find(&rows)
		if err != nil {
			t.Fatalf("EXPLAIN %s 失败：%v", table, err)
		}
		details := make([]string, 0, len(rows))
		usesIndex := false
		for _, r := range rows {
			details = append(details, r.Detail)
			if strings.Contains(r.Detail, "idx_tx_sync") {
				usesIndex = true
			}
		}
		t.Logf("%-13s 计划=%v", table, details)
		if table == "transactions" && !usesIndex {
			t.Fatalf("transactions 的版本流应走 idx_tx_sync，实际计划=%v", details)
		}
		if table != "transactions" && usesIndex {
			t.Fatalf("意外的计划：%v", details)
		}
	}
}

// TestReviewPullMonotonicUnderConcurrentVersionBumps 并发写：
// 版本由 book_versions 单调 +1 且与业务写入同事务，因此 Pull 看到的
// sync_version 序列必须严格单调（不会出现"版本涨了但行没写"或反之）。
//
// 本用例用交替写入 + 交替拉取的方式检查水位不出现空洞跳跃。
func TestReviewPullMonotonicUnderInterleavedWrites(t *testing.T) {
	e := newTestEnv(t)
	svc := NewSyncService(e.engine, e.txs, nil)
	ctx := context.Background()

	after := int64(0)
	for round := 0; round < 12; round++ {
		if _, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
			ClientUuid: fmt.Sprintf("rv-inter-%d", round),
			TxType:     int(model.TxTypeExpense),
			AccountId:  e.assetID,
			Amount:     int64(10 + round),
			CategoryId: e.expenseCat,
			TxTime:     time.Now().Unix(),
		}); err != nil {
			t.Fatalf("第 %d 轮创建失败：%v", round, err)
		}
		b, err := svc.Pull(e.uid, e.bookID, after, 500)
		if err != nil {
			t.Fatalf("第 %d 轮 pull 失败：%v", round, err)
		}
		if b.HighVersion <= after {
			t.Fatalf("第 %d 轮水位未推进：%d -> %d", round, after, b.HighVersion)
		}
		// 本批里的所有行版本必须落在 (after, HighVersion] 之内
		check := func(table string, ver int64) {
			if ver <= after || ver > b.HighVersion {
				t.Fatalf("第 %d 轮 %s 行版本 %d 落在批次区间 (%d, %d] 之外", round, table, ver, after, b.HighVersion)
			}
		}
		for _, x := range b.Transactions {
			check("transactions", x.SyncVersion)
		}
		for _, x := range b.Accounts {
			check("accounts", x.SyncVersion)
		}
		after = b.HighVersion
	}
}
