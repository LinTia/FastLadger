package service

// 报表与导出（WBS 6.1 / 6.2，P2）——纯读聚合，零迁移、零账务写入。
//
// 口径约束（逐条对应文档，不得自创）：
//  1. 【读路径可见性】与 List / Get / Pull / ListDeleted 同一红线：先
//     requireMember（canViewAllForTx 内），再叠加 txVisibilityCond
//     （PRD §7.3 服务端权威；关闭互看时 member 只能聚合本人记录，
//     否则报表/导出就是绕过可见性开关的后门）。
//  2. 【月份边界】PRD §12 决策 #4：MVP 仅支持 CNY；故月份边界固定按
//     中国标准时间（UTC+8）切分，与服务器部署时区无关（确定性优先，
//     服务器在 UTC 时区也不会把月初 8 小时算错月）。
//  3. 【聚合范围】转账（type=3）不计入收支（PRD §4.3.1/§5.3），
//     月度趋势只聚合收入(1)/支出(2)；软删行（deleted=1）一律排除。
//  4. 【导出上限】行数上限 10000（2C2G NAS 资源口径，PRD §8.1），
//     超出截断并经 X-Export-Truncated 头如实告知，绝不静默少给。
//  5. 【CSV 公式注入】sanitizeCSVCell 对 `= + - @` 开头或含 \t \r 的
//     单元格前置单引号（WBS 6.2 硬约束；Excel 会把 = 开头单元格当公式执行）。
//  6. 【金额】导出统一为"元"字符串（整数分做纯整数字符串换算，禁 float）。
import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

// cstZone 中国标准时间（PRD §12 决策 #4：MVP 仅 CNY）。
// 用固定偏移而非 time.LoadLocation：不依赖服务器 tzdata，部署环境无关。
var cstZone = time.FixedZone("CST", 8*3600)

// CSTZone 报表月份边界的时区（CST，UTC+8）——handler 缺省月份用。
func CSTZone() *time.Location { return cstZone }

// exportRowLimit 导出行数上限（含）——超出即截断，truncated=true。
const exportRowLimit = 10000

// ReportService 报表与导出（只读，不写任何表）。
type ReportService struct {
	engine *xorm.Engine
}

func NewReportService(engine *xorm.Engine) *ReportService {
	return &ReportService{engine: engine}
}

// MonthPoint 月度收支聚合点（Month 为 "2006-01" 形式，CST 口径）。
type MonthPoint struct {
	Month   string `json:"month"`
	Income  int64  `json:"income"`
	Expense int64  `json:"expense"`
}

// CategorySlice 分类占比切片（按金额降序）。
type CategorySlice struct {
	CategoryId int64  `json:"categoryId"`
	Name       string `json:"name"`
	Total      int64  `json:"total"`
	Count      int64  `json:"count"`
}

// Monthly 最近 months 个月的收支趋势（含当前月；months 归一化 1~24，默认 6）。
//
// 转账不计入收支；可见性条件逐月叠加（owner/admin 或开互看的 member 聚合全本，
// 关互看的 member 只聚合本人记录）。
func (s *ReportService) Monthly(uid, bookID int64, months int) ([]MonthPoint, error) {
	canViewAll, err := canViewAllForTx(s.engine, uid, bookID)
	if err != nil {
		return nil, err
	}
	if months <= 0 {
		months = 6
	}
	if months > 24 {
		months = 24
	}
	now := time.Now().In(cstZone)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cstZone)
	out := make([]MonthPoint, 0, months)
	for i := months - 1; i >= 0; i-- {
		start := first.AddDate(0, -i, 0)
		end := start.AddDate(0, 1, 0)
		where := []string{"book_id = ?", "deleted = 0", "tx_time >= ?", "tx_time < ?"}
		args := []interface{}{bookID, start.Unix(), end.Unix()}
		if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
			where = append(where, vcond)
			args = append(args, vargs...)
		}
		var row struct {
			Income  int64 `xorm:"income"`
			Expense int64 `xorm:"expense"`
		}
		_, err := s.engine.SQL(
			"SELECT COALESCE(SUM(CASE WHEN \"type\" = ? THEN amount END), 0) AS income, "+
				"COALESCE(SUM(CASE WHEN \"type\" = ? THEN amount END), 0) AS expense "+
				"FROM transactions WHERE "+strings.Join(where, " AND "),
			append([]interface{}{model.TxTypeIncome, model.TxTypeExpense}, args...)...,
		).Get(&row)
		if err != nil {
			return nil, err
		}
		out = append(out, MonthPoint{Month: start.Format("2006-01"), Income: row.Income, Expense: row.Expense})
	}
	return out, nil
}

// CategoryShare 指定月份（"2006-01"，CST）某收支方向的分类占比，按金额降序。
//
// txType 仅允许收入(1)/支出(2)——传转账(3)是调用方语义错误，直接 400 拒绝。
// 分类名经 LEFT JOIN 取当册快照；category_id=0（历史无主数据）聚合为"未分类"。
func (s *ReportService) CategoryShare(uid, bookID int64, month string, txType int8) ([]CategorySlice, error) {
	if txType != model.TxTypeIncome && txType != model.TxTypeExpense {
		return nil, fmt.Errorf("%w：分类占比只支持收入或支出", ErrInvalidArgument)
	}
	start, end, err := monthRangeCST(month)
	if err != nil {
		return nil, err
	}
	canViewAll, err := canViewAllForTx(s.engine, uid, bookID)
	if err != nil {
		return nil, err
	}
	where := []string{"t.book_id = ?", "t.deleted = 0", `"t"."type" = ?`, "t.tx_time >= ?", "t.tx_time < ?"}
	args := []interface{}{bookID, txType, start.Unix(), end.Unix()}
	if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
		where = append(where, vcond)
		args = append(args, vargs...)
	}
	rows := make([]CategorySlice, 0, 16)
	err = s.engine.SQL(
		"SELECT t.category_id AS category_id, IFNULL(c.name, '') AS name, "+
			"SUM(t.amount) AS total, COUNT(1) AS count "+
			"FROM transactions t LEFT JOIN categories c ON c.id = t.category_id "+
			"WHERE "+strings.Join(where, " AND ")+" "+
			"GROUP BY t.category_id ORDER BY total DESC",
		args...,
	).Find(&rows)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].CategoryId == 0 || rows[i].Name == "" {
			rows[i].Name = "未分类"
		}
	}
	return rows, nil
}

// monthRangeCST "2006-01" → 该月在 CST 下的 [start, end) unix 秒区间。
func monthRangeCST(month string) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01", month, cstZone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w：month 必须为 YYYY-MM 格式", ErrInvalidArgument)
	}
	end := start.AddDate(0, 1, 0)
	return start, end, nil
}

// ExportQuery 导出过滤条件（字段语义与 TxListQuery 一致；Keyword 不支持——
// 原型导出页只暴露时间范围与收支类型，账户/分类筛选由列表页承担）。
type ExportQuery struct {
	TxType     int8 // 0=全部
	AccountId  int64
	CategoryId int64
	MinTime    int64
	MaxTime    int64
}

// ExportRow 导出行（金额仍为整数分；元字符串化在 Cell 内完成）。
// Tx 刻意不内嵌（xorm 对内嵌结构体的 SELECT * 列映射不可靠）：
// 先按成熟模式扫 []model.Transaction，再在此包装名称映射。
type ExportRow struct {
	Tx            model.Transaction `json:"tx"`
	AccountName   string            `json:"accountName"`
	ToAccountName string            `json:"toAccountName"`
	CategoryName  string            `json:"categoryName"`
	ChannelName   string            `json:"channelName"`
}

// ExportRows 导出数据行（tx_time 升序）。返回 truncated=true 表示超过上限已截断。
func (s *ReportService) ExportRows(uid, bookID int64, q ExportQuery) ([]ExportRow, bool, error) {
	canViewAll, err := canViewAllForTx(s.engine, uid, bookID)
	if err != nil {
		return nil, false, err
	}
	where := []string{"book_id = ?", "deleted = 0"}
	args := []interface{}{bookID}
	if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
		where = append(where, vcond)
		args = append(args, vargs...)
	}
	if q.TxType > 0 {
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
	if q.MinTime > 0 {
		where = append(where, "tx_time >= ?")
		args = append(args, q.MinTime)
	}
	if q.MaxTime > 0 {
		where = append(where, "tx_time <= ?")
		args = append(args, q.MaxTime)
	}
	// 多取一行探测截断，而不是先 COUNT 再查（省一次全表扫描）
	listSQL := "SELECT * FROM transactions WHERE " + strings.Join(where, " AND ") +
		" ORDER BY tx_time ASC, id ASC LIMIT ?"
	listArgs := append(args, exportRowLimit+1)
	var txs []model.Transaction
	if err := s.engine.SQL(listSQL, listArgs...).Find(&txs); err != nil {
		return nil, false, err
	}
	truncated := false
	if int64(len(txs)) > exportRowLimit {
		txs = txs[:exportRowLimit]
		truncated = true
	}
	// 名称映射：账户/分类名取当册快照（含已删——历史行仍应显示原名）
	accNames, err := s.nameMap("accounts", bookID)
	if err != nil {
		return nil, false, err
	}
	catNames, err := s.nameMap("categories", bookID)
	if err != nil {
		return nil, false, err
	}
	out := make([]ExportRow, 0, len(txs))
	for _, t := range txs {
		row := ExportRow{Tx: t}
		row.AccountName = accNames[t.AccountId]
		if t.ToAccountId > 0 {
			row.ToAccountName = accNames[t.ToAccountId]
		}
		row.CategoryName = catNames[t.CategoryId]
		row.ChannelName = paymentChannelName(t.PaymentChannel)
		out = append(out, row)
	}
	return out, truncated, nil
}

func (s *ReportService) nameMap(table string, bookID int64) (map[int64]string, error) {
	var rows []struct {
		Id   int64  `xorm:"id"`
		Name string `xorm:"name"`
	}
	// 表名来自代码内两处固定调用点，非用户输入，无注入面
	if err := s.engine.SQL("SELECT id, name FROM "+table+" WHERE book_id = ?", bookID).Find(&rows); err != nil {
		return nil, err
	}
	m := make(map[int64]string, len(rows))
	for _, r := range rows {
		m[r.Id] = r.Name
	}
	return m, nil
}

// paymentChannelName 支付渠道展示名（《枚举字典》§3）。
func paymentChannelName(c int8) string {
	switch c {
	case model.PaymentChannelDirect:
		return "直接"
	case model.PaymentChannelWeChat:
		return "微信"
	case model.PaymentChannelAlipay:
		return "支付宝"
	case model.PaymentChannelUnionPay:
		return "云闪付"
	case model.PaymentChannelPOS:
		return "POS 刷卡"
	case model.PaymentChannelOther:
		return "其他"
	default:
		return "直接"
	}
}

// cents2yuan 整数分 → "1,234.56"（纯整数字符串运算，禁 float——金额口径与 Web 端一致）。
func cents2yuan(cents int64) string {
	neg := cents < 0
	abs := cents
	if neg {
		abs = -cents
	}
	s := strconv.FormatInt(abs, 10)
	for len(s) < 3 {
		s = "0" + s
	}
	yuan, fen := s[:len(s)-2], s[len(s)-2:]
	// 千分位
	var b strings.Builder
	for i, c := range []byte(yuan) {
		if i > 0 && (len(yuan)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(c)
	}
	out := b.String() + "." + fen
	if neg {
		out = "-" + out
	}
	return out
}

// 导出字段白名单（顺序即 CSV 列序）。键为 API `fields` 参数取值。
var exportFields = []struct {
	key string
	// header CSV 表头（中文，与原型"导出字段"勾选项一致）
	header string
}{
	{"time", "时间"},
	{"type", "类型"},
	{"amount", "金额(元)"},
	{"category", "分类"},
	{"account", "账户"},
	{"toAccount", "对方账户"},
	{"channel", "渠道"},
	{"merchant", "商户"},
	{"note", "备注"},
}

// ExportFieldKeys 字段白名单键集合（handler 校验用）。
var ExportFieldKeys = func() map[string]bool {
	m := make(map[string]bool, len(exportFields))
	for _, f := range exportFields {
		m[f.key] = true
	}
	return m
}()

// Cell 取一行某导出字段的字符串值（未知键由 handler 白名单先行拦截）。
func (r *ExportRow) Cell(key string) string {
	switch key {
	case "time":
		return time.Unix(r.Tx.TxTime, 0).In(cstZone).Format("2006-01-02 15:04:05")
	case "type":
		return model.TxTypeName(r.Tx.TxType)
	case "amount":
		return cents2yuan(r.Tx.Amount)
	case "category":
		if r.CategoryName == "" {
			return "未分类"
		}
		return r.CategoryName
	case "account":
		return r.AccountName
	case "toAccount":
		return r.ToAccountName
	case "channel":
		return r.ChannelName
	case "merchant":
		return r.Tx.Merchant
	case "note":
		return r.Tx.Note
	}
	return ""
}

// ParseFields "time,amount" → 白名单内的键切片（空 = 全部字段，按默认列序）。
func ParseFields(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		keys := make([]string, 0, len(exportFields))
		for _, f := range exportFields {
			keys = append(keys, f.key)
		}
		return keys, nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !ExportFieldKeys[p] {
			return nil, fmt.Errorf("%w：未知导出字段 %q", ErrInvalidArgument, p)
		}
		keys = append(keys, p)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w：fields 不能为空", ErrInvalidArgument)
	}
	return keys, nil
}

// BuildCSV 按选定字段产出 CSV 字节（UTF-8 BOM + CRLF，Excel 直开无乱码——PRD §4.9）。
func BuildCSV(rows []ExportRow, fields []string) ([]byte, error) {
	var b strings.Builder
	b.WriteByte(0xEF) // UTF-8 BOM：Excel 识别 UTF-8 的关键
	b.WriteByte(0xBB)
	b.WriteByte(0xBF)
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	headers := make([]string, 0, len(fields))
	byKey := make(map[string]string, len(exportFields))
	for _, f := range exportFields {
		byKey[f.key] = f.header
	}
	for _, k := range fields {
		headers = append(headers, byKey[k])
	}
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for i := range rows {
		cells := make([]string, 0, len(fields))
		for _, k := range fields {
			cells = append(cells, sanitizeCSVCell(rows[i].Cell(k)))
		}
		if err := w.Write(cells); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// BuildExportJSON 按选定字段产出 JSON 文件字节（对象数组；字段键为白名单键）。
func BuildExportJSON(rows []ExportRow, fields []string) ([]byte, error) {
	out := make([]map[string]string, 0, len(rows))
	for i := range rows {
		m := make(map[string]string, len(fields))
		for _, k := range fields {
			m[k] = rows[i].Cell(k)
		}
		out = append(out, m)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// sanitizeCSVCell CSV 公式注入防护（WBS 6.2 硬约束）：
// 以 `= + - @` 开头或含 \t \r 的单元格前置单引号，Excel/Sheets 按文本处理。
func sanitizeCSVCell(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsAny(s, "\t\r") {
		return "'" + s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}
