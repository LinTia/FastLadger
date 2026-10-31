// Package authzreview 权限 / 多租户隔离 / 邀请生命周期 的**复审证据用例**（reviewer-authz）。
//
// 为什么单独放一个包：本仓正有多名复审员并行往 internal/service、internal/api 里
// 加临时探针用例，那些文件在编辑中途会短暂编译不过；把证据用例放在独立包里，
// 只依赖被测包的**非测试**代码，因此任何人的中间态都不会挡住本包的验证。
// 结论确认后，这些用例可以原样搬进 internal/service / internal/api 作为回归测试。
//
// 判定约定：
//   - 断言的是"应成立的安全不变量"，**断言失败即缺陷成立**（红灯就是证据）；
//   - AuthzCtl* 前缀的用例是反向对照，必须【通过】——它们证明"看起来可疑的路径其实没问题"。
//
// 环境：需要 CGO + gcc（mattn/go-sqlite3）。本机 Application Control 会间歇拦下
// 新生成的测试二进制（"An Application Control policy has blocked this file"），
// 重试即可，那不是测试失败。
package authzreview

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/api"
	"fastledger/internal/config"
	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"
	"fastledger/internal/ws"

	"github.com/gin-gonic/gin"
	"xorm.io/xorm"
)

// ---------- 环境 ----------

// engine 真实临时 SQLite 库（已应用全量迁移）；环境不具备则明确 Skip。
func engine(t *testing.T) *xorm.Engine {
	t.Helper()
	e, err := database.Init(filepath.Join(t.TempDir(), "authz_review.db"))
	if err != nil {
		t.Skipf("跳过：SQLite 不可用（需 CGO / gcc）：%v", err)
	}
	if err := e.Ping(); err != nil {
		_ = e.Close()
		t.Skipf("跳过：SQLite 连接不可用（需 CGO / gcc）：%v", err)
	}
	if _, err := migrate.Apply(e); err != nil {
		_ = e.Close()
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

// env 一个 owner + admin + member 的家庭账本。
type env struct {
	t       *testing.T
	engine  *xorm.Engine
	owner   int64
	admin   int64
	member  int64
	bookID  int64
	books   *service.BookService
	members *service.MemberService
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := engine(t)
	books := service.NewBookService(e, version.NewRepo())
	as := service.NewAuthService(e, "authz-review-secret-0123456789abcdef", time.Hour)
	members, err := service.NewMemberService(e, as, 7)
	if err != nil {
		t.Fatalf("NewMemberService 失败：%v", err)
	}
	newUser := func(name string) int64 {
		u := &model.User{Username: name, PasswordHash: "x"}
		if _, err := e.Insert(u); err != nil {
			t.Fatalf("插入用户 %s 失败：%v", name, err)
		}
		return u.Id
	}
	owner := newUser("authz_owner")
	b, err := books.Create(owner, "家账", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建账本失败：%v", err)
	}
	admin := newUser("authz_admin")
	member := newUser("authz_member")
	now := time.Now().Unix()
	for uid, role := range map[int64]int8{admin: model.RoleAdmin, member: model.RoleMember} {
		if _, err := e.Insert(&model.BookMember{BookId: b.Id, Uid: uid, Role: role,
			CanViewOthers: true, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatalf("加入成员失败：%v", err)
		}
	}
	return &env{t: t, engine: e, owner: owner, admin: admin, member: member,
		bookID: b.Id, books: books, members: members}
}

func countAudit(t *testing.T, e *xorm.Engine, cond string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	if _, err := e.SQL("SELECT COUNT(1) FROM audit_logs WHERE "+cond, args...).Get(&n); err != nil {
		t.Fatalf("统计审计行失败：%v", err)
	}
	return n
}

func setBookDeleted(t *testing.T, e *xorm.Engine, bookID int64) {
	t.Helper()
	now := time.Now().Unix()
	if _, err := e.Exec("UPDATE books SET deleted = 1, deleted_at = ?, updated_at = ? WHERE id = ?",
		now, now, bookID); err != nil {
		t.Fatalf("软删账本失败：%v", err)
	}
}

func countMembers(t *testing.T, e *xorm.Engine, bookID int64) int64 {
	t.Helper()
	var n int64
	if _, err := e.SQL("SELECT COUNT(1) FROM book_members WHERE book_id = ? AND deleted = 0",
		bookID).Get(&n); err != nil {
		t.Fatalf("统计成员失败：%v", err)
	}
	return n
}

// ---------- 发现 A1：账本删除（软删）不留任何审计 ----------

// TestAuthzBookDeleteMustLeaveAudit 账本被删除必须留痕。
//
// 期望：audit_logs 至少出现一行 entity_type='book'（action=delete），
//
//	回答"谁在什么时候删掉了哪个账本"。
//
// 实际：BookService.Delete（book.go:321-353）只做一次 UPDATE books + Commit，
//
//	事务内没有任何 insertAudit —— 全程 0 行。
//
// 影响：账本删除是租户级最高影响的操作（整本数据对外消失、所有成员连带失效、
//
//	所有在途邀请同时作废），事后无从追溯。而同一张表上"可见性开关"这种
//	低影响变更反而有审计（book.go:304），口径自相矛盾。
func TestAuthzBookDeleteMustLeaveAudit(t *testing.T) {
	e := newEnv(t)
	if err := e.books.Delete(e.owner, e.bookID); err != nil {
		t.Fatalf("删除账本失败：%v", err)
	}
	n := countAudit(t, e.engine, "entity_type = ? AND entity_id = ?", model.AuditEntityBook, e.bookID)
	if n == 0 {
		t.Fatalf("账本 %d 已被 owner=%d 删除，但 audit_logs 中没有任何 book 级留痕（0 行）；"+
			"期望 ≥1 行（action=delete, actor_uid=%d, entity_id=%d）",
			e.bookID, e.owner, e.owner, e.bookID)
	}
}

// TestAuthzBookCreateMustLeaveAudit 账本创建同样没有留痕（与删除对称的同一类缺口）。
func TestAuthzBookCreateMustLeaveAudit(t *testing.T) {
	e := newEnv(t)
	b, err := e.books.Create(e.owner, "第二本账", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建账本失败：%v", err)
	}
	n := countAudit(t, e.engine, "entity_type = ? AND entity_id = ?", model.AuditEntityBook, b.Id)
	if n == 0 {
		t.Fatalf("新建账本 %d 没有审计留痕；期望 ≥1 行（who/when 可追溯）", b.Id)
	}
}

// ---------- 发现 A2：已删账本在 WS 鉴权口径下仍然"是成员" ----------

// TestAuthzCanViewDetailDeletedBookMustFail 已软删账本的可见性/资格判定必须失败关闭。
//
// 期望：返回错误（与 HTTP 读路径同一口径 —— requireMemberTx / roleOfTx 的 SQL 都
//
//	JOIN 了 `books b ON b.id = m.book_id AND b.deleted = 0`，已删即非成员）。
//
// 实际：visibility.go:64 `if row.BookDeleted { return false, nil }` —— 无错误，
//
//	对调用方而言等同于"仍是成员，只是关闭互看"。
//
// 影响：HTTP 读路径都在前面另加了一次 requireMember（因此被挡住），但 WS 订阅路径
//
//	直接把 CanViewDetail 当 checker 用（cmd/server/main.go:135），
//	而 hub.go:430 只在 checker 返回 err 时才跳过订阅 —— 已删账本会被订阅成功。
//	RefreshVisibility（账本删除时 notifyBookDeleted 会调用）同样按 member=true
//	处理，于是订阅一直保留到令牌到期。
func TestAuthzCanViewDetailDeletedBookMustFail(t *testing.T) {
	e := newEnv(t)
	setBookDeleted(t, e.engine, e.bookID)

	canView, err := service.CanViewDetail(e.engine, e.member, e.bookID)
	if err == nil {
		t.Fatalf("CanViewDetail 对已删账本 %d 返回 (canView=%v, err=nil)："+
			"ws.Hub.Subscribe 会据此把该账本写进订阅表（err==nil 即视为仍是成员）；"+
			"期望返回 ErrNotMember 之类的错误以拒绝订阅", e.bookID, canView)
	}
}

// TestAuthzWSSubscribeDeletedBookMustBeRejected 同上，但从 Hub 的订阅结果直接取证。
func TestAuthzWSSubscribeDeletedBookMustBeRejected(t *testing.T) {
	e := newEnv(t)
	setBookDeleted(t, e.engine, e.bookID)

	// 与生产 main.go:135 完全相同的注入方式
	hub := ws.NewHub(func(uid, bookID int64) (bool, error) {
		return service.CanViewDetail(e.engine, uid, bookID)
	})
	client := ws.NewClient()
	client.UID = e.member
	hub.Register(client)

	serr := hub.Subscribe(client, []int64{e.bookID}, e.member)
	if serr == nil && client.Subscribed(e.bookID) {
		t.Fatalf("已删账本 %d 仍被订阅成功（Subscribe 返回 nil 且订阅表已写入，canView=%v）；"+
			"期望订阅被拒绝", e.bookID, client.CanView(e.bookID))
	}
}

// ---------- 发现 A3：兑换被拒的"账本已满"分支不留审计 ----------

// TestAuthzInviteRedeemBookFullMustLeaveAudit 成员上限拒绝也必须留痕。
//
// 期望：所有"兑换被拒"分支都经 inviteFailed 写一行 result=rejected 的审计
//
//	（member.go 文件头【策略三】明文：失败写一条 result=rejected + 粗粒度 reason）。
//
// 实际：member.go:821 `if memberCount >= maxBookMembers { ... return nil, ErrBookFull }`
//
//	—— 是全函数唯一绕过 inviteFailed 的失败出口，也是唯一没有 reason 码的出口。
//
// 影响：账本顶到 20 人上限后，之后所有入户尝试在审计里完全不可见；
//
//	"有人持续试图挤进已满的账本"恰恰是最该被看见的信号之一。
func TestAuthzInviteRedeemBookFullMustLeaveAudit(t *testing.T) {
	e := newEnv(t)

	// 把成员数顶到上限 20（owner+admin+member 已有 3 人）
	now := time.Now().Unix()
	for i := 0; i < 17; i++ {
		u := &model.User{
			Username:     "authz_fill_" + strconv.Itoa(i) + "_" + time.Now().Format("150405.000000000"),
			PasswordHash: "x",
		}
		if _, err := e.engine.Insert(u); err != nil {
			t.Fatalf("插入用户失败：%v", err)
		}
		if _, err := e.engine.Insert(&model.BookMember{BookId: e.bookID, Uid: u.Id,
			Role: model.RoleMember, CanViewOthers: true, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatalf("加入成员失败：%v", err)
		}
	}
	if got := countMembers(t, e.engine, e.bookID); got != 20 {
		t.Fatalf("构造前提失败：成员数 %d，应为 20（maxBookMembers）", got)
	}

	inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	before := countAudit(t, e.engine, "book_id = ? AND before_json LIKE ?", e.bookID, "%invite_redeem%")

	_, jerr := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_newcomer", "", "password123", "10.0.0.1")
	if !errors.Is(jerr, service.ErrBookFull) {
		t.Fatalf("期望 ErrBookFull，实际 %v", jerr)
	}
	after := countAudit(t, e.engine, "book_id = ? AND before_json LIKE ?", e.bookID, "%invite_redeem%")
	if after != before+1 {
		t.Fatalf("兑换被拒（账本已满）没有写审计：before=%d after=%d；"+
			"期望 after=before+1（result=rejected, reason=book_full）", before, after)
	}
}

// ---------- 发现 A4：未认证入口可往任意 book_id 的审计流写行 ----------

// TestAuthzJoinRejectAuditMustNotTargetForeignBook 兑换被拒时不该把账记到"调用方自称"的账本上。
//
// 期望：邀请不属于该账本时，不把这条尝试记进该账本的审计流（或至少不带 actor_uid=0
//
//	的系统哨兵，避免看起来像服务端自己产生的事件）。
//
// 实际：JoinViaInvite 每个失败分支都 `s.inviteFailed(inviteEventRedeem, bookID, ...)`
//
//	—— bookID 直接取自 URL 路径参数，与邀请的真实归属无关；actor 恒为
//	model.AuditActorSystem(0)。
//
// 影响：匿名者可往任意账本的审计流灌行（唯一约束是 10 次/小时/IP），污染
//
//	"这本账发生过什么"的追溯链；且这些行带 actor_uid=0，排查时会被误读成系统行为。
func TestAuthzJoinRejectAuditMustNotTargetForeignBook(t *testing.T) {
	e := newEnv(t)
	other, err := e.books.Create(e.owner, "另一本", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建第二本账失败：%v", err)
	}
	inv, err := e.members.CreateInvite(e.owner, other.Id, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	// 用第一本账的路径去兑换第二本账的邀请码
	_, jerr := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_attacker", "", "password123", "10.0.0.2")
	if !errors.Is(jerr, service.ErrInviteInvalid) {
		t.Fatalf("期望 ErrInviteInvalid，实际 %v", jerr)
	}
	n := countAudit(t, e.engine, "book_id = ? AND actor_uid = ?", e.bookID, model.AuditActorSystem)
	if n != 0 {
		t.Fatalf("匿名兑换失败在账本 %d 的审计流里写入了 %d 行（actor_uid=0 系统哨兵）；"+
			"该邀请根本不属于这本账，不应把这条尝试记到它头上", e.bookID, n)
	}
}

// ---------- 发现 A5：兑换限流可被伪造的 X-Forwarded-For 绕过 ----------

// TestAuthzJoinRateLimitMustSurviveForgedXFF 兑换限流不能靠客户端可控的请求头分桶（A5，已修复→回归守卫）。
//
// 断言口径（修复后）：
//   - 固定来源连续 12 次兑换（10 次/小时配额）→ 超出后出现 429；
//   - **每次换一个伪造 X-Forwarded-For 也必须出现 429**：router.go 已 SetTrustedProxies(nil)，
//     ClientIP 恒取 TCP 对端（与固定来源同一个桶），请求头拼不出新配额。
//
// （原缺陷：gin 默认信任 0.0.0.0/0，ClientIP 直接取 XFF 最左段，每换一个头就换一个桶，
// "同 IP 每小时 ≤10 次"形同虚设。）
func TestAuthzJoinRateLimitMustSurviveForgedXFF(t *testing.T) {
	e := engine(t)
	gin.SetMode(gin.TestMode)
	as := service.NewAuthService(e, "authz-review-secret-0123456789abcdef", time.Hour)
	books := service.NewBookService(e, version.NewRepo())
	ms, err := service.NewMemberService(e, as, 7)
	if err != nil {
		t.Fatalf("NewMemberService 失败：%v", err)
	}
	owner := &model.User{Username: "api_owner", PasswordHash: "x"}
	if _, err := e.Insert(owner); err != nil {
		t.Fatalf("插入 owner 失败：%v", err)
	}
	b, err := books.Create(owner.Id, "家账", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建账本失败：%v", err)
	}
	r := api.NewRouter(api.Deps{
		Config:        &config.Config{},
		AuthService:   as,
		Books:         books,
		Members:       ms,
		Accounts:      service.NewAccountService(e, version.NewRepo()),
		Categories:    service.NewCategoryService(e, version.NewRepo()),
		Txs:           service.NewTxService(e, version.NewRepo()),
		SchemaVersion: 1,
	})

	post := func(user string, xff string) int {
		body := `{"code":"deadbeefdeadbeefdeadbeefdeadbeef","username":"` + user +
			`","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/books/"+strconv.FormatInt(b.Id, 10)+"/members/join", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.7:5555"
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// 对照：固定来源（不带 XFF）应当被限流
	limited := 0
	for i := 0; i < 12; i++ {
		if c := post("fixed_"+strconv.Itoa(i), ""); c == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited == 0 {
		t.Fatalf("对照失败：固定来源连续 12 次兑换没有一次 429，限流器未生效（请核对本用例前提）")
	}

	// 被测：每次换一个伪造的 X-Forwarded-For —— 修复后必须【仍然】被限流
	//（XFF 不再参与 ClientIP 计算，所有请求与固定来源共用同一个桶，
	// 而该桶的 10 次/小时配额已被上面的对照请求耗尽大半）
	forged := 0
	for i := 0; i < 12; i++ {
		if c := post("forged_"+strconv.Itoa(i), "198.51.100."+strconv.Itoa(i+1)); c == http.StatusTooManyRequests {
			forged++
		}
	}
	if forged == 0 {
		t.Fatalf("12 次兑换每次带不同的 X-Forwarded-For，没有一次被限流（429=%d）："+
			"限流桶键仍取自客户端可控请求头——请核对 router.go 的 SetTrustedProxies(nil) 是否还在", forged)
	}
}

// TestAuthzForgedIPExhaustsLimiterAndBlocksEveryone 伪造 IP 与限流器容量的攻防（A5/INFRA-04，已修复→回归守卫）。
//
// 原缺陷链路：`ipBucket`（member.go）原样透传客户端可控的 IP 字符串 + gin 默认信任全部代理
// ⇒ "一个伪造头 = 一个限流 key"，1 万个一次性 IP 即可打满 maxLimiterKeys=10000，
// 而 fail-closed 语义下正常用户的新 key 全部被拒（邀请入户是唯一入户通道）。
//
// 修复后的语义（分四点断言）：
//  1. **容量有界**（R44 不变量保持）：打满后 Size 恒为上限，内存不会随攻击线性增长；
//  2. **fail-closed**：满容量期间新 key 被拒——这是明确取舍而非缺陷（淘汰最旧会让
//     攻击者冲掉正常用户的计数，见 limiter.go 容量语义注释）；
//  3. **容量可自愈**：Allow 在容量满时会先强制清扫过期条目，攻击 key 一过窗容量即刻
//     回收，无需人工干预或重启（恢复时长上界 = 攻击 key 的窗口长度）；
//  4. **根因已断**：router.go 已 SetTrustedProxies(nil)，伪造 XFF 制造不了新 IP——
//     打满容量从此需要 1 万个真实来源，"改个请求头就能焊死大门"的链路不复存在。
func TestAuthzForgedIPExhaustsLimiterAndBlocksEveryone(t *testing.T) {
	// ① 容量有界
	l := service.NewLimiter()
	// 1 万个一次性伪造 IP，窗口与真实邀请入户一致（1 小时）
	for i := 0; i < 10001; i++ {
		l.Allow("invite:join:ip:198.51."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256), 10, time.Hour)
	}
	if l.Size() != 10000 {
		t.Fatalf("容量应有界于 10000（R44 不变量），实际 %d", l.Size())
	}
	// ② fail-closed：打满期间正常用户的新 key 被拒（明确取舍）
	if l.Allow("invite:join:ip:203.0.113.1", 10, time.Hour) {
		t.Fatal("容量被伪造 IP 打满后，正常新 IP 仍被放行——fail-closed 语义被破坏，请核对 limiter.go")
	}
	// 立刻强制回收：一条都回收不了（所有攻击条目的窗口都还是 1 小时）——
	// 这就是"恢复时长上界 = 攻击 key 的窗口长度"的直接体现。
	if n := l.Sweep(); n != 0 {
		t.Fatalf("窗口未过时 Sweep 回收了 %d 条：清理不得回收仍在窗口内的计数（R44 不变量）", n)
	}

	// ③ 容量自愈（INFRA-04 反向的新语义）：用小容量 + 短窗口做确定性验证。
	//    固定 120ms 等待对应 50ms 窗口：时钟单调，120ms 必然 > 50ms，不存在慢机器偶发；
	//    这不是"用 sleep 代替确定性并发测试"，而是时间型行为本身需要时间流逝。
	recov := service.NewBoundedLimiter(100)
	for i := 0; i < 100; i++ {
		if !recov.Allow("invite:join:ip:198.51.100."+strconv.Itoa(i), 10, 50*time.Millisecond) {
			t.Fatalf("恢复场景前提失败：容量打满前第 %d 个 key 不应被拒", i)
		}
	}
	if recov.Allow("invite:join:ip:203.0.113.2", 10, time.Hour) {
		t.Fatal("恢复场景前提失败：容量打满后新 key 应被拒")
	}
	time.Sleep(120 * time.Millisecond) // 等待 50ms 攻击窗口确定性地过期
	if !recov.Allow("invite:join:ip:203.0.113.2", 10, time.Hour) {
		t.Fatalf("攻击 key 过窗后容量应自愈（Allow 容量满时先强制清扫过期条目），实际仍被拒；Size=%d", recov.Size())
	}
	if recov.Size() != 1 {
		t.Fatalf("自愈后应只剩新 key 1 条，实际 %d", recov.Size())
	}
}

// ---------- 发现 A7：账户 / 分类的写操作不留审计 ----------

// TestAuthzAccountAndCategoryWritesMustLeaveAudit 账本共享配置的写入必须留痕。
//
// 期望：账户/分类的 create/update/delete 至少各写一行审计（与交易同一口径：
//
//	交易 create/delete/restore 都写了，见 tx.go:309 / tx.go:881）。
//
// 实际：AccountService.Create/Update/Delete、CategoryService.Create/Update/Delete
//
//	全程没有 insertAudit/AppendAudit 调用（全仓审计写入点共 12 处，无一处属于它们）。
//
// 影响：账户与分类是账本共享配置，删掉一个账户会让历史交易的引用悬空、
//
//	改掉一个分类会改变统计口径；这些变更现在"改了就是改了"，无从追溯。
func TestAuthzAccountAndCategoryWritesMustLeaveAudit(t *testing.T) {
	e := newEnv(t)
	acc := service.NewAccountService(e.engine, version.NewRepo())
	cat := service.NewCategoryService(e.engine, version.NewRepo())

	if _, err := acc.Create(e.owner, e.bookID, service.AccountCreateInput{
		Name: "测试账户", Category: model.CategoryCash, Currency: "CNY",
	}); err != nil {
		t.Fatalf("建账户失败：%v", err)
	}
	if _, err := cat.Create(e.owner, e.bookID, service.CategoryCreateInput{
		Name: "测试分类", CatType: model.CategoryTypeExpense,
	}); err != nil {
		t.Fatalf("建分类失败：%v", err)
	}
	n := countAudit(t, e.engine, "book_id = ? AND (entity_type = 'account' OR entity_type = 'category')", e.bookID)
	if n == 0 {
		t.Fatalf("账本 %d 上新建了账户与分类，但 audit_logs 中没有任何相关留痕（0 行）；"+
			"期望 ≥2 行（交易 create 有审计，共享配置却完全没有，口径不一致）", e.bookID)
	}
}

// ---------- 反向对照：下列用例必须【通过】，证明这些路径没有问题 ----------

// AuthzCtlCrossBookInviteRedeemRejected 邀请码不能跨账本兑换，且原邀请不被消耗。
func AuthzCtlCrossBookInviteRedeemRejected(t *testing.T) {
	e := newEnv(t)
	other, err := e.books.Create(e.owner, "另一本", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建第二本账失败：%v", err)
	}
	inv, err := e.members.CreateInvite(e.owner, other.Id, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	if _, jerr := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_x", "", "password123", "10.0.0.3"); !errors.Is(jerr, service.ErrInviteInvalid) {
		t.Fatalf("跨账本兑换应被拒，实际 %v", jerr)
	}
	var fresh model.Invite
	if _, err := e.engine.ID(inv.Id).Get(&fresh); err != nil {
		t.Fatalf("读邀请失败：%v", err)
	}
	if fresh.UsedAt != 0 || fresh.UsedBy != 0 {
		t.Fatalf("跨账本兑换把原邀请消耗掉了：used_at=%d used_by=%d", fresh.UsedAt, fresh.UsedBy)
	}
}

// AuthzCtlCrossBookRevokeRejected 跨账本撤销既不生效，也不泄露"该账本存在"。
func AuthzCtlCrossBookRevokeRejected(t *testing.T) {
	e := newEnv(t)
	other, err := e.books.Create(e.owner, "另一本", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建第二本账失败：%v", err)
	}
	inv, err := e.members.CreateInvite(e.owner, other.Id, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	rerr := e.members.RevokeInvite(e.admin, other.Id, inv.Id) // admin 不是 other 的成员
	if !errors.Is(rerr, service.ErrInviteNotFound) && !errors.Is(rerr, service.ErrNotMember) {
		t.Fatalf("非本账本成员撤销应表现为邀请不存在/非成员，实际 %v", rerr)
	}
	var fresh model.Invite
	if _, err := e.engine.ID(inv.Id).Get(&fresh); err != nil {
		t.Fatalf("读邀请失败：%v", err)
	}
	if fresh.Revoked {
		t.Fatal("跨账本撤销把邀请真的撤销了")
	}
}

// AuthzCtlDeletedBookMemberOpsBlocked 已删账本的成员管理 / 邀请入口必须全部挡住（GPT-R43 复核）。
func AuthzCtlDeletedBookMemberOpsBlocked(t *testing.T) {
	e := newEnv(t)
	inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	setBookDeleted(t, e.engine, e.bookID)

	if _, err := e.members.List(e.bookID); err == nil {
		t.Fatal("已删账本仍能列出成员")
	}
	if err := e.members.ChangeRole(e.owner, e.bookID, e.member, model.RoleAdmin); err == nil {
		t.Fatal("已删账本仍能改角色")
	}
	if err := e.members.Remove(e.owner, e.bookID, e.member); err == nil {
		t.Fatal("已删账本仍能移除成员")
	}
	if _, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", ""); err == nil {
		t.Fatal("已删账本仍能签发邀请")
	}
	if err := e.members.RevokeInvite(e.owner, e.bookID, inv.Id); err == nil {
		t.Fatal("已删账本仍能撤销邀请")
	}
	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_late", "", "password123", "10.0.0.4"); err == nil {
		t.Fatal("已删账本仍能凭邀请入户")
	}
}

// AuthzCtlExpiryConsistentAcrossPaths 过期判定在兑换与撤销两条路径上口径一致。
func AuthzCtlExpiryConsistentAcrossPaths(t *testing.T) {
	e := newEnv(t)
	inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	if _, err := e.engine.Exec("UPDATE invites SET expires_at = ? WHERE id = ?",
		time.Now().Unix()-10, inv.Id); err != nil {
		t.Fatalf("改有效期失败：%v", err)
	}
	if rerr := e.members.RevokeInvite(e.owner, e.bookID, inv.Id); !errors.Is(rerr, service.ErrInviteExpired) {
		t.Fatalf("撤销路径：过期邀请应报 ErrInviteExpired，实际 %v", rerr)
	}
	if _, jerr := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_late2", "", "password123", "10.0.0.5"); !errors.Is(jerr, service.ErrInviteExpired) {
		t.Fatalf("兑换路径：过期邀请应报 ErrInviteExpired，实际 %v", jerr)
	}
}

// AuthzCtlInviteSingleUseEnforced 邀请码不可重放。
func AuthzCtlInviteSingleUseEnforced(t *testing.T) {
	e := newEnv(t)
	inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_first", "", "password123", "10.0.0.6"); err != nil {
		t.Fatalf("首次兑换失败：%v", err)
	}
	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_second", "", "password123", "10.0.0.6"); !errors.Is(err, service.ErrInviteAlreadyUsed) {
		t.Fatalf("重放兑换应报 ErrInviteAlreadyUsed，实际 %v", err)
	}
}

// AuthzCtlWSSubscribeForeignBookRejected 订阅自己不是成员的账本会被静默跳过。
func AuthzCtlWSSubscribeForeignBookRejected(t *testing.T) {
	e := newEnv(t)
	other, err := e.books.Create(e.owner, "另一本", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建第二本账失败：%v", err)
	}
	hub := ws.NewHub(func(uid, bookID int64) (bool, error) {
		return service.CanViewDetail(e.engine, uid, bookID)
	})
	client := ws.NewClient()
	hub.Register(client)
	if err := hub.Subscribe(client, []int64{other.Id}, e.member); err != nil {
		t.Fatalf("非成员订阅不应报错（静默跳过），实际 %v", err)
	}
	if client.Subscribed(other.Id) {
		t.Fatal("非成员竟然订阅成功了")
	}
}

// AuthzCtlBoundEmailNormalizedBothEnds 邮箱绑定两端同一规范化口径（大小写 / 空白不绕过）。
func AuthzCtlBoundEmailNormalizedBothEnds(t *testing.T) {
	e := newEnv(t)
	inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "  Alice@Example.COM ")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	var stored model.Invite
	if _, err := e.engine.ID(inv.Id).Get(&stored); err != nil {
		t.Fatalf("读邀请失败：%v", err)
	}
	if stored.BoundEmail != "alice@example.com" {
		t.Fatalf("签发端未规范化：bound_email=%q", stored.BoundEmail)
	}
	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_alice", "ALICE@example.com", "password123", "10.0.0.7"); err != nil {
		t.Fatalf("大小写不同的本人邮箱应能兑换，实际 %v", err)
	}
}

// AuthzCtlBoundEmailMismatchRejected 邮箱不匹配时拒绝兑换，且邀请不被消耗。
func AuthzCtlBoundEmailMismatchRejected(t *testing.T) {
	e := newEnv(t)
	inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "alice@example.com")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	if _, err := e.members.JoinViaInvite(e.bookID, inv.Code, "authz_bob", "bob@example.com", "password123", "10.0.0.8"); !errors.Is(err, service.ErrInviteBoundMismatch) {
		t.Fatalf("邮箱不匹配应被拒，实际 %v", err)
	}
	var fresh model.Invite
	if _, err := e.engine.ID(inv.Id).Get(&fresh); err != nil {
		t.Fatalf("读邀请失败：%v", err)
	}
	if fresh.UsedAt != 0 {
		t.Fatal("被拒的兑换把邀请消耗了")
	}
}

// TestAuthzControls 一次性跑完所有反向对照（必须全绿）。
func TestAuthzControls(t *testing.T) {
	t.Run("CrossBookRedeem", AuthzCtlCrossBookInviteRedeemRejected)
	t.Run("CrossBookRevoke", AuthzCtlCrossBookRevokeRejected)
	t.Run("DeletedBookMemberOps", AuthzCtlDeletedBookMemberOpsBlocked)
	t.Run("ExpiryConsistent", AuthzCtlExpiryConsistentAcrossPaths)
	t.Run("SingleUse", AuthzCtlInviteSingleUseEnforced)
	t.Run("WSSubscribeForeignBook", AuthzCtlWSSubscribeForeignBookRejected)
	t.Run("BoundEmailNormalized", AuthzCtlBoundEmailNormalizedBothEnds)
	t.Run("BoundEmailMismatch", AuthzCtlBoundEmailMismatchRejected)
}
