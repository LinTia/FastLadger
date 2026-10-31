package authzreview

// zz_authz_adversarial2_test.go —— R-1 收尾（807b516）的**第二轮对抗变体**。
//
// 与 zz_authz_adversarial_test.go 的分工：那边验证"R-1~R-7 修复后原攻击是否关闭"，
// 这里只打**修复之后的新变体**，每条都从"如果 loginKey 换成 netip 前缀聚合，我还能怎么绕"出发。
//
//   - ZZ-7（绿）IPv4-mapped IPv6（::ffff:a.b.c.d）与原生 IPv4 必须落进同一个 /24 桶；
//   - ZZ-8（绿）不可解析的 ClientIP 归 login:invalid 共享桶（fail-closed，不因解析失败绕过）；
//   - ZZ-9（红，跨域残留）邀请兑换的限流桶键（member.go ipBucket）仍是原样透传——
//     R-1 的 /24//64 聚合口径尚未对齐到邀请入口，IPv6 轮转仍可摊薄其 10 次/小时配额。
//     该文件归审计域（software-engineer-4），本用例只做证据，不改生产代码。

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/api"
	"fastledger/internal/config"
	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"

	"github.com/gin-gonic/gin"
)

// zzJoinRouter 构造可打 /members/join 的完整路由（默认限流参数，与生产接线一致）。
func zzJoinRouter(t *testing.T) (*gin.Engine, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := engine(t)
	as := service.NewAuthService(e, "zz2-review-secret-0123456789abcdef", time.Hour)
	books := service.NewBookService(e, version.NewRepo())
	ms, err := service.NewMemberService(e, as, 7)
	if err != nil {
		t.Fatalf("NewMemberService 失败：%v", err)
	}
	owner := &model.User{Username: "zz2_owner", PasswordHash: "x"}
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
	return r, b.Id
}

// zzPostJoin 从指定来源发一次无效兑换（code 为假值，不产生任何业务写入，只消耗限流配额）。
func zzPostJoin(r *gin.Engine, bookID int64, user, remoteAddr string) int {
	body := `{"code":"deadbeefdeadbeefdeadbeefdeadbeef","username":"` + user + `","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/books/"+strconv.FormatInt(bookID, 10)+"/members/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestZZ7IPv4MappedIPv6SharesV4Bucket IPv4-mapped IPv6 不得造出新桶。
//
// 攻击：同一个 /24 内，一部分请求用原生 IPv4（203.0.113.9），另一部分用
// IPv4-mapped 形态（[::ffff:203.0.113.9]）——若 loginKey 只按字符串前缀而忘了
// netip.Addr.Unmap()，两种形态会各成一个 /64 桶，配额翻倍。
// 期望：两者落进同一个 203.0.113.0/24 桶，合计 11 次失败登录必须出现 429。
func TestZZ7IPv4MappedIPv6SharesV4Bucket(t *testing.T) {
	r, user := zzLoginRouter(t)

	limited := 0
	// 前 10 次走原生 IPv4（恰好打满默认配额）
	for i := 0; i < 10; i++ {
		if c := zzPostLoginFrom(r, user, "wrong-native-"+strconv.Itoa(i),
			"203.0.113.9:8100", ""); c == http.StatusTooManyRequests {
			limited++
		}
	}
	// 再走 IPv4-mapped 形态：同一台机器的另一种写法
	for i := 0; i < 3; i++ {
		if c := zzPostLoginFrom(r, user, "wrong-mapped-"+strconv.Itoa(i),
			"[::ffff:203.0.113.9]:8100", ""); c == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited == 0 {
		t.Fatalf("IPv4-mapped IPv6 与原生 IPv4 混用 13 次失败登录没有一次 429：" +
			"loginKey 未对 mapped 形态做 Unmap，同一来源两种写法可拿到两份配额")
	}
}

// TestZZ8UnparsableClientIPFailsClosed 不可解析的 ClientIP 必须进共享桶（fail-closed）。
//
// 攻击：构造 gin 无法解析出 IP 的 RemoteAddr（无端口 / 主机名形态）——
// 若 loginKey 对解析失败的值"原样拼 key"，攻击者可以用任意垃圾字符串制造无限新桶。
// 期望：全部落 login:invalid 共享桶，11 次失败登录后出现 429。
func TestZZ8UnparsableClientIPFailsClosed(t *testing.T) {
	r, user := zzLoginRouter(t)

	limited := 0
	junk := []string{"garbage-host:1234", "no-port-at-all", "999.999.999.999:1"}
	for round := 0; round < 4; round++ {
		for _, addr := range junk {
			if c := zzPostLoginFrom(r, user, "wrong-junk-"+strconv.Itoa(round)+"-"+addr,
				addr, ""); c == http.StatusTooManyRequests {
				limited++
			}
		}
	}
	// 12 次全部应落在同一个 login:invalid 桶（配额 10）→ 至少出现 2 次 429
	if limited < 2 {
		t.Fatalf("12 次不可解析来源的失败登录只出现 %d 次 429（期望 ≥2）："+
			"loginKey 对解析失败值可能按原样分桶（fail-open），请核对 login:invalid 分支", limited)
	}
}

// TestZZ9InviteJoinStillBypassedByIPv6Rotation 邀请兑换入口的限流口径必须对齐 R-1。
//
// 原缺陷（已修复，本用例翻转为回归守卫）：`member.go` 的 `ipBucket` 当时是
// "TrimSpace 后原样透传"，`JoinViaInvite` 的桶键 = "invite:join:ip:" + 完整 ClientIP，
// 与 R-1 修复前的 loginKey 同构。SetTrustedProxies(nil) 之后 XFF 伪造已断（ZZ1/A5 守卫），
// 但 IPv6 隐私扩展下的**真实地址轮转**依然有效：12 个地址各拿一份新配额，
// 10 次/小时的配额被地址数摊薄成 0。
//
// 修复后口径：ipBucket 与 auth.go 的 loginKey 一致——IPv4 /24、IPv6 /64、
// IPv4-mapped 先 Unmap()。因此同一 /64 内轮转地址与单地址共用同一个桶，配额照常耗尽。
//
// 断言口径：
//   - 对照：同一地址连打 12 次 → 必须出现 429（限流本身生效）；
//   - 被测：同 /64 内轮换 12 个不同地址 → **同样必须出现 429**（不能被地址数摊薄）。
func TestZZ9InviteJoinStillBypassedByIPv6Rotation(t *testing.T) {
	r, bookID := zzJoinRouter(t)

	// 对照：同一地址连打 12 次（配额 10/小时）→ 必须出现 429
	same := 0
	for i := 0; i < 12; i++ {
		if c := zzPostJoin(r, bookID, "fixed_"+strconv.Itoa(i), "[2001:db8:1::1]:7000"); c == http.StatusTooManyRequests {
			same++
		}
	}
	if same == 0 {
		t.Fatalf("对照失败：同一地址 12 次兑换没有一次 429，邀请限流未生效（请核对本用例前提）")
	}

	// 被测：同一 /64 内轮转 12 个地址 —— 必须与单地址同桶，配额已被上面的对照耗尽
	rotated := 0
	for i := 0; i < 12; i++ {
		addr := "[2001:db8:1::" + strconv.Itoa(i+2) + "]:7000"
		if c := zzPostJoin(r, bookID, "rot_"+strconv.Itoa(i), addr); c == http.StatusTooManyRequests {
			rotated++
		}
	}
	if rotated == 0 {
		t.Fatalf("同 /64 内轮转 12 个地址，一次 429 都没有：ipBucket 未按网段聚合，" +
			"IPv6 地址轮转让 10 次/小时配额形同虚设（请核对 member.go 的 ipBucket 是否与 loginKey 同口径）")
	}
}

// TestZZ10InviteJoinIPv4RotationSharesSlash24 IPv4 侧的同一攻法：同 /24 内换地址不得换桶。
//
// 与 ZZ-9 对称：若 ipBucket 只对 IPv6 做了聚合、IPv4 仍原样透传，租一台 /24 内的
// 多地址主机（或一个运营商 NAT 池）就能反复刷新配额。两条都必须与登录侧同口径。
func TestZZ10InviteJoinIPv4RotationSharesSlash24(t *testing.T) {
	r, bookID := zzJoinRouter(t)

	// 同一 /24 内轮转 12 个地址：配额 10/小时 → 必须出现 429
	rotated := 0
	for i := 0; i < 12; i++ {
		addr := "203.0.113." + strconv.Itoa(i+1) + ":7000"
		if c := zzPostJoin(r, bookID, "v4rot_"+strconv.Itoa(i), addr); c == http.StatusTooManyRequests {
			rotated++
		}
	}
	if rotated == 0 {
		t.Fatal("同 /24 内轮转 12 个 IPv4 地址，一次 429 都没有：IPv4 侧未按 /24 聚合")
	}

	// 另一个 /24 必须仍有完整配额（聚合不能把全网段并成一个桶）
	if c := zzPostJoin(r, bookID, "v4other", "198.51.100.9:7000"); c == http.StatusTooManyRequests {
		t.Fatal("不同 /24 的来源不应与上一个网段共享配额（聚合粒度过粗）")
	}
}
