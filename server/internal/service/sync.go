package service

// 增量同步（sync/pull + sync/push）—— 阶段 3.1 / 3.2。
//
// 设计口径（照抄，不得自创）：
//   - 《新手开发完整指南》§22.2：Pull 必须在**同一只读事务（一致快照）**内确定水位并读取
//     四类实体，按【版本边界】分页（safeVersion 回扩），杜绝"先读交易 → 别人提交 V6 →
//     再读账户看到 V6、却漏了交易 V6"的跨表交错。四个 Find **全部检查错误**：
//     若查询失败却静默返回空集，客户端会照常把游标推进到 safeVersion，
//     导致账户/分类/持仓变更永久丢失且不再重试。
//   - 《转账入口裁决》GPT-ADR01 + 《落地方案》§6.1：交易创建**唯一入口**是
//     TxService.Create；sync/push 只是**批量适配层**，逐条调用它，
//     绝不新增第二个创建入口、绝不复制转账/幂等/版本递增逻辑。
//   - PRD §7.3 可见性：Pull 的交易读取叠加可见性条件（member + 关闭互看时只下发本人记录）；
//     accounts / categories / holdings 是账本共享配置，**不做**可见性过滤。
//   - PRD §12.1 决策 4：可见性变化**不递增** sync_version，改用事件携带
//     force_full_resync=true 触发客户端 afterVersion=0 定向全量重拉。
import (
	"context"
	"errors"
	"fmt"

	"fastledger/internal/instance"
	"fastledger/internal/model"

	"xorm.io/xorm"
)

const (
	// MaxSyncBatch 单批【目标规模】（非硬上限）：防止"多版本"场景一次性全量加载 OOM
	//（内存预算 < 150MB）。注意：同一 sync_version 的行必须整批返回（不能拆版本），
	// 因此单个版本若含海量行仍会整批加载；故**写路径**须限制单事务变更条数，读路径无法兜底。
	MaxSyncBatch = 500
	// maxPushItems 单次 sync/push 的项数上限（写路径限流：一个批次内变更条数受控）
	maxPushItems = 200
)

// 离线补传的单条结果状态（对齐 PRD / WBS 阶段 3.2 的冲突语义）
const (
	// PushStatusCreated 新建成功
	PushStatusCreated = "created"
	// PushStatusReplayed 幂等重放（同一 clientUuid 已成功提交过，返回原结果）
	PushStatusReplayed = "replayed"
	// PushStatusConflict **真正的业务冲突**（ErrCreateConflict / ErrConcurrentConflict /
	// 回执不完整）：客户端应让用户选择"覆盖 / 放弃 / 另存副本"，其中"另存副本"通常意味着
	// 换一个新的 clientUuid 重试。
	PushStatusConflict = "conflict"
	// PushStatusInvalid 该条命令非法（400 语义：缺 clientUuid / 金额非法 / 账户或分类不存在）
	// 客户端应作废或修正该条，原样重试不会成功。
	PushStatusInvalid = "invalid"
	// PushStatusForbidden 无权限（403 语义：不是该账本成员）
	PushStatusForbidden = "forbidden"
	// PushStatusError 服务端未预期错误（500 语义）：**可原样重试，且禁止更换 clientUuid**。
	//
	// 为什么不能并入 conflict：conflict 在客户端会触发"覆盖 / 放弃 / 另存副本"，
	// 而"另存副本"意味着用新 clientUuid 重试——那会把一次服务端内部错误
	// 变成两笔真实记账（重复记账）。error 的语义是"还没成功，用同一个 clientUuid 再试"。
	PushStatusError = "error"
)

// 逐条结果的 errorCode（供客户端做机器可判的分支，message 仅供人读）
const (
	// ErrCodeClientUUIDConflict 同一 clientUuid 被用于不同请求内容
	ErrCodeClientUUIDConflict = "CLIENT_UUID_CONFLICT"
	// ErrCodeIncompleteReceipt 该 clientUuid 已有不完整回执（需换 clientUuid 或人工介入）
	ErrCodeIncompleteReceipt = "INCOMPLETE_RECEIPT"
	// ErrCodeConcurrentConflict 数据被并发修改，可重试
	ErrCodeConcurrentConflict = "CONCURRENT_CONFLICT"
	// ErrCodeInternal 服务内部错误（可重试；不泄露实现细节）
	ErrCodeInternal = "INTERNAL_ERROR"
	// ErrCodeForbidden 非账本成员
	ErrCodeForbidden = "FORBIDDEN"
	// ErrCodeInvalidRequest 请求非法
	ErrCodeInvalidRequest = "INVALID_REQUEST"
	// ErrCodeAccountNotFound 账户不存在/已删除（该条离线命令应作废或改投其它账户）
	ErrCodeAccountNotFound = "ACCOUNT_NOT_FOUND"
	// ErrCodeCategoryNotFound 分类不存在/已删除
	ErrCodeCategoryNotFound = "CATEGORY_NOT_FOUND"
)

// SyncBatch 一次 pull 的完整结果。
//
// 客户端规则（指南 §22.2）：**只有把一个批次里所有实体都完整应用后**，才把本地游标
// 推进到 HighVersion，并用它作为下一次的 afterVersion；中途失败必须整批重放。
type SyncBatch struct {
	// HighVersion 本批上界（版本边界回扩后的安全版本）；客户端应用完这一批后游标推进到它
	HighVersion int64 `json:"highVersion"`
	// Transactions 交易（已按可见性过滤：member + 关闭互看时只有本人记录）
	Transactions []model.Transaction `json:"transactions"`
	// Accounts 账户（账本共享配置，不做可见性过滤）
	Accounts []model.Account `json:"accounts"`
	// Categories 分类（账本共享配置，不做可见性过滤）
	Categories []model.Category `json:"categories"`
	// Holdings 持仓（账本共享配置，不做可见性过滤）
	Holdings []model.Holding `json:"holdings"`
	// RestoreEpoch 恢复世代（客户端发现不一致即停止增量、全量重建，见指南 §26.1）
	RestoreEpoch int64 `json:"restoreEpoch"`
	// InstanceId 实例标识（与 RestoreEpoch 配套，用于识别"换了个实例"）
	InstanceId string `json:"instanceId"`
}

// PushItem 离线补传的单项命令。
//
// 字段与 CreateRequest **完全一致**（下方 toCreateRequest 是 1:1 拷贝，不含任何业务逻辑），
// 唯一区别是**不带 `binding:"required"` 标签**：
// 若沿用 CreateRequest 的绑定标签，gin 的校验会把"某一条缺 clientUuid"升级成
// **整批 400**，与"单项失败不影响后续项、每项独立结果"的契约冲突。
type PushItem struct {
	ClientUuid     string `json:"clientUuid"`
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

// toCreateRequest 转成唯一创建入口的入参（纯字段拷贝，不夹带任何校验/副作用）
func (p PushItem) toCreateRequest() CreateRequest {
	return CreateRequest{
		ClientUuid:     p.ClientUuid,
		TxType:         p.TxType,
		AccountId:      p.AccountId,
		ToAccountId:    p.ToAccountId,
		Amount:         p.Amount,
		CategoryId:     p.CategoryId,
		PaymentChannel: p.PaymentChannel,
		TxTime:         p.TxTime,
		TimezoneOffset: p.TimezoneOffset,
		Comment:        p.Comment,
		Source:         p.Source,
	}
}

// PushResult 单项补传结果（逐条独立，整体 HTTP 200，便于客户端逐条重试）
type PushResult struct {
	// ClientUuid 回显操作键，客户端据此与本地离线队列对齐
	ClientUuid string `json:"clientUuid"`
	// Status created / replayed / conflict / invalid / forbidden / error
	Status string `json:"status"`
	// Id 主记录 ID（单笔即该交易；转账为转出记录 ID，便于与 related_id 配对）
	Id int64 `json:"id"`
	// TransactionIds 转账为 [转出ID, 转入ID]；单笔为 1 个元素（附加字段，便于离线核对整笔）
	TransactionIds []int64 `json:"transactionIds,omitempty"`
	// Version 本次业务变更命中的账本版本（重放时为原创建版本）
	Version int64 `json:"version"`
	// ErrorCode 机器可判的错误码（仅失败时有值）
	ErrorCode string `json:"errorCode,omitempty"`
	// Message 人读的错误说明（仅失败时有值，不含内部细节）
	Message string `json:"message,omitempty"`
}

// SyncService 增量同步服务。
//
// 依赖关系刻意保持单向：SyncService → TxService（唯一创建入口）→ version.Source。
// SyncService **不得**被 ws 之外的业务包反向依赖，且 service 包**不得**导入 internal/ws。
type SyncService struct {
	engine *xorm.Engine
	txs    *TxService
	// meta 启动时加载的实例元信息（恢复世代）。恢复流程要求先停进程再 bump 后启动，
	// 故运行期只需启动时加载的值，不必每次 pull 读盘（见 internal/instance 注释）。
	meta *instance.Meta
}

// NewSyncService 构造同步服务。
//
// meta 允许为 nil（单元测试/降级场景）：此时批次里的世代信息回填零值，
// 客户端应按"未知世代"处理，不影响增量语义。
func NewSyncService(engine *xorm.Engine, txs *TxService, meta *instance.Meta) *SyncService {
	return &SyncService{engine: engine, txs: txs, meta: meta}
}

// generation 返回 (restoreEpoch, instanceId)；meta 缺失时返回零值。
func (s *SyncService) generation() (int64, string) {
	if s.meta == nil {
		return 0, ""
	}
	return s.meta.RestoreEpoch, s.meta.InstanceId
}

// normalizeSyncLimit 归一化批次大小：<=0 或 > MaxSyncBatch 一律回落 MaxSyncBatch。
//
// 不信任调用方：limit 是"目标规模"，过小会让同一版本被反复重扫（客户端游标永不推进的风险），
// 过大则破坏内存预算，故只允许 (0, MaxSyncBatch] 之间的显式值。
func normalizeSyncLimit(limit int) int {
	if limit <= 0 || limit > MaxSyncBatch {
		return MaxSyncBatch
	}
	return limit
}

// Pull 拉取增量（一致快照 + 版本边界分页）。
//
// 逐行对应指南 §22.2 的骨架，差异只有两处（均为本阶段硬性要求，见文件头）：
//  1. 交易读取叠加可见性条件（txVisibilityCond）；
//  2. 批次附带 restoreEpoch / instanceId。
func (s *SyncService) Pull(uid, bookID, afterVersion int64, limit int) (*SyncBatch, error) {
	if uid <= 0 || bookID <= 0 {
		return nil, fmt.Errorf("%w：用户或账本缺失", ErrInvalidArgument)
	}
	if afterVersion < 0 {
		// 非法游标按全量处理：宁可从头发一次，也不要因负游标读出空集让客户端误以为"已同步"
		afterVersion = 0
	}
	limit = normalizeSyncLimit(limit)

	if s.engine == nil {
		return nil, fmt.Errorf("%w：同步服务未初始化", ErrInvalidArgument)
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	// 只读事务：提交/回滚均可释放快照，回滚是能覆盖所有失败路径的一步
	defer func() { _ = sess.Rollback() }()

	epoch, instanceID := s.generation()
	emptyBatch := func(high int64) *SyncBatch {
		return &SyncBatch{
			HighVersion:  high,
			Transactions: make([]model.Transaction, 0),
			Accounts:     make([]model.Account, 0),
			Categories:   make([]model.Category, 0),
			Holdings:     make([]model.Holding, 0),
			RestoreEpoch: epoch,
			InstanceId:   instanceID,
		}
	}

	// 0) 成员资格校验下沉到服务入口：非成员/已退出直接 403，避免某个 handler 漏校验时
	//    构成越权读取（含关闭互看的私账）。闭合 D-R06。
	if err := requireMemberTx(sess, bookID, uid); err != nil {
		return nil, err
	}
	// 0.1) 同一快照内确定可见性（PRD §7.3）：member + 关闭互看时只下发本人交易
	canViewAll, err := canViewDetailTx(sess, uid, bookID)
	if err != nil {
		return nil, err
	}

	// 1) 在快照内确定本次水位
	var bv model.BookVersion
	if _, err := sess.Where("book_id = ?", bookID).Get(&bv); err != nil {
		return nil, err
	}

	// 2) 确定本批版本上界 safeVersion：按【版本边界】分页，绝不把同一 sync_version
	//    的一组变更拆到两批。先取增量交易前 limit 行所在的最高版本（含软删墓碑，
	//    不过滤 deleted），再把该版本的全部行（含同版本的账户/分类/持仓）一起纳入。
	var safeVersion int64
	_, err = sess.SQL(`SELECT COALESCE(MAX(sync_version), 0) FROM (
		SELECT sync_version FROM transactions
		WHERE book_id = ? AND sync_version > ?
		ORDER BY sync_version ASC LIMIT ?
	)`, bookID, afterVersion, limit).Get(&safeVersion)
	if err != nil {
		return nil, err
	}

	// 兜底：本批没有交易增量时，可能只有账户/分类/持仓变更，
	// 从这三张表取增量的最高版本，避免漏掉非交易变更。
	if safeVersion == 0 {
		_, err = sess.SQL(`SELECT COALESCE(MAX(v), 0) FROM (
			SELECT sync_version AS v FROM accounts   WHERE book_id=? AND sync_version>?
			UNION ALL
			SELECT sync_version FROM categories WHERE book_id=? AND sync_version>?
			UNION ALL
			SELECT sync_version FROM holdings  WHERE book_id=? AND sync_version>?
		)`, bookID, afterVersion, bookID, afterVersion, bookID, afterVersion).
			Get(&safeVersion)
		if err != nil {
			return nil, err
		}
	}
	if safeVersion == 0 {
		return emptyBatch(bv.Version), nil // 确实没有任何增量
	}

	// 3) 所有实体统一在 (afterVersion, safeVersion] 内、同一快照读取，
	//    保证同一版本的多表/多行一起可见；limit 是目标规模，同版本回扩可能略超。
	batch := emptyBatch(safeVersion)
	cond := "book_id=? AND sync_version>? AND sync_version<=?"

	// 3.1 交易：叠加可见性条件（关闭互看时 member 只拿本人记录，且排除 creator_uid=0 的未知归属）
	txQuery := sess.Where(cond, bookID, afterVersion, safeVersion)
	if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
		txQuery = txQuery.And(vcond, vargs...)
	}
	if err := txQuery.OrderBy("sync_version ASC, id ASC").Find(&batch.Transactions); err != nil {
		return nil, err
	}

	// 3.2 三类共享配置：必须逐个检查错误——若查询失败却静默返回空集，
	//     客户端会照常把游标推进到 safeVersion，导致变更永久丢失且不再重试。
	if err := sess.Where(cond, bookID, afterVersion, safeVersion).
		OrderBy("sync_version ASC, id ASC").Find(&batch.Accounts); err != nil {
		return nil, err
	}
	if err := sess.Where(cond, bookID, afterVersion, safeVersion).
		OrderBy("sync_version ASC, id ASC").Find(&batch.Categories); err != nil {
		return nil, err
	}
	if err := sess.Where(cond, bookID, afterVersion, safeVersion).
		OrderBy("sync_version ASC, id ASC").Find(&batch.Holdings); err != nil {
		return nil, err
	}
	return batch, nil
}

// Push 离线队列补传：逐条调用 TxService.Create（唯一创建入口）。
//
// 约束：
//   - **没有**任何新建交易逻辑：既不自己开事务，也不复制转账/幂等/版本递增；
//   - 单项失败不影响后续项（逐条独立），整体由 handler 返回 200 + 每项结果；
//   - items 为空或超长（> 200）属于整批非法 → 400；
//   - ctx 由 handler 传入（请求取消/超时对整批生效），每项 Create 用同一个 ctx。
func (s *SyncService) Push(ctx context.Context, uid, bookID int64, items []PushItem) ([]PushResult, error) {
	if uid <= 0 || bookID <= 0 {
		return nil, fmt.Errorf("%w：用户或账本缺失", ErrInvalidArgument)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w：items 不能为空", ErrInvalidArgument)
	}
	if len(items) > maxPushItems {
		return nil, fmt.Errorf("%w：单次补传不得超过 %d 项", ErrInvalidArgument, maxPushItems)
	}
	if s.txs == nil {
		return nil, fmt.Errorf("%w：交易服务未初始化", ErrInvalidArgument)
	}

	results := make([]PushResult, 0, len(items))
	for _, item := range items {
		// 逐条独立提交：单条失败只记该条的失败原因，继续下一条。
		// ctx 为空时回落到 Background（防御：调用方误传 nil 不应 panic）。
		callCtx := ctx
		if callCtx == nil {
			callCtx = context.Background()
		}
		res, err := s.txs.Create(callCtx, uid, bookID, item.toCreateRequest())
		results = append(results, pushResultOf(item.ClientUuid, res, err))
	}
	return results, nil
}

// pushResultOf 把 Create 的结果/错误映射为单项补传结果（纯函数，便于单测）。
func pushResultOf(clientUUID string, res *CreateResult, err error) PushResult {
	out := PushResult{ClientUuid: clientUUID}
	if err != nil {
		status, code, msg := pushStatusOf(err)
		out.Status = status
		out.ErrorCode = code
		out.Message = msg
		return out
	}
	if res == nil {
		// 不可达（Create 成功必返回结果），但不留 nil 解引用
		out.Status = PushStatusError
		out.ErrorCode = ErrCodeInternal
		out.Message = "服务端暂时无法处理，请用同一 clientUuid 稍后重试"
		return out
	}
	if res.Replayed {
		out.Status = PushStatusReplayed
	} else {
		out.Status = PushStatusCreated
	}
	out.TransactionIds = append([]int64(nil), res.TransactionIds...)
	if len(res.TransactionIds) > 0 {
		out.Id = res.TransactionIds[0]
	}
	out.Version = res.Version
	return out
}

// pushStatusOf 错误 → (status, errorCode, message)。
//
// 分类依据《转账入口裁决》§2 的 HTTP 语义（400/403/409/500）：
//   - 非法命令/目标不存在 → invalid（客户端应作废或修正该条，重试无意义）；
//   - 非成员 → forbidden；
//   - 真正的业务冲突（clientUuid 内容不一致 / 并发 / 回执不完整） → conflict
//     （客户端让用户选"覆盖 / 放弃 / 另存副本"）；
//   - 未预期错误 → error + INTERNAL_ERROR：**可原样重试，禁止换 clientUuid**
//     （并入 conflict 会诱导客户端"另存副本"，把内部错误变成重复记账）。
func pushStatusOf(err error) (status, code, message string) {
	switch {
	case errors.Is(err, ErrCreateForbidden), errors.Is(err, ErrNotMember), errors.Is(err, ErrForbidden):
		return PushStatusForbidden, ErrCodeForbidden, "不是该账本成员，无记账权限"
	case errors.Is(err, ErrCreateConflict):
		return PushStatusConflict, ErrCodeClientUUIDConflict, "同一 clientUuid 已用于不同内容，请检查请求或更换 clientUuid"
	case errors.Is(err, ErrIncompleteReceipt):
		return PushStatusConflict, ErrCodeIncompleteReceipt, "该 clientUuid 已有不完整回执，请更换 clientUuid 或联系实例管理员处理"
	case errors.Is(err, ErrConcurrentConflict):
		return PushStatusConflict, ErrCodeConcurrentConflict, "数据已被并发修改，请重试"
	case errors.Is(err, ErrAccountNotFound):
		return PushStatusInvalid, ErrCodeAccountNotFound, "账户不存在或已删除"
	case errors.Is(err, ErrCategoryNotFound):
		return PushStatusInvalid, ErrCodeCategoryNotFound, "分类不存在或已删除"
	case errors.Is(err, ErrInvalidCreate), errors.Is(err, ErrInvalidArgument):
		return PushStatusInvalid, ErrCodeInvalidRequest, "该条命令非法（请检查 clientUuid/金额/账户/分类）"
	default:
		// 不泄露实现细节；措辞明确"用同一个 clientUuid 重试"，避免客户端另存副本造成重复记账
		return PushStatusError, ErrCodeInternal, "服务端暂时无法处理，请用同一 clientUuid 稍后重试"
	}
}
