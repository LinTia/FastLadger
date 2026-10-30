package service

// GPT-R48：Pull 的 limit 必须约束【所有】同步实体，而不是只约束交易行。
//
// 分两层：
//  1. mergeSafeVersion 的纯算术（不碰数据库，本机可直接跑）——水位算错是最隐蔽的缺陷，
//     必须能在本机验证；
//  2. 真实 SQLite 的端到端（无 CGO 时显式 Skip 并打印原因，由 CI 执行）。

import (
	"context"
	"fmt"
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

// ---------------------------------------------------------------------------
// 真实 SQLite 端到端（无 CGO 时跳过）
// ---------------------------------------------------------------------------

// pullAll 反复 pull 直到没有增量，返回每批的规模与最终游标。
func pullAll(t *testing.T, e *testEnv, limit int) (batches []int, high int64) {
	t.Helper()
	svc := NewSyncService(e.engine, e.txs, nil)
	cursor := int64(0)
	for i := 0; i < 1000; i++ {
		b, err := svc.Pull(e.uid, e.bookID, cursor, limit)
		if err != nil {
			t.Fatalf("pull(cursor=%d) 失败：%v", cursor, err)
		}
		if b.HighVersion < cursor {
			t.Fatalf("游标倒退：%d → %d", cursor, b.HighVersion)
		}
		size := len(b.Transactions) + len(b.Accounts) + len(b.Categories) + len(b.Holdings)
		if b.HighVersion == cursor {
			if size != 0 {
				t.Fatalf("游标未推进（%d）却返回了 %d 行", cursor, size)
			}
			return batches, cursor
		}
		batches = append(batches, size)
		cursor = b.HighVersion
	}
	t.Fatal("pull 未在 1000 批内收敛（疑似死循环）")
	return nil, 0
}

// TestR48OnlyAccountChangesAreBatched 只有账户变更：每批规模受 limit 约束，
// 绝不能像旧实现那样"一次全量返回"。
func TestR48OnlyAccountChangesAreBatched(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())

	const total = 20
	for i := 0; i < total; i++ {
		_, err := as.Create(e.uid, e.bookID, AccountCreateInput{
			Name: fmt.Sprintf("账户%d", i), Category: model.CategoryCash, Currency: "CNY",
		})
		must(t, err)
	}

	const limit = 5
	batches, _ := pullAll(t, e, limit)
	if len(batches) == 0 {
		t.Fatal("应至少有一批增量")
	}
	for i, n := range batches {
		// 每个账户各占一个版本 ⇒ 每批恰好 limit 行（除非是最后一批）
		if n > limit {
			t.Fatalf("第 %d 批返回 %d 行，超过 limit=%d（水位没被 limit 约束住）", i+1, n, limit)
		}
	}
	sum := 0
	for _, n := range batches {
		sum += n
	}
	// seed 阶段已有账户/分类，这里只断言"新增的 20 个账户被分批下发且一批不超过 limit"
	if len(batches) < total/limit {
		t.Fatalf("20 个账户按 limit=5 至少应分 %d 批，实际 %d 批", total/limit, len(batches))
	}
}

// TestR48MixedChangesBoundedByLimit 混合变更（交易 + 账户 + 分类）：
// 每批总行数（跨实体合计）受 limit 约束。
func TestR48MixedChangesBoundedByLimit(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	cs := NewCategoryService(e.engine, version.NewRepo())
	ctx := context.Background()

	// 交错产生交易 / 账户 / 分类变更，每类各 6 次（共 18 个版本，每版本 1~2 行）
	for i := 0; i < 6; i++ {
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
	batches, _ := pullAll(t, e, limit)
	if len(batches) == 0 {
		t.Fatal("应至少有一批增量")
	}
	for i, n := range batches {
		// 转账会产生 2 行同版本记录（本用例没有转账），故每批应恰好 ≤ limit；
		// 允许的唯一超出是"同一 sync_version 整体回扩"，本用例每个版本只有 1 行。
		if n > limit {
			t.Fatalf("第 %d 批返回 %d 行，超过 limit=%d", i+1, n, limit)
		}
	}
}

// TestR48SameVersionReturnedAsWhole 同一 sync_version 的大量记录必须【整体】返回：
// 允许因此超过 limit，但游标必须一次推进到该版本（不允许拆版本）。
func TestR48SameVersionReturnedAsWhole(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())

	// 建 1 个账户占位，再把所有账户行改到同一个 sync_version（模拟批量迁移产生的同版本写入）
	_, err := as.Create(e.uid, e.bookID, AccountCreateInput{
		Name: "批量账户", Category: model.CategoryCash, Currency: "CNY",
	})
	must(t, err)
	execSQL(t, e, "UPDATE accounts SET sync_version = 900 WHERE book_id = ?", e.bookID)

	const limit = 2
	svc := NewSyncService(e.engine, e.txs, nil)
	b, err := svc.Pull(e.uid, e.bookID, 0, limit)
	must(t, err)

	// 同版本回扩：本批必须把版本 900 的所有账户行一次带齐（旧实现会只带 limit 行、
	// 但把游标推到 900，导致其余行永久丢失）
	var atV900 int64
	if _, err := e.engine.SQL(
		"SELECT COUNT(1) FROM accounts WHERE book_id = ? AND sync_version = 900", e.bookID).
		Get(&atV900); err != nil {
		t.Fatal(err)
	}
	if int64(len(b.Accounts)) != atV900 {
		t.Fatalf("版本 900 的 %d 行账户必须整体返回，实际 %d 行", atV900, len(b.Accounts))
	}
	if b.HighVersion != 900 {
		t.Fatalf("游标应推进到 900，实际 %d", b.HighVersion)
	}
}
