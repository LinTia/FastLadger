package service

// GPT-R47：账户分组（parent_id）必须无环。
//
// 这些用例依赖真实 SQLite（CGO），本机无 gcc 时会**显式跳过并打印原因**
// （见 sqlite_guard_test.go），由 CI（ubuntu-latest）执行；断言一条都没放宽。
//
// 覆盖：两节点环、三层环、更长（10 层）的环、父链里有既存环（历史脏数据）、
// 正常多层树不得误报、以及并发下"两个方向同时挂"也不可能成环。
//
// 与相邻问题的边界（刻意不混入）：
//   - GPT-R53：删除父账户留下悬空引用 → 那是"被指向的一方能不能消失"；
//   - GPT-R31：交易引用账户 → 那是"账户能不能被删"。
//     本文件只回答"能不能指向"这一件事。

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// newAccount 建一个账户（默认顶层）
func newAccount(t *testing.T, e *testEnv, name string, parentID int64) *model.Account {
	t.Helper()
	as := NewAccountService(e.engine, version.NewRepo())
	a, err := as.Create(e.uid, e.bookID, AccountCreateInput{
		Name: name, Category: model.CategoryCash, Currency: "CNY", ParentId: parentID,
	})
	must(t, err)
	return a
}

// parentOf 直接读一行账户的 parent_id（含已删除行，便于核查脏数据）
func parentOf(t *testing.T, e *testEnv, id int64) int64 {
	t.Helper()
	var p int64
	has, err := e.engine.SQL("SELECT parent_id FROM accounts WHERE id = ?", id).Get(&p)
	if err != nil {
		t.Fatalf("读账户 %d 的 parent_id 失败：%v", id, err)
	}
	if !has {
		t.Fatalf("账户 %d 不存在", id)
	}
	return p
}

// updateParent 走服务层修改 parent_id（被测入口）
func updateParent(t *testing.T, e *testEnv, id, parentID int64) error {
	t.Helper()
	as := NewAccountService(e.engine, version.NewRepo())
	_, err := as.Update(e.uid, e.bookID, id, AccountUpdateInput{ParentId: &parentID})
	return err
}

// TestR47AccountCycleDetected 两节点环（A→B→A）必须被拒绝，且不得留下部分写入。
func TestR47AccountCycleDetected(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)

	a := newAccount(t, e, "A", 0)
	b := newAccount(t, e, "B", a.Id) // B 的上级是 A

	// 把 A 的上级改成 B → A→B→A
	err := updateParent(t, e, a.Id, b.Id)
	if !errors.Is(err, ErrAccountCycle) {
		t.Fatalf("A→B→A 必须返回 ErrAccountCycle，实际 %v", err)
	}
	// 被拒后不得有任何写入：A 仍是顶层，B 仍挂在 A 下
	if got := parentOf(t, e, a.Id); got != 0 {
		t.Fatalf("被拒后 A 的 parent_id 应仍为 0，实际 %d", got)
	}
	if got := parentOf(t, e, b.Id); got != a.Id {
		t.Fatalf("被拒后 B 的 parent_id 应仍为 %d，实际 %d", a.Id, got)
	}
}

// TestR47SelfParentRejected 单节点环（上级指向自身）同样归入环错误。
func TestR47SelfParentRejected(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	a := newAccount(t, e, "A", 0)

	if err := updateParent(t, e, a.Id, a.Id); !errors.Is(err, ErrAccountCycle) {
		t.Fatalf("上级指向自身应返回 ErrAccountCycle，实际 %v", err)
	}
	if got := parentOf(t, e, a.Id); got != 0 {
		t.Fatalf("被拒后 parent_id 应保持 0，实际 %d", got)
	}
}

// TestR47LongCycleDetected 三层环与 10 层环：只查一两层的实现会漏掉这些。
func TestR47LongCycleDetected(t *testing.T) {
	requireSQLite(t)

	cases := []struct {
		name  string
		depth int
	}{
		{"三层环 A→B→C→A", 3},
		{"五层环", 5},
		{"十层环", 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newTestEnv(t)
			// 建链：root ← n1 ← n2 ← ... ← n(depth-1)
			root := newAccount(t, e, "root", 0)
			parent := root
			tail := root
			for i := 1; i < c.depth; i++ {
				tail = newAccount(t, e, fmt.Sprintf("n%d", i), parent.Id)
				parent = tail
			}
			// 把 root 挂到链尾 → 形成 depth 长度的环
			err := updateParent(t, e, root.Id, tail.Id)
			if !errors.Is(err, ErrAccountCycle) {
				t.Fatalf("%d 层环必须返回 ErrAccountCycle，实际 %v", c.depth, err)
			}
			if got := parentOf(t, e, root.Id); got != 0 {
				t.Fatalf("被拒后 root 的 parent_id 应仍为 0，实际 %d", got)
			}
		})
	}
}

// TestR47CreateRejectedWhenParentChainAlreadyCyclic 历史脏数据里已存在环时，
// 新建账户不得再挂进去（否则环会继续扩大）。
func TestR47CreateRejectedWhenParentChainAlreadyCyclic(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)

	a := newAccount(t, e, "A", 0)
	b := newAccount(t, e, "B", a.Id)
	// 直接改库制造既存环（真实成因：修复前的旧版本写进去的脏数据）
	execSQL(t, e, "UPDATE accounts SET parent_id = ? WHERE id = ?", b.Id, a.Id)

	as := NewAccountService(e.engine, version.NewRepo())
	_, err := as.Create(e.uid, e.bookID, AccountCreateInput{
		Name: "C", Category: model.CategoryCash, Currency: "CNY", ParentId: a.Id,
	})
	if !errors.Is(err, ErrAccountCycle) {
		t.Fatalf("父链已存在环时新建账户必须被拒，实际 %v", err)
	}
	var n int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE name = 'C'").Get(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("被拒后不得落库，实际插入了 %d 行", n)
	}
}

// TestR47DeepTreeNotFalsePositive 正常的多层结构（含"整棵子树搬家"）不得误报成环。
func TestR47DeepTreeNotFalsePositive(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)

	// 一条 12 层的深链：正常数据，绝不能被判成环
	root := newAccount(t, e, "root", 0)
	parent := root
	var tail *model.Account = root
	for i := 1; i < 12; i++ {
		tail = newAccount(t, e, fmt.Sprintf("d%d", i), parent.Id)
		parent = tail
	}
	// 深链中间再挂一个新账户（合法）
	leaf := newAccount(t, e, "leaf", tail.Id)
	if got := parentOf(t, e, leaf.Id); got != tail.Id {
		t.Fatalf("合法挂载被误判：leaf 的 parent_id 应为 %d，实际 %d", tail.Id, got)
	}
	// 另一棵独立的子树
	otherRoot := newAccount(t, e, "otherRoot", 0)
	otherChild := newAccount(t, e, "otherChild", otherRoot.Id)
	// 把 otherChild 整棵搬到 root 下（合法：root 不在 otherChild 的子树里）
	must(t, updateParent(t, e, otherChild.Id, root.Id))
	if got := parentOf(t, e, otherChild.Id); got != root.Id {
		t.Fatalf("合法搬家被误判，实际 parent_id=%d", got)
	}
	// 再搬回来也合法
	must(t, updateParent(t, e, otherChild.Id, otherRoot.Id))
	if got := parentOf(t, e, otherChild.Id); got != otherRoot.Id {
		t.Fatalf("搬回应生效，实际 parent_id=%d", got)
	}
	// 取消分组（parent_id=0）永远合法
	must(t, updateParent(t, e, otherChild.Id, 0))
	if got := parentOf(t, e, otherChild.Id); got != 0 {
		t.Fatalf("取消分组应生效，实际 parent_id=%d", got)
	}
}

// TestR47ConcurrentOppositeMovesCannotFormCycle 并发下两个相反方向同时挂，
// 不允许最终出现环（SQLite 单写者 + 检查与写入同事务）。
//
// 用 channel 屏障钉住"同时起跑"，不用随机 sleep：
// 两个 goroutine 分别尝试 A.parent=B 与 B.parent=A，若都成功就成环。
func TestR47ConcurrentOppositeMovesCannotFormCycle(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	a := newAccount(t, e, "A", 0)
	b := newAccount(t, e, "B", 0)

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i, move := range []func() error{
		func() error { return updateParent(t, e, a.Id, b.Id) },
		func() error { return updateParent(t, e, b.Id, a.Id) },
	} {
		wg.Add(1)
		go func(i int, move func() error) {
			defer wg.Done()
			<-start
			errs[i] = move()
		}(i, move)
	}
	close(start)
	wg.Wait()

	pa, pb := parentOf(t, e, a.Id), parentOf(t, e, b.Id)
	if pa == b.Id && pb == a.Id {
		t.Fatalf("并发下形成了环：A.parent=%d, B.parent=%d（errs=%v）", pa, pb, errs)
	}
	// 至少有一个方向被环检测挡下，或两者都没形成环——不允许"环静默落库"
	if pa == b.Id && !errors.Is(errs[1], ErrAccountCycle) && errs[1] != nil {
		t.Fatalf("B→A 未成功却被报了非环错误：%v", errs[1])
	}
}

// TestR47DanglingAncestorRejected 父链中出现已删除账户（悬空引用）时拒绝挂载。
func TestR47DanglingAncestorRejected(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)

	mid := newAccount(t, e, "mid", 0)
	leaf := newAccount(t, e, "leaf", mid.Id)
	// 直接改库把 mid 软删，制造"父链上有已删除节点"的脏数据
	execSQL(t, e, "UPDATE accounts SET deleted = 1, deleted_at = ? WHERE id = ?",
		time.Now().Unix(), mid.Id)

	if err := updateParent(t, e, leaf.Id, mid.Id); err == nil {
		t.Fatal("父链上存在已删除账户时必须拒绝挂载（不得制造新的悬空引用）")
	}
}
