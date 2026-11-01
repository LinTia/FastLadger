package service

// 投资与净资产（WBS 6.4 / 6.5）——表结构来自迁移 0001，零迁移。
//
// 口径约束（逐条对应文档，不得自创）：
//  1. 【精度】数量与价格【字符串存储】，全部运算走 shopspring/decimal，禁 float
//     （WBS 6.4 硬约束）；decimal 解析失败必须报错，绝不静默取零。
//  2. 【非负校验】数量 > 0、价格 ≥ 0、成本 ≥ 0（WBS 6.4）。
//  3. 【版本】instruments / holdings 含 sync_version 列 → 写路径经 versions
//     VersionSource 递增账本版本（WBS 6.4：AddHolding 带 VersionSource）。
//  4. 【审计】标的与持仓同为账本级共享配置：写路径留痕（实体 instrument / holding，
//     《枚举字典》§14.2）。行情录入（price append + 持仓现价刷新）同样留痕。
//  5. 【净资产公式】《枚举字典》§2：负债账户（10~15）balance 正 = 欠款；
//     总资产 = Σ资产账户(1~9)余额 + Σ持仓市值；总负债 = Σ负债账户(10~15)余额；
//     净资产 = 总资产 − 总负债（PRD §4.11 / WBS 6.5）。
//  6. 【市值】市值(分) = quantity × current_price × 100，decimal 半进到分；
//     浮动盈亏(分) = 市值 − cost_amount。
//  7. 【行情】手动录入（MVP 不做自动行情，PRD §12 决策 #3）：append 历史行 +
//     同事务刷新该标的全簿持仓的 current_price（冗余缓存），失败保留旧价
//     （PRD §9.8：行情获取失败保留上次价格，不影响手动数据）。
import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"

	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

var (
	// ErrInstrumentNotFound 标的不存在
	ErrInstrumentNotFound = errors.New("标的不存在")
	// ErrHoldingNotFound 持仓不存在
	ErrHoldingNotFound = errors.New("持仓不存在")
	// ErrInstrumentInUse 标的仍被持仓引用
	ErrInstrumentInUse = errors.New("标的仍存在持仓记录，无法删除")
	// ErrInvalidQuantity 数量非法（必须为正的十进制数）
	ErrInvalidQuantity = errors.New("数量必须为大于 0 的数字")
	// ErrInvalidPrice 价格非法（必须为 ≥0 的十进制数）
	ErrInvalidPrice = errors.New("价格必须为不小于 0 的数字")
)

// InvestService 标的 / 持仓 / 行情 / 净资产
type InvestService struct {
	engine   *xorm.Engine
	versions version.Source
}

func NewInvestService(engine *xorm.Engine, versions version.Source) *InvestService {
	return &InvestService{engine: engine, versions: versions}
}

// parseDecimal 解析十进制字符串（禁 float 的唯一入口；失败必须报错）
func parseDecimal(s, what string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("%w：%s 不是合法数字（%q）", ErrInvalidPrice, what, s)
	}
	return d, nil
}

// ---------- 标的 ----------

// InstrumentInput 标的入参
type InstrumentInput struct {
	Name   string
	Type   int8
	Market string
	Code   string
}

// CreateInstrument 新建标的（owner/admin，版本递增 + 审计）。
func (s *InvestService) CreateInstrument(uid, bookID int64, in InstrumentInput) (*model.Instrument, error) {
	if len([]rune(in.Name)) == 0 || len([]rune(in.Name)) > 64 {
		return nil, fmt.Errorf("%w：标的名称 1~64 字符", ErrInvalidArgument)
	}
	if !model.IsValidInstrumentType(in.Type) {
		return nil, fmt.Errorf("%w：标的类型非法（1~6）", ErrInvalidArgument)
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
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	i := &model.Instrument{
		BookId: bookID, Name: in.Name, Type: in.Type,
		Market: in.Market, Code: in.Code,
		SyncVersion: ver, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := sess.Insert(i); err != nil {
		return nil, err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionCreate,
		model.AuditEntityInstrument, i.Id, map[string]interface{}{
			"name": i.Name, "type": i.Type, "market": i.Market, "code": i.Code,
		}, "api"); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return i, nil
}

// UpdateInstrument 修改标的（owner/admin）。
func (s *InvestService) UpdateInstrument(uid, bookID, instrumentID int64, in InstrumentInput) (*model.Instrument, error) {
	if len([]rune(in.Name)) == 0 || len([]rune(in.Name)) > 64 {
		return nil, fmt.Errorf("%w：标的名称 1~64 字符", ErrInvalidArgument)
	}
	if !model.IsValidInstrumentType(in.Type) {
		return nil, fmt.Errorf("%w：标的类型非法（1~6）", ErrInvalidArgument)
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
	var before model.Instrument
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", instrumentID, bookID).Get(&before)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrInstrumentNotFound
	}
	beforeJSON := map[string]interface{}{"name": before.Name, "type": before.Type, "market": before.Market, "code": before.Code}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	before.Name, before.Type, before.Market, before.Code = in.Name, in.Type, in.Market, in.Code
	before.SyncVersion = ver
	before.UpdatedAt = time.Now().Unix()
	if _, err := sess.ID(before.Id).AllCols().Update(&before); err != nil {
		return nil, err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionUpdate,
		model.AuditEntityInstrument, before.Id, map[string]interface{}{
			"before": beforeJSON, "after": map[string]interface{}{
				"name": before.Name, "type": before.Type, "market": before.Market, "code": before.Code,
			},
		}, "api"); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &before, nil
}

// DeleteInstrument 软删标的（owner/admin）。仍有持仓引用时拒绝（悬空引用口径同分类）。
func (s *InvestService) DeleteInstrument(uid, bookID, instrumentID int64) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()
	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return err
	}
	var before model.Instrument
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", instrumentID, bookID).Get(&before)
	if err != nil {
		return err
	}
	if !has {
		return ErrInstrumentNotFound
	}
	var n int64
	if _, err := sess.SQL("SELECT COUNT(1) FROM holdings WHERE instrument_id = ? AND deleted = 0", instrumentID).Get(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrInstrumentInUse
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	before.Deleted, before.DeletedAt, before.UpdatedAt, before.SyncVersion = true, now, now, ver
	if _, err := sess.ID(before.Id).Cols("deleted", "deleted_at", "updated_at", "sync_version").Update(&before); err != nil {
		return err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionDelete,
		model.AuditEntityInstrument, before.Id, map[string]interface{}{
			"name": before.Name, "type": before.Type,
		}, "api"); err != nil {
		return err
	}
	return sess.Commit()
}

// ListInstruments 标的库（全体成员可读）。
func (s *InvestService) ListInstruments(uid, bookID int64) ([]model.Instrument, error) {
	if err := requireMember(s.engine, bookID, uid); err != nil {
		return nil, err
	}
	out := make([]model.Instrument, 0, 16)
	err := s.engine.SQL(
		`SELECT * FROM instruments WHERE book_id = ? AND deleted = 0 ORDER BY id ASC`, bookID).Find(&out)
	return out, err
}

// ---------- 持仓 ----------

// HoldingInput 持仓入参（quantity/currentPrice 为十进制字符串，禁 float）
type HoldingInput struct {
	AccountId    int64
	InstrumentId int64
	Quantity     string
	CostAmount   int64 // 整数分
	CurrentPrice string
}

// AddHolding 新建持仓（owner/admin；WBS 6.4：带 VersionSource 版本递增 + 审计）。
func (s *InvestService) AddHolding(uid, bookID int64, in HoldingInput) (*model.Holding, error) {
	qty, err := parseDecimal(in.Quantity, "数量")
	if err != nil {
		return nil, fmt.Errorf("%w：%v", ErrInvalidQuantity, err)
	}
	if !qty.IsPositive() {
		return nil, ErrInvalidQuantity
	}
	price := decimal.Zero
	if strings.TrimSpace(in.CurrentPrice) != "" && in.CurrentPrice != "0" {
		price, err = parseDecimal(in.CurrentPrice, "现价")
		if err != nil {
			return nil, err
		}
	}
	if price.IsNegative() {
		return nil, ErrInvalidPrice
	}
	if in.CostAmount < 0 {
		return nil, fmt.Errorf("%w：成本不能为负", ErrInvalidArgument)
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
	if err := ensureBookAccount(sess, bookID, in.AccountId); err != nil {
		return nil, err
	}
	var instr model.Instrument
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", in.InstrumentId, bookID).Get(&instr)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrInstrumentNotFound
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	h := &model.Holding{
		BookId: bookID, AccountId: in.AccountId, InstrumentId: in.InstrumentId,
		Quantity: qty.String(), CostAmount: in.CostAmount,
		CurrentPrice: price.String(), SyncVersion: ver,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := sess.Insert(h); err != nil {
		return nil, err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionCreate,
		model.AuditEntityHolding, h.Id, map[string]interface{}{
			"accountId": h.AccountId, "instrumentId": h.InstrumentId,
			"quantity": h.Quantity, "costAmount": h.CostAmount, "currentPrice": h.CurrentPrice,
		}, "api"); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return h, nil
}

// ensureBookAccount 账户属于本账本且未删（持仓挂靠校验；不限制账户分类——
// PRD 允许把实物黄金等计入任意账户口径，挂靠限制交给 UI 引导）。
func ensureBookAccount(sess *xorm.Session, bookID, accountID int64) error {
	var n int64
	if _, err := sess.SQL("SELECT COUNT(1) FROM accounts WHERE id = ? AND book_id = ? AND deleted = 0",
		accountID, bookID).Get(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// UpdateHolding 修改持仓数量/成本/现价（owner/admin）。
func (s *InvestService) UpdateHolding(uid, bookID, holdingID int64, in HoldingInput) (*model.Holding, error) {
	qty, err := parseDecimal(in.Quantity, "数量")
	if err != nil {
		return nil, fmt.Errorf("%w：%v", ErrInvalidQuantity, err)
	}
	if !qty.IsPositive() {
		return nil, ErrInvalidQuantity
	}
	price, err := parseDecimal(in.CurrentPrice, "现价")
	if err != nil {
		return nil, err
	}
	if price.IsNegative() {
		return nil, ErrInvalidPrice
	}
	if in.CostAmount < 0 {
		return nil, fmt.Errorf("%w：成本不能为负", ErrInvalidArgument)
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
	var before model.Holding
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", holdingID, bookID).Get(&before)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrHoldingNotFound
	}
	beforeJSON := map[string]interface{}{"quantity": before.Quantity, "costAmount": before.CostAmount, "currentPrice": before.CurrentPrice}
	afterJSON := map[string]interface{}{"quantity": qty.String(), "costAmount": in.CostAmount, "currentPrice": price.String()}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	before.Quantity, before.CostAmount, before.CurrentPrice = qty.String(), in.CostAmount, price.String()
	before.SyncVersion = ver
	before.UpdatedAt = time.Now().Unix()
	if _, err := sess.ID(before.Id).AllCols().Update(&before); err != nil {
		return nil, err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionUpdate,
		model.AuditEntityHolding, before.Id, map[string]interface{}{"before": beforeJSON, "after": afterJSON}, "api"); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &before, nil
}

// DeleteHolding 软删持仓（owner/admin）。
func (s *InvestService) DeleteHolding(uid, bookID, holdingID int64) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()
	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return err
	}
	var before model.Holding
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", holdingID, bookID).Get(&before)
	if err != nil {
		return err
	}
	if !has {
		return ErrHoldingNotFound
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	before.Deleted, before.DeletedAt, before.UpdatedAt, before.SyncVersion = true, now, now, ver
	if _, err := sess.ID(before.Id).Cols("deleted", "deleted_at", "updated_at", "sync_version").Update(&before); err != nil {
		return err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionDelete,
		model.AuditEntityHolding, before.Id, map[string]interface{}{
			"quantity": before.Quantity, "costAmount": before.CostAmount,
		}, "api"); err != nil {
		return err
	}
	return sess.Commit()
}

// HoldingView 持仓视图：配置 + decimal 市值/盈亏（分）
type HoldingView struct {
	model.Holding
	InstrumentName string `json:"instrumentName"`
	InstrumentType int8   `json:"instrumentType"`
	MarketValue    int64  `json:"marketValue"` // 市值（整数分）
	ProfitLoss     int64  `json:"profitLoss"`  // 浮动盈亏（整数分）= 市值 − 成本
}

// ListHoldings 持仓列表 + 市值/盈亏（全体成员可读；只读 holdings 表，不读交易）。
func (s *InvestService) ListHoldings(uid, bookID int64) ([]HoldingView, error) {
	if err := requireMember(s.engine, bookID, uid); err != nil {
		return nil, err
	}
	hs := make([]model.Holding, 0, 16)
	if err := s.engine.SQL(
		`SELECT * FROM holdings WHERE book_id = ? AND deleted = 0 ORDER BY id ASC`, bookID).Find(&hs); err != nil {
		return nil, err
	}
	names := map[int64]model.Instrument{}
	var instrs []model.Instrument
	if err := s.engine.SQL(
		`SELECT * FROM instruments WHERE book_id = ? AND deleted = 0`, bookID).Find(&instrs); err != nil {
		return nil, err
	}
	for _, i := range instrs {
		names[i.Id] = i
	}
	out := make([]HoldingView, 0, len(hs))
	for _, h := range hs {
		v := HoldingView{Holding: h}
		if instr, ok := names[h.InstrumentId]; ok {
			v.InstrumentName = instr.Name
			v.InstrumentType = instr.Type
		}
		mv, err := marketValueCents(h.Quantity, h.CurrentPrice)
		if err != nil {
			return nil, err
		}
		v.MarketValue = mv
		v.ProfitLoss = mv - h.CostAmount
		out = append(out, v)
	}
	return out, nil
}

// marketValueCents 市值(分) = quantity × price × 100（decimal 半进到分，禁 float）。
func marketValueCents(quantity, price string) (int64, error) {
	q, err := parseDecimal(quantity, "数量")
	if err != nil {
		return 0, fmt.Errorf("%w：%v", ErrInvalidQuantity, err)
	}
	p, err := parseDecimal(price, "现价")
	if err != nil {
		return 0, err
	}
	yuan := q.Mul(p)                                    // 元
	cents := yuan.Mul(decimal.NewFromInt(100)).Round(0) // 分
	return cents.IntPart(), nil
}

// SetPrice 手动录入行情（MVP 唯一价格来源，PRD §12 决策 #3）：
// append instrument_prices 历史行 + 同事务刷新该标的全簿持仓 current_price。
func (s *InvestService) SetPrice(uid, bookID, instrumentID int64, price string, priceTime int64) error {
	p, err := parseDecimal(price, "价格")
	if err != nil {
		return err
	}
	if p.IsNegative() {
		return ErrInvalidPrice
	}
	if priceTime <= 0 {
		priceTime = time.Now().Unix()
	}
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()
	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return err
	}
	var instr model.Instrument
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", instrumentID, bookID).Get(&instr)
	if err != nil {
		return err
	}
	if !has {
		return ErrInstrumentNotFound
	}
	if _, err := sess.Insert(&model.InstrumentPrice{InstrumentId: instrumentID, Price: p.String(), PriceTime: priceTime}); err != nil {
		return err
	}
	// 刷新该标的全簿持仓现价（冗余缓存；持仓行有 sync_version → 递增版本）
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return err
	}
	if _, err := sess.Exec(
		`UPDATE holdings SET current_price = ?, sync_version = ?, updated_at = ? WHERE instrument_id = ? AND book_id = ? AND deleted = 0`,
		p.String(), ver, time.Now().Unix(), instrumentID, bookID); err != nil {
		return err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionUpdate,
		model.AuditEntityInstrument, instrumentID, map[string]interface{}{
			"action": "set_price", "price": p.String(), "priceTime": priceTime,
		}, "api"); err != nil {
		return err
	}
	return sess.Commit()
}

// ---------- 净资产（WBS 6.5） ----------

// NetWorthEntry 按账户分类的汇总行
type NetWorthEntry struct {
	Category int8  `json:"category"`
	Total    int64 `json:"total"` // 整数分；负债侧为欠款正数
	Count    int   `json:"count"`
}

// NetWorthSummary 资产总览（PRD §4.11 / WBS 6.5）
type NetWorthSummary struct {
	Assets           []NetWorthEntry `json:"assets"`        // 分类 1~9
	Liabilities      []NetWorthEntry `json:"liabilities"`   // 分类 10~15（正数 = 欠款）
	HoldingsValue    int64           `json:"holdingsValue"` // 持仓总市值（分）
	TotalAssets      int64           `json:"totalAssets"`   // Σ资产 + 持仓市值
	TotalLiabilities int64           `json:"totalLiabilities"`
	NetWorth         int64           `json:"netWorth"` // 总资产 − 总负债
}

// NetWorth 净资产汇总。负债账户 balance 正 = 欠款（《枚举字典》§2）：
// 净资产 = Σ资产账户(1~9) + Σ持仓市值 − Σ负债账户(10~15)。
func (s *InvestService) NetWorth(uid, bookID int64) (*NetWorthSummary, error) {
	if err := requireMember(s.engine, bookID, uid); err != nil {
		return nil, err
	}
	var accounts []model.Account
	if err := s.engine.SQL(
		`SELECT * FROM accounts WHERE book_id = ? AND deleted = 0`, bookID).Find(&accounts); err != nil {
		return nil, err
	}
	sum := &NetWorthSummary{Assets: []NetWorthEntry{}, Liabilities: []NetWorthEntry{}}
	assetIdx := map[int8]int{}
	liabIdx := map[int8]int{}
	for _, a := range accounts {
		if model.IsLiabilityCategory(a.Category) {
			idx, ok := liabIdx[a.Category]
			if !ok {
				idx = len(sum.Liabilities)
				liabIdx[a.Category] = idx
				sum.Liabilities = append(sum.Liabilities, NetWorthEntry{Category: a.Category})
			}
			sum.Liabilities[idx].Total += a.Balance
			sum.Liabilities[idx].Count++
			sum.TotalLiabilities += a.Balance
		} else {
			idx, ok := assetIdx[a.Category]
			if !ok {
				idx = len(sum.Assets)
				assetIdx[a.Category] = idx
				sum.Assets = append(sum.Assets, NetWorthEntry{Category: a.Category})
			}
			sum.Assets[idx].Total += a.Balance
			sum.Assets[idx].Count++
			sum.TotalAssets += a.Balance
		}
	}
	// 持仓市值并入总资产（PRD §4.11：投资账户包含现金与持仓市值）
	hs, err := s.ListHoldings(uid, bookID)
	if err != nil {
		return nil, err
	}
	for _, h := range hs {
		sum.HoldingsValue += h.MarketValue
	}
	sum.TotalAssets += sum.HoldingsValue
	sum.NetWorth = sum.TotalAssets - sum.TotalLiabilities
	return sum, nil
}
