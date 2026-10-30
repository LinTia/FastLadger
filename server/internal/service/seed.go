package service

// 新账本的默认账户与默认两级分类（《新手开发完整指南》§12 / WBS 2.1）。
//
// 与账本创建处于【同一事务】：要么账本+默认数据一起成功，要么一起回滚，
// 避免出现"有账本但没有可用账户/分类"的半成品状态。
import (
	"time"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

// defaultAccountSpec 默认账户（现金 / 储蓄卡 / 信用卡，覆盖 90% 手动记账场景）
type defaultAccountSpec struct {
	Name     string
	Category int8
}

// defaultCategorySpec 默认大类及其小类
type defaultCategorySpec struct {
	Name     string
	Children []string
}

var defaultAccounts = []defaultAccountSpec{
	{Name: "现金", Category: model.CategoryCash},
	{Name: "储蓄卡", Category: model.CategoryDemandDeposit},
	{Name: "信用卡", Category: model.CategoryCreditCard},
}

var defaultExpenseCategories = []defaultCategorySpec{
	{Name: "餐饮", Children: []string{"早餐", "午餐", "晚餐", "外卖"}},
	{Name: "交通", Children: []string{"公交地铁", "打车", "加油"}},
	{Name: "购物", Children: []string{"日用品", "服饰", "数码"}},
	{Name: "居住", Children: []string{"房租", "水电燃气"}},
	{Name: "娱乐", Children: []string{"电影", "旅行"}},
	{Name: "医疗", Children: []string{"门诊", "药品"}},
}

var defaultIncomeCategories = []defaultCategorySpec{
	{Name: "工资", Children: []string{"月薪", "加班费"}},
	{Name: "奖金", Children: []string{"年终奖"}},
	{Name: "其他收入", Children: []string{"红包", "利息"}},
}

// seedBookDefaults 在调用方事务内写入默认账户与默认分类，统一使用账本当前版本号。
func seedBookDefaults(sess *xorm.Session, bookID, ver int64) error {
	now := time.Now().Unix()
	for _, spec := range defaultAccounts {
		a := &model.Account{
			BookId:      bookID,
			Name:        spec.Name,
			Category:    spec.Category,
			Currency:    "CNY",
			Balance:     0,
			SyncVersion: ver,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := sess.Insert(a); err != nil {
			return err
		}
	}
	if err := seedCategories(sess, bookID, ver, model.CategoryTypeExpense, defaultExpenseCategories, now); err != nil {
		return err
	}
	return seedCategories(sess, bookID, ver, model.CategoryTypeIncome, defaultIncomeCategories, now)
}

// seedCategories 写入一组大类及其小类（只允许两级）
func seedCategories(sess *xorm.Session, bookID, ver int64, catType int8,
	specs []defaultCategorySpec, now int64) error {

	for _, spec := range specs {
		parent := &model.Category{
			BookId:      bookID,
			Name:        spec.Name,
			ParentId:    0,
			CatType:     catType,
			SyncVersion: ver,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := sess.Insert(parent); err != nil {
			return err
		}
		for _, childName := range spec.Children {
			child := &model.Category{
				BookId:      bookID,
				Name:        childName,
				ParentId:    parent.Id,
				CatType:     catType,
				SyncVersion: ver,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if _, err := sess.Insert(child); err != nil {
				return err
			}
		}
	}
	return nil
}
