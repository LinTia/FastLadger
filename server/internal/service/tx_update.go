package service

// 交易更新（WBS 5.9b）——PUT /books/:id/transactions/:txId 的服务层实现。
//
// 设计依据：docs/FastLedger-交易更新语义设计.md（CI 守卫解除 PUT 禁令的前置裁决）。
// 要点：
//  1. 全量替换语义；txType 与既有行不一致 → 400（改类型 = 删除 + 重建，见设计 §2）；
//  2. 权限与 Delete 同口径：事务内重读角色，member 仅能改本人（GPT-R28）；
//  3. 转账整笔双侧联动，禁止单边更新（collectTransferGroup，GPT-R26）；
//  4. 余额净额重算：差额计算 + createChangeBalance 原子写入，净额为 0 的账户不碰；
//  5. 幂等 op=update（复用 idempotency_records，零迁移），摘要域 transaction.update/v1；
//  6. baseVersion 乐观锁（>0 时校验行 sync_version，过期 → 409）；
//  7. 每行一条 update 审计，before_json = 变更前整行快照。
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

// txUpdateHashDomain 更新请求摘要的域标签；与 create/mutate 域分离（设计 §7）。
const txUpdateHashDomain = "transaction.update/v1"

// UpdateRequest 全量替换命令（PUT 语义）。
//
// BaseVersion 是并发控制元数据而非业务内容：不参与 request_hash（与 clientUuid 同理），
// 否则 409 后客户端重拉重试会被误判为"同键不同内容"。
type UpdateRequest struct {
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
	BaseVersion    int64  `json:"baseVersion"`
}

// UpdateResult 更新结果（幂等重放返回同一版本与原行 ID）
type UpdateResult struct {
	Version        int64   `json:"version"`
	Replayed       bool    `json:"replayed"`
	TransactionIds []int64 `json:"transactionIds"`
}

// UpdateReceipt 更新一笔交易（转账整笔）。返回 replayed 供 HTTP 层决定是否广播（GPT-C04）。
func (s *TxService) UpdateReceipt(ctx context.Context, uid, bookID, txID int64, req UpdateRequest) (*UpdateResult, error) {
	r := normalizeUpdate(req)
	if err := validateUpdate(uid, bookID, txID, r); err != nil {
		return nil, err
	}
	hash, err := updateRequestHash(r)
	if err != nil {
		return nil, err
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if ctx != nil {
		sess.Context(ctx)
	}
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer func() { _ = sess.Rollback() }()

	// 1) 权限：写事务内重读角色（GPT-R28），与 Delete 完全同口径
	role, err := roleOfTx(sess, bookID, uid)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	canEditOthers := CanEditOthersTx(role)

	// 2) 幂等占位：op=update；同键命中即重放原回执（不产生任何写入）
	now := time.Now().Unix()
	res, err := sess.Exec(`INSERT INTO idempotency_records
		(book_id, uid, client_uuid, op, request_hash, result_type, result_ids, result_version, created_at)
		VALUES (?, ?, ?, ?, ?, 0, '', 0, ?)
		ON CONFLICT(book_id, uid, op, client_uuid) DO NOTHING`,
		bookID, uid, r.ClientUuid, model.IdemOpUpdate, hash, now)
	if err != nil {
		return nil, fmt.Errorf("写入幂等回执占位失败：%w", err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if inserted == 0 {
		return replayUpdateReceipt(sess, bookID, uid, r.ClientUuid, hash)
	}
	if inserted != 1 {
		return nil, ErrIncompleteReceipt
	}

	// 3) 目标必须存活；转账定位整笔（对侧缺失/配对损坏 → 整笔拒绝，GPT-R26）
	row, err := loadTxRowByState(sess, bookID, txID, true)
	if err != nil {
		return nil, err
	}
	// 类型不可改（设计 §2）：改类型 = 删除 + 重建
	if int8(r.TxType) != row.TxType {
		return nil, fmt.Errorf("%w：交易类型不可修改（当前 %s），如需变更请删除后重新记账",
			ErrInvalidCreate, model.TxTypeName(row.TxType))
	}
	group, err := collectTransferGroup(sess, bookID, row, true)
	if err != nil {
		return nil, err
	}
	for _, gr := range group {
		if !canEditOthers && gr.CreatorUid != uid {
			return nil, ErrForbidden
		}
	}

	// 4) baseVersion 乐观锁：整笔各行必须仍处于客户端所见的版本（设计 §6）
	if r.BaseVersion > 0 {
		for _, gr := range group {
			if gr.SyncVersion != r.BaseVersion {
				return nil, fmt.Errorf("%w：交易 %d 已被他人更新（行版本 %d，请求基于 %d），请刷新后重试",
					ErrConcurrentConflict, gr.Id, gr.SyncVersion, r.BaseVersion)
			}
		}
	}

	// 5) 校验并加载新引用（同一事务内读，与余额写入同一快照）
	fromAcct, err := loadTxAccount(sess, bookID, r.AccountId)
	if err != nil {
		return nil, err
	}
	var toAcct *model.Account
	if r.TxType == int(model.TxTypeTransfer) {
		if toAcct, err = loadTxAccount(sess, bookID, r.ToAccountId); err != nil {
			return nil, err
		}
		if !sameCurrency(fromAcct, toAcct) {
			return nil, fmt.Errorf("%w：MVP 不支持跨币种转账", ErrInvalidCreate)
		}
	} else if err := loadCategoryForType(sess, bookID, r.CategoryId, row.TxType); err != nil {
		return nil, err
	}

	// 6) 余额净额重算（设计 §5）：差额聚合，净额为 0 的账户不写
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	if ver <= 0 {
		return nil, ErrIncompleteReceipt
	}
	deltas, err := updateBalanceDeltas(sess, bookID, group, r, fromAcct, toAcct)
	if err != nil {
		return nil, err
	}
	for _, acctID := range sortedAcctIDs(deltas) {
		if d := deltas[acctID]; d != 0 {
			if err := createChangeBalance(sess, bookID, acctID, d, ver); err != nil {
				return nil, err
			}
		}
	}

	// 7) 更新交易行 + 审计（before_json = 变更前快照）
	ids := make([]int64, 0, len(group))
	for _, gr := range group {
		rowFrom, rowTo := rowAccountIDs(gr, fromAcct, toAcct)
		if err := updateTxRow(sess, bookID, gr, r, rowFrom, rowTo, ver, now); err != nil {
			return nil, err
		}
		if err := insertAudit(sess, bookID, uid, model.AuditActionUpdate, "transaction", gr.Id, gr, "server"); err != nil {
			return nil, err
		}
		ids = append(ids, gr.Id)
	}

	// 8) 回写完整回执；影响行数不是 1 即整笔回滚
	kind := model.IdempotencyResultSingle
	if len(group) == 2 {
		kind = model.IdempotencyResultTransfer
	}
	res, err = sess.Exec(`UPDATE idempotency_records SET result_ids = ?, result_type = ?, result_version = ?
		WHERE book_id = ? AND uid = ? AND op = ? AND client_uuid = ? AND request_hash = ?`,
		encodeResultIDs(ids), kind, ver, bookID, uid, model.IdemOpUpdate, r.ClientUuid, hash)
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
	return &UpdateResult{Version: ver, Replayed: false, TransactionIds: ids}, nil
}

// replayUpdateReceipt 更新的同键重放：核对请求摘要后返回原版本，不产生任何写入。
//
// 与 delete/restore 的 GPT-C01 不同，这里【不做终态校验】：更新重放没有任何写入，
// 不存在"重复反转余额"的风险；返回的是该次更新自身的回执版本（与 Create 重放同语义），
// 后续其他更新各持独立回执，客户端游标由 pull 水位驱动（设计 §7）。
func replayUpdateReceipt(sess *xorm.Session, bookID, uid int64, clientUuid, hash string) (*UpdateResult, error) {
	rec := &model.IdempotencyRecord{}
	has, err := sess.SQL(
		`SELECT request_hash, result_ids, result_type, result_version
		 FROM idempotency_records WHERE book_id = ? AND uid = ? AND op = ? AND client_uuid = ?`,
		bookID, uid, model.IdemOpUpdate, clientUuid).Get(rec)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrIncompleteReceipt
	}
	if rec.RequestHash != hash {
		return nil, ErrCreateConflict
	}
	ids, err := decodeResultIDs(rec.ResultIds, rec.ResultType)
	if err != nil || rec.ResultVersion <= 0 {
		return nil, fmt.Errorf("%w：该 clientUuid 已有不完整回执，请更换 clientUuid 或联系实例管理员处理",
			ErrIncompleteReceipt)
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &UpdateResult{Version: rec.ResultVersion, Replayed: true, TransactionIds: ids}, nil
}

// updateBalanceDeltas 计算"新内容 − 旧内容"的按账户净额（设计 §5）。
//
// 旧行的施加量依赖其账户分类（负债取反），故按行加载旧账户；
// 加载失败（账户已删/不存在）说明数据不一致，整笔拒绝而非静默算 0。
func updateBalanceDeltas(sess *xorm.Session, bookID int64, group []*model.Transaction, r UpdateRequest, fromAcct, toAcct *model.Account) (map[int64]int64, error) {
	deltas := map[int64]int64{}
	add := func(acctID int64, v int64) { deltas[acctID] += v }

	for _, gr := range group {
		oldAcct, err := loadTxAccount(sess, bookID, gr.AccountId)
		if err != nil {
			return nil, fmt.Errorf("交易 %d 引用的旧账户不可用，无法安全重算余额：%w", gr.Id, err)
		}
		oldFlow, err := txRowFlow(gr)
		if err != nil {
			return nil, err
		}
		// 旧施加量（回退）：Create 当时对该账户的 delta 取负
		add(gr.AccountId, -signedBalanceDelta(oldAcct.Category, oldFlow))

		// 新施加量
		if gr.TxType == model.TxTypeTransfer {
			// 转账：转出行在 from、转入行在 to（group[0] 为转出侧，collectTransferGroup 保证）
			isOut := gr.Direction == model.TxDirectionOut
			acct := toAcct
			flow := r.Amount
			if isOut {
				acct = fromAcct
				flow = -r.Amount
			}
			add(acct.Id, signedBalanceDelta(acct.Category, flow))
		} else {
			flow := r.Amount
			if gr.TxType == model.TxTypeExpense {
				flow = -flow
			}
			add(fromAcct.Id, signedBalanceDelta(fromAcct.Category, flow))
		}
	}
	return deltas, nil
}

// updateTxRow 更新单条交易行。
//
// WHERE 带 sync_version（本事务内读到的快照值）：SQLite WAL 快照隔离下它必然匹配，
// 若不匹配说明有绕过事务的写入，失败关闭。显式刷 sync_version/updated_at（GPT-C03）。
func updateTxRow(sess *xorm.Session, bookID int64, row *model.Transaction, r UpdateRequest, fromID, toID int64, ver, now int64) error {
	res, err := sess.Exec(`UPDATE transactions SET
		account_id = ?, to_account_id = ?, category_id = ?, amount = ?,
		payment_channel = ?, note = ?, tx_time = ?, timezone_offset = ?,
		sync_version = ?, updated_at = ?
		WHERE id = ? AND book_id = ? AND deleted = 0 AND sync_version = ?`,
		fromID, toID, r.CategoryId, r.Amount,
		r.PaymentChannel, r.Comment, r.TxTime, r.TimezoneOffset,
		ver, now,
		row.Id, bookID, row.SyncVersion)
	if err != nil {
		return fmt.Errorf("更新交易 %d 失败：%w", row.Id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("%w：交易 %d 更新影响 %d 行", ErrConcurrentConflict, row.Id, n)
	}
	return nil
}

// rowAccountIDs 计算该行更新后的 (account_id, to_account_id)：
// 转账转出侧 = (新 from, 新 to)、转入侧 = (新 to, 新 from)；单笔 = (新账户, 0)。
func rowAccountIDs(gr *model.Transaction, fromAcct, toAcct *model.Account) (int64, int64) {
	if gr.TxType != model.TxTypeTransfer {
		return fromAcct.Id, 0
	}
	if gr.Direction == model.TxDirectionOut {
		return fromAcct.Id, toAcct.Id
	}
	return toAcct.Id, fromAcct.Id
}

// sortedAcctIDs 返回按 ID 升序的账户键（确定性的写入顺序，便于测试与审计）。
func sortedAcctIDs(deltas map[int64]int64) []int64 {
	ids := make([]int64, 0, len(deltas))
	for id := range deltas {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

func sameCurrency(a, b *model.Account) bool {
	return a.Currency == b.Currency
}

// normalizeUpdate 归一化命令：去空白，保证重试时摘要稳定
func normalizeUpdate(r UpdateRequest) UpdateRequest {
	r.ClientUuid = strings.TrimSpace(r.ClientUuid)
	r.Comment = strings.TrimSpace(r.Comment)
	return r
}

// validateUpdate 服务层强制校验。字段规则与 validateCreate 对齐；
// "txType 与既有行一致"在事务内校验（需要读行），此处只做命令级校验。
func validateUpdate(uid, bookID, txID int64, r UpdateRequest) error {
	if uid <= 0 || bookID <= 0 || txID <= 0 {
		return fmt.Errorf("%w：用户、账本或交易 ID 缺失", ErrInvalidArgument)
	}
	if r.ClientUuid == "" || len(r.ClientUuid) > maxClientUUIDLen {
		return fmt.Errorf("%w：clientUuid 必填且不超过 %d 字节", ErrInvalidArgument, maxClientUUIDLen)
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
	if r.TimezoneOffset < -840 || r.TimezoneOffset > 840 {
		return fmt.Errorf("%w：timezoneOffset 取值超出范围", ErrInvalidCreate)
	}
	if r.BaseVersion < 0 {
		return fmt.Errorf("%w：baseVersion 不能为负", ErrInvalidArgument)
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

// updateRequestHash 完整请求摘要：带域标签的确定性编码。
// clientUuid（幂等键）与 baseVersion（并发元数据）不参与业务身份（设计 §1/§7）。
func updateRequestHash(r UpdateRequest) (string, error) {
	payload := struct {
		Domain  string        `json:"domain"`
		Request UpdateRequest `json:"request"`
	}{
		Domain:  txUpdateHashDomain,
		Request: r,
	}
	payload.Request.ClientUuid = ""
	payload.Request.BaseVersion = 0
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("编码幂等请求摘要失败：%w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
