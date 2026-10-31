package service

// 阶段 3（同步 + 可见性）的纯逻辑单元测试。
//
// 本机无 gcc、CGO 不可用（mattn/go-sqlite3 能编译不能运行），因此这里只覆盖
// 【不碰数据库】的判定逻辑：
//  1. txVisibilityCond 的三种组合，以及 creator_uid=0 哨兵不放行；
//  2. limit 归一化；
//  3. push 逐条状态映射（created/replayed/conflict/invalid/forbidden）；
//  4. Push 的整批非法边界（空 / 超长）；
//  5. SyncBatch 的 JSON 契约（lowerCamel + restoreEpoch/instanceId）。
import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"fastledger/internal/model"
)

// TestTxVisibilityCond 可见性条件三组合（PRD §7.3 + WBS 归属哨兵约束）
func TestTxVisibilityCond(t *testing.T) {
	// 1) 能看全部（owner / admin / 开关打开）：不加任何条件
	cond, args := txVisibilityCond(true, 7)
	if cond != "" || args != nil {
		t.Fatalf("canViewAll=true 应返回空条件，实际 cond=%q args=%v", cond, args)
	}

	// 2) 只能看本人：条件必须同时含 creator_uid = ? 与 creator_uid <> 0
	cond, args = txVisibilityCond(false, 7)
	if cond == "" {
		t.Fatal("canViewAll=false 必须返回可见性条件")
	}
	if len(args) != 1 || args[0].(int64) != int64(7) {
		t.Fatalf("参数应为 [7]，实际 %v", args)
	}
	if !strings.Contains(cond, "creator_uid = ?") {
		t.Fatalf("条件缺少 creator_uid = ?：%q", cond)
	}
	if !strings.Contains(cond, "creator_uid <> 0") {
		t.Fatalf("条件必须显式排除 creator_uid=0 的历史未知归属：%q", cond)
	}
	// 禁止"放行"写法：一旦出现 IN (...,0) 之类，会把未知归属泄露给 member
	for _, forbidden := range []string{"IN (", "OR creator_uid = 0", "creator_uid = 0"} {
		if strings.Contains(cond, forbidden) {
			t.Fatalf("可见性条件出现放行写法 %q：%q", forbidden, cond)
		}
	}

	// 3) uid 非法（<=0）时同样返回过滤条件：creator_uid=0 AND creator_uid<>0 恒假，
	//    一条都查不到（fail-closed），绝不因为参数异常而放行全部
	cond, args = txVisibilityCond(false, 0)
	if cond == "" {
		t.Fatal("uid=0 也必须返回过滤条件（fail-closed）")
	}
	if len(args) != 1 || args[0].(int64) != int64(0) {
		t.Fatalf("参数应为 [0]，实际 %v", args)
	}
}

// TestNormalizeSyncLimit 批次大小归一化（<=0 或 >500 一律 500）
func TestNormalizeSyncLimit(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{-1, MaxSyncBatch},
		{0, MaxSyncBatch},
		{1, 1},
		{250, 250},
		{MaxSyncBatch, MaxSyncBatch},
		{MaxSyncBatch + 1, MaxSyncBatch},
		{100000, MaxSyncBatch},
	}
	for _, c := range cases {
		if got := normalizeSyncLimit(c.in); got != c.want {
			t.Errorf("normalizeSyncLimit(%d)=%d，期望 %d", c.in, got, c.want)
		}
	}
}

// TestPushStatusOfMapping 错误 → 逐条状态映射（推送结果的状态枚举不得越界）
func TestPushStatusOfMapping(t *testing.T) {
	validStatus := map[string]bool{
		PushStatusCreated:   true,
		PushStatusReplayed:  true,
		PushStatusConflict:  true,
		PushStatusInvalid:   true,
		PushStatusForbidden: true,
		PushStatusError:     true,
	}
	cases := []struct {
		name   string
		err    error
		status string
		code   string
	}{
		{"非成员", ErrCreateForbidden, PushStatusForbidden, ErrCodeForbidden},
		{"非成员-通用", ErrNotMember, PushStatusForbidden, ErrCodeForbidden},
		{"无权限", ErrForbidden, PushStatusForbidden, ErrCodeForbidden},
		{"幂等内容冲突", ErrCreateConflict, PushStatusConflict, ErrCodeClientUUIDConflict},
		{"回执不完整", ErrIncompleteReceipt, PushStatusConflict, ErrCodeIncompleteReceipt},
		{"并发冲突", ErrConcurrentConflict, PushStatusConflict, ErrCodeConcurrentConflict},
		{"账户不存在", ErrAccountNotFound, PushStatusInvalid, ErrCodeAccountNotFound},
		{"分类不存在", ErrCategoryNotFound, PushStatusInvalid, ErrCodeCategoryNotFound},
		{"交易不存在", ErrTransactionNotFound, PushStatusInvalid, ErrCodeInvalidRequest},
		{"请求非法", ErrInvalidCreate, PushStatusInvalid, ErrCodeInvalidRequest},
		{"参数非法", ErrInvalidArgument, PushStatusInvalid, ErrCodeInvalidRequest},
		{"未预期错误", errors.New("boom"), PushStatusError, ErrCodeInternal},
	}
	for _, c := range cases {
		status, code, msg := pushStatusOf(c.err)
		if status != c.status || code != c.code {
			t.Errorf("%s：得到 (%s,%s)，期望 (%s,%s)", c.name, status, code, c.status, c.code)
		}
		if !validStatus[status] {
			t.Errorf("%s：状态 %q 不在契约枚举内", c.name, status)
		}
		if msg == "" {
			t.Errorf("%s：message 不能为空", c.name)
		}
		// 包了一层 fmt.Errorf("%w") 的错误也必须能识别（Create 的实际返回形态）
		wrapped := errors.Join(errors.New("外层"), c.err)
		if s, k, _ := pushStatusOf(wrapped); s != c.status || k != c.code {
			t.Errorf("%s：包装后映射丢失，得到 (%s,%s)", c.name, s, k)
		}
	}
}

// TestPushResultOf created / replayed / nil 结果
func TestPushResultOf(t *testing.T) {
	created := pushResultOf("uuid-1", &CreateResult{
		ResultType: model.IdempotencyResultSingle, TransactionIds: []int64{9}, Version: 12, Replayed: false,
	}, nil)
	if created.Status != PushStatusCreated || created.Id != 9 || created.Version != 12 || created.ErrorCode != "" {
		t.Fatalf("created 结果不符：%+v", created)
	}

	replayed := pushResultOf("uuid-2", &CreateResult{
		ResultType: model.IdempotencyResultTransfer, TransactionIds: []int64{3, 4}, Version: 8, Replayed: true,
	}, nil)
	if replayed.Status != PushStatusReplayed || replayed.Id != 3 || replayed.Version != 8 {
		t.Fatalf("replayed 结果不符：%+v", replayed)
	}
	if len(replayed.TransactionIds) != 2 {
		t.Fatalf("转账重放应回带两个交易 ID：%+v", replayed)
	}

	failed := pushResultOf("uuid-3", nil, ErrCreateForbidden)
	if failed.Status != PushStatusForbidden || failed.ErrorCode != ErrCodeForbidden {
		t.Fatalf("失败结果不符：%+v", failed)
	}
	if failed.Version != 0 || failed.Id != 0 {
		t.Fatalf("失败结果不应带版本/ID：%+v", failed)
	}

	// res 为 nil 且无错误（不可达）也不能 panic
	if got := pushResultOf("uuid-4", nil, nil); got.Status != PushStatusError {
		t.Fatalf("nil 结果应回落为 conflict，实际 %+v", got)
	}
}

// TestPushRejectsWholeBatchInvalid 整批非法：空 items / 超长 → 400 语义
func TestPushRejectsWholeBatchInvalid(t *testing.T) {
	s := &SyncService{}

	if _, err := s.Push(context.Background(), 1, 1, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("空 items 应返回 ErrInvalidArgument，实际 %v", err)
	}
	if _, err := s.Push(context.Background(), 1, 1, []PushItem{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("空 items 应返回 ErrInvalidArgument，实际 %v", err)
	}
	tooMany := make([]PushItem, maxPushItems+1)
	if _, err := s.Push(context.Background(), 1, 1, tooMany); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("超长 items 应返回 ErrInvalidArgument，实际 %v", err)
	}
	if _, err := s.Push(context.Background(), 0, 1, []PushItem{{ClientUuid: "x"}}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("uid 缺失应返回 ErrInvalidArgument，实际 %v", err)
	}
	if _, err := s.Push(context.Background(), 1, 0, []PushItem{{ClientUuid: "x"}}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("bookID 缺失应返回 ErrInvalidArgument，实际 %v", err)
	}
	// 合法项数但服务未注入（txs=nil）：属于整批服务不可用，不能静默当成功
	if _, err := s.Push(context.Background(), 1, 1, []PushItem{{ClientUuid: "x"}}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("txs 未初始化应返回错误，实际 %v", err)
	}
}

// TestPushItemToCreateRequestIsLossless PushItem → CreateRequest 必须是 1:1 拷贝，
// 否则离线补传会静默丢字段（例如 timezoneOffset / source 变成 0）。
func TestPushItemToCreateRequestIsLossless(t *testing.T) {
	item := PushItem{
		ClientUuid:     "uuid-lossless",
		TxType:         int(model.TxTypeTransfer),
		AccountId:      11,
		ToAccountId:    12,
		Amount:         3650,
		CategoryId:     21,
		PaymentChannel: int(model.PaymentChannelAlipay),
		TxTime:         1793260800,
		TimezoneOffset: 480,
		Comment:        "还款",
		Source:         int(model.SourceRecognition),
	}
	got := item.toCreateRequest()
	if got.ClientUuid != item.ClientUuid || got.TxType != item.TxType ||
		got.AccountId != item.AccountId || got.ToAccountId != item.ToAccountId ||
		got.Amount != item.Amount || got.CategoryId != item.CategoryId ||
		got.PaymentChannel != item.PaymentChannel || got.TxTime != item.TxTime ||
		got.TimezoneOffset != item.TimezoneOffset || got.Comment != item.Comment ||
		got.Source != item.Source {
		t.Fatalf("PushItem → CreateRequest 丢字段：%+v → %+v", item, got)
	}
}

// TestSyncBatchJSONContract 批次字段名与世代信息（客户端契约，改动即破坏兼容）
func TestSyncBatchJSONContract(t *testing.T) {
	b := SyncBatch{
		HighVersion:  42,
		Transactions: make([]model.Transaction, 0),
		Accounts:     make([]model.Account, 0),
		Categories:   make([]model.Category, 0),
		Holdings:     make([]model.Holding, 0),
		RestoreEpoch: 3,
		InstanceId:   "abc123",
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}
	for _, key := range []string{
		"highVersion", "transactions", "accounts", "categories", "holdings", "restoreEpoch", "instanceId",
	} {
		if _, ok := m[key]; !ok {
			t.Fatalf("批次缺少字段 %q：%s", key, raw)
		}
	}
	// 空集合必须是 [] 而不是 null（客户端 app 直接遍历不会 null 崩溃）
	for _, key := range []string{"transactions", "accounts", "categories", "holdings"} {
		if string(m[key]) != "[]" {
			t.Fatalf("%s 应为空数组，实际 %s", key, m[key])
		}
	}
	if string(m["restoreEpoch"]) != "3" || string(m["instanceId"]) != `"abc123"` {
		t.Fatalf("世代字段值不符：%s", raw)
	}
}
