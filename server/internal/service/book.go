package service

// 账本：创建 / 列表 / 详情 / 修改 / 删除。
//
// 创建账本时同时初始化 book_versions（版本水位）并把创建者写入 owner 成员，
// 全在同一事务内完成，避免出现"有账本但没版本记录"的不一致。
import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"

	"xorm.io/xorm"
)

var (
	// ErrBookNotFound 账本不存在
	ErrBookNotFound = errors.New("账本不存在")
	// ErrForbidden 无权限
	ErrForbidden = errors.New("无权限执行该操作")
	// ErrNotMember 非本账本成员
	ErrNotMember = errors.New("不是该账本成员")
)

// BookService 账本服务
type BookService struct {
	engine   *xorm.Engine
	versions version.Source
}

// NewBookService 构造账本服务
func NewBookService(engine *xorm.Engine, versions version.Source) *BookService {
	return &BookService{engine: engine, versions: versions}
}

// Create 创建账本（创建者自动为 owner）
func (s *BookService) Create(uid int64, name string, bookType int8) (*model.Book, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		// 归入 ErrInvalidArgument：由 mapSvcErr 映射为 400，而不是 500
		return nil, fmt.Errorf("%w：账本名称不能为空", ErrInvalidArgument)
	}
	if bookType != model.BookTypePersonal && bookType != model.BookTypeFamily {
		bookType = model.BookTypeFamily
	}

	now := time.Now().Unix()
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	b := &model.Book{
		Name:                 name,
		OwnerUid:             uid,
		BookType:             bookType,
		DefaultCurrency:      "CNY",
		MembersCanViewOthers: true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if _, err := sess.Insert(b); err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if _, err := sess.Insert(&model.BookVersion{BookId: b.Id, Version: 0}); err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if _, err := sess.Insert(&model.BookMember{
		BookId: b.Id, Uid: uid, Role: model.RoleOwner, CanViewOthers: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	// 默认账户与默认分类与账本同事务创建（WBS 2.1：新账本开箱可用）
	seedVer, err := s.versions.Next(sess, b.Id)
	if err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if err := seedBookDefaults(sess, b.Id, seedVer); err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}

// List 我加入的账本（不含已删除）
func (s *BookService) List(uid int64) ([]model.Book, error) {
	var books []model.Book
	err := s.engine.SQL(
		`SELECT b.* FROM books b
		 INNER JOIN book_members m ON m.book_id = b.id AND m.deleted = 0
		 WHERE m.uid = ? AND b.deleted = 0 ORDER BY b.id DESC`, uid).Find(&books)
	return books, err
}

// Get 账本详情
func (s *BookService) Get(bookID int64) (*model.Book, error) {
	var b model.Book
	has, err := s.engine.ID(bookID).Where("deleted = 0").Get(&b)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrBookNotFound
	}
	return &b, nil
}

// BookUpdateResult 账本设置更新的结果（handler 据此决定"是否真的发生了可见性变化"）。
//
// 为什么要把这个信息回传给 handler：可见性变化需要触发 WS 事件
// visibility_changed + forceFullResync（PRD §12.1 决策 2/3），
// 而"只改了名字"或"可见性传了同样的值"都不该惊动客户端去全量重拉。
type BookUpdateResult struct {
	// NameChanged 名称是否真的写入了新值
	NameChanged bool
	// VisibilityChanged 可见性开关是否真的发生了变化（false = 没变或没传该字段）
	VisibilityChanged bool
}

// Update 修改账本设置（名称 / 可见性开关）：owner 或 admin。
//
// **不变量（《落地方案》§5.2）**：可见性开关只有**一套写实现**（SetVisibility），
// 两个 HTTP 入口共用它——`PUT /books/:id`（名称 + 开关，文档既有入口）与
// `PUT /books/:id/visibility`（便捷入口）。任一入口发生**实际变化**都必须广播
// visibility_changed，值没变则不得广播。
//
// 因此本方法只负责：① 名称变了就写 name（并校验 RowsAffected==1）；② 传了开关就委托
// SetVisibility，把它的 changed 结果原样带给 handler。绝不在这里拼第二个
// members_can_view_others 的 UPDATE —— 那才是"两条写路径"的真实风险来源。
func (s *BookService) Update(actor, bookID int64, name string, canViewOthers *bool) (*BookUpdateResult, error) {
	b, err := s.Get(bookID)
	if err != nil {
		return nil, err
	}
	role, err := RoleOf(s.engine, bookID, actor)
	if err != nil {
		return nil, err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return nil, ErrForbidden
	}

	out := &BookUpdateResult{}
	// 名称：空串 = 不改名；与当前值相同 = 不写库（避免无谓 UPDATE 与 updated_at 抖动）
	if trimmed := strings.TrimSpace(name); trimmed != "" && trimmed != b.Name {
		b.Name = trimmed
		b.UpdatedAt = time.Now().Unix()
		n, uerr := s.engine.ID(bookID).Where("deleted = 0").
			Cols("name", "updated_at").Update(b)
		if uerr != nil {
			return nil, uerr
		}
		if n != 1 {
			// 读与写之间账本被并发删除：0 行不能当成功返回（与 ChangeRole/Remove/SetVisibility 同一口径）
			return nil, fmt.Errorf("%w：账本已不存在或已被删除", ErrConcurrentConflict)
		}
		out.NameChanged = true
	}

	// 可见性：一律委托唯一实现，让"是否真的变了"只有一处判定
	if canViewOthers != nil {
		changed, cerr := s.SetVisibility(actor, bookID, *canViewOthers)
		if cerr != nil {
			return nil, cerr
		}
		out.VisibilityChanged = changed
	}
	return out, nil
}

// CurrentVersion 读取账本当前版本水位（只读）。
//
// WS 事件信封要求所有事件都带 version（协调事件不递增版本，只回报当前值），
// 由本方法统一取值，避免各处手写 SELECT 造成口径漂移。
func (s *BookService) CurrentVersion(bookID int64) (int64, error) {
	var v int64
	if _, err := s.engine.SQL("SELECT version FROM book_versions WHERE book_id = ?", bookID).Get(&v); err != nil {
		return 0, err
	}
	return v, nil
}

// SetVisibility 设置账本级可见性开关（books.members_can_view_others）—— **唯一写入口**。
//
// 返回 `changed`：本次调用是否真的改变了开关值。幂等写入（传入与当前相同的值）返回
// (false, nil) 且**不写库**——handler 据此决定是否广播 visibility_changed：
// 事件契约里该事件的触发条件是"开关**实际变化**"，否则每次重复提交都会让所有在线端
// 白做一次 afterVersion=0 的全量重拉。
//
// 四条硬约束（PRD §7.3 + §12.1 决策 3/4）：
//  1. **仅 owner / admin 可改**；member 一律 ErrForbidden（权限矩阵《落地方案》§5.5：
//     "账本可见性开关" member = ✗）。
//  2. **不递增 sync_version**：可见性变化不是数据变更，它不改变任何实体的业务内容；
//     污染版本号语义会让"版本边界分页"把无关账本变更捆进同一批。
//     客户端重建靠 WS 事件携带 forceFullResync=true 触发 afterVersion=0 的定向全量重拉。
//  3. 变更**仅对未来同步生效**：成员此前已查看/已导出的数据服务端无法收回（PRD §7.3 隐私边界），
//     该话术由前端常驻显示；服务端只负责让后续 pull 立刻按新规则过滤。
//  4. 对外只有 `PUT /books/:id/visibility` 一个入口（`PUT /books/:id` 收到该字段会被
//     handler 显式 400），避免"两条写路径只修一条"的历史坑。
func (s *BookService) SetVisibility(actor, bookID int64, canView bool) (bool, error) {
	if actor <= 0 || bookID <= 0 {
		return false, fmt.Errorf("%w：用户或账本缺失", ErrInvalidArgument)
	}
	// 账本存在性（软删后不可改设置）
	b, err := s.Get(bookID)
	if err != nil {
		return false, err
	}
	role, err := RoleOf(s.engine, bookID, actor)
	if err != nil {
		return false, err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return false, ErrForbidden
	}
	if b.MembersCanViewOthers == canView {
		// 幂等：没有实际变化就不写库、不广播
		return false, nil
	}
	now := time.Now().Unix()
	b.MembersCanViewOthers = canView
	b.UpdatedAt = now
	n, err := s.engine.ID(bookID).Where("deleted = 0").
		Cols("members_can_view_others", "updated_at").Update(b)
	if err != nil {
		return false, err
	}
	if n != 1 {
		// 读与写之间账本被并发删除：0 行不能当成功返回（与 ChangeRole/Remove 同一口径）
		return false, fmt.Errorf("%w：账本已不存在或已被删除", ErrConcurrentConflict)
	}
	return true, nil
}

// Delete 删除账本：仅 owner（落地方案 §5.5：admin 不可删除账本）
func (s *BookService) Delete(actor, bookID int64) error {
	role, err := RoleOf(s.engine, bookID, actor)
	if err != nil {
		return err
	}
	if role != model.RoleOwner {
		return ErrForbidden
	}
	now := time.Now().Unix()
	_, err = s.engine.ID(bookID).Update(&model.Book{Deleted: true, DeletedAt: now, UpdatedAt: now})
	return err
}
