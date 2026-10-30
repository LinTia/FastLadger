package api

// 记账核心 HTTP 适配层：账户 / 分类 / 交易（对齐《落地方案》§5.1 REST 契约）。
//
// 统一约定（与既有 handler 一致）：
//   - 成功 ok()，失败 fail()，service 错误一律经 mapSvcErr 映射；
//   - clientUuid 双层校验：handler 绑定 required，service 层强制非空；
//   - book_id 只来自可信路径参数，uid 只来自认证上下文。
import (
	"net/http"
	"strconv"
	"strings"

	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// ledgerHandler 账户 / 分类 / 交易的 handler 集合
//
// ⚠️ 写路径的 WS 通知清单（漏一个就是"A 端改了、B 端不刷新"，且极难查）：
//   - 账户 增/改/删  → notifyDataChanged(bookID, <本次 sync_version>)   ← 已接
//   - 分类 增/改/删  → notifyDataChanged(bookID, <本次 sync_version>)   ← 已接
//   - 交易 创建      → notifyDataChanged(bookID, res.Version)          ← 已接（tx_handler.go）
//   - 交易 改/删     → 阶段 4（TxService.Delete/Restore）上线时必须同时接
//   - 持仓 增/改/删  → **TODO（holdings 写接口尚未实现）**：
//     上线持仓写接口时**必须同时**调用 notifyDataChanged(bookID, ver)，
//     否则客户端永远收不到"持仓变了"，只能等下一次手工 pull。
//   - 账户余额变动（交易引起）→ 已由交易创建的通知覆盖
//
// 判定标准只有一条：**这次写入是否让 book_versions.version 递增**。递增就必须通知。
type ledgerHandler struct {
	accounts   *service.AccountService
	categories *service.CategoryService
	txs        *service.TxService
}

// pathID 解析路径中的正整数 ID；非法返回 400
func pathID(c *gin.Context, name, msg string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, msg)
		return 0, false
	}
	return id, true
}

// queryInt64 解析可选整数查询参数（缺省返回 0，非法返回 400）
func queryInt64(c *gin.Context, name string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, true
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "查询参数 "+name+" 非法")
		return 0, false
	}
	return v, true
}

// queryInt 解析可选 int 查询参数（缺省返回 0，非法返回 400）
func queryInt(c *gin.Context, name string) (int, bool) {
	v, good := queryInt64(c, name)
	if !good {
		return 0, false
	}
	return int(v), true
}

// ---------- 账户 ----------

type accountCreateReq struct {
	Name     string `json:"name"`
	Category int8   `json:"category"`
	Currency string `json:"currency"`
	ParentId int64  `json:"parentId"`
}

type accountUpdateReq struct {
	Name     *string `json:"name"`
	ParentId *int64  `json:"parentId"`
}

func (h *ledgerHandler) listAccounts(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	list, err := h.accounts.List(currentUID(c), bookID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, list)
}

func (h *ledgerHandler) getAccount(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	accountID, good := pathID(c, "aid", "账户 ID 非法")
	if !good {
		return
	}
	a, err := h.accounts.Get(currentUID(c), bookID, accountID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, a)
}

func (h *ledgerHandler) createAccount(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req accountCreateReq
	if !bindJSON(c, &req) {
		return
	}
	a, err := h.accounts.Create(currentUID(c), bookID, service.AccountCreateInput{
		Name:     req.Name,
		Category: req.Category,
		Currency: req.Currency,
		ParentId: req.ParentId,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 广播 data_changed（账户是账本共享配置，其他在线端必须知道"有新版本"）
	notifyDataChanged(bookID, a.SyncVersion)
	ok(c, a)
}

func (h *ledgerHandler) updateAccount(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	accountID, good := pathID(c, "aid", "账户 ID 非法")
	if !good {
		return
	}
	var req accountUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	a, err := h.accounts.Update(currentUID(c), bookID, accountID, service.AccountUpdateInput{
		Name:     req.Name,
		ParentId: req.ParentId,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 广播 data_changed（本次写入的 sync_version 即 a.SyncVersion）
	notifyDataChanged(bookID, a.SyncVersion)
	ok(c, a)
}

func (h *ledgerHandler) deleteAccount(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	accountID, good := pathID(c, "aid", "账户 ID 非法")
	if !good {
		return
	}
	ver, err := h.accounts.Delete(currentUID(c), bookID, accountID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 广播 data_changed（删除同样是账本数据变更，版本已递增）
	notifyDataChanged(bookID, ver)
	ok(c, gin.H{"id": accountID})
}
