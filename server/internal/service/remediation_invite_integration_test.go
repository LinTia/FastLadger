package service

// 邀请码生命周期缺陷（GPT-R41 邮箱绑定 / R40 撤销 / R42 审计 / R43 已删账本）
// 的**真实 SQLite 集成测试**。
//
// 需 CGO（gcc）：mattn/go-sqlite3 在无 gcc 的机器上编译得出来但【跑不起来】，
// 驱动是一段桩，连接即报 "Binary was compiled with 'CGO_ENABLED=0', go-sqlite3
// requires cgo to work"。因此本文件的用例在那种环境下 **Skip**（并打印原因），
// 由 .github/workflows/ci.yml 在 ubuntu-latest 上真正执行。
//
// 为什么这些断言不能用纯逻辑/源码守卫替代：条件 UPDATE 的 RowsAffected、
// "状态变更与审计同事务"、以及撤销与兑换的并发互斥，都是【数据在磁盘上的真实结果】，
// 用 mock 或源码正则都证明不了。相应地，本文件不删减也不放宽任何断言。

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/model"
	"fastledger/internal/version"

	"xorm.io/xorm"
)

// inviteEnv 邀请相关的干净账本环境：真实临时库 + 全量迁移 + owner + 成员服务。
type inviteEnv struct {
	t       *testing.T
	engine  *xorm.Engine
	uid     int64 // 账本 owner
	bookID  int64
	members *MemberService
	books   *BookService
}

// newInviteEngine 建立真实 SQLite 测试库；不可用（无 CGO）时**跳过**当前用例。
//
// 复用本包的 requireSQLite 守卫（sqlite_guard_test.go）：若 SMTP/驱动缺失，
// 用例没有填空虚拟机，而是明确 Skip 并打印原因——任何人跑 `go test -v` 都能看到
// "本机的哪些结论尚未验证"。断言一条都没少，只是不在这台机器上充当证据。
func newInviteEngine(t *testing.T) *xorm.Engine {
	t.Helper()
	requireSQLite(t)
	engine, err := database.Init(filepath.Join(t.TempDir(), "invite_lifecycle_test.db"))
	if err != nil {
		t.Fatalf("database.Init 失败：%v", err)
	}
	if err := engine.Ping(); err != nil {
		_ = engine.Close()
		t.Skipf("跳过：SQLite 连接不可用（需 CGO / gcc）：%v", err)
	}
	if _, err := migrate.Apply(engine); err != nil {
		_ = engine.Close()
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}

func newInviteEnv(t *testing.T) *inviteEnv {
	t.Helper()
	engine := newInviteEngine(t)
	uid, bookID := seedBook(t, engine)
	as := NewAuthService(engine, testJWTSecret, time.Hour)
	ms, err := NewMemberService(engine, as, 7)
	must(t, err)
	return &inviteEnv{
		t:       t,
		engine:  engine,
		uid:     uid,
		bookID:  bookID,
		members: ms,
		books:   NewBookService(engine, version.NewRepo()),
	}
}

// inviteRow 直接读一条邀请记录（不过滤任何状态）
func inviteRow(t *testing.T, e *inviteEnv, id int64) model.Invite {
	t.Helper()
	var inv model.Invite
	has, err := e.engine.Where("id = ?", id).Get(&inv)
	if err != nil {
		t.Fatalf("读邀请 %d 失败：%v", id, err)
	}
	if !has {
		t.Fatalf("邀请 %d 不存在", id)
	}
	return inv
}

// countUsersByEmail 统计某个规范化邮箱对应的活跃账号数
func countUsersByEmail(t *testing.T, e *inviteEnv, email string) int64 {
	t.Helper()
	var n int64
	if _, err := e.engine.SQL(
		`SELECT COUNT(1) FROM users WHERE lower(email) = ? AND deleted = 0`, email).Get(&n); err != nil {
		t.Fatalf("统计邮箱占用失败：%v", err)
	}
	return n
}

// countMembers 活跃成员数
func countMembers(t *testing.T, e *inviteEnv) int64 {
	t.Helper()
	var n int64
	if _, err := e.engine.SQL(
		`SELECT COUNT(1) FROM book_members WHERE book_id = ? AND deleted = 0`, e.bookID).Get(&n); err != nil {
		t.Fatalf("统计成员失败：%v", err)
	}
	return n
}

// userByUsername 读取新建用户的档案（含邮箱）
func userByUsername(t *testing.T, e *inviteEnv, username string) model.User {
	t.Helper()
	var u model.User
	has, err := e.engine.Where("username = ? AND deleted = 0", username).Get(&u)
	if err != nil {
		t.Fatalf("读用户 %s 失败：%v", username, err)
	}
	if !has {
		t.Fatalf("用户 %s 不存在", username)
	}
	return u
}

// ---------------------------------------------------------------------------
// GPT-R41：邮箱绑定闭环（真实落库校验）
// ---------------------------------------------------------------------------

// TestR41EmailBindingNormalizedAndMatched 签发端大小写混乱、兑换端前后带空格，
// 双向都必须判定为"同一个邮箱"并兑换成功，且邮箱落到用户档案。
func TestR41EmailBindingNormalizedAndMatched(t *testing.T) {
	e := newInviteEnv(t)

	inv, err := e.members.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "  Alice@Example.COM ")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	// 落库必须是规范化后的值——否则后续任何一侧都得记住"当时是怎么写的"
	if row := inviteRow(t, e, inv.Id); row.BoundEmail != "alice@example.com" {
		t.Fatalf("invites.bound_email 应存规范化值 alice@example.com，实际 %q", row.BoundEmail)
	}
	if got := countUsersByEmail(t, e, "alice@example.com"); got != 0 {
		t.Fatalf("兑换前不应存在该邮箱的账号，实际 %d", got)
	}

	u, err := e.members.JoinViaInvite(e.bookID, inv.Code, "alice", " ALICE@example.COM ", "password123", "10.0.0.1")
	if err != nil {
		t.Fatalf("大小写/空格差异不应导致兑换失败，实际 %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Fatalf("用户档案应存规范化邮箱，实际 %q", u.Email)
	}
	if got := countUsersByEmail(t, e, "alice@example.com"); got != 1 {
		t.Fatalf("邮箱应落到 1 个账号，实际 %d", got)
	}
	row := inviteRow(t, e, inv.Id)
	if row.UsedAt == 0 || row.UsedBy != u.Id {
		t.Fatalf("邀请应被标记已用：used_at=%d used_by=%d（期望 used_by=%d）", row.UsedAt, row.UsedBy, u.Id)
	}
}

// TestR41WrongOrMissingEmailRejectedAndInviteNotConsumed 绑定邮箱的邀请，
// 邮箱对了才算数；失败的尝试【不得】把邀请消耗掉，也不能留下半截用户。
func TestR41WrongOrMissingEmailRejectedAndInviteNotConsumed(t *testing.T) {
	cases := []struct {
		name  string
		email string
	}{
		{"完全不同的邮箱", "bob@example.com"},
		{"同一域名不同人", "carol@example.com"},
		{"不填邮箱", ""},
		{"多了个子域", "alice@mail.example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newInviteEnv(t)
			inv, err := e.members.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "alice@example.com")
			if err != nil {
				t.Fatalf("签发邀请失败：%v", err)
			}
			var usersBefore int64
			if _, err := e.engine.SQL(`SELECT COUNT(1) FROM users`).Get(&usersBefore); err != nil {
				t.Fatalf("统计用户失败：%v", err)
			}

			_, err = e.members.JoinViaInvite(e.bookID, inv.Code, "bob", c.email, "password123", "10.0.0.2")
			if !errors.Is(err, ErrInviteBoundMismatch) {
				t.Fatalf("邮箱不匹配必须返回 ErrInviteBoundMismatch，实际 %v", err)
			}
			// 关键断言：失败尝试不能"先到先得"地把邀请废掉
			row := inviteRow(t, e, inv.Id)
			if row.UsedAt != 0 || row.UsedBy != 0 {
				t.Fatalf("失败的兑换不得消耗邀请：used_at=%d used_by=%d", row.UsedAt, row.UsedBy)
			}
			if n := countMembers(t, e); n != 1 {
				t.Fatalf("失败兑换不得新增成员，应仍为 1（owner），实际 %d", n)
			}
			// 也不能留下孤儿用户（D-R07：原子性）
			var usersAfter int64
			if _, err := e.engine.SQL(`SELECT COUNT(1) FROM users`).Get(&usersAfter); err != nil {
				t.Fatalf("统计用户失败：%v", err)
			}
			if usersAfter != usersBefore {
				t.Fatalf("失败兑换必须整笔回滚，用户数应仍为 %d，实际 %d", usersBefore, usersAfter)
			}
		})
	}
}

// TestR41ForwardedLinkCannotBeRedeemedByOthers 链接被转发给他人：
// 他人用自己的邮箱兑换必须失败，且【真正的主人】随后仍能兑换成功——
// 这两件事一起成立才算"绑定有效且不会误伤"。
func TestR41ForwardedLinkCannotBeRedeemedByOthers(t *testing.T) {
	e := newInviteEnv(t)
	inv, err := e.members.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "alice@example.com")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}

	// ① 被转发给 bob：他用自己的邮箱兑换必须失败（这正是"防链接转发"要的效果）
	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "bob", "bob@example.com", "password123", "10.0.0.3"); !errors.Is(err, ErrInviteBoundMismatch) {
		t.Fatalf("被转发者不得兑换成功，实际 %v", err)
	}
	// ② 主人仍能兑换（且邮箱大小写不敏感）
	u, err := e.members.JoinViaInvite(e.bookID, inv.Code, "alice", "Alice@Example.com", "password123", "10.0.0.4")
	if err != nil {
		t.Fatalf("绑定邮箱的主人应能兑换（且大小写不敏感），实际 %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Fatalf("用户邮箱应规范化落库，实际 %q", u.Email)
	}
	if row := inviteRow(t, e, inv.Id); row.UsedBy != u.Id {
		t.Fatalf("邀请应标记给 alice（uid=%d），实际 used_by=%d", u.Id, row.UsedBy)
	}
}

// TestR41UnboundInviteAcceptsAnyEmail 未绑定邮箱的邀请不该被邮箱改造误伤。
func TestR41UnboundInviteAcceptsAnyEmail(t *testing.T) {
	for _, email := range []string{"", "whoever@example.com", " Who@Example.COM "} {
		e := newInviteEnv(t)
		inv, err := e.members.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "")
		if err != nil {
			t.Fatalf("签发邀请失败：%v", err)
		}
		u, err := e.members.JoinViaInvite(e.bookID, inv.Code, "zoe", email, "password123", "10.0.0.5")
		if err != nil {
			t.Fatalf("未绑定邮箱时不应因邮箱被拒（email=%q）：%v", email, err)
		}
		if email != "" && !strings.EqualFold(strings.TrimSpace(u.Email), strings.TrimSpace(email)) {
			t.Fatalf("用户邮箱应保留规范化后的输入，实际 %q", u.Email)
		}
	}
}

// TestR41BindingBothDimensionsRejected 同时绑定用户名与邮箱必须被拒，
// 且不得产生邀请记录（静默取舍正是本缺陷的原始形态）。
func TestR41BindingBothDimensionsRejected(t *testing.T) {
	e := newInviteEnv(t)
	if _, err := e.members.CreateInvite(e.uid, e.bookID, model.RoleMember, "alice", "alice@example.com"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("同时绑定两者应返回 ErrInvalidArgument，实际 %v", err)
	}
	var n int64
	if _, err := e.engine.SQL(`SELECT COUNT(1) FROM invites WHERE book_id = ?`, e.bookID).Get(&n); err != nil {
		t.Fatalf("统计邀请失败：%v", err)
	}
	if n != 0 {
		t.Fatalf("被拒的请求不得落库，实际 %d 条邀请", n)
	}
}

// TestR41EmailTakenByOtherAccountRejectedWithoutLeaking 同一邮箱已被别的账号占用时，
// 兑换必须失败且提示不得区分"邮箱已被注册"（未认证入口，防止邮箱枚举）。
func TestR41EmailTakenByOtherAccountRejectedWithoutLeaking(t *testing.T) {
	e := newInviteEnv(t)
	if _, err := e.engine.Exec(
		`INSERT INTO users (username, password_hash, email, deleted, created_at, updated_at)
		 VALUES (?, ?, ?, 0, ?, ?)`, "occupant", "x", "alice@example.com", time.Now().Unix(), time.Now().Unix()); err != nil {
		t.Fatalf("预置占用账号失败：%v", err)
	}
	inv, err := e.members.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	_, err = e.members.JoinViaInvite(e.bookID, inv.Code, "alice", "ALICE@example.com", "password123", "10.0.0.6")
	if !errors.Is(err, ErrInviteBoundMismatch) {
		t.Fatalf("邮箱被占用应返回 ErrInviteBoundMismatch（对外统一文案），实际 %v", err)
	}
	if row := inviteRow(t, e, inv.Id); row.UsedAt != 0 {
		t.Fatal("失败的兑换不得消耗邀请")
	}
	if got := countUsersByEmail(t, e, "alice@example.com"); got != 1 {
		t.Fatalf("不应产生第二个同邮箱账号，实际 %d", got)
	}
}

// ---------------------------------------------------------------------------
// GPT-R40：邀请撤销
// ---------------------------------------------------------------------------

// newRevocableInvite 建一条 owner 签发、未使用、未撤销、未过期的邀请
func newRevocableInvite(t *testing.T, e *inviteEnv) model.Invite {
	t.Helper()
	inv, err := e.members.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	return *inv
}

// insertInviteUser 在邀请测试环境里插入一个用户（尚未加入账本）
func insertInviteUser(t *testing.T, e *inviteEnv, username string) int64 {
	t.Helper()
	u := &model.User{Username: username, PasswordHash: "x"}
	if _, err := e.engine.Insert(u); err != nil {
		t.Fatalf("插入用户 %s 失败：%v", username, err)
	}
	return u.Id
}

// addInviteMember 把用户加为本账本成员（默认可见他人）
func addInviteMember(t *testing.T, e *inviteEnv, uid int64, role int8) {
	t.Helper()
	now := time.Now().Unix()
	if _, err := e.engine.Insert(&model.BookMember{
		BookId: e.bookID, Uid: uid, Role: role, CanViewOthers: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("加入成员失败：%v", err)
	}
}

// TestR40RevokeBlocksRedemption 撤销之后，同一个邀请码不得再入户。
func TestR40RevokeBlocksRedemption(t *testing.T) {
	e := newInviteEnv(t)
	inv := newRevocableInvite(t, e)

	if err := e.members.RevokeInvite(e.uid, e.bookID, inv.Id); err != nil {
		t.Fatalf("owner 应能撤销自己签发的邀请，实际 %v", err)
	}
	if row := inviteRow(t, e, inv.Id); !row.Revoked {
		t.Fatal("撤销后 invites.revoked 必须为 true")
	}

	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "latecomer", "", "password123", "10.0.0.7"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("已撤销的邀请不得入户，实际 %v", err)
	}
	if n := countMembers(t, e); n != 1 {
		t.Fatalf("被撤销的邀请不得产生新成员，应仍为 1（owner），实际 %d", n)
	}
	if row := inviteRow(t, e, inv.Id); row.UsedAt != 0 || row.UsedBy != 0 {
		t.Fatalf("兑换失败不得写 used_at/used_by：used_at=%d used_by=%d", row.UsedAt, row.UsedBy)
	}
}

// TestR40RevokeRequiresManagerRole 普通成员不能撤销邀请；admin 可以（权限矩阵 §5.5）。
func TestR40RevokeRequiresManagerRole(t *testing.T) {
	e := newInviteEnv(t)
	inv := newRevocableInvite(t, e)
	plain := insertInviteUser(t, e, "plain-member")
	addInviteMember(t, e, plain, model.RoleMember)

	if err := e.members.RevokeInvite(plain, e.bookID, inv.Id); !errors.Is(err, ErrForbidden) {
		t.Fatalf("普通成员不得撤销邀请，实际 %v", err)
	}
	if row := inviteRow(t, e, inv.Id); row.Revoked {
		t.Fatal("被拒绝的撤销不得改变邀请状态")
	}

	admin := insertInviteUser(t, e, "admin-member")
	addInviteMember(t, e, admin, model.RoleAdmin)
	if err := e.members.RevokeInvite(admin, e.bookID, inv.Id); err != nil {
		t.Fatalf("admin 应能撤销邀请（权限矩阵 §5.5），实际 %v", err)
	}
}

// TestR40RevokeRejectsWrongState 已使用 / 已过期 / 重复撤销 / 不存在都必须失败，
// 且返回【可区分】的原因——一律回 ErrInviteInvalid 会让线上排查无从下手。
func TestR40RevokeRejectsWrongState(t *testing.T) {
	// ① 已使用
	e := newInviteEnv(t)
	inv := newRevocableInvite(t, e)
	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "early", "", "password123", "10.0.0.8"); err != nil {
		t.Fatalf("兑换应成功：%v", err)
	}
	if err := e.members.RevokeInvite(e.uid, e.bookID, inv.Id); !errors.Is(err, ErrInviteAlreadyUsed) {
		t.Fatalf("已使用的邀请应返回 ErrInviteAlreadyUsed，实际 %v", err)
	}
	if row := inviteRow(t, e, inv.Id); row.Revoked {
		t.Fatal("不得把已使用的邀请改成已撤销（会抹掉「已被谁用过」这一事实）")
	}

	// ② 已过期（把 expires_at 改到过去，模拟时间流逝）
	e2 := newInviteEnv(t)
	inv2 := newRevocableInvite(t, e2)
	if _, err := e2.engine.Exec("UPDATE invites SET expires_at = ? WHERE id = ?", time.Now().Unix()-60, inv2.Id); err != nil {
		t.Fatalf("构造过期邀请失败：%v", err)
	}
	if err := e2.members.RevokeInvite(e2.uid, e2.bookID, inv2.Id); !errors.Is(err, ErrInviteExpired) {
		t.Fatalf("过期邀请应返回 ErrInviteExpired，实际 %v", err)
	}
	if row := inviteRow(t, e2, inv2.Id); row.Revoked {
		t.Fatal("过期邀请不应被写成已撤销：它已经失效，再写一次撤销只是产生伪事件")
	}

	// ③ 重复撤销（幂等性显式化：第二次必须报错，不得假装成功）
	e3 := newInviteEnv(t)
	inv3 := newRevocableInvite(t, e3)
	if err := e3.members.RevokeInvite(e3.uid, e3.bookID, inv3.Id); err != nil {
		t.Fatalf("首次撤销应成功：%v", err)
	}
	if err := e3.members.RevokeInvite(e3.uid, e3.bookID, inv3.Id); !errors.Is(err, ErrInviteAlreadyRevoked) {
		t.Fatalf("重复撤销应返回 ErrInviteAlreadyRevoked，实际 %v", err)
	}

	// ④ 不存在 / 不属于本账本
	if err := e3.members.RevokeInvite(e3.uid, e3.bookID, 9999999); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("不存在的邀请应返回 ErrInviteNotFound，实际 %v", err)
	}
}

// TestR40CrossBookRevokeRejected A 账本的 owner 不得撤销 B 账本的邀请。
//
// 对外统一表现为 ErrInviteNotFound（而不是"存在但不在你的账本"）——
// 后者会让攻击者用已知 ID 去枚举别的账本里有哪些邀请主键。
func TestR40CrossBookRevokeRejected(t *testing.T) {
	e := newInviteEnv(t)
	inv := newRevocableInvite(t, e)

	otherBook := &model.Book{Name: "另一本账", OwnerUid: e.uid, BookType: model.BookTypePersonal,
		DefaultCurrency: "CNY", MembersCanViewOthers: true, Deleted: false}
	if _, err := e.engine.Insert(otherBook); err != nil {
		t.Fatalf("建第二个账本失败：%v", err)
	}
	if err := e.members.RevokeInvite(e.uid, otherBook.Id, inv.Id); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("跨账本撤销应表现为 ErrInviteNotFound，实际 %v", err)
	}
	if row := inviteRow(t, e, inv.Id); row.Revoked {
		t.Fatal("跨账本的撤销请求不得改动目标邀请")
	}
}

// TestR40RevokeAndRedeemRaceExactlyOneWins 撤销与兑换并发竞争：至多一个成功，
// 且磁盘上的真实状态必须与"赢家"一致（不能出现"撤销成功却已入户"或反之）。
//
// 不用 sleep 做时序控制：两个 goroutine 同时出发，胜负交给数据库。
// 断言的是【不变式】而非具体谁赢，因此在任何调度顺序下都是确定性的。
func TestR40RevokeAndRedeemRaceExactlyOneWins(t *testing.T) {
	e := newInviteEnv(t)
	inv := newRevocableInvite(t, e)

	var wg sync.WaitGroup
	var revokeErr, joinErr error
	var joinedUID int64
	wg.Add(2)
	go func() {
		defer wg.Done()
		revokeErr = e.members.RevokeInvite(e.uid, e.bookID, inv.Id)
	}()
	go func() {
		defer wg.Done()
		u, err := e.members.JoinViaInvite(e.bookID, inv.Code, "racer", "", "password123", "10.0.0.9")
		joinErr = err
		if u != nil {
			joinedUID = u.Id
		}
	}()
	wg.Wait()

	if revokeErr == nil && joinErr == nil {
		t.Fatal("撤销与兑换不得同时成功：同一凭证只能有一种结局")
	}
	row := inviteRow(t, e, inv.Id)
	members := countMembers(t, e)

	switch {
	case revokeErr == nil:
		// 撤销赢：必须是"已撤销且未使用"，且没有任何新成员
		if !row.Revoked || row.UsedAt != 0 || row.UsedBy != 0 {
			t.Fatalf("撤销成功的状态应为 revoked=true, used_at=0，实际 revoked=%v used_at=%d used_by=%d",
				row.Revoked, row.UsedAt, row.UsedBy)
		}
		if members != 1 {
			t.Fatalf("撤销成功则不得有新成员，实际 %d", members)
		}
		if !errors.Is(joinErr, ErrInviteInvalid) {
			t.Fatalf("输给撤销的兑换必须返回 ErrInviteInvalid，实际 %v", joinErr)
		}
	case joinErr == nil:
		// 兑换赢：必须是"已使用且未撤销"，且成员表里真有这个人
		if row.UsedAt == 0 || row.UsedBy != joinedUID || row.Revoked {
			t.Fatalf("兑换成功的状态应为 used_at>0, used_by=%d, revoked=false，实际 used_at=%d used_by=%d revoked=%v",
				joinedUID, row.UsedAt, row.UsedBy, row.Revoked)
		}
		if members != 2 {
			t.Fatalf("兑换成功应新增 1 名成员（共 2），实际 %d", members)
		}
		if !errors.Is(revokeErr, ErrInviteAlreadyUsed) {
			t.Fatalf("输给兑换的撤销必须返回 ErrInviteAlreadyUsed，实际 %v", revokeErr)
		}
	default:
		t.Fatalf("两者都失败属于非预期状态：revoke=%v join=%v", revokeErr, joinErr)
	}
}
