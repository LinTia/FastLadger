package service

// GPT-R48：Pull 的 limit 必须约束【所有】同步实体，而不是只约束交易行。
//
// 分两层：
//  1. mergeSafeVersion 的纯算术（不碰数据库，本机可直接跑）——水位算错是最隐蔽的缺陷，
//     必须能在本机验证；
//  2. 真实 SQLite 的端到端（无 CGO 时显式 Skip 并打印原因，由 CI 执行）。
//
// ⚠️ fixture 陷阱（CI 上踩过一次，务必记住）：seedBookDefaults 把 33 行默认数据
// （3 个默认账户 + 30 个默认两级分类）**全部写在同一个 sync_version** 里（与账本创建同事务，
// 只取一次 version）。因此从 afterVersion=0 拉取时，第一批必然是"同一版本整体回扩"——
// 这是规格允许的**唯一**例外，可能远超 limit。要验证"每批受 limit 约束"，
// 必须先把游标放到 seed 之后（见 maxSyncVersion）。

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// TestR48MergeSafeVersionCountsAllEntities 水位由四类实体共同决定：
// 只有账户变更、只有分类变更、混合变更三种情形都必须被 limit 约束住。
func TestR48MergeSafeVersionCountsAllEntities(t *testing.T) {
	cases := []struct {
		name     string
		tx       []int64
		accounts []int64
		category []int64
		holdings []int64
		limit    int
		wantHigh int64
	}{
		{
			name:     "只有账户变更：水位按账户流取第 3 个",
			accounts: []int64{1, 2, 3, 4, 5},
			limit:    3,
			wantHigh: 3,
		},
		{
			name:     "只有分类变更：同样受 limit 约束（旧实现会取 MAX=5）",
			category: []int64{1, 2, 3, 4, 5},
			limit:    2,
			wantHigh: 2,
		},
		{
			name:     "只有持仓变更",
			holdings: []int64{10, 20, 30},
			limit:    2,
			wantHigh: 20,
		},
		{
			name:     "混合变更：交易与账户交错，第 4 个版本是 4",
			tx:       []int64{1, 3, 5},
			accounts: []int64{2, 4, 6},
			limit:    4,
			wantHigh: 4,
		},
		{
			name:     "交易为空时不得取账户流的 MAX",
			accounts: []int64{7, 8, 9, 10},
			limit:    1,
			wantHigh: 7,
		},
		{
			name:     "增量不足一批：取最大版本（本批即最后一批）",
			tx:       []int64{1, 2},
			accounts: []int64{3},
			limit:    100,
			wantHigh: 3,
		},
		{
			name:     "无任何增量",
			limit:    10,
			wantHigh: 0,
		},
		{
			name:     "同一版本大量记录：第 limit 个仍是该版本（回扩后一起下发）",
			tx:       []int64{7, 7, 7, 7, 7, 7, 7},
			accounts: []int64{8},
			limit:    5,
			wantHigh: 7,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mergeSafeVersion([][]int64{c.tx, c.accounts, c.category, c.holdings}, c.limit)
			if got != c.wantHigh {
				t.Fatalf("mergeSafeVersion(limit=%d) = %d，期望 %d", c.limit, got, c.wantHigh)
			}
		})
	}
}

// TestR48MergeSafeVersionNeverExceedsBoundaryVersion 水位不得越过"第 limit 个版本"：
// 逐档 limit 扫描一遍，断言是单调不减的，且 limit=1 时取全局最小版本。
func TestR48MergeSafeVersionNeverExceedsBoundaryVersion(t *testing.T) {
	streams := [][]int64{
		{1, 1, 4, 9},           // transactions（含同版本重复行）
		{2, 3, 5},              // accounts
		{6},                    // categories
		{7, 8, 10, 11, 12, 13}, // holdings
	}
	want := []int64{1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 13, 13}
	for limit := 1; limit <= len(want); limit++ {
		if got := mergeSafeVersion(streams, limit); got != want[limit-1] {
			t.Fatalf("limit=%d：得到 %d，期望 %d", limit, got, want[limit-1])
		}
	}
	// 退化参数：limit<=0 不得给出一个"看起来像水位"的值
	if got := mergeSafeVersion(streams, 0); got != 0 {
		t.Fatalf("limit=0 应返回 0，实际 %d", got)
	}
}

// repeatVersions 生成 n 个相同版本号（模拟同一版本写入多行）
func repeatVersions(v int64, n int) []int64 {
	out := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, v)
	}
	return out
}

// simulateSafeVersion 用内存数据复刻 Pull 的水位计算：
// 每个实体按版本升序取 (afterVersion, +∞) 内的前 limit 行（即 syncVersionStream 的语义），
// 再交给真实的 mergeSafeVersion。
//
// 存在意义：CI 暴露过一次"fixture 与预期不符"（seed 把 33 行写在同一个版本），
// 这条用例把那个场景固化在**本机可跑**的纯逻辑层，不必等 CI 才发现。
func simulateSafeVersion(tables map[string][]int64, afterVersion int64, limit int) int64 {
	streams := make([][]int64, 0, len(syncEntityTables))
	for _, table := range syncEntityTables {
		rows := tables[table]
		s := make([]int64, 0, limit)
		for _, v := range rows {
			if v <= afterVersion {
				continue
			}
			s = append(s, v)
			if len(s) >= limit {
				break
			}
		}
		streams = append(streams, s)
	}
	return mergeSafeVersion(streams, limit)
}

// TestR48SeedBulkVersionIsOnlyAllowedException 复刻 CI 上踩到的真实版本分布：
// seed 把 3 个默认账户 + 30 个默认分类写在**同一个版本**，随后每个新建账户各占一个版本。
// 断言：从 0 拉时第一批就是"同版本回扩"（唯一允许的例外），越过该版本后批大小回到 limit。
func TestR48SeedBulkVersionIsOnlyAllowedException(t *testing.T) {
	const seedVer = int64(1)
	accounts := append(repeatVersions(seedVer, 3), 2, 3, 4) // seed 3 行 + 环境 3 行
	categories := append(repeatVersions(seedVer, 30), 5, 6) // seed 30 行 + 环境 2 行
	tables := map[string][]int64{
		"transactions": {},
		"accounts":     accounts,
		"categories":   categories,
		"holdings":     {},
	}

	// ① 从 0 拉：第 5 个版本仍是 seedVer ⇒ 该版本 33 行整体返回（合法例外）
	if got := simulateSafeVersion(tables, 0, 5); got != seedVer {
		t.Fatalf("第一批水位应为 seed 版本 %d，实际 %d", seedVer, got)
	}
	// ② 越过 seed 版本之后：批大小重新受 limit 约束
	for _, after := range []int64{1, 2, 3} {
		sv := simulateSafeVersion(tables, after, 5)
		rows := 0
		for _, vs := range [][]int64{accounts, categories} {
			for _, v := range vs {
				if v > after && v <= sv {
					rows++
				}
			}
		}
		if rows > 5 {
			t.Fatalf("afterVersion=%d：回扩只允许发生在边界版本，实际返回 %d 行 > limit=5", after, rows)
		}
	}
	// ③ 追加 20 个各占一个版本的账户后：每批恰好 limit 行（与端到端用例同一口径）
	newbie := make([]int64, 0, 20)
	for i := 0; i < 20; i++ {
		newbie = append(newbie, int64(7+i))
	}
	tables["accounts"] = append(append([]int64{}, accounts...), newbie...)
	const limit = 5
	batches := 0
	total := 0
	cursor := int64(6)
	for i := 0; i < 100; i++ {
		sv := simulateSafeVersion(tables, cursor, limit)
		if sv <= cursor {
			break
		}
		n := 0
		for _, vs := range [][]int64{tables["accounts"], tables["categories"]} {
			for _, v := range vs {
				if v > cursor && v <= sv {
					n++
				}
			}
		}
		if n > limit {
			t.Fatalf("第 %d 批返回 %d 行，超过 limit=%d", batches+1, n, limit)
		}
		if n == 0 {
			break
		}
		batches++
		total += n
		cursor = sv
	}
	if total != 20 || batches != 4 {
		t.Fatalf("20 个新增账户应按 limit=5 分 4 批下发，实际 %d 批 / %d 行", batches, total)
	}
}

// TestR48MixedChangesCompositionSimulation 复刻 TestR48MixedChangesBoundedByLimit 的真库
// 版本分布，用真实 mergeSafeVersion 验证"19 行 / 7+7+5 / 账户 7 行"这个推导。
//
// 相对版本（起点视为 0），每轮 i（0-based）：
//   - 交易 v=3i+1、新建账户 v=3i+2、新建分类 v=3i+3；
//   - 每笔支出还会走 createChangeBalance 改写被扣款账户的余额，并把该账户行的
//     sync_version 抬到本笔交易的版本 ⇒ 环境里那个既有账户（现金）最终停在 v=16，
//     于是它被"搬"进本轮范围，成为第 19 行（库里它仍只有一行，不是重复下发）。
func TestR48MixedChangesCompositionSimulation(t *testing.T) {
	txs := make([]int64, 0, 6)
	accts := make([]int64, 0, 7)
	cats := make([]int64, 0, 6)
	for i := 0; i < 6; i++ {
		v := int64(3*i + 1)
		txs = append(txs, v)
		accts = append(accts, v+1)
		cats = append(cats, v+2)
	}
	accts = append(accts, 16) // 既有账户「现金」被最后一笔支出搬到 v16
	slices.Sort(accts)

	tables := map[string][]int64{
		"transactions": txs,
		"accounts":     accts,
		"categories":   cats,
		"holdings":     {},
	}

	const limit = 7
	var got batchStat
	var sizes []int
	cursor := int64(0)
	for i := 0; i < 100; i++ {
		sv := simulateSafeVersion(tables, cursor, limit)
		if sv <= cursor {
			break
		}
		cur := batchStat{}
		for _, v := range txs {
			if v > cursor && v <= sv {
				cur.transactions++
			}
		}
		for _, v := range accts {
			if v > cursor && v <= sv {
				cur.accounts++
			}
		}
		for _, v := range cats {
			if v > cursor && v <= sv {
				cur.categories++
			}
		}
		if cur.total() == 0 {
			break
		}
		if cur.total() > limit {
			t.Fatalf("第 %d 批 %d 行超过 limit=%d", len(sizes)+1, cur.total(), limit)
		}
		sizes = append(sizes, cur.total())
		got = got.add(cur)
		cursor = sv
	}

	want := batchStat{transactions: 6, accounts: 7, categories: 6, holdings: 0}
	if got != want {
		t.Fatalf("构成应为 交易=6 账户=7 分类=6 持仓=0（合计 19），实际 %+v（合计 %d）", got, got.total())
	}
	// 7 + 7 + 5：最后一批不足 limit 属正常，前面的批必须恰好等于 limit
	if len(sizes) != 3 || sizes[0] != limit || sizes[1] != limit || sizes[2] != 5 {
		t.Fatalf("分批应为 [7 7 5]，实际 %v", sizes)
	}
}

// ---------------------------------------------------------------------------
// 真实 SQLite 端到端（无 CGO 时跳过）
// ---------------------------------------------------------------------------

// maxSyncVersion 读取账本当前最大 sync_version（四类实体合计）。
//
// 用途：把端到端用例的起点放在 seed 之后——seed 把 33 行写在同一个版本里，
// 从 0 开始拉的第一批必然是"同版本回扩"，没法用来验证 limit 的约束。
func maxSyncVersion(t *testing.T, e *testEnv) int64 {
	t.Helper()
	var v int64
	if _, err := e.engine.SQL(`SELECT COALESCE(MAX(v), 0) FROM (
		SELECT MAX(sync_version) AS v FROM transactions WHERE book_id = ?
		UNION ALL SELECT MAX(sync_version) FROM accounts WHERE book_id = ?
		UNION ALL SELECT MAX(sync_version) FROM categories WHERE book_id = ?
		UNION ALL SELECT MAX(sync_version) FROM holdings WHERE book_id = ?
	)`, e.bookID, e.bookID, e.bookID, e.bookID).Get(&v); err != nil {
		t.Fatalf("读取最大 sync_version 失败：%v", err)
	}
	return v
}

// minSyncVersion 读取账本当前最小 sync_version（即 seed 写入的那个版本）
func minSyncVersion(t *testing.T, e *testEnv) int64 {
	t.Helper()
	var v int64
	if _, err := e.engine.SQL(`SELECT COALESCE(MIN(v), 0) FROM (
		SELECT MIN(sync_version) AS v FROM accounts WHERE book_id = ?
		UNION ALL SELECT MIN(sync_version) FROM categories WHERE book_id = ?
	)`, e.bookID, e.bookID).Get(&v); err != nil {
		t.Fatalf("读取最小 sync_version 失败：%v", err)
	}
	return v
}

// entitiesAtVersion 统计某个 sync_version 上的实体行数（账户 + 分类）
func entitiesAtVersion(t *testing.T, e *testEnv, ver int64) int64 {
	t.Helper()
	var n int64
	if _, err := e.engine.SQL(`SELECT COUNT(1) FROM (
		SELECT id FROM accounts   WHERE book_id = ? AND sync_version = ?
		UNION ALL SELECT id FROM categories WHERE book_id = ? AND sync_version = ?
	)`, e.bookID, ver, e.bookID, ver).Get(&n); err != nil {
		t.Fatalf("统计版本 %d 的行数失败：%v", ver, err)
	}
	return n
}

// batchStat 一批的构成。
//
// 为什么按实体拆分统计：只断言"总行数"时，一旦数字对不上就说不清是哪类实体多发/漏发
// （CI 上踩过一次：多出来的 1 行其实是"余额变动被搬到新版本的既有账户"）。
type batchStat struct {
	transactions int
	accounts     int
	categories   int
	holdings     int
}

func (b batchStat) total() int {
	return b.transactions + b.accounts + b.categories + b.holdings
}

func (b batchStat) add(o batchStat) batchStat {
	b.transactions += o.transactions
	b.accounts += o.accounts
	b.categories += o.categories
	b.holdings += o.holdings
	return b
}

// pullAll 从 startVersion 起反复 pull 直到没有增量，返回每批的构成与最终游标。
func pullAll(t *testing.T, e *testEnv, startVersion int64, limit int) (batches []batchStat, high int64) {
	t.Helper()
	svc := NewSyncService(e.engine, e.txs, nil)
	cursor := startVersion
	for i := 0; i < 1000; i++ {
		b, err := svc.Pull(e.uid, e.bookID, cursor, limit)
		if err != nil {
			t.Fatalf("pull(cursor=%d) 失败：%v", cursor, err)
		}
		if b.HighVersion < cursor {
			t.Fatalf("游标倒退：%d → %d", cursor, b.HighVersion)
		}
		cur := batchStat{
			transactions: len(b.Transactions),
			accounts:     len(b.Accounts),
			categories:   len(b.Categories),
			holdings:     len(b.Holdings),
		}
		if cur.total() == 0 {
			// 没有可下发的行即收敛（book_versions 可能比最后一行 sync_version 略高，
			// 那时 HighVersion 会带着空批前进，不能把它算成一批）
			return batches, cursor
		}
		if b.HighVersion <= cursor {
			t.Fatalf("返回了 %d 行但游标未推进（%d → %d）", cur.total(), cursor, b.HighVersion)
		}
		batches = append(batches, cur)
		cursor = b.HighVersion
	}
	t.Fatal("pull 未在 1000 批内收敛（疑似死循环）")
	return nil, 0
}

// sumBatches 汇总各批构成
func sumBatches(batches []batchStat) batchStat {
	var s batchStat
	for _, b := range batches {
		s = s.add(b)
	}
	return s
}

// TestR48OnlyAccountChangesAreBatched 只有账户变更：每批规模受 limit 约束，
// 绝不能像旧实现那样"一次全量返回"。
//
// 起点取 seed 之后（seed 的 33 行同版本会触发合法的"同版本回扩"，会掩盖 limit 的约束），
// 之后每个新建账户各占一个独立版本 ⇒ 每批恰好 limit 行。
func TestR48OnlyAccountChangesAreBatched(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	start := maxSyncVersion(t, e)
	as := NewAccountService(e.engine, version.NewRepo())

	const total = 20
	for i := 0; i < total; i++ {
		_, err := as.Create(e.uid, e.bookID, AccountCreateInput{
			Name: fmt.Sprintf("账户%d", i), Category: model.CategoryCash, Currency: "CNY",
		})
		must(t, err)
	}

	const limit = 5
	batches, _ := pullAll(t, e, start, limit)
	if len(batches) == 0 {
		t.Fatal("应至少有一批增量")
	}
	for i, b := range batches {
		if b.total() > limit {
			t.Fatalf("第 %d 批返回 %d 行，超过 limit=%d（水位没被 limit 约束住）", i+1, b.total(), limit)
		}
	}
	// 全部 20 个新账户都要被下发（起点在 seed 之后，不涉及交易/持仓），
	// 且恰好分 total/limit 批——每个账户各占一个版本，不存在同版本回扩。
	got := sumBatches(batches)
	if got.accounts != total || got.transactions != 0 || got.categories != 0 || got.holdings != 0 {
		t.Fatalf("下发构成应为 账户=%d 交易=0 分类=0 持仓=0，实际 %+v", total, got)
	}
	if len(batches) != total/limit {
		t.Fatalf("%d 个账户按 limit=%d 应恰好分 %d 批，实际 %d 批（%+v）",
			total, limit, total/limit, len(batches), batches)
	}
}

// TestR48MixedChangesBoundedByLimit 混合变更（交易 + 账户 + 分类）：
// 每批总行数（跨实体合计）受 limit 约束。
//
// ⚠️ 期望值推导里有一个反直觉的点（CI 上栽过一次），写在最前面：
// 记一笔交易会把【所用账户】的 sync_version 一并抬到本笔交易的版本
// （TxService.Create → createChangeBalance：`UPDATE accounts SET balance=..., sync_version=?`，
//
//	一次 s.versions.Next 的版本被交易行与账户行共用）。
//
// 因此本用例里"每轮新建的 18 行"之外还会多 1 行：环境里那个既有账户（e.assetID「现金」）
// 被 6 笔支出连续抬版本后，最终停在**最后一笔交易的版本**上——库里它仍只有一行，
// 只是从"start 之前"被搬进了本轮范围，所以是 **+1 而不是 +6**，也不是重复下发。
func TestR48MixedChangesBoundedByLimit(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	start := maxSyncVersion(t, e)
	as := NewAccountService(e.engine, version.NewRepo())
	cs := NewCategoryService(e.engine, version.NewRepo())
	ctx := context.Background()

	// 交错产生交易 / 账户 / 分类变更，每类各 6 次（共 18 行，每行一个独立版本）
	// （外加上面说明的"被抬版本的既有账户"1 行 ⇒ 合计 19，见下方断言）
	const rounds = 6
	for i := 0; i < rounds; i++ {
		_, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{
			ClientUuid: fmt.Sprintf("r48-tx-%d", i), Amount: 100 * int64(i+1),
		}))
		must(t, err)
		_, err = as.Create(e.uid, e.bookID, AccountCreateInput{
			Name: fmt.Sprintf("混合账户%d", i), Category: model.CategoryCash, Currency: "CNY",
		})
		must(t, err)
		_, err = cs.Create(e.uid, e.bookID, CategoryCreateInput{
			Name: fmt.Sprintf("混合分类%d", i), CatType: model.CategoryTypeExpense,
		})
		must(t, err)
	}

	const limit = 7
	batches, _ := pullAll(t, e, start, limit)
	if len(batches) == 0 {
		t.Fatal("应至少有一批增量")
	}
	for i, b := range batches {
		// 本用例没有转账，且每批都必须 ≤ limit（同版本回扩只可能发生在边界版本，
		// 若某批超限说明水位没被 limit 约束住）
		if b.total() > limit {
			t.Fatalf("第 %d 批返回 %d 行，超过 limit=%d", i+1, b.total(), limit)
		}
	}

	// 期望构成（逐项写死，避免"数字对不上却说不清"）：
	//   - 交易 6 行：每轮 1 笔支出；
	//   - 分类 6 行：每轮 1 个新建分类；
	//   - 账户 7 行 = 每轮 1 个新建账户（6）+ **环境里那个既有账户（e.assetID「现金」）1 行**。
	//     为什么会有这第 19 行：支出走 createChangeBalance 改写账户余额时，会把该账户行的
	//     sync_version 抬到本笔交易的版本——余额变了，客户端必须重新收到这一行。
	//     它本来在 start 之前（不在本轮范围），因为余额被改写才被"搬"进范围，属正确语义，
	//     不是重复下发（库里它仍只有一行）。
	//   合计 19 行 ⇒ 按 limit=7 分 3 批：7 + 7 + 5。
	//   为什么第 3 批是 5（不是 7）：把版本摊开就清楚了——相对版本下
	//     交易 {1,4,7,10,13,16}、新建账户 {2,5,8,11,14,17}、被抬版本的现金 {16}、
	//     分类 {3,6,9,12,15,18}，合并流第 7 个 = 7、第 14 个 = 14，
	//     于是前两批各 7 行，余下 {15,16,16,17,18} 共 5 行 —— 最后一批不足 limit 属正常收尾，
	//     不是水位失控（若是失控，前两批就会超过 7）。
	got := sumBatches(batches)
	want := batchStat{transactions: rounds, accounts: rounds + 1, categories: rounds, holdings: 0}
	if got != want {
		t.Fatalf("下发构成应为 交易=%d 账户=%d 分类=%d 持仓=%d（合计 %d），实际 %+v（合计 %d，分批 %+v）",
			want.transactions, want.accounts, want.categories, want.holdings, want.total(),
			got, got.total(), batches)
	}
	if len(batches) != 3 {
		t.Fatalf("19 行按 limit=7 应分 3 批（7+7+5），实际 %d 批（%+v）", len(batches), batches)
	}
}

// TestR48OnlyCategoryChangesAreBatched 只有分类变更（**没有任何交易**）：
// 旧实现走的是"无交易增量 → 三类表取 MAX"的兜底分支，会把全部分类一次性返回；
// 现在水位同样由 limit 决定，必须分批。这条直接命中旧实现的兜底分支。
func TestR48OnlyCategoryChangesAreBatched(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	start := maxSyncVersion(t, e)
	cs := NewCategoryService(e.engine, version.NewRepo())

	const total = 12
	for i := 0; i < total; i++ {
		_, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{
			Name: fmt.Sprintf("新分类%d", i), CatType: model.CategoryTypeExpense,
		})
		must(t, err)
	}

	const limit = 5
	batches, _ := pullAll(t, e, start, limit)
	if len(batches) == 0 {
		t.Fatal("应至少有一批增量")
	}
	for i, b := range batches {
		if b.total() > limit {
			t.Fatalf("第 %d 批返回 %d 行，超过 limit=%d（非交易变更没被 limit 约束）", i+1, b.total(), limit)
		}
	}
	// 只有分类变更：不涉及交易与余额改写，故账户/交易/持仓都应为 0
	got := sumBatches(batches)
	want := batchStat{categories: total}
	if got != want {
		t.Fatalf("下发构成应为 分类=%d 交易=0 账户=0 持仓=0，实际 %+v", total, got)
	}
	// 12 行 / limit=5 ⇒ 3 批（5+5+2）；旧实现的"取 MAX"兜底只会返回 1 批
	if len(batches) != 3 {
		t.Fatalf("%d 个分类按 limit=%d 应分 3 批（5+5+2），实际 %d 批（%+v）", total, limit, len(batches), batches)
	}
}

// TestR48SameVersionReturnedAsWhole 同一 sync_version 的大量记录必须【整体】返回：
// 允许因此超过 limit，但游标必须一次推进到该版本（不允许拆版本）。
//
// fixture 直接用 seed 数据：3 个默认账户 + 30 个默认分类写在**同一个版本**里，
// 这正是"同版本回扩"的真实场景（不靠 UPDATE 伪造）。
func TestR48SameVersionReturnedAsWhole(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)

	seedVer := minSyncVersion(t, e)
	bulk := entitiesAtVersion(t, e, seedVer)
	const limit = 5
	if bulk <= limit {
		t.Fatalf("fixture 失效：版本 %d 上只有 %d 行，不足以验证同版本回扩（需 > %d 行）",
			seedVer, bulk, limit)
	}

	svc := NewSyncService(e.engine, e.txs, nil)
	b, err := svc.Pull(e.uid, e.bookID, 0, limit)
	must(t, err)

	// ① 该版本的全部行必须一次带齐（旧实现会只带 limit 行却把游标推到该版本，
	//    导致其余行永久丢失）
	if got := int64(len(b.Accounts) + len(b.Categories)); got != bulk {
		t.Fatalf("版本 %d 的 %d 行必须整体返回，实际 %d 行", seedVer, bulk, got)
	}
	if b.HighVersion != seedVer {
		t.Fatalf("游标应推进到 %d，实际 %d", seedVer, b.HighVersion)
	}
	// ② 回扩只允许发生在边界版本：越过该版本之后，批大小重新受 limit 约束
	next, err := svc.Pull(e.uid, e.bookID, b.HighVersion, limit)
	must(t, err)
	if n := len(next.Transactions) + len(next.Accounts) + len(next.Categories) + len(next.Holdings); n > limit {
		t.Fatalf("同版本回扩只适用于边界版本：第二批返回 %d 行，超过 limit=%d", n, limit)
	}
}
