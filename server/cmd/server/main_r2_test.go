package main

// INFRA-07 / INFRA-08 / N4 的回归测试（cmd/server 侧）。
//
//   - INFRA-07：user create 必须复用 API 侧输入边界（用户名 1~64 字符 / ≤255 字节、
//     密码 ≤72 字节），不能像原先那样只查"非空 + ≥8 位"。
//   - INFRA-08：db purge --dry-run 只扫描统计、绝不 DELETE；正式执行两段式，
//     中途失败报告已完成/未完成的账本清单。
//   - N4：instance bump 的"新建 vs 读取已有"由 instance.LoadOrCreateWithStatus 回报
//     （见 internal/instance/meta_r2_test.go），这里只测绝对路径工具不破坏路径。

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/service"
	"xorm.io/xorm"
)

// serviceTxService 别名：故障注入钩子的签名写起来太长
type serviceTxService = service.TxService

// openDBWithSchema 打开真实 SQLite 并迁移到最新（CGO 不可用时跳过）
func openDBWithSchema(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := database.Init(filepath.Join(t.TempDir(), "r2.db"))
	if err != nil {
		t.Skipf("真实 SQLite 不可用（需 CGO/gcc）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	return engine
}

// seedTwoBooksWithTombstones 两个账本各插 2 条到期墓碑，返回账本 id
func seedTwoBooksWithTombstones(t *testing.T, engine *xorm.Engine, now int64) (int64, int64) {
	t.Helper()
	db := engine.DB().DB
	old := now - 400*24*3600
	for _, id := range []int64{1, 2} {
		if _, err := db.Exec(
			`INSERT INTO books(id, name, owner_uid, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`,
			id, "book"+string(rune('0'+id)), now, now); err != nil {
			t.Fatalf("插入账本 %d 失败：%v", id, err)
		}
		for i := 0; i < 2; i++ {
			if _, err := db.Exec(
				`INSERT INTO transactions(book_id, type, account_id, amount, tx_time, deleted, created_at, updated_at, deleted_at)
				 VALUES (?, 2, 1, 100, ?, 1, ?, ?, ?)`, id, old, old, old, old); err != nil {
				t.Fatalf("插入墓碑失败：%v", err)
			}
		}
	}
	return 1, 2
}

// countTable 统计表行数（可带 WHERE 前缀，如 "transactions WHERE book_id=1"）
func countTable(t *testing.T, engine *xorm.Engine, tableAndWhere string) int64 {
	t.Helper()
	var n int64
	if err := engine.DB().DB.QueryRow("SELECT COUNT(1) FROM " + tableAndWhere).Scan(&n); err != nil {
		t.Fatalf("统计失败：%v", err)
	}
	return n
}

// TestR2PurgeDryRunScansWithoutDeleting --dry-run 必须给出准确统计且一行都不删
func TestR2PurgeDryRunScansWithoutDeleting(t *testing.T) {
	engine := openDBWithSchema(t)
	now := time.Now().Unix()
	id1, id2 := seedTwoBooksWithTombstones(t, engine, now)

	res, opts, err := purgeWithOptions(engine,
		[]string{"--dry-run", "--recycle-ttl-days", "1", "--audit-retention-days", "1"})
	if err != nil {
		t.Fatalf("dry-run 不应报错：%v", err)
	}
	if !opts.DryRun || !res.DryRun {
		t.Fatal("应处于 dry-run 状态")
	}
	if res.Recycled != 4 {
		t.Fatalf("预览应统计到 4 条到期墓碑，实际 %d（明细：%s）", res.Recycled, res.bookDetail())
	}
	if len(res.Books) != 2 {
		t.Fatalf("应按账本分列（2 个账本），实际 %v", res.Books)
	}
	// 核心：一行都没删
	if got := countTable(t, engine, "transactions"); got != 4 {
		t.Fatalf("dry-run 不得删除任何数据，transactions 仍应有 4 行，实际 %d", got)
	}
	if got := countTable(t, engine, "transactions WHERE book_id = "+itoa(id1)); got != 2 {
		t.Fatalf("账本 %d 的墓碑不应被删除，实际剩 %d", id1, got)
	}
	if got := countTable(t, engine, "transactions WHERE book_id = "+itoa(id2)); got != 2 {
		t.Fatalf("账本 %d 的墓碑不应被删除，实际剩 %d", id2, got)
	}
	// 明细字符串应包含两个账本的统计（运维要的就是"将删多少条"）
	if !strings.Contains(res.bookDetail(), itoa(id1)) || !strings.Contains(res.bookDetail(), itoa(id2)) {
		t.Fatalf("明细应包含各账本统计，实际 %q", res.bookDetail())
	}

	// 去掉 --dry-run 后真正执行，且删的条数与预览一致
	res2, _, err := purgeWithOptions(engine,
		[]string{"--recycle-ttl-days", "1", "--audit-retention-days", "1"})
	if err != nil {
		t.Fatalf("正式执行不应报错：%v", err)
	}
	if res2.Recycled != 4 {
		t.Fatalf("正式执行应清除 4 条（与预览一致），实际 %d", res2.Recycled)
	}
	if got := countTable(t, engine, "transactions"); got != 0 {
		t.Fatalf("正式执行后应清空墓碑，实际剩 %d", got)
	}
}

// TestR2PurgeTwoPhaseReportsProgress 中途失败必须报告已完成/未完成账本。
//
// 两段式的意义：回收站按账本逐个清（非原子），失败时运维必须知道清到哪了、
// 还剩哪些没动，而不是拿到一句干巴巴的"失败"。
func TestR2PurgeTwoPhaseReportsProgress(t *testing.T) {
	engine := openDBWithSchema(t)
	now := time.Now().Unix()
	id1, id2 := seedTwoBooksWithTombstones(t, engine, now)

	// 故障注入：账本 1 成功、账本 2 失败（审计也不许动）
	origBook, origAudit := purgeBookFn, purgeAuditFn
	purgeBookFn = func(txs *serviceTxService, bookID int64, ttlDays int, now int64) (int64, error) {
		if bookID == id2 {
			return 0, errInjectedPurge
		}
		return origBook(txs, bookID, ttlDays, now)
	}
	purgeAuditFn = func(_ *xorm.Engine, _ int, _ int64) (int64, error) {
		return 0, errors.New("不应被调用")
	}
	t.Cleanup(func() { purgeBookFn, purgeAuditFn = origBook, origAudit })

	res, _, err := purgeWithOptions(engine,
		[]string{"--recycle-ttl-days", "1", "--audit-retention-days", "1"})
	if err == nil {
		t.Fatal("账本 2 失败时整体应报错")
	}
	if !strings.Contains(err.Error(), "已完成") || !strings.Contains(err.Error(), "未完成") {
		t.Fatalf("错误信息应包含进度清单，实际：%v", err)
	}
	if !strings.Contains(err.Error(), itoa(id1)) {
		t.Fatalf("已完成清单应包含账本 %d，实际：%v", id1, err)
	}
	if res == nil || len(res.Books) == 0 || res.Books[0].Pending != 2 {
		t.Fatalf("结果应保留账本 1 的实际清除数，实际 %+v", res)
	}
	// 账本 1 真的被清了，账本 2 一行未动，审计未动
	if got := countTable(t, engine, "transactions WHERE book_id = "+itoa(id1)); got != 0 {
		t.Fatalf("账本 %d 应已清除，实际剩 %d", id1, got)
	}
	if got := countTable(t, engine, "transactions WHERE book_id = "+itoa(id2)); got != 2 {
		t.Fatalf("账本 %d 不得被触碰，实际剩 %d", id2, got)
	}
}

// errInjectedPurge 故障注入哨兵
var errInjectedPurge = errors.New("注入的清除故障")

// TestR2CLIUsernameValidation INFRA-07：CLI 与 API 同一套用户名边界
func TestR2CLIUsernameValidation(t *testing.T) {
	cases := []struct {
		name  string
		input string
		ok    bool
	}{
		{"普通名", "alice", true},
		{"中文 3 字符", "张三四", true},
		{"单字符", "a", true},
		{"空白", "   ", false},
		{"空串", "", false},
		{"恰好 64 字符", strings.Repeat("a", 64), true},
		{"65 字符", strings.Repeat("a", 65), false},
		{"64 个汉字（128 字节）", strings.Repeat("汉", 64), true},
		// 字节口径的真正约束点在 4 字节字符：63 个 emoji = 252 字节 ≤ 255 且 64 码点内
		{"63 个 emoji（252 字节）", strings.Repeat("\U0001F600", 63), true},
		{"64 个 emoji（256 字节，超字节上限）", strings.Repeat("\U0001F600", 64), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateCLIUsername(c.input)
			if c.ok && err != nil {
				t.Fatalf("%q 应通过：%v", c.input, err)
			}
			if !c.ok && err == nil {
				t.Fatalf("%q 应被拒绝", c.input)
			}
		})
	}
}

// TestR2PrepareCLIUserNormalizesWhitespace V-3：CLI 必须与 API 同样 TrimSpace 后入库。
//
// 原缺陷：API 侧 normalizeAndHash 会 TrimSpace，而 CLI 原样入库——
// "  spacy  " 经 CLI 建出的账号，在 service.Login 的
// `WHERE username = TrimSpace(input)` 精确匹配下永远登录不上。
func TestR2PrepareCLIUserNormalizesWhitespace(t *testing.T) {
	cases := []struct {
		name     string
		username string
		want     string
		ok       bool
	}{
		{"带首尾空格", "  spacy  ", "spacy", true},
		{"带制表符", "\talice\t", "alice", true},
		{"纯空白等价于空", "   ", "", false},
		{"已干净", "bob", "bob", true},
		{"内部空格保留", "a b", "a b", true},
		{"TrimSpace 后超长", " " + strings.Repeat("a", 65) + " ", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := prepareCLIUser(c.username, "12345678")
			if !c.ok {
				if err == nil {
					t.Fatalf("%q 应被拒绝", c.username)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q 应通过：%v", c.username, err)
			}
			if got != c.want {
				t.Fatalf("入库值应为归一化后的 %q，实际 %q", c.want, got)
			}
		})
	}
	// 密码口径同步回归（≤72 字节，与 API 的 bcrypt 上限一致）
	if _, _, err := prepareCLIUser("alice", strings.Repeat("p", 73)); err == nil {
		t.Fatal("73 字节密码应被拒绝（bcrypt 会静默截断）")
	}
	if _, _, err := prepareCLIUser("alice", "short"); err == nil {
		t.Fatal("短于 8 位密码应被拒绝")
	}
}

// TestR2UnbackfilledTransferWarning INFRA-11：存在未回填方向的历史转账时必须告警
func TestR2UnbackfilledTransferWarning(t *testing.T) {
	engine := openDBWithSchema(t)
	now := time.Now().Unix()
	db := engine.DB().DB
	// 一条 related_id=0 的历史转账（0006 不会回填它）+ 一条普通支出（不应触发）
	if _, err := db.Exec(
		`INSERT INTO transactions(book_id, type, account_id, amount, tx_time, related_id, direction, created_at, updated_at)
		 VALUES (1, 3, 1, 500, ?, 0, 0, ?, ?)`, now, now, now); err != nil {
		t.Fatalf("插入历史转账失败：%v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO transactions(book_id, type, account_id, amount, tx_time, direction, created_at, updated_at)
		 VALUES (1, 2, 1, 100, ?, 0, ?, ?)`, now, now, now); err != nil {
		t.Fatalf("插入普通支出失败：%v", err)
	}

	var buf strings.Builder
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	warnUnbackfilledTransfers(engine)

	if !strings.Contains(buf.String(), "未回填方向") {
		t.Fatalf("应打印未回填告警，实际日志：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "1 条") {
		t.Fatalf("告警应统计到 1 条，实际日志：%s", buf.String())
	}
}

// itoa 局部的小工具（避免为测试引入 strconv 别名冲突）
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
