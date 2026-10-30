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
	if err := h.accounts.Delete(currentUID(c), bookID, accountID); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"id": accountID})
}
