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

// Update 修改账本（名称 / 可见性开关）：owner 或 admin
func (s *BookService) Update(actor, bookID int64, name string, canViewOthers *bool) error {
	b, err := s.Get(bookID)
	if err != nil {
		return err
	}
	role, err := RoleOf(s.engine, bookID, actor)
	if err != nil {
		return err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return ErrForbidden
	}
	now := time.Now().Unix()
	if name = strings.TrimSpace(name); name != "" {
		b.Name = name
	}
	if canViewOthers != nil {
		b.MembersCanViewOthers = *canViewOthers
	}
	b.UpdatedAt = now
	_, err = s.engine.ID(bookID).Cols("name", "members_can_view_others", "updated_at").Update(b)
	return err
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
