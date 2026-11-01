package api

// 投资与净资产 HTTP 适配（WBS 6.4 / 6.5）。
// 写操作 owner/admin（服务层 requireManagerTx 强制）；读全体成员。
import (
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

type investHandler struct {
	svc *service.InvestService
}

// ---------- 标的 ----------

func (h *investHandler) listInstruments(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	items, err := h.svc.ListInstruments(currentUID(c), bookID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"items": items})
}

type instrumentReq struct {
	Name   string `json:"name"`
	Type   int8   `json:"type"`
	Market string `json:"market"`
	Code   string `json:"code"`
}

func (h *investHandler) createInstrument(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req instrumentReq
	if !bindJSON(c, &req) {
		return
	}
	i, err := h.svc.CreateInstrument(currentUID(c), bookID, service.InstrumentInput{
		Name: req.Name, Type: req.Type, Market: req.Market, Code: req.Code,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, i)
}

func (h *investHandler) updateInstrument(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	iid, good := pathID(c, "instrumentId", "标的 ID 非法")
	if !good {
		return
	}
	var req instrumentReq
	if !bindJSON(c, &req) {
		return
	}
	i, err := h.svc.UpdateInstrument(currentUID(c), bookID, iid, service.InstrumentInput{
		Name: req.Name, Type: req.Type, Market: req.Market, Code: req.Code,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, i)
}

func (h *investHandler) deleteInstrument(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	iid, good := pathID(c, "instrumentId", "标的 ID 非法")
	if !good {
		return
	}
	if err := h.svc.DeleteInstrument(currentUID(c), bookID, iid); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// setPrice 手动录入行情（MVP 唯一价格来源，PRD §12 决策 #3）。
func (h *investHandler) setPrice(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	iid, good := pathID(c, "instrumentId", "标的 ID 非法")
	if !good {
		return
	}
	var req struct {
		Price     string `json:"price"`
		PriceTime int64  `json:"priceTime"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.SetPrice(currentUID(c), bookID, iid, req.Price, req.PriceTime); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"updated": true})
}

// ---------- 持仓 ----------

func (h *investHandler) listHoldings(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	items, err := h.svc.ListHoldings(currentUID(c), bookID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"items": items})
}

type holdingReq struct {
	AccountId    int64  `json:"accountId"`
	InstrumentId int64  `json:"instrumentId"`
	Quantity     string `json:"quantity"`
	CostAmount   int64  `json:"costAmount"`
	CurrentPrice string `json:"currentPrice"`
}

func (h *investHandler) createHolding(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req holdingReq
	if !bindJSON(c, &req) {
		return
	}
	hd, err := h.svc.AddHolding(currentUID(c), bookID, service.HoldingInput{
		AccountId: req.AccountId, InstrumentId: req.InstrumentId,
		Quantity: req.Quantity, CostAmount: req.CostAmount, CurrentPrice: req.CurrentPrice,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, hd)
}

func (h *investHandler) updateHolding(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	hid, good := pathID(c, "holdingId", "持仓 ID 非法")
	if !good {
		return
	}
	var req holdingReq
	if !bindJSON(c, &req) {
		return
	}
	hd, err := h.svc.UpdateHolding(currentUID(c), bookID, hid, service.HoldingInput{
		Quantity: req.Quantity, CostAmount: req.CostAmount, CurrentPrice: req.CurrentPrice,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, hd)
}

func (h *investHandler) deleteHolding(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	hid, good := pathID(c, "holdingId", "持仓 ID 非法")
	if !good {
		return
	}
	if err := h.svc.DeleteHolding(currentUID(c), bookID, hid); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// ---------- 净资产（WBS 6.5） ----------

func (h *investHandler) netWorth(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	sum, err := h.svc.NetWorth(currentUID(c), bookID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, sum)
}
