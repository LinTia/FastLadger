package service

// 交易（transactions）—— 收入 / 支出 / 转账的【唯一公开创建入口】。
//
// 依据：
//   - 《转账入口裁决》(GPT-ADR01)：唯一入口 TxService.Create，拥有唯一事务、
//     唯一幂等回执占位与唯一版本递增；内部 createTransferInTx 不另开事务、不再加版本。
//   - 《新手开发完整指南》§13.1（可执行骨架）与 §13.2（接线/幂等约束）。
//
// 硬约束：
//  1. 转账一次写双边：两条记录 + 两侧账户余额，同一事务、同一版本；
//  2. 余额改动只有一条路径 createChangeBalance：单条原子 UPDATE，带 book_id 与
//     deleted=0，并校验 RowsAffected==1（禁止"先读后写"）；
//  3. clientUuid 必填由服务层强制（不依赖 handler）；
//  4. 幂等：事务内 INSERT ... ON CONFLICT DO NOTHING + RowsAffected 判定；
//     命中后重读完整回执并核对 request_hash / result_type，一致返回原结果，
//     不一致报冲突；回执回写失败整笔回滚；
//  5. 负债账户（category 10~15）balance 正=欠款，故 delta 取反。
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fastledger/internal/model"
	"fastledger/internal/version"

	"xorm.io/xorm"
)

var (
	// ErrInvalidCreate 交易创建请求非法（HTTP 400）
	ErrInvalidCreate = errors.New("交易创建请求非法")
	// ErrCreateConflict 同一 clientUuid 被用于不同操作内容（HTTP 409）
	ErrCreateConflict = errors.New("clientUuid 已被不同的请求内容使用")
	// ErrCreateForbidden 非本账本成员，无记账权限（HTTP 403）
	ErrCreateForbidden = errors.New("不是该账本成员，无记账权限")
	// ErrIncompleteReceipt 已提交的幂等回执不完整（HTTP 409，可重试）
	ErrIncompleteReceipt = errors.New("幂等回执不完整，请重试")
	// ErrTransactionNotFound 交易不存在（HTTP 404）
	ErrTransactionNotFound = errors.New("交易不存在")
	// ErrConcurrentConflict 并发冲突：目标行已被并发修改（HTTP 409，可重试）
	ErrConcurrentConflict = errors.New("并发冲突，请重试")
	// ErrInvalidArgument 通用入参非法（HTTP 400）
	ErrInvalidArgument = errors.New("请求参数非法")
	// ErrEntityDeleted 目标实体已被删除（HTTP 409）。
	//
	// WBS 4.5 旧离线数据回流：必须与"目标不存在"（404）区分开——
	// 已删是【冲突】，客户端应丢弃该条或转"冲突 / 另存"，
	// 服务端绝不静默复活已删数据。
	ErrEntityDeleted = errors.New("目标已被删除")
	// ErrTxDirectionMissing 转账记录缺少方向标记（direction=0），无法精确反向余额（HTTP 409）。
	//
	// 迁移 0006 会回填历史转账行；若仍出现 0，说明数据需要修复。
	// 此时删除/恢复【必须失败】而不是把余额增量算成 0（那会"钱没回滚却不报错"）。
	ErrTxDirectionMissing = errors.New("转账记录缺少方向标记")
)

const (
	// maxClientUUIDLen 幂等键长度上限（transactions.client_uuid VARCHAR(64)）
	maxClientUUIDLen = 64
	// maxCommentRunes 备注长度上限（transactions.note VARCHAR(255)）
	maxCommentRunes = 255
	// maxAmountCents 单笔金额上界（1 亿元 = 10_000_000_000 分）。
	// 明显超过家庭记账量级，用于在入口就挡住"必然溢出"的金额，而不是让余额 UPDATE 溢出后
	// 报一个可重试的并发冲突（那种错误重试永远不会成功）。
	maxAmountCents int64 = 10_000_000_000
	// txHashDomain 请求摘要的域标签（《转账入口裁决》§3）
	txHashDomain = "transaction.create/v1"
	// txMutateHashDomain 删除/恢复请求摘要的域标签；与 create 域分离，
	// 使同一 clientUuid 的 create 与 delete/restore 摘要天然不同（WBS 4.4）
	txMutateHashDomain = "transaction.mutate/v1"

	defaultTxPageSize = 20
	maxTxPageSize     = 200
)

// CreateRequest 统一创建命令（收入 / 支出 / 转账共用）。
//
// JSON 用 clientUuid，数据库列为 client_uuid；同一命令既用于手动创建，
// 也用于离线 sync/push 的每一项。
type CreateRequest struct {
	ClientUuid     string `json:"clientUuid" binding:"required"`
	TxType         int    `json:"txType"`
	AccountId      int64  `json:"accountId"`
	ToAccountId    int64  `json:"toAccountId"`
	Amount         int64  `json:"amount"`
	CategoryId     int64  `json:"categoryId"`
	PaymentChannel int    `json:"paymentChannel"`
	TxTime         int64  `json:"txTime"`
	TimezoneOffset int    `json:"timezoneOffset"`
	Comment        string `json:"comment"`
	Source         int    `json:"source"`
}

// CreateResult 创建结果（幂等重放返回同一组 ID 与原创建版本）
type CreateResult struct {
	ResultType     int8    `json:"resultType"`     // 1=单笔 2=转账
	TransactionIds []int64 `json:"transactionIds"` // 转账为 [转出ID, 转入ID]
	Version        int64   `json:"version"`
	Replayed       bool    `json:"replayed"`
}

// TxListQuery 交易列表查询条件（全部可选）
type TxListQuery struct {
	MaxTime    int64
	MinTime    int64
	TxType     int8
	AccountId  int64
	CategoryId int64
	Keyword    string
	Page       int
	Count      int
}

// TxService 交易服务（唯一创建入口）
type TxService struct {
	engine   *xorm.Engine
	versions version.Source
}

// NewTxService 构造交易服务（版本号经 VersionSource 注入，禁止另写版本递增 SQL）
func NewTxService(engine *xorm.Engine, versions version.Source) *TxService {
	return &TxService{engine: engine, versions: versions}
}

// Create 唯一公开的创建入口：收入 / 支出 / 转账 / 离线补传全部走这里。
//
// 一次业务命令只占位一次幂等回执、只开启一个事务、只递增一次账本版本。
func (s *TxService) Create(ctx context.Context, uid, bookID int64, req CreateRequest) (*CreateResult, error) {
	r := normalizeCreate(req)
	if err := validateCreate(uid, bookID, r); err != nil {
		return nil, err
	}
	hash, err := createRequestHash(r)
	if err != nil {
		return nil, err
	}
	kind := resultTypeOf(r)

	sess := s.engine.NewSession()
	defer sess.Close()
	if ctx != nil {
		sess.Context(ctx)
	}
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	// Commit 成功后 Rollback 无害；它负责释放每一条失败路径
	defer func() { _ = sess.Rollback() }()

	// 1) 成员资格复查（服务端权威判定，不信任请求体）
	if err := requireMemberTx(sess, bookID, uid); err != nil {
		if errors.Is(err, ErrNotMember) {
			return nil, ErrCreateForbidden
		}
		return nil, err
	}

	// 2) 幂等占位：只吞掉 (book_id,uid,op,client_uuid) 唯一键冲突，其余错误直接失败并回滚
	res, err := sess.Exec(`INSERT INTO idempotency_records
		(book_id, uid, client_uuid, op, request_hash, result_type, result_ids, result_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, '', 0, ?)
		ON CONFLICT(book_id, uid, op, client_uuid) DO NOTHING`,
		bookID, uid, r.ClientUuid, model.IdemOpCreate, hash, kind, time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("写入幂等回执占位失败：%w", err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if inserted == 0 {
		// 同键命中：重读完整回执并核对，一致返回原结果，不一致报冲突
		return s.replayReceipt(sess, bookID, uid, r.ClientUuid, model.IdemOpCreate, hash, kind)
	}
	if inserted != 1 {
		return nil, ErrIncompleteReceipt
	}

	// 3) 账户 / 分类校验（同一事务内读，保证与余额写入看到同一快照）
	from, err := loadTxAccount(sess, bookID, r.AccountId)
	if err != nil {
		return nil, err
	}
	var to *model.Account
	if r.TxType == int(model.TxTypeTransfer) {
		to, err = loadTxAccount(sess, bookID, r.ToAccountId)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(from.Currency, to.Currency) {
			return nil, fmt.Errorf("%w：MVP 不支持跨币种转账", ErrInvalidCreate)
		}
	} else if err := loadCategoryForType(sess, bookID, r.CategoryId, int8(r.TxType)); err != nil {
		return nil, err
	}

	// 4) 唯一一次版本递增：转账的两条记录与两侧账户共用该版本
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	if ver <= 0 {
		return nil, ErrIncompleteReceipt
	}

	var ids []int64
	if r.TxType == int(model.TxTypeTransfer) {
		ids, err = s.createTransferInTx(sess, uid, bookID, r, from, to, ver)
	} else {
		var id int64
		id, err = s.createSingleInTx(sess, uid, bookID, r, from, ver)
		ids = []int64{id}
	}
	if err != nil {
		return nil, err
	}

	// 5) 回写完整回执（result_ids + result_version）；影响行数不是 1 即整笔回滚
	res, err = sess.Exec(`UPDATE idempotency_records SET result_ids = ?, result_version = ?
		WHERE book_id = ? AND uid = ? AND op = ? AND client_uuid = ? AND request_hash = ? AND result_type = ?`,
		encodeResultIDs(ids), ver, bookID, uid, model.IdemOpCreate, r.ClientUuid, hash, kind)
	if err != nil {
		return nil, fmt.Errorf("回写幂等回执失败：%w", err)
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if updated != 1 {
		return nil, ErrIncompleteReceipt
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &CreateResult{ResultType: kind, TransactionIds: ids, Version: ver, Replayed: false}, nil
}

// replayReceipt 同键命中后的重放分支：重读完整回执并核对操作类型与完整请求摘要。
//
// 回执不完整（result_type=0 / result_ids 缺失 / result_version<=0）不是成功；
// 内容或类型不一致返回 ErrCreateConflict（409）。
func (s *TxService) replayReceipt(sess *xorm.Session, bookID, uid int64, clientUUID, op, hash string, kind int8) (*CreateResult, error) {
	rec := &model.IdempotencyRecord{}
	has, err := sess.SQL(
		`SELECT id, book_id, uid, client_uuid, op, request_hash, result_ids, result_type, result_version, created_at
		 FROM idempotency_records WHERE book_id = ? AND uid = ? AND op = ? AND client_uuid = ?`,
		bookID, uid, op, clientUUID).Get(rec)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrIncompleteReceipt
	}
	if rec.RequestHash != hash || rec.ResultType != kind {
		return nil, ErrCreateConflict
	}
	ids, err := decodeResultIDs(rec.ResultIds, kind)
	if err != nil || rec.ResultVersion <= 0 {
		// 死结：占位已提交但结果未回写（正常路径不会发生，因为占位与回写在同一事务）。
		// 重试同一个 clientUuid 只会不断命中同一条不完整回执，因此不能提示"重试"，
		// 必须如实告知需要人工介入；自动接管/补全属于阶段 4 的删除恢复设计范畴。
		log.Printf("幂等回执不完整：book_id=%d uid=%d client_uuid=%q result_type=%d result_ids=%q result_version=%d",
			bookID, uid, clientUUID, rec.ResultType, rec.ResultIds, rec.ResultVersion)
		return nil, fmt.Errorf("%w：该 clientUuid 已有不完整回执，请更换 clientUuid 或联系实例管理员处理",
			ErrIncompleteReceipt)
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &CreateResult{ResultType: kind, TransactionIds: ids, Version: rec.ResultVersion, Replayed: true}, nil
}

// createSingleInTx 收入 / 支出：改一侧余额 + 写一条记录（同一事务同一版本）。
func (s *TxService) createSingleInTx(sess *xorm.Session, uid, bookID int64, r CreateRequest, account *model.Account, ver int64) (int64, error) {
	// flow 为【签名现金流】：流出为负、流入为正；负债账户的存储 delta 需取反
	flow := r.Amount
	if int8(r.TxType) == model.TxTypeExpense {
		flow = -flow
	}
	if err := createChangeBalance(sess, bookID, account.Id, signedBalanceDelta(account.Category, flow), ver); err != nil {
		return 0, err
	}
	row := newTxRow(uid, bookID, r, ver)
	row.AccountId = account.Id
	row.CategoryId = r.CategoryId
	if err := insertTxRow(sess, row); err != nil {
		return 0, err
	}
	// 审计（WBS 4.3）：create 无"变更前"状态，before_json 留空
	if err := insertAudit(sess, bookID, uid, model.AuditActionCreate, "transaction", row.Id, nil, "server"); err != nil {
		return 0, err
	}
	return row.Id, nil
}

// createTransferInTx 转账分支：两侧余额 + 两条记录同生共死，不另开事务、不再递增版本。
func (s *TxService) createTransferInTx(sess *xorm.Session, uid, bookID int64, r CreateRequest, from, to *model.Account, ver int64) ([]int64, error) {
	// 转出：现金流为负；转入：现金流为正。负债账户经 signedBalanceDelta 取反
	if err := createChangeBalance(sess, bookID, from.Id, signedBalanceDelta(from.Category, -r.Amount), ver); err != nil {
		return nil, err
	}
	if err := createChangeBalance(sess, bookID, to.Id, signedBalanceDelta(to.Category, r.Amount), ver); err != nil {
		return nil, err
	}

	out := newTxRow(uid, bookID, r, ver)
	out.AccountId = from.Id
	out.ToAccountId = to.Id
	out.Direction = model.TxDirectionOut // 转出侧：资金离场，余额减少
	if err := insertTxRow(sess, out); err != nil {
		return nil, err
	}
	// 转入记录的 creator_uid 与转出记录一致：整笔转账归属同一个人
	in := newTxRow(uid, bookID, r, ver)
	in.AccountId = to.Id
	in.ToAccountId = from.Id
	in.RelatedId = out.Id
	in.Direction = model.TxDirectionIn // 转入侧：资金入场，余额增加
	if err := insertTxRow(sess, in); err != nil {
		return nil, err
	}
	// 转出记录的 related_id 指回转入记录，形成双向配对键（阶段 4 整笔删除/恢复依赖）
	res, err := sess.Exec(
		`UPDATE transactions SET related_id = ? WHERE id = ? AND book_id = ? AND deleted = 0`,
		in.Id, out.Id, bookID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrIncompleteReceipt
	}
	out.RelatedId = in.Id
	// 审计（WBS 4.3）：转账两侧各留一条 create
	for _, r2 := range []*model.Transaction{out, in} {
		if err := insertAudit(sess, bookID, uid, model.AuditActionCreate, "transaction", r2.Id, nil, "server"); err != nil {
			return nil, err
		}
	}
	return []int64{out.Id, in.Id}, nil
}

// signedBalanceDelta 把【签名现金流】转换为账户存储余额的增量。
//
// 资产（1~9）：delta = flow（消费减少、收入增加）；
// 负债（10~15）：balance 正=欠款，故 delta = -flow（消费欠更多、还款欠更少）。
func signedBalanceDelta(category int8, flow int64) int64 {
	if model.IsLiabilityCategory(category) {
		return -flow
	}
	return flow
}

// createChangeBalance 余额变动的唯一实现：单条原子 UPDATE。
//
// 必须带 book_id + deleted=0，并校验 RowsAffected==1；BETWEEN 上下界用于拒绝溢出。
// 禁止"先读余额再写回"，否则并发下会丢更新。
//
// 影响行数为 0 时必须区分两种情况，否则会把"金额溢出"错误地当成"可重试的并发冲突"：
//   - 账户仍存在且未删除 → 金额越过 int64 可表示范围，返回 ErrInvalidCreate(400)，重试永远无意义；
//   - 账户不存在/已删除 → 返回 ErrConcurrentConflict(409)，调用方可按同一 clientUuid 重试。
func createChangeBalance(sess *xorm.Session, bookID, accountID, delta, ver int64) error {
	low, high := int64(math.MinInt64), int64(math.MaxInt64)
	if delta > 0 {
		high -= delta
	}
	if delta < 0 {
		low -= delta
	}
	res, err := sess.Exec(`UPDATE accounts SET balance = balance + ?, sync_version = ?, updated_at = ?
		WHERE id = ? AND book_id = ? AND deleted = 0 AND balance BETWEEN ? AND ?`,
		delta, ver, time.Now().Unix(), accountID, bookID, low, high)
	if err != nil {
		return fmt.Errorf("更新账户余额失败（account_id=%d）：%w", accountID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	if n != 0 {
		return fmt.Errorf("%w：账户 %d 余额更新影响 %d 行", ErrConcurrentConflict, accountID, n)
	}

	// 影响 0 行：再确认账户是否仍存在（同一事务内读自己的写，看到的是最新状态）
	var probe model.Account
	has, perr := sess.Where("id = ? AND book_id = ?", accountID, bookID).Get(&probe)
	if perr != nil {
		return perr
	}
	if has && !probe.Deleted {
		return fmt.Errorf("%w：金额超出可表示范围（账户 %d 当前余额 %d，本次变动 %d）",
			ErrInvalidCreate, accountID, probe.Balance, delta)
	}
	return fmt.Errorf("%w：账户 %d 已删除或并发修改，余额未更新", ErrConcurrentConflict, accountID)
}

// newTxRow 构造交易行（转账的两条记录仅在 account_id / to_account_id / related_id 上不同）。
//
// creatorUid 必须落库：PRD §7.3 可见性过滤与阶段 4「member 仅能改删本人交易」
// 都靠它判定归属；留 0 会让审计与权限建立在错误数据上。
func newTxRow(creatorUid, bookID int64, r CreateRequest, ver int64) *model.Transaction {
	return &model.Transaction{
		BookId:         bookID,
		CreatorUid:     creatorUid,
		TxType:         int8(r.TxType),
		Amount:         r.Amount,
		CategoryId:     r.CategoryId,
		PaymentChannel: int8(r.PaymentChannel),
		Note:           r.Comment,
		TxTime:         r.TxTime,
		TimezoneOffset: r.TimezoneOffset,
		Source:         int8(r.Source),
		ClientUuid:     r.ClientUuid,
		SyncVersion:    ver,
	}
}

// insertTxRow 插入一条交易并确认影响行数与自增主键
func insertTxRow(sess *xorm.Session, row *model.Transaction) error {
	n, err := sess.Insert(row)
	if err != nil {
		return err
	}
	if n != 1 || row.Id <= 0 {
		return ErrIncompleteReceipt
	}
	return nil
}

// loadTxAccount 读取本账本内未删除的账户；MVP 仅支持 CNY 账户
func loadTxAccount(sess *xorm.Session, bookID, accountID int64) (*model.Account, error) {
	a := &model.Account{}
	// 不过滤 deleted：必须把"从未存在"与"已删除"区分开（WBS 4.5）。
	// 若继续用 deleted=0 过滤，离线补传引用已删账户时只能报"不存在"，
	// 客户端无法据此判断该丢弃还是另存，也无法阻止已删数据被静默复活。
	has, err := sess.Where("id = ? AND book_id = ?", accountID, bookID).Get(a)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("%w：账户 %d 不存在", ErrAccountNotFound, accountID)
	}
	if a.Deleted {
		return nil, fmt.Errorf("%w：账户 %d 已被删除", ErrEntityDeleted, accountID)
	}
	if !model.IsValidAccountCategory(a.Category) {
		return nil, fmt.Errorf("%w：账户 %d 分类非法", ErrInvalidCreate, accountID)
	}
	if !strings.EqualFold(a.Currency, "CNY") {
		return nil, fmt.Errorf("%w：MVP 仅支持 CNY 账户", ErrInvalidCreate)
	}
	return a, nil
}

// loadCategoryForType 校验分类：属于本账本、未删除、且 cat_type 与交易类型一致
func loadCategoryForType(sess *xorm.Session, bookID, categoryID int64, txType int8) error {
	var c model.Category
	// 同 loadTxAccount：区分"从未存在"与"已删除"（WBS 4.5）
	has, err := sess.Where("id = ? AND book_id = ?", categoryID, bookID).Get(&c)
	if err != nil {
		return err
	}
	if !has {
		return fmt.Errorf("%w：分类 %d 不存在", ErrCategoryNotFound, categoryID)
	}
	if c.Deleted {
		return fmt.Errorf("%w：分类 %d 已被删除", ErrEntityDeleted, categoryID)
	}
	if c.CatType != txType {
		return fmt.Errorf("%w：分类收支类型与交易类型不一致", ErrInvalidCreate)
	}
	return nil
}

// List 交易列表（分页 + 过滤）；服务端先复核成员资格与可见性（PRD §7.3）。
//
// 读路径必须与 Pull / Get 一致地叠加可见性条件：关闭互看时 member 只能看到本人记录，
// 否则"列表接口"会成为绕过可见性开关的后门（服务端权威，不能靠前端隐藏）。
func (s *TxService) List(uid, bookID int64, q TxListQuery) ([]model.Transaction, int64, error) {
	canViewAll, err := canViewAllForTx(s.engine, uid, bookID)
	if err != nil {
		return nil, 0, err
	}
	where := []string{"book_id = ?", "deleted = 0"}
	args := []interface{}{bookID}
	// 可见性条件同时进入 COUNT 与列表查询，保证 total 与实际可见条数一致
	if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
		where = append(where, vcond)
		args = append(args, vargs...)
	}
	if q.MaxTime > 0 {
		where = append(where, "tx_time <= ?")
		args = append(args, q.MaxTime)
	}
	if q.MinTime > 0 {
		where = append(where, "tx_time >= ?")
		args = append(args, q.MinTime)
	}
	if q.TxType > 0 {
		// type 是 SQLite 的非保留关键字，但显式加引号更稳妥
		where = append(where, `"type" = ?`)
		args = append(args, q.TxType)
	}
	if q.AccountId > 0 {
		where = append(where, "(account_id = ? OR to_account_id = ?)")
		args = append(args, q.AccountId, q.AccountId)
	}
	if q.CategoryId > 0 {
		where = append(where, "category_id = ?")
		args = append(args, q.CategoryId)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, `(note LIKE ? ESCAPE '\' OR merchant LIKE ? ESCAPE '\')`)
		pattern := likePattern(kw)
		args = append(args, pattern, pattern)
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if _, err := s.engine.SQL("SELECT COUNT(1) FROM transactions WHERE "+cond, args...).Get(&total); err != nil {
		return nil, 0, err
	}

	page, count := normalizePaging(q.Page, q.Count)
	listSQL := "SELECT * FROM transactions WHERE " + cond + " ORDER BY tx_time DESC, id DESC LIMIT ? OFFSET ?"
	listArgs := append(append([]interface{}{}, args...), count, (page-1)*count)
	out := make([]model.Transaction, 0, count)
	if err := s.engine.SQL(listSQL, listArgs...).Find(&out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Get 交易详情（可见性不可见时返回 ErrTransactionNotFound → HTTP 404）。
//
// 为什么是 404 而不是 403：403 会泄露"这笔记录确实存在，只是你无权查看"，
// 相当于把关闭互看下的他人记录存在性暴露给 member。404 与"记录不存在"不可区分，
// 是可见性过滤场景唯一不泄露信息的选择。
func (s *TxService) Get(uid, bookID, txID int64) (*model.Transaction, error) {
	canViewAll, err := canViewAllForTx(s.engine, uid, bookID)
	if err != nil {
		return nil, err
	}
	where := "id = ? AND book_id = ? AND deleted = 0"
	args := []interface{}{txID, bookID}
	if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
		where += " AND " + vcond
		args = append(args, vargs...)
	}
	var tx model.Transaction
	has, err := s.engine.Where(where, args...).Get(&tx)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrTransactionNotFound
	}
	return &tx, nil
}

// normalizeCreate 归一化命令：去空白，保证重试时摘要稳定（不改变业务语义）
func normalizeCreate(r CreateRequest) CreateRequest {
	r.ClientUuid = strings.TrimSpace(r.ClientUuid)
	r.Comment = strings.TrimSpace(r.Comment)
	return r
}

// validateCreate 服务层强制校验（不依赖 handler）。
//
// clientUuid 缺失/全空白即拒绝：空键会被当作"每次都新建"，破坏幂等。
func validateCreate(uid, bookID int64, r CreateRequest) error {
	if uid <= 0 || bookID <= 0 {
		return fmt.Errorf("%w：用户或账本缺失", ErrInvalidCreate)
	}
	if r.ClientUuid == "" || len(r.ClientUuid) > maxClientUUIDLen {
		return fmt.Errorf("%w：clientUuid 必填且不超过 %d 字节", ErrInvalidCreate, maxClientUUIDLen)
	}
	if r.Amount <= 0 || r.AccountId <= 0 || r.TxTime <= 0 {
		return fmt.Errorf("%w：金额必须为正整数分，账户与发生时间必填", ErrInvalidCreate)
	}
	if r.Amount > maxAmountCents {
		return fmt.Errorf("%w：金额超出上限（单笔最多 %d 分）", ErrInvalidCreate, maxAmountCents)
	}
	if r.TxType < int(model.TxTypeIncome) || r.TxType > int(model.TxTypeTransfer) {
		return fmt.Errorf("%w：txType 仅支持 1=收入 2=支出 3=转账", ErrInvalidCreate)
	}
	if r.PaymentChannel < int(model.PaymentChannelDirect) || r.PaymentChannel > int(model.PaymentChannelOther) {
		return fmt.Errorf("%w：paymentChannel 取值超出范围", ErrInvalidCreate)
	}
	// Source 仅开放 0=手动 1=识别；2=导入为预留值（枚举字典 §10）
	if r.Source < int(model.SourceManual) || r.Source > int(model.SourceRecognition) {
		return fmt.Errorf("%w：source 取值超出范围", ErrInvalidCreate)
	}
	if r.TimezoneOffset < -840 || r.TimezoneOffset > 840 {
		return fmt.Errorf("%w：timezoneOffset 取值超出范围", ErrInvalidCreate)
	}
	if !utf8.ValidString(r.Comment) || utf8.RuneCountInString(r.Comment) > maxCommentRunes {
		return fmt.Errorf("%w：备注必须为合法 UTF-8 且不超过 %d 字", ErrInvalidCreate, maxCommentRunes)
	}
	if r.TxType == int(model.TxTypeTransfer) {
		if r.ToAccountId <= 0 || r.ToAccountId == r.AccountId || r.CategoryId != 0 {
			return fmt.Errorf("%w：转账必须指定不同的对手账户，且不带分类", ErrInvalidCreate)
		}
		return nil
	}
	if r.ToAccountId != 0 || r.CategoryId <= 0 {
		return fmt.Errorf("%w：收入/支出必须指定分类，且不带对手账户", ErrInvalidCreate)
	}
	return nil
}

// createRequestHash 完整请求摘要：带域标签的确定性编码，排除操作键与服务端生成字段。
//
// 分类、渠道、备注、时间等已接受的业务字段全部参与；只用金额做摘要会导致
// "改了分类/渠道仍被判为重放"（GPT-R20）。
func createRequestHash(r CreateRequest) (string, error) {
	payload := struct {
		Domain  string        `json:"domain"`
		Request CreateRequest `json:"request"`
	}{
		Domain:  txHashDomain,
		Request: r,
	}
	// 操作键只用于定位回执，不属于业务身份
	payload.Request.ClientUuid = ""
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("编码幂等请求摘要失败：%w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// resultTypeOf 交易类型 → 幂等结果类型（枚举字典 §12）
func resultTypeOf(r CreateRequest) int8 {
	if r.TxType == int(model.TxTypeTransfer) {
		return model.IdempotencyResultTransfer
	}
	return model.IdempotencyResultSingle
}

// encodeResultIDs 把交易 ID 列表编码为 result_ids（"id" 或 "outId,inId"）
func encodeResultIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

// decodeResultIDs 解码并校验回执结果：单笔 1 个 ID、转账 2 个不同 ID
func decodeResultIDs(value string, resultType int8) ([]int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrIncompleteReceipt
	}
	parts := strings.Split(value, ",")
	want := 1
	if resultType == model.IdempotencyResultTransfer {
		want = 2
	}
	if len(parts) != want {
		return nil, ErrIncompleteReceipt
	}
	ids := make([]int64, want)
	for i, p := range parts {
		v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || v <= 0 {
			return nil, ErrIncompleteReceipt
		}
		ids[i] = v
	}
	if want == 2 && ids[0] == ids[1] {
		return nil, ErrIncompleteReceipt
	}
	return ids, nil
}

// NormalizePaging 对外暴露的分页归一化：handler 必须用它的返回值回显 page/count，
// 否则响应里会出现"请求 count=1000、实际只返回 200"的不一致。
func NormalizePaging(page, count int) (int, int) {
	return normalizePaging(page, count)
}

// normalizePaging 分页边界（默认 20，上限 200），杜绝 count=0 或超大页
func normalizePaging(page, count int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if count <= 0 {
		count = defaultTxPageSize
	}
	if count > maxTxPageSize {
		count = maxTxPageSize
	}
	return page, count
}

// likePattern 转义 LIKE 通配符，避免用户输入的 % / _ 变成通配
func likePattern(kw string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(kw) + "%"
}

// ---- 阶段 4：删除 / 恢复 / 审计 ----

// Delete 删除一笔交易（整笔，含转账对侧）：软删 + 余额精确反向 + 版本递增 + 审计。
//
// 权限（PRD §7.3 / 落地方案 §5.5 修订版）：owner/admin 可删任意；member 仅本人
// （creator_uid==uid，且 creator_uid=0 的遗留记录只允许 owner/admin）。删除"整笔"——
// 若 txID 指向转账的一侧，对侧（related_id 配对）一并处理，禁止单边路径（WBS 4.2）。
//
// 余额反向依赖 transactions.direction（0004 补入）：转出侧 flow=-amount、转入侧
// flow=+amount；单笔按 type 判定。createChangeBalance 复用同一原子 UPDATE，
// 全组共享一个递增版本（WBS 4.6：单一事务边界）。
func (s *TxService) Delete(ctx context.Context, uid, bookID, txID int64, clientUuid string) (int64, error) {
	return s.mutateVisibility(ctx, uid, bookID, txID, clientUuid, true, model.AuditActionDelete, model.IdemOpDelete)
}

// Restore 恢复一笔已软删的交易（整笔）。与 Delete 对称：余额回滚到删除前、deleted 复位、
// 版本递增、写审计，复用同一事务与版本来源（WBS 4.4）。
func (s *TxService) Restore(ctx context.Context, uid, bookID, txID int64, clientUuid string) (int64, error) {
	return s.mutateVisibility(ctx, uid, bookID, txID, clientUuid, false, model.AuditActionRestore, model.IdemOpRestore)
}

// mutateVisibility Delete/Restore 的统一实现。wantDeleted=true 表示本次要把目标置为"已删"。
//
// clientUuid 必填（服务层强制，不依赖 handler）：删除/恢复若失去幂等，客户端离线重试
// 会把余额反向两次。回执与 Create 共用 idempotency_records，靠 op 列区分（迁移 0005）。
func (s *TxService) mutateVisibility(ctx context.Context, uid, bookID, txID int64, clientUuid string, wantDeleted bool, action, op string) (int64, error) {
	clientUuid = strings.TrimSpace(clientUuid)
	if uid <= 0 || bookID <= 0 || txID <= 0 {
		return 0, fmt.Errorf("%w：用户/账本/交易 ID 缺失", ErrInvalidArgument)
	}
	if clientUuid == "" || len(clientUuid) > maxClientUUIDLen {
		return 0, fmt.Errorf("%w：clientUuid 必填且不超过 %d 字节", ErrInvalidArgument, maxClientUUIDLen)
	}
	hash := mutateRequestHash(op, bookID, uid, txID)

	// 权限：非成员直接失败；member 仅能操作本人（creator_uid 哨兵：0 视为仅 owner/admin 可见/可改）
	role, err := RoleOf(s.engine, bookID, uid)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			return 0, ErrForbidden
		}
		return 0, err
	}
	canEditOthers := CanEditOthersTx(role)

	sess := s.engine.NewSession()
	defer sess.Close()
	if ctx != nil {
		sess.Context(ctx)
	}
	if err := sess.Begin(); err != nil {
		return 0, err
	}
	defer func() { _ = sess.Rollback() }()

	// 1) 幂等占位（与 Create 对称）：同键命中即重放原回执，绝不重复反转余额。
	// 失败路径（404/无权限）会整体回滚，占位一并撤销，客户端可用同一键重试。
	now := time.Now().Unix()
	res, err := sess.Exec(`INSERT INTO idempotency_records
		(book_id, uid, client_uuid, op, request_hash, result_type, result_ids, result_version, created_at)
		VALUES (?, ?, ?, ?, ?, 0, '', 0, ?)
		ON CONFLICT(book_id, uid, op, client_uuid) DO NOTHING`,
		bookID, uid, clientUuid, op, hash, now)
	if err != nil {
		return 0, fmt.Errorf("写入幂等回执占位失败：%w", err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if inserted == 0 {
		return replayMutateReceipt(sess, bookID, uid, clientUuid, op, hash)
	}
	if inserted != 1 {
		return 0, ErrIncompleteReceipt
	}

	// 2) 目标当前必须处于与操作相反的删除态（删前须存活、恢复前须已删），否则 404
	row, err := loadTxRowByState(sess, bookID, txID, wantDeleted)
	if err != nil {
		return 0, err
	}
	group, err := collectTransferGroup(sess, bookID, row, wantDeleted)
	if err != nil {
		return 0, err
	}
	for _, r := range group {
		if !canEditOthers && r.CreatorUid != uid {
			return 0, ErrForbidden
		}
	}

	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return 0, err
	}

	for _, r := range group {
		// 失败关闭：转账若缺方向标记，无法判定该侧是转出还是转入，
		// 余额反向会静默算成 0（钱不回滚却不报错）。迁移 0006 负责回填。
		if r.TxType == model.TxTypeTransfer && r.Direction == model.TxDirectionSingle {
			return 0, fmt.Errorf("%w：转账记录 %d 缺少方向标记，无法精确反向余额", ErrTxDirectionMissing, r.Id)
		}
		acct, err := loadTxAccount(sess, bookID, r.AccountId)
		if err != nil {
			return 0, err
		}
		// Create 对该账户施加的 delta；删除反向（取负），恢复再施加（取正）
		apply := signedBalanceDelta(acct.Category, txRowFlow(r))
		if wantDeleted {
			apply = -apply
		}
		if err := createChangeBalance(sess, bookID, r.AccountId, apply, ver); err != nil {
			return 0, err
		}
		newDeleted := 0
		if wantDeleted {
			newDeleted = 1
		}
		res, err := sess.Exec(
			`UPDATE transactions SET deleted = ?, deleted_at = ?, sync_version = ?
			 WHERE id = ? AND book_id = ? AND deleted = ?`,
			newDeleted, now, ver, r.Id, bookID, boolToInt(!wantDeleted))
		if err != nil {
			return 0, fmt.Errorf("更新交易删除状态失败：%w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		if n != 1 {
			return 0, ErrIncompleteReceipt
		}
		// 审计：before_json 记录变更前状态（此刻 r 仍是变更前快照）
		if err := insertAudit(sess, bookID, uid, action, "transaction", r.Id, r, "server"); err != nil {
			return 0, err
		}
	}

	// 3) 回写完整回执（result_ids / result_type / result_version）；影响行数不是 1 即整笔回滚。
	// result_type 复用 Single/Transfer 语义：单笔 1 个 ID、整笔转账 2 个 ID，
	// 重放时 decodeResultIDs 才能按预期个数校验。
	kind := model.IdempotencyResultSingle
	if len(group) == 2 {
		kind = model.IdempotencyResultTransfer
	}
	ids := make([]int64, 0, len(group))
	for _, r := range group {
		ids = append(ids, r.Id)
	}
	res, err = sess.Exec(`UPDATE idempotency_records
		SET result_ids = ?, result_type = ?, result_version = ?
		WHERE book_id = ? AND uid = ? AND op = ? AND client_uuid = ? AND request_hash = ?`,
		encodeResultIDs(ids), kind, ver, bookID, uid, op, clientUuid, hash)
	if err != nil {
		return 0, fmt.Errorf("回写幂等回执失败：%w", err)
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if updated != 1 {
		return 0, ErrIncompleteReceipt
	}

	if err := sess.Commit(); err != nil {
		return 0, err
	}
	return ver, nil
}

// replayMutateReceipt 删除/恢复的同键重放：核对请求摘要后返回原版本，不重复反转余额。
func replayMutateReceipt(sess *xorm.Session, bookID, uid int64, clientUuid, op, hash string) (int64, error) {
	rec := &model.IdempotencyRecord{}
	has, err := sess.SQL(
		`SELECT request_hash, result_ids, result_type, result_version
		 FROM idempotency_records WHERE book_id = ? AND uid = ? AND op = ? AND client_uuid = ?`,
		bookID, uid, op, clientUuid).Get(rec)
	if err != nil {
		return 0, err
	}
	if !has {
		return 0, ErrIncompleteReceipt
	}
	if rec.RequestHash != hash {
		return 0, ErrCreateConflict
	}
	if _, err := decodeResultIDs(rec.ResultIds, rec.ResultType); err != nil || rec.ResultVersion <= 0 {
		log.Printf("幂等回执不完整：book_id=%d uid=%d op=%q client_uuid=%q result_type=%d result_ids=%q result_version=%d",
			bookID, uid, op, clientUuid, rec.ResultType, rec.ResultIds, rec.ResultVersion)
		return 0, fmt.Errorf("%w：该 clientUuid 已有不完整回执，请更换 clientUuid 或联系实例管理员处理",
			ErrIncompleteReceipt)
	}
	if err := sess.Commit(); err != nil {
		return 0, err
	}
	return rec.ResultVersion, nil
}

// mutateRequestHash 删除/恢复的请求摘要。clientUuid 不参与（它只是定位回执的键）。
func mutateRequestHash(op string, bookID, uid, txID int64) string {
	payload := struct {
		Domain string `json:"domain"`
		Op     string `json:"op"`
		BookID int64  `json:"bookId"`
		Uid    int64  `json:"uid"`
		TxID   int64  `json:"txId"`
	}{txMutateHashDomain, op, bookID, uid, txID}
	b, _ := json.Marshal(payload) // 仅字符串与整数字段，不会失败
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// loadTxRowByState 按"是否已删除"加载交易行；状态不符返回 ErrTransactionNotFound（→HTTP 404）。
func loadTxRowByState(sess *xorm.Session, bookID, txID int64, expectLive bool) (*model.Transaction, error) {
	deleted := 0
	if !expectLive {
		deleted = 1
	}
	var r model.Transaction
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = ?", txID, bookID, deleted).Get(&r)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrTransactionNotFound
	}
	return &r, nil
}

// collectTransferGroup 收集"整笔"：转账（related_id>0）返回两侧；否则仅自身。
// wantLive 表示对侧也需处于相同删除态（删除时找未删对侧，恢复时找已删对侧）。
func collectTransferGroup(sess *xorm.Session, bookID int64, row *model.Transaction, wantLive bool) ([]*model.Transaction, error) {
	if row.TxType != model.TxTypeTransfer || row.RelatedId <= 0 {
		return []*model.Transaction{row}, nil
	}
	deleted := 0
	if !wantLive {
		deleted = 1
	}
	var counterpart model.Transaction
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = ?", row.RelatedId, bookID, deleted).Get(&counterpart)
	if err != nil {
		return nil, err
	}
	if !has {
		// 对侧已不在预期状态：仍按"整笔"处理存在的这一侧，避免悬空配对
		return []*model.Transaction{row}, nil
	}
	return []*model.Transaction{row, &counterpart}, nil
}

// txRowFlow 该记录在 Create 时对其所在账户的现金流符号（收入为正、支出为负）。
// 转账靠 direction 区分转出（本侧 -amount）/ 转入（本侧 +amount）；单笔按 type 判定。
func txRowFlow(r *model.Transaction) int64 {
	switch r.Direction {
	case model.TxDirectionOut:
		return -r.Amount
	case model.TxDirectionIn:
		return r.Amount
	default: // 单笔
		if r.TxType == model.TxTypeIncome {
			return r.Amount
		}
		if r.TxType == model.TxTypeExpense {
			return -r.Amount
		}
		return 0
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// insertAudit 写入一条审计日志（append-only）。before 为变更前实体的序列化；
// create 没有"变更前"状态，传 nil 即 before_json 留空（实体本身仍在库里可查）。
func insertAudit(sess *xorm.Session, bookID, actorUid int64, action, entityType string, entityID int64, before interface{}, source string) error {
	var beforeJSON string
	if before != nil {
		b, err := json.Marshal(before)
		if err != nil {
			return fmt.Errorf("审计 before_json 序列化失败：%w", err)
		}
		beforeJSON = string(b)
	}
	a := &model.AuditLog{
		BookId:     bookID,
		ActorUid:   actorUid,
		Action:     action,
		EntityType: entityType,
		EntityId:   entityID,
		BeforeJson: beforeJSON,
		At:         time.Now().Unix(),
		Source:     source,
	}
	if _, err := sess.Insert(a); err != nil {
		return fmt.Errorf("写入审计日志失败：%w", err)
	}
	return nil
}

// 编译期确保 versions 为版本唯一实现
var _ version.Source = version.Repo{}
