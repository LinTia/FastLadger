package service

// 记账核心的纯逻辑单元测试（不依赖数据库：本机无 gcc，CGO 驱动不可运行）。
//
// 覆盖阶段 2 的硬性验收点：
//  1. clientUuid 缺失时【服务层】必须拒绝（不是只在 handler 校验）；
//  2. 转账规则（对手账户必填/不同/不带分类）与单笔规则（分类必填/不带对手）；
//  3. 幂等摘要必须覆盖完整 CreateRequest，而不只是金额；
//  4. 负债符号：储蓄 1000 + 信用欠 0，刷卡 100 后净资产 900（不是 1100），还款后仍 900；
//  5. 幂等回执 result_ids 的解码/校验（单笔 1 个、转账 2 个不同 ID）。
import (
	"errors"
	"strings"
	"testing"

	"fastledger/internal/model"
)

// baseCreate 构造一条合法的支出命令（各用例在其上做单点改动）
func baseCreate() CreateRequest {
	return CreateRequest{
		ClientUuid:     "59c921d1-58cc-48d8-a5ae-1e870ae71fc2",
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

// TestValidateCreateRequiresClientUuid clientUuid 缺失/全空白/超长必须由服务层拒绝
func TestValidateCreateRequiresClientUuid(t *testing.T) {
	cases := []struct {
		name string
		uuid string
	}{
		{"缺失", ""},
		{"全空白", "   "},
		{"超长", strings.Repeat("a", maxClientUUIDLen+1)},
	}
	for _, c := range cases {
		r := baseCreate()
		r.ClientUuid = c.uuid
		r = normalizeCreate(r) // 服务层入口先归一化，再校验
		err := validateCreate(1, 1, r)
		if !errors.Is(err, ErrInvalidCreate) {
			t.Fatalf("%s 的 clientUuid 必须被拒绝，实际：%v", c.name, err)
		}
	}

	// 正例：合法命令必须通过
	if err := validateCreate(1, 1, normalizeCreate(baseCreate())); err != nil {
		t.Fatalf("合法命令不应被拒绝：%v", err)
	}
}

// TestValidateCreateTypeRules 单笔与转账的字段互斥规则
func TestValidateCreateTypeRules(t *testing.T) {
	uid, bookID := int64(1), int64(1)

	// 支出必须带分类、不能带对手账户
	r := baseCreate()
	r.ToAccountId = 2
	if err := validateCreate(uid, bookID, normalizeCreate(r)); !errors.Is(err, ErrInvalidCreate) {
		t.Fatalf("支出带对手账户应被拒绝，实际：%v", err)
	}
	r = baseCreate()
	r.CategoryId = 0
	if err := validateCreate(uid, bookID, normalizeCreate(r)); !errors.Is(err, ErrInvalidCreate) {
		t.Fatalf("支出缺分类应被拒绝，实际：%v", err)
	}

	// 转账必须带不同的对手账户、且不带分类
	tr := CreateRequest{
		ClientUuid: "uuid-transfer",
		TxType:     int(model.TxTypeTransfer),
		AccountId:  1, ToAccountId: 2, Amount: 10000, TxTime: 1793260800,
	}
	if err := validateCreate(uid, bookID, normalizeCreate(tr)); err != nil {
		t.Fatalf("合法转账不应被拒绝：%v", err)
	}
	for _, mutate := range []func(*CreateRequest){
		func(x *CreateRequest) { x.ToAccountId = 0 },                                  // 缺对手账户
		func(x *CreateRequest) { x.ToAccountId = x.AccountId },                        // 对手账户相同
		func(x *CreateRequest) { x.CategoryId = 5 },                                   // 转账带分类
		func(x *CreateRequest) { x.Amount = 0 },                                       // 金额非正
		func(x *CreateRequest) { x.TxTime = 0 },                                       // 缺发生时间
		func(x *CreateRequest) { x.TxType = 9 },                                       // 非法类型
		func(x *CreateRequest) { x.PaymentChannel = 9 },                               // 非法渠道
		func(x *CreateRequest) { x.TimezoneOffset = 900 },                             // 非法时区
		func(x *CreateRequest) { x.Source = 5 },                                       // 非法来源
		func(x *CreateRequest) { x.Comment = strings.Repeat("字", maxCommentRunes+1) }, // 备注超长
	} {
		bad := tr
		mutate(&bad)
		if err := validateCreate(uid, bookID, normalizeCreate(bad)); !errors.Is(err, ErrInvalidCreate) {
			t.Fatalf("非法转账用例未被拒绝：%+v，实际：%v", bad, err)
		}
	}
}

// TestRequestHashCoversWholeRequest 摘要必须覆盖完整 CreateRequest（GPT-R20）
func TestRequestHashCoversWholeRequest(t *testing.T) {
	base := normalizeCreate(baseCreate())
	h0, err := createRequestHash(base)
	if err != nil {
		t.Fatalf("计算摘要失败：%v", err)
	}
	// 同内容重复计算结果必须一致（重试稳定）
	again, err := createRequestHash(base)
	if err != nil || again != h0 {
		t.Fatalf("同一请求摘要必须稳定：%v / %v", again, err)
	}

	// 操作键只用于定位回执，不参与业务身份
	otherKey := base
	otherKey.ClientUuid = "another-client-uuid"
	if h, _ := createRequestHash(otherKey); h != h0 {
		t.Fatal("clientUuid 不应参与业务摘要")
	}

	// 除 clientUuid 外的业务字段全部参与摘要
	mutations := map[string]func(*CreateRequest){
		"txType":         func(x *CreateRequest) { x.TxType = int(model.TxTypeIncome) },
		"accountId":      func(x *CreateRequest) { x.AccountId = 2 },
		"toAccountId":    func(x *CreateRequest) { x.ToAccountId = 3 },
		"amount":         func(x *CreateRequest) { x.Amount = 3651 },
		"categoryId":     func(x *CreateRequest) { x.CategoryId = 12 },
		"paymentChannel": func(x *CreateRequest) { x.PaymentChannel = int(model.PaymentChannelAlipay) },
		"txTime":         func(x *CreateRequest) { x.TxTime++ },
		"comment":        func(x *CreateRequest) { x.Comment = "晚餐" },
		"timezoneOffset": func(x *CreateRequest) { x.TimezoneOffset = 0 },
		"source":         func(x *CreateRequest) { x.Source = int(model.SourceRecognition) },
	}
	for name, mutate := range mutations {
		v := base
		mutate(&v)
		h, err := createRequestHash(v)
		if err != nil {
			t.Fatalf("%s 摘要计算失败：%v", name, err)
		}
		if h == h0 {
			t.Errorf("%s 未参与幂等摘要（会误判为重放）", name)
		}
	}
}

// netAssets 净资产 = Σ(资产 1~9) − Σ(负债 10~15)，余额均为整数分
func netAssets(accounts []model.Account) int64 {
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

// applyDelta 按 signedBalanceDelta 规则把签名现金流落到账户余额上
func applyDelta(a *model.Account, flow int64) {
	a.Balance += signedBalanceDelta(a.Category, flow)
}

// TestSignedBalanceDeltaKeepsNetAssets 负债符号：刷卡不虚增净资产，还款守恒
//
// 场景：储蓄 1000 元（asset 2，balance=100000 分）+ 信用卡欠款 0（liability 10）。
// 刷卡消费 100 元 → 信用卡 balance 变 +10000（欠更多），净资产应为 90000（900 元），
// 而不是把负债当负余额时的 110000（1100 元）。
func TestSignedBalanceDeltaKeepsNetAssets(t *testing.T) {
	savings := model.Account{Id: 1, Category: model.CategoryDemandDeposit, Balance: 100000}
	credit := model.Account{Id: 2, Category: model.CategoryCreditCard, Balance: 0}

	if got := netAssets([]model.Account{savings, credit}); got != 100000 {
		t.Fatalf("初始净资产应为 100000，实际 %d", got)
	}

	// ① 支出 100 元付在信用卡上：flow = -10000
	applyDelta(&credit, -10000)
	if credit.Balance != 10000 {
		t.Fatalf("信用卡欠款应为 +10000（正=欠款），实际 %d", credit.Balance)
	}
	if got := netAssets([]model.Account{savings, credit}); got != 90000 {
		t.Fatalf("刷卡后净资产应为 90000（900 元），实际 %d", got)
	}
	if got := netAssets([]model.Account{savings, credit}); got == 110000 {
		t.Fatal("净资产被虚增为 1100 元：负债符号写反了")
	}

	// ② 还款：储蓄卡 → 信用卡 转账 100 元（转出 flow=-10000，转入 flow=+10000）
	applyDelta(&savings, -10000)
	applyDelta(&credit, 10000)
	if savings.Balance != 90000 || credit.Balance != 0 {
		t.Fatalf("还款后应为储蓄 90000 / 信用卡 0，实际 %d / %d", savings.Balance, credit.Balance)
	}
	if got := netAssets([]model.Account{savings, credit}); got != 90000 {
		t.Fatalf("还款不应改变净资产（仍应 90000），实际 %d", got)
	}
}

// TestSignedBalanceDeltaAssetTransfer 资产→资产：一减一增，净资产不变
func TestSignedBalanceDeltaAssetTransfer(t *testing.T) {
	cash := model.Account{Category: model.CategoryCash, Balance: 5000}
	wallet := model.Account{Category: model.CategoryEWallet, Balance: 0}
	before := netAssets([]model.Account{cash, wallet})

	applyDelta(&cash, -3000)
	applyDelta(&wallet, 3000)

	if cash.Balance != 2000 || wallet.Balance != 3000 {
		t.Fatalf("资产互转后余额不符：%d / %d", cash.Balance, wallet.Balance)
	}
	if got := netAssets([]model.Account{cash, wallet}); got != before {
		t.Fatalf("资产互转净资产应守恒：%d → %d", before, got)
	}
}

// TestResultIDCodec 幂等回执 result_ids 编码/解码（单笔 1 个、转账 2 个不同 ID）
func TestResultIDCodec(t *testing.T) {
	if got := encodeResultIDs([]int64{12, 13}); got != "12,13" {
		t.Fatalf("编码结果不符：%s", got)
	}

	single, err := decodeResultIDs("12", model.IdempotencyResultSingle)
	if err != nil || len(single) != 1 || single[0] != 12 {
		t.Fatalf("单笔回执解码失败：%v %v", single, err)
	}
	transfer, err := decodeResultIDs("12,13", model.IdempotencyResultTransfer)
	if err != nil || len(transfer) != 2 || transfer[0] != 12 || transfer[1] != 13 {
		t.Fatalf("转账回执解码失败：%v %v", transfer, err)
	}

	bad := []struct {
		value string
		kind  int8
	}{
		{"", model.IdempotencyResultSingle},        // 空回执
		{"12", model.IdempotencyResultTransfer},    // 转账只有 1 个 ID
		{"12,13", model.IdempotencyResultSingle},   // 单笔却有 2 个 ID
		{"12,12", model.IdempotencyResultTransfer}, // 转账两个 ID 相同
		{"0", model.IdempotencyResultSingle},       // 非正 ID
		{"abc", model.IdempotencyResultSingle},     // 非法 ID
		{"12,", model.IdempotencyResultTransfer},   // 尾随逗号
	}
	for _, c := range bad {
		if _, err := decodeResultIDs(c.value, c.kind); !errors.Is(err, ErrIncompleteReceipt) {
			t.Errorf("非法回执 %q（kind=%d）应被拒绝，实际：%v", c.value, c.kind, err)
		}
	}
}

// TestResultTypeOf 交易类型 → 幂等结果类型（枚举字典 §12）
func TestResultTypeOf(t *testing.T) {
	if got := resultTypeOf(baseCreate()); got != model.IdempotencyResultSingle {
		t.Fatalf("支出应为单笔回执，实际 %d", got)
	}
	transfer := baseCreate()
	transfer.TxType = int(model.TxTypeTransfer)
	if got := resultTypeOf(transfer); got != model.IdempotencyResultTransfer {
		t.Fatalf("转账应为转账回执，实际 %d", got)
	}
}

// TestNormalizePaging 分页边界：默认 20、上限 200、非法值兜底
func TestNormalizePaging(t *testing.T) {
	if p, c := normalizePaging(0, 0); p != 1 || c != defaultTxPageSize {
		t.Fatalf("默认分页不符：page=%d count=%d", p, c)
	}
	if _, c := normalizePaging(-3, maxTxPageSize+50); c != maxTxPageSize {
		t.Fatalf("count 应被截断到 %d，实际 %d", maxTxPageSize, c)
	}
	// handler 回显必须与真正生效的值一致（对外导出同一份归一化逻辑）
	if p1, c1 := NormalizePaging(2, maxTxPageSize+1); p1 != 2 || c1 != maxTxPageSize {
		t.Fatalf("NormalizePaging 与内部归一化不一致：page=%d count=%d", p1, c1)
	}
}

// TestValidateCreateAmountUpperBound 金额上界：挡住"必然溢出"的金额（否则余额 UPDATE
// 溢出会被误判成可重试的 409，重试永不成功）
func TestValidateCreateAmountUpperBound(t *testing.T) {
	r := normalizeCreate(baseCreate())

	r.Amount = maxAmountCents
	if err := validateCreate(1, 1, r); err != nil {
		t.Fatalf("金额等于上限（%d 分）应通过：%v", maxAmountCents, err)
	}
	r.Amount = maxAmountCents + 1
	if err := validateCreate(1, 1, r); !errors.Is(err, ErrInvalidCreate) {
		t.Fatalf("金额超上限应被 ErrInvalidCreate 拒绝，实际：%v", err)
	}
}

// TestNewTxRowWritesCreatorAndCommandContext creator_uid / source / timezone_offset 必须落库。
//
// 若只加字段不接线，阶段 4「member 仅能改删本人交易」与审计归属会建立在错误数据上。
func TestNewTxRowWritesCreatorAndCommandContext(t *testing.T) {
	const (
		creator int64 = 7
		bookID  int64 = 42
		ver     int64 = 5
	)
	r := normalizeCreate(baseCreate())
	row := newTxRow(creator, bookID, r, ver)

	if row.CreatorUid != creator {
		t.Fatalf("creator_uid 应为 %d，实际 %d", creator, row.CreatorUid)
	}
	if row.BookId != bookID {
		t.Fatalf("book_id 应为 %d，实际 %d", bookID, row.BookId)
	}
	if row.SyncVersion != ver {
		t.Fatalf("sync_version 应为 %d，实际 %d", ver, row.SyncVersion)
	}
	if int(row.Source) != r.Source {
		t.Fatalf("source 应为 %d，实际 %d", r.Source, row.Source)
	}
	if row.TimezoneOffset != r.TimezoneOffset {
		t.Fatalf("timezone_offset 应为 %d，实际 %d", r.TimezoneOffset, row.TimezoneOffset)
	}
	if row.TxTime != r.TxTime || row.ClientUuid != r.ClientUuid {
		t.Fatalf("tx_time/client_uuid 未按命令落库：%+v", row)
	}
}
