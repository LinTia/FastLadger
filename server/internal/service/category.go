package service

// 分类（categories）CRUD —— 两级结构：parent_id=0 为大类，否则为小类。
//
// 约束：
//  1. 子类的 cat_type 必须与父类一致，且父类必须是大类（只允许两级）；
//  2. `book_id` 只来自可信路径参数，服务端校验归属；
//  3. 分类属于账本级共享配置，写操作需要 owner 或 admin（枚举字典 §6）。
import (
	"errors"
	"sort"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"

	"xorm.io/xorm"
)

var (
	// ErrCategoryNotFound 分类不存在
	ErrCategoryNotFound = errors.New("分类不存在")
	// ErrCategoryInvalid 分类参数非法
	ErrCategoryInvalid = errors.New("分类参数非法")
	// ErrCategoryHasChildren 该分类仍有子分类
	ErrCategoryHasChildren = errors.New("该分类下仍有子分类，无法删除")
	// ErrCategoryInUse 该分类仍被交易引用
	ErrCategoryInUse = errors.New("分类仍存在交易记录，无法删除")
)

// CategoryService 分类服务
type CategoryService struct {
	engine   *xorm.Engine
	versions version.Source
}

// NewCategoryService 构造分类服务
func NewCategoryService(engine *xorm.Engine, versions version.Source) *CategoryService {
	return &CategoryService{engine: engine, versions: versions}
}

// CategoryCreateInput 新建分类入参
type CategoryCreateInput struct {
	Name     string
	ParentId int64
	CatType  int8
}

// CategoryUpdateInput 修改分类入参（nil 表示不修改）
type CategoryUpdateInput struct {
	Name     *string
	ParentId *int64
	CatType  *int8
}

// CategoryTreeNode 两级分类树节点（大类 + 其小类）
type CategoryTreeNode struct {
	model.Category
	Children []model.Category `json:"children"`
}

// List 分类平铺列表（按收入/支出、大类优先排序，前端可自行组装两级树）
func (s *CategoryService) List(uid, bookID int64) ([]model.Category, error) {
	if err := requireMember(s.engine, bookID, uid); err != nil {
		return nil, err
	}
	out := make([]model.Category, 0, 16)
	err := s.engine.SQL(
		`SELECT * FROM categories WHERE book_id = ? AND deleted = 0
		 ORDER BY cat_type ASC, parent_id ASC, id ASC`, bookID).Find(&out)
	return out, err
}

// Tree 组装两级分类树（大类 → 小类）；孤儿小类挂到根，避免数据静默丢失
func (s *CategoryService) Tree(uid, bookID int64) ([]CategoryTreeNode, error) {
	flat, err := s.List(uid, bookID)
	if err != nil {
		return nil, err
	}
	nodes := make([]CategoryTreeNode, 0, len(flat))
	index := make(map[int64]int, len(flat))
	for _, c := range flat {
		if c.ParentId != 0 {
			continue
		}
		index[c.Id] = len(nodes)
		nodes = append(nodes, CategoryTreeNode{Category: c, Children: []model.Category{}})
	}
	for _, c := range flat {
		if c.ParentId == 0 {
			continue
		}
		if i, ok := index[c.ParentId]; ok {
			nodes[i].Children = append(nodes[i].Children, c)
			continue
		}
		// 父类已删除/不存在：作为根节点返回，前端仍可展示与修正
		nodes = append(nodes, CategoryTreeNode{Category: c, Children: []model.Category{}})
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].CatType != nodes[j].CatType {
			return nodes[i].CatType < nodes[j].CatType
		}
		return nodes[i].Id < nodes[j].Id
	})
	return nodes, nil
}

// Create 新建分类
func (s *CategoryService) Create(uid, bookID int64, in CategoryCreateInput) (*model.Category, error) {
	name, err := normalizeName(in.Name, "分类名称")
	if err != nil {
		return nil, err
	}
	if in.CatType != model.CategoryTypeIncome && in.CatType != model.CategoryTypeExpense {
		return nil, ErrCategoryInvalid
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return nil, err
	}
	if in.ParentId != 0 {
		if err := ensureParentCategory(sess, bookID, in.ParentId, in.CatType); err != nil {
			return nil, err
		}
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	c := &model.Category{
		BookId:      bookID,
		Name:        name,
		ParentId:    in.ParentId,
		CatType:     in.CatType,
		SyncVersion: ver,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := sess.Insert(c); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}

// Update 修改分类（名称 / 父级）；cat_type 不允许修改（会与历史交易的收支方向冲突）
func (s *CategoryService) Update(uid, bookID, categoryID int64, in CategoryUpdateInput) (*model.Category, error) {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return nil, err
	}
	var c model.Category
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", categoryID, bookID).Get(&c)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrCategoryNotFound
	}
	if in.CatType != nil && *in.CatType != c.CatType {
		return nil, ErrCategoryInvalid
	}
	if in.Name != nil {
		name, nerr := normalizeName(*in.Name, "分类名称")
		if nerr != nil {
			return nil, nerr
		}
		c.Name = name
	}
	if in.ParentId != nil {
		parentID := *in.ParentId
		if parentID == categoryID {
			return nil, ErrCategoryInvalid
		}
		if parentID != 0 {
			if err := ensureParentCategory(sess, bookID, parentID, c.CatType); err != nil {
				return nil, err
			}
			var childCount int64
			if _, err := sess.SQL(
				`SELECT COUNT(1) FROM categories WHERE book_id = ? AND parent_id = ? AND deleted = 0`,
				bookID, categoryID).Get(&childCount); err != nil {
				return nil, err
			}
			if childCount > 0 {
				return nil, ErrCategoryHasChildren
			}
		}
		c.ParentId = parentID
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	c.SyncVersion = ver
	c.UpdatedAt = time.Now().Unix()
	n, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", categoryID, bookID).
		Cols("name", "parent_id", "sync_version", "updated_at").
		Update(&model.Category{Name: c.Name, ParentId: c.ParentId, SyncVersion: ver, UpdatedAt: c.UpdatedAt})
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrConcurrentConflict
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Delete 软删分类；存在子分类或被交易引用时拒绝
func (s *CategoryService) Delete(uid, bookID, categoryID int64) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return err
	}
	var c model.Category
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", categoryID, bookID).Get(&c)
	if err != nil {
		return err
	}
	if !has {
		return ErrCategoryNotFound
	}
	var children int64
	if _, err := sess.SQL(
		`SELECT COUNT(1) FROM categories WHERE book_id = ? AND parent_id = ? AND deleted = 0`,
		bookID, categoryID).Get(&children); err != nil {
		return err
	}
	if children > 0 {
		return ErrCategoryHasChildren
	}
	var used int64
	if _, err := sess.SQL(
		`SELECT COUNT(1) FROM transactions WHERE book_id = ? AND category_id = ? AND deleted = 0`,
		bookID, categoryID).Get(&used); err != nil {
		return err
	}
	if used > 0 {
		return ErrCategoryInUse
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	res, err := sess.Exec(
		`UPDATE categories SET deleted = 1, deleted_at = ?, sync_version = ?, updated_at = ?
		 WHERE id = ? AND book_id = ? AND deleted = 0`, now, ver, now, categoryID, bookID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCategoryNotFound
	}
	return sess.Commit()
}

// ensureParentCategory 校验父分类：属于本账本、未删除、是大类、且 cat_type 一致
func ensureParentCategory(sess *xorm.Session, bookID, parentID int64, catType int8) error {
	var p model.Category
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", parentID, bookID).Get(&p)
	if err != nil {
		return err
	}
	if !has {
		return ErrCategoryNotFound
	}
	if p.ParentId != 0 || p.CatType != catType {
		return ErrCategoryInvalid
	}
	return nil
}
