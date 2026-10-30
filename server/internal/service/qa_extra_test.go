package service

// QA 独立补充测试（Edward / 严过关）。
//
// 本机无 gcc、CGO 不可用（mattn/go-sqlite3 能编译不能运行），因此这里只做
// 【纯逻辑验证】，不连数据库。目的不是复述工程师的 tx_test.go，而是换一套
// 独立口径去证伪：
//  1. 枚举字典 §2 负债符号：全部 15 类账户逐一验证（工程师只验了 2 类）；
//  2. 转账净资产守恒：15×15 = 225 种账户组合的组合不变量（工程师只验了 2 组）；
//  3. PRD §10.3 还款场景的精确数字口径（1000 元储蓄 + 刷卡 100 → 净资产 900 元）；
//  4. requestHash 的独立判据：自行构造规范 JSON 再哈希，不借用被测编码函数；
//  5. validateCreate 的边界值表（64/65 字节、±840、255/256 字、0 值枚举等）；
//  6. 交易行构造：两条转账记录必须同版本、时间来自命令、操作键落到 client_uuid；
//  7. 回执编解码与分页的边界。

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"fastledger/internal/model"
)

// qaBaseExpense 独立构造的基准支出命令（与工程师用例不共享数据，避免同源错误）
func qaBaseExpense() CreateRequest {
	return CreateRequest{
		ClientUuid:     "3f2a7c1e-9b4d-4e2a-8f61-0d5b7c9a1e33",
		TxType:         int(model.TxTypeExpense),
		AccountId:      1,
		Amount:         3650,
		CategoryId:     11,
		PaymentChannel: int(model.PaymentChannelWeChat),
		TxTime:         1793260800,
		TimezoneOffset: 480,
		Comment:        "午餐",
		Source:         int(model.SourceManual),
	}
}

// qaBaseTransfer 独立构造的基准转账命令（储蓄卡 → 信用卡）
func qaBaseTransfer() CreateRequest {
	return CreateRequest{
		ClientUuid:  "a1b2c3d4-0000-4000-8000-000000000001",
		TxType:      int(model.TxTypeTransfer),
		AccountId:   1,
		ToAccountId: 2,
		Amount:      10000,
		TxTime:      1793260800,
		Comment:     "信用卡还款",
	}
}

// qaNetAssets 净资产口径：Σ(资产 1~9) − Σ(负债 10~15)，全部整数分。
func qaNetAssets(accounts []model.Account) int64 {
	var total int64
	for _, a := range accounts {
		if model.IsLiabilityCategory(a.Category) {
			total -= a.Balance
		} else {
			total += a.Balance
		}
	}
	return total
}

// qaNetContribution 一次余额变动对净资产的贡献：
// 资产 balance 增加 → 净资产增加；负债 balance 增加（欠更多）→ 净资产减少。
func qaNetContribution(category int8, delta int64) int64 {
	if model.IsLiabilityCategory(category) {
		return -delta
	}
	return delta
}

// TestQALiabilitySymbolAllCategories 逐一验证 15 类账户的符号策略
//
// 资产 1~9：流入 delta=+amount、流出 delta=-amount；
// 负债 10~15：balance 正=欠款，故 delta 全部取反。
func TestQALiabilitySymbolAllCategories(t *testing.T) {
	const amount int64 = 10000

	for c := int8(model.CategoryCash); c <= model.CategoryPayable; c++ {
		wantLiability := c >= model.CategoryCreditCard
		if got := model.IsLiabilityCategory(c); got != wantLiability {
			t.Fatalf("账户分类 %d 的负债判定错误：got=%v want=%v", c, got, wantLiability)
		}
		if !model.IsValidAccountCategory(c) {
			t.Fatalf("账户分类 %d 应属合法枚举（1~15）", c)
		}

		// 流出（支出 / 转出）
		wantOut := -amount
		if wantLiability {
			wantOut = amount // 欠款增加
		}
		if got := signedBalanceDelta(c, -amount); got != wantOut {
			t.Errorf("分类 %d 流出 100 元的存储 delta 应为 %d，实际 %d", c, wantOut, got)
		}

		// 流入（收入 / 转入）
		wantIn := amount
		if wantLiability {
			wantIn = -amount // 欠款减少
		}
		if got := signedBalanceDelta(c, amount); got != wantIn {
			t.Errorf("分类 %d 流入 100 元的存储 delta 应为 %d，实际 %d", c, wantIn, got)
		}
	}

	// 越界分类不得被当成负债（16 不是合法枚举，也不得取反）
	if model.IsLiabilityCategory(0) || model.IsLiabilityCategory(16) {
		t.Error("0 与 16 不得被判为负债分类")
	}
	if model.IsValidAccountCategory(0) || model.IsValidAccountCategory(16) {
		t.Error("0 与 16 不得被判为合法账户分类")
	}
}

// TestQATransferKeepsNetAssetsAllPairs 转账组合不变量（225 种组合）
//
// 转账的硬性要求不是"两个原始余额一减一增"（还信用卡时两边都减），
// 而是【两侧净资产贡献相抵】。这里对所有 资产/负债 × 资产/负债 组合断言和为 0，
// 覆盖工程师用例遗漏的 负债→资产（取现）与 负债→负债（借新还旧）。
func TestQATransferKeepsNetAssetsAllPairs(t *testing.T) {
	const amount int64 = 12345

	for from := int8(model.CategoryCash); from <= model.CategoryPayable; from++ {
		for to := int8(model.CategoryCash); to <= model.CategoryPayable; to++ {
			fromDelta := signedBalanceDelta(from, -amount) // 转出
			toDelta := signedBalanceDelta(to, amount)      // 转入
			sum := qaNetContribution(from, fromDelta) + qaNetContribution(to, toDelta)
			if sum != 0 {
				t.Fatalf("转账 %d→%d 的净资产贡献合计应为 0，实际 %d（fromDelta=%d toDelta=%d）",
					from, to, sum, fromDelta, toDelta)
			}
		}
	}
}

// TestQAPRD10_3RepaymentNumbers PRD §10.3 验收的精确数字口径
//
// 储蓄 1000 元（100000 分）+ 信用卡欠款 0；刷卡 100 元 → 信用卡 +10000、净资产 90000；
// 还款 100 元 → 储蓄 90000 / 信用卡 0，净资产仍 90000。绝不允许出现 110000。
func TestQAPRD10_3RepaymentNumbers(t *testing.T) {
	savings := model.Account{Id: 1, Category: model.CategoryDemandDeposit, Balance: 100000}
	credit := model.Account{Id: 2, Category: model.CategoryCreditCard, Balance: 0}

	if got := qaNetAssets([]model.Account{savings, credit}); got != 100000 {
		t.Fatalf("初始净资产应为 100000 分，实际 %d", got)
	}

	// ① 刷卡 100 元：支出，账户=信用卡，flow = -10000
	credit.Balance += signedBalanceDelta(credit.Category, -10000)
	if credit.Balance != 10000 {
		t.Fatalf("刷卡后信用卡 balance 应为 +10000（正=欠款），实际 %d", credit.Balance)
	}
	if got := qaNetAssets([]model.Account{savings, credit}); got != 90000 {
		t.Fatalf("刷卡后净资产应为 90000 分（900 元），实际 %d", got)
	}

	// ② 还款 100 元：储蓄卡 → 信用卡 转账，转出 flow=-10000、转入 flow=+10000
	savings.Balance += signedBalanceDelta(savings.Category, -10000)
	credit.Balance += signedBalanceDelta(credit.Category, 10000)
	if savings.Balance != 90000 || credit.Balance != 0 {
		t.Fatalf("还款后应为 储蓄 90000 / 信用卡 0，实际 %d / %d", savings.Balance, credit.Balance)
	}
	if got := qaNetAssets([]model.Account{savings, credit}); got != 90000 {
		t.Fatalf("还款不应改变净资产（应仍为 90000），实际 %d", got)
	}
	if got := qaNetAssets([]model.Account{savings, credit}); got == 110000 {
		t.Fatal("净资产被虚增为 110000（把负债当成负余额了）")
	}
}

// TestQATransferRequiresZeroCategory 转账不带分类是"不计入收支统计"的结构性前提：
// 只有 category_id=0 且 type=3，后续按分类汇总才不会把还款算成支出。
func TestQATransferRequiresZeroCategory(t *testing.T) {
	tr := qaBaseTransfer()
	if err := validateCreate(1, 1, normalizeCreate(tr)); err != nil {
		t.Fatalf("合法转账不应被拒绝：%v", err)
	}
	bad := tr
	bad.CategoryId = 11
	if err := validateCreate(1, 1, normalizeCreate(bad)); !errors.Is(err, ErrInvalidCreate) {
		t.Fatalf("转账带分类必须被拒绝（否则会重复计入支出），实际：%v", err)
	}
}

// TestQATxRowBothLegsShareOneVersion 两条转账记录必须同版本、同账本、同发生时间
//
// 依据《转账入口裁决》§4："两条关联交易、两侧账户与回执在同一事务内成功或回滚。
// 交易和账户写同一版本，发生时间从命令传入"，"不能落成 Unix 0"。
func TestQATxRowBothLegsShareOneVersion(t *testing.T) {
	const (
		creatorUid int64 = 9 // 刻意与 bookID 取不同值：两个 int64 参数写反时立刻失败
		bookID     int64 = 7
		ver        int64 = 42
	)
	r := normalizeCreate(qaBaseTransfer())
	r.Source = int(model.SourceRecognition) // 模拟"识别后人工确认入账"
	r.TimezoneOffset = -300

	out := newTxRow(creatorUid, bookID, r, ver)
	in := newTxRow(creatorUid, bookID, r, ver)

	for name, row := range map[string]*model.Transaction{"转出": out, "转入": in} {
		if row.SyncVersion != ver {
			t.Errorf("%s记录版本应为 %d，实际 %d", name, ver, row.SyncVersion)
		}
		if row.BookId != bookID {
			t.Errorf("%s记录 book_id 应为 %d，实际 %d", name, bookID, row.BookId)
		}
		if row.CreatorUid != creatorUid {
			t.Errorf("%s记录 creator_uid 应为 %d，实际 %d（归属丢失会让可见性与阶段 4 权限失效）",
				name, creatorUid, row.CreatorUid)
		}
		if row.TxType != model.TxTypeTransfer {
			t.Errorf("%s记录 type 应为 3（转账），实际 %d", name, row.TxType)
		}
		if row.CategoryId != 0 {
			t.Errorf("%s记录的 category_id 必须为 0（转账不计入收支），实际 %d", name, row.CategoryId)
		}
		if row.TxTime != r.TxTime {
			t.Errorf("%s记录 tx_time 应来自命令 %d，实际 %d", name, r.TxTime, row.TxTime)
		}
		if row.TimezoneOffset != r.TimezoneOffset {
			t.Errorf("%s记录 timezone_offset 应为 %d，实际 %d", name, r.TimezoneOffset, row.TimezoneOffset)
		}
		if row.Source != int8(r.Source) {
			t.Errorf("%s记录 source 应为 %d（手动/识别须可分），实际 %d", name, r.Source, row.Source)
		}
		if row.ClientUuid != r.ClientUuid {
			t.Errorf("%s记录 client_uuid 应为 %q，实际 %q", name, r.ClientUuid, row.ClientUuid)
		}
		if row.Amount != r.Amount {
			t.Errorf("%s记录 amount 应为 %d，实际 %d", name, r.Amount, row.Amount)
		}
	}

	// 单笔：分类必须落库，且不带对手账户
	single := newTxRow(creatorUid, bookID, normalizeCreate(qaBaseExpense()), ver)
	if single.CategoryId != 11 {
		t.Errorf("支出记录的 category_id 应为 11，实际 %d", single.CategoryId)
	}
	if single.ToAccountId != 0 {
		t.Errorf("单笔记录不得带 to_account_id，实际 %d", single.ToAccountId)
	}
	if single.CreatorUid != creatorUid || single.BookId != bookID {
		t.Errorf("单笔记录归属/账本错误：creator=%d book=%d", single.CreatorUid, single.BookId)
	}
	if single.TimezoneOffset != 480 || single.Source != int8(model.SourceManual) {
		t.Errorf("单笔记录的命令上下文未落库：tz=%d source=%d", single.TimezoneOffset, single.Source)
	}
}

// TestQARecordCreatorUidWhenModelSupportsIt 审计归属守卫（迁移 0003 后转为强断言）
//
// 《新手开发完整指南》§6.4 与《落地方案》§6.4 要求 transactions.creator_uid NOT NULL；
// PRD §7.3 可见性与阶段 4「member 仅能改删本人交易」依赖它。创建时必须落库、不得留 0。
func TestQARecordCreatorUidWhenModelSupportsIt(t *testing.T) {
	rt := reflect.TypeOf(model.Transaction{})
	if _, ok := rt.FieldByName("CreatorUid"); !ok {
		t.Skip("model.Transaction 尚无 CreatorUid 字段：creator_uid 未落库（偏离未修）")
	}

	const uid int64 = 9
	r := normalizeCreate(qaBaseExpense())
	row := newTxRow(uid, 1, r, 5)
	fv := reflect.ValueOf(row).Elem().FieldByName("CreatorUid")
	if fv.Int() != uid {
		t.Fatalf("CreatorUid 应落为 %d，实际 %d（字段存在但未接线）", uid, fv.Int())
	}
	// 0 是"未知归属"的哨兵值，绝不能作为新建交易的归属
	if fv.Int() == 0 {
		t.Fatal("新建交易的 creator_uid 不得为 0")
	}
}

// TestQARequestHashMatchesDocumentedScheme 用独立判据验证摘要方案
//
// 《转账入口裁决》§3：摘要 = 带 transaction.create/v1 域标签的完整 CreateRequest
// 的确定性编码，排除操作键（clientUuid）。这里不用被测函数做编码，而是手写规范 JSON
// 再自行 sha256，避免"实现和自己的编码比较"这种循环验证。
func TestQARequestHashMatchesDocumentedScheme(t *testing.T) {
	r := normalizeCreate(qaBaseExpense())

	got, err := createRequestHash(r)
	if err != nil {
		t.Fatalf("计算摘要失败：%v", err)
	}
	if len(got) != 64 {
		t.Fatalf("sha256 十六进制摘要应为 64 字符，实际 %d：%s", len(got), got)
	}
	if strings.ToLower(got) != got {
		t.Fatalf("摘要应为小写十六进制，实际 %s", got)
	}

	// 规范 JSON：字段顺序 = CreateRequest 声明顺序，clientUuid 置空（操作键不参与业务身份）
	const canonical = `{"domain":"transaction.create/v1","request":{` +
		`"clientUuid":"","txType":2,"accountId":1,"toAccountId":0,"amount":3650,` +
		`"categoryId":11,"paymentChannel":1,"txTime":1793260800,` +
		`"timezoneOffset":480,"comment":"午餐","source":0}}`
	sum := sha256.Sum256([]byte(canonical))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("摘要与文档方案不一致：\n got=%s\nwant=%s\ncanonical=%s", got, want, canonical)
	}

	// 只换操作键 → 摘要不变（否则同一请求重试会被误判冲突）
	other := r
	other.ClientUuid = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	if h, _ := createRequestHash(other); h != got {
		t.Error("clientUuid 不应参与业务摘要")
	}

	// 归一化发生在摘要之前：备注尾部空白不得改变摘要（重试稳定性）
	padded := r
	padded.Comment = "午餐   "
	if h, _ := createRequestHash(normalizeCreate(padded)); h != got {
		t.Error("备注归一化后摘要应保持一致")
	}

	// 任一业务字段变化 → 摘要必须变
	for name, mutate := range map[string]func(*CreateRequest){
		"txType":         func(x *CreateRequest) { x.TxType = int(model.TxTypeIncome) },
		"accountId":      func(x *CreateRequest) { x.AccountId = 2 },
		"toAccountId":    func(x *CreateRequest) { x.ToAccountId = 3 },
		"amount":         func(x *CreateRequest) { x.Amount = 3651 },
		"categoryId":     func(x *CreateRequest) { x.CategoryId = 12 },
		"paymentChannel": func(x *CreateRequest) { x.PaymentChannel = int(model.PaymentChannelPOS) },
		"txTime":         func(x *CreateRequest) { x.TxTime = 1793260801 },
		"timezoneOffset": func(x *CreateRequest) { x.TimezoneOffset = -480 },
		"comment":        func(x *CreateRequest) { x.Comment = "晚餐" },
		"source":         func(x *CreateRequest) { x.Source = int(model.SourceRecognition) },
	} {
		v := r
		mutate(&v)
		h, err := createRequestHash(v)
		if err != nil {
			t.Fatalf("%s 摘要计算失败：%v", name, err)
		}
		if h == got {
			t.Errorf("%s 未参与摘要：改了该字段仍会被判为重放", name)
		}
	}
}

// TestQAValidateCreateBoundaries 边界值表（服务层强制校验，不依赖 handler）
func TestQAValidateCreateBoundaries(t *testing.T) {
	const (
		uid    int64 = 1
		bookID int64 = 1
	)

	cases := []struct {
		name   string
		uid    int64
		bookID int64
		mutate func(*CreateRequest)
		wantOK bool
	}{
		{"合法支出", uid, bookID, nil, true},
		{"uid=0", 0, bookID, nil, false},
		{"uid<0", -1, bookID, nil, false},
		{"bookID=0", uid, 0, nil, false},
		{"bookID<0", uid, -1, nil, false},
		{"clientUuid 64 字节", uid, bookID, func(x *CreateRequest) {
			x.ClientUuid = strings.Repeat("a", maxClientUUIDLen)
		}, true},
		{"clientUuid 65 字节", uid, bookID, func(x *CreateRequest) {
			x.ClientUuid = strings.Repeat("a", maxClientUUIDLen+1)
		}, false},
		{"clientUuid 空", uid, bookID, func(x *CreateRequest) { x.ClientUuid = "" }, false},
		{"clientUuid 全空白", uid, bookID, func(x *CreateRequest) { x.ClientUuid = " \t\n " }, false},
		{"amount=1 分", uid, bookID, func(x *CreateRequest) { x.Amount = 1 }, true},
		{"amount=0", uid, bookID, func(x *CreateRequest) { x.Amount = 0 }, false},
		{"amount<0", uid, bookID, func(x *CreateRequest) { x.Amount = -1 }, false},
		{"accountId=0", uid, bookID, func(x *CreateRequest) { x.AccountId = 0 }, false},
		{"txTime=0", uid, bookID, func(x *CreateRequest) { x.TxTime = 0 }, false},
		{"收入 txType=1", uid, bookID, func(x *CreateRequest) { x.TxType = int(model.TxTypeIncome) }, true},
		{"txType=0", uid, bookID, func(x *CreateRequest) { x.TxType = 0 }, false},
		{"txType=4（v0.3 未开放）", uid, bookID, func(x *CreateRequest) { x.TxType = 4 }, false},
		{"channel=0 直接", uid, bookID, func(x *CreateRequest) { x.PaymentChannel = 0 }, true},
		{"channel=5 其他", uid, bookID, func(x *CreateRequest) { x.PaymentChannel = int(model.PaymentChannelOther) }, true},
		{"channel=-1", uid, bookID, func(x *CreateRequest) { x.PaymentChannel = -1 }, false},
		{"channel=6", uid, bookID, func(x *CreateRequest) { x.PaymentChannel = 6 }, false},
		{"source=0 手动", uid, bookID, func(x *CreateRequest) { x.Source = 0 }, true},
		{"source=1 识别", uid, bookID, func(x *CreateRequest) { x.Source = 1 }, true},
		{"source=2 导入（预留不可用）", uid, bookID, func(x *CreateRequest) { x.Source = 2 }, false},
		{"source=-1", uid, bookID, func(x *CreateRequest) { x.Source = -1 }, false},
		{"tz=-840", uid, bookID, func(x *CreateRequest) { x.TimezoneOffset = -840 }, true},
		{"tz=840", uid, bookID, func(x *CreateRequest) { x.TimezoneOffset = 840 }, true},
		{"tz=-841", uid, bookID, func(x *CreateRequest) { x.TimezoneOffset = -841 }, false},
		{"tz=841", uid, bookID, func(x *CreateRequest) { x.TimezoneOffset = 841 }, false},
		{"备注 255 字", uid, bookID, func(x *CreateRequest) {
			x.Comment = strings.Repeat("字", maxCommentRunes)
		}, true},
		{"备注 256 字", uid, bookID, func(x *CreateRequest) {
			x.Comment = strings.Repeat("字", maxCommentRunes+1)
		}, false},
		{"备注非法 UTF-8", uid, bookID, func(x *CreateRequest) { x.Comment = "\xff\xfe坏字节" }, false},
		{"支出带对手账户", uid, bookID, func(x *CreateRequest) { x.ToAccountId = 2 }, false},
		{"支出缺分类", uid, bookID, func(x *CreateRequest) { x.CategoryId = 0 }, false},
	}

	for _, c := range cases {
		r := normalizeCreate(func() CreateRequest {
			x := qaBaseExpense()
			if c.mutate != nil {
				c.mutate(&x)
			}
			return x
		}())
		err := validateCreate(c.uid, c.bookID, r)
		if c.wantOK && err != nil {
			t.Errorf("%s：应通过，实际被拒绝（%v）", c.name, err)
		}
		if !c.wantOK && !errors.Is(err, ErrInvalidCreate) {
			t.Errorf("%s：应被 ErrInvalidCreate 拒绝，实际 %v", c.name, err)
		}
	}
}

// TestQANormalizeCreateClientUuid 归一化必须去空白，否则"空白键"会绕过必填
func TestQANormalizeCreateClientUuid(t *testing.T) {
	out := normalizeCreate(CreateRequest{ClientUuid: "  key-1  ", Comment: "  午餐  "})
	if out.ClientUuid != "key-1" {
		t.Errorf("clientUuid 应被 trim，实际 %q", out.ClientUuid)
	}
	if out.Comment != "午餐" {
		t.Errorf("comment 应被 trim，实际 %q", out.Comment)
	}
	// 归一化后仅剩空白的键必须被校验拦下
	if err := validateCreate(1, 1, normalizeCreate(CreateRequest{ClientUuid: "   "})); !errors.Is(err, ErrInvalidCreate) {
		t.Errorf("空白 clientUuid 必须在服务层被拒绝，实际 %v", err)
	}
}

// TestQAResultIDCodecStrict 回执 result_ids 编解码的边界与严格性
func TestQAResultIDCodecStrict(t *testing.T) {
	// 往返一致
	for _, tc := range []struct {
		ids  []int64
		kind int8
	}{
		{[]int64{1}, model.IdempotencyResultSingle},
		{[]int64{9223372036854775807}, model.IdempotencyResultSingle},
		{[]int64{12, 13}, model.IdempotencyResultTransfer},
	} {
		enc := encodeResultIDs(tc.ids)
		dec, err := decodeResultIDs(enc, tc.kind)
		if err != nil {
			t.Fatalf("编码 %v 后解码失败：%v", tc.ids, err)
		}
		if len(dec) != len(tc.ids) {
			t.Fatalf("往返长度不一致：%v → %v", tc.ids, dec)
		}
		for i := range dec {
			if dec[i] != tc.ids[i] {
				t.Fatalf("往返值不一致：%v → %v", tc.ids, dec)
			}
		}
	}

	bad := []struct {
		value string
		kind  int8
	}{
		{"", model.IdempotencyResultSingle},
		{"   ", model.IdempotencyResultSingle},
		{"12", model.IdempotencyResultTransfer},
		{"12,13", model.IdempotencyResultSingle},
		{"12,13,14", model.IdempotencyResultTransfer},
		{"12,12", model.IdempotencyResultTransfer},
		{"0", model.IdempotencyResultSingle},
		{"-5", model.IdempotencyResultSingle},
		{"abc", model.IdempotencyResultSingle},
		{"12,", model.IdempotencyResultTransfer},
		{",12", model.IdempotencyResultTransfer},
		{"12,,13", model.IdempotencyResultTransfer},
		{"99999999999999999999", model.IdempotencyResultSingle},
	}
	for _, c := range bad {
		if _, err := decodeResultIDs(c.value, c.kind); !errors.Is(err, ErrIncompleteReceipt) {
			t.Errorf("非法回执 %q（kind=%d）应报 ErrIncompleteReceipt，实际 %v", c.value, c.kind, err)
		}
	}

	// 宽松但无害：数字两侧空白与多余空格不允许出现在生产回写里，解析仍可容忍
	if got, err := decodeResultIDs(" 12 , 13 ", model.IdempotencyResultTransfer); err != nil ||
		len(got) != 2 || got[0] != 12 || got[1] != 13 {
		t.Errorf("带空白的转账回执应可解析，实际 %v / %v", got, err)
	}
}

// TestQANormalizePagingEdge 分页边界：默认 20、上限 200、非法值兜底
func TestQANormalizePagingEdge(t *testing.T) {
	cases := []struct {
		page, count         int
		wantPage, wantCount int
	}{
		{0, 0, 1, defaultTxPageSize},
		{-1, -1, 1, defaultTxPageSize},
		{1, 1, 1, 1},
		{3, 20, 3, 20},
		{1, maxTxPageSize, 1, maxTxPageSize},
		{1, maxTxPageSize + 1, 1, maxTxPageSize},
		{0, 1000, 1, maxTxPageSize},
	}
	for _, c := range cases {
		p, n := normalizePaging(c.page, c.count)
		if p != c.wantPage || n != c.wantCount {
			t.Errorf("normalizePaging(%d,%d) = (%d,%d)，期望 (%d,%d)",
				c.page, c.count, p, n, c.wantPage, c.wantCount)
		}
	}
}

// TestQAAmountUpperBound 金额上界 maxAmountCents（1 亿元）的边界
//
// 上界的作用是把"必然溢出"挡在入口（400），而不是让余额 UPDATE 溢出后报一个
// 可重试的 409 并发冲突（那种错误重试永远不会成功）。
// 同时必须确认上界不误伤合法大额：恰好等于上界必须通过。
func TestQAAmountUpperBound(t *testing.T) {
	if maxAmountCents <= 0 {
		t.Fatalf("maxAmountCents 必须为正，实际 %d", maxAmountCents)
	}

	// 恰好等于上界：必须通过（支出与转账都验，避免只给一种类型加了界）
	expense := normalizeCreate(qaBaseExpense())
	expense.Amount = maxAmountCents
	if err := validateCreate(1, 1, expense); err != nil {
		t.Fatalf("金额恰好 %d 分应通过，实际被拒绝：%v", maxAmountCents, err)
	}
	transfer := normalizeCreate(qaBaseTransfer())
	transfer.Amount = maxAmountCents
	if err := validateCreate(1, 1, transfer); err != nil {
		t.Fatalf("转账金额恰好 %d 分应通过，实际被拒绝：%v", maxAmountCents, err)
	}

	// 超过上界 / 极端值：必须 400（ErrInvalidCreate），不能落到"可重试的 409"
	for _, amount := range []int64{maxAmountCents + 1, math.MaxInt64} {
		bad := normalizeCreate(qaBaseExpense())
		bad.Amount = amount
		if err := validateCreate(1, 1, bad); !errors.Is(err, ErrInvalidCreate) {
			t.Errorf("金额 %d 分应被 ErrInvalidCreate 拒绝，实际 %v", amount, err)
		}
	}

	// 上界必须远小于 int64 上限，确保合法金额永远不可能触发余额溢出保护
	if maxAmountCents >= math.MaxInt64/2 {
		t.Fatalf("上界 %d 过大，无法保证余额不溢出", maxAmountCents)
	}
}

// TestQANormalizePagingExported 导出版本必须与内部分页规则一致
//
// handler 现在用 service.NormalizePaging 的返回值回显 page/count；一旦两者漂移，
// 响应里就会出现"请求 count=1000、实际只返回 200"的不一致。
func TestQANormalizePagingExported(t *testing.T) {
	for _, c := range []struct{ page, count int }{
		{0, 0}, {1, 1}, {2, 50}, {0, 1000}, {-5, -5}, {3, maxTxPageSize + 1},
	} {
		ep, ec := NormalizePaging(c.page, c.count)
		ip, ic := normalizePaging(c.page, c.count)
		if ep != ip || ec != ic {
			t.Errorf("NormalizePaging(%d,%d)=(%d,%d) 与内部实现 (%d,%d) 不一致",
				c.page, c.count, ep, ec, ip, ic)
		}
	}
	if p, c := NormalizePaging(0, 1000); p != 1 || c != maxTxPageSize {
		t.Errorf("NormalizePaging(0,1000) 应为 (1,%d)，实际 (%d,%d)", maxTxPageSize, p, c)
	}
}

// TestQAResultTypeOfMapping 交易类型 → 回执类型（枚举字典 §12）
func TestQAResultTypeOfMapping(t *testing.T) {
	for _, tc := range []struct {
		txType int
		want   int8
	}{
		{int(model.TxTypeIncome), model.IdempotencyResultSingle},
		{int(model.TxTypeExpense), model.IdempotencyResultSingle},
		{int(model.TxTypeTransfer), model.IdempotencyResultTransfer},
	} {
		r := qaBaseExpense()
		r.TxType = tc.txType
		if got := resultTypeOf(r); got != tc.want {
			t.Errorf("txType=%d 的回执类型应为 %d，实际 %d", tc.txType, tc.want, got)
		}
	}
}

// TestQACreateRequestBindingTag handler 层与 service 层双层必填（《转账入口裁决》§2）
func TestQACreateRequestBindingTag(t *testing.T) {
	rt := reflect.TypeOf(CreateRequest{})
	f, ok := rt.FieldByName("ClientUuid")
	if !ok {
		t.Fatal("CreateRequest 必须含 ClientUuid 字段")
	}
	if got := f.Tag.Get("json"); got != "clientUuid" {
		t.Errorf("JSON 键应为 clientUuid（数据库列才是 client_uuid），实际 %q", got)
	}
	if got := f.Tag.Get("binding"); got != "required" {
		t.Errorf("ClientUuid 应有 binding:\"required\"（handler 层必填），实际 %q", got)
	}

	res := reflect.TypeOf(CreateResult{})
	for field, want := range map[string]string{
		"ResultType":     "resultType",
		"TransactionIds": "transactionIds",
		"Version":        "version",
		"Replayed":       "replayed",
	} {
		sf, ok := res.FieldByName(field)
		if !ok {
			t.Fatalf("CreateResult 缺少字段 %s", field)
		}
		if got := strings.Split(sf.Tag.Get("json"), ",")[0]; got != want {
			t.Errorf("CreateResult.%s 的 JSON 键应为 %s，实际 %s", field, want, got)
		}
	}
}
