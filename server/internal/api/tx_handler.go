package api

// 交易 HTTP 适配：唯一的创建路径 POST /books/:id/transactions（收入/支出/转账共用）。
//
// 说明：
//   - 不提供独立的 /transfer 入口（《转账入口裁决》§5：禁止第二条创建路径）；
//   - PUT / DELETE 交易属于阶段 4「删除 / 恢复 / 审计」，本阶段只做创建与查询。
import (
	"net/http"

	"fastledger/internal/model"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *ledgerHandler) createTransaction(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req service.CreateRequest
	if !bindJSON(c, &req) {
		return
	}
	// uid 来自认证上下文，bookID 来自可信路径；请求体不接收覆盖字段
	res, err := h.txs.Create(c.Request.Context(), currentUID(c), bookID, req)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 通知在线成员"有新版本，去 pull"（不携带明细；失败只记日志，不能反转已提交结果）
	//
	// 幂等重放（Replayed=true）**不广播**：那次变更在首次提交时已经通知过，
	// 重复事件会让所有在线端白拉一次（与 syncHandler.push 的口径一致）。
	if !res.Replayed {
		notifyDataChanged(bookID, res.Version)
	}
	ok(c, res)
}

func (h *ledgerHandler) listTransactions(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	maxTime, good := queryInt64(c, "maxTime")
	if !good {
		return
	}
	minTime, good := queryInt64(c, "minTime")
	if !good {
		return
	}
	txType, good := queryInt(c, "type")
	if !good {
		return
	}
	if txType < 0 || txType > int(model.TxTypeTransfer) {
		fail(c, http.StatusBadRequest, "查询参数 type 非法")
		return
	}
	accountID, good := queryInt64(c, "accountId")
	if !good {
		return
	}
	categoryID, good := queryInt64(c, "categoryId")
	if !good {
		return
	}
	page, good := queryInt(c, "page")
	if !good {
		return
	}
	count, good := queryInt(c, "count")
	if !good {
		return
	}

	list, total, err := h.txs.List(currentUID(c), bookID, service.TxListQuery{
		MaxTime:    maxTime,
		MinTime:    minTime,
		TxType:     int8(txType),
		AccountId:  accountID,
		CategoryId: categoryID,
		Keyword:    c.Query("keyword"),
		Page:       page,
		Count:      count,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 回显归一化后的分页参数，保证与真正生效的值一致（请求 count=1000 实际只返回 200）
	effPage, effCount := service.NormalizePaging(page, count)
	ok(c, gin.H{"items": list, "total": total, "page": effPage, "count": effCount})
}

func (h *ledgerHandler) getTransaction(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	txID, good := pathID(c, "txId", "交易 ID 非法")
	if !good {
		return
	}
	tx, err := h.txs.Get(currentUID(c), bookID, txID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, tx)
}
