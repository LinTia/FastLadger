package api

// 预算 HTTP 适配（WBS 6.3）。写操作 owner/admin（服务层 requireManagerTx 强制），
// 读操作全体成员；"已用"聚合按可见性过滤（服务层 txVisibilityCond）。
import (
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

type budgetHandler struct {
	svc *service.BudgetService
}

func (h *budgetHandler) list(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	items, err := h.svc.List(currentUID(c), bookID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"items": items})
}

type budgetReq struct {
	Name       string `json:"name"`
	BudgetType int8   `json:"budgetType"`
	CategoryId int64  `json:"categoryId"`
	PeriodType int8   `json:"periodType"`
	Amount     int64  `json:"amount"`
	Rollover   bool   `json:"rollover"`
	Enabled    *bool  `json:"enabled"`
}

func (h *budgetHandler) create(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req budgetReq
	if !bindJSON(c, &req) {
		return
	}
	b, err := h.svc.Create(currentUID(c), bookID, service.BudgetInput{
		Name:       req.Name,
		BudgetType: req.BudgetType,
		CategoryId: req.CategoryId,
		PeriodType: req.PeriodType,
		Amount:     req.Amount,
		Rollover:   req.Rollover,
		Enabled:    req.Enabled,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, b)
}

func (h *budgetHandler) update(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	budgetID, good := pathID(c, "budgetId", "预算 ID 非法")
	if !good {
		return
	}
	var req budgetReq
	if !bindJSON(c, &req) {
		return
	}
	b, err := h.svc.Update(currentUID(c), bookID, budgetID, service.BudgetInput{
		Name:     req.Name,
		Amount:   req.Amount,
		Rollover: req.Rollover,
		Enabled:  req.Enabled,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, b)
}

func (h *budgetHandler) remove(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	budgetID, good := pathID(c, "budgetId", "预算 ID 非法")
	if !good {
		return
	}
	if err := h.svc.Delete(currentUID(c), bookID, budgetID); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}
