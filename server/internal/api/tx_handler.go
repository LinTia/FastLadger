package api

// 交易 HTTP 适配：唯一的创建路径 POST /books/:id/transactions（收入/支出/转账共用），
// 以及阶段 4 的删除（DELETE）/ 恢复（POST .../restore）——软删、余额精确反向、版本递增、审计。
//
// 说明：
//   - 不提供独立的 /transfer 入口（《转账入口裁决》§5：禁止第二条创建路径）；
//   - 删除/恢复在服务层统一实现（mutateVisibility），handler 仅做鉴权与版本通知。
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

// deleteTransaction 阶段 4：删除一笔交易（整笔，含转账对侧）。
// 软删 + 余额精确反向 + 版本递增 + 审计 + 幂等回执，均在服务层完成。
//
// clientUuid 经查询参数传入（DELETE 携带 body 在部分代理/客户端下不可靠）；
// 缺失即 400（服务层强制，避免重试把余额反向两次）。
//
// GPT-C04：与 createTransaction 口径一致，幂等重放（replayed=true）**不广播**——
// 那次变更在首次提交时已经通知过，重复事件会让所有在线端白拉一次。
// C01 修复后，终态不匹配的重放会走 404（mapSvcErr），天然不广播。
func (h *ledgerHandler) deleteTransaction(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	txID, good := pathID(c, "txId", "交易 ID 非法")
	if !good {
		return
	}
	ver, replayed, err := h.txs.DeleteReceipt(c.Request.Context(), currentUID(c), bookID, txID, c.Query("clientUuid"))
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 通知在线成员拉取（删除也是版本递增事件；失败只记日志，不反转结果）
	if !replayed {
		notifyDataChanged(bookID, ver)
	}
	ok(c, gin.H{"version": ver})
}

// restoreTransaction 阶段 4：恢复一笔已软删的交易（整笔）。与删除对称。
//
// GPT-C04：幂等重放（replayed=true）不广播，口径同 deleteTransaction / createTransaction。
func (h *ledgerHandler) restoreTransaction(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	txID, good := pathID(c, "txId", "交易 ID 非法")
	if !good {
		return
	}
	ver, replayed, err := h.txs.RestoreReceipt(c.Request.Context(), currentUID(c), bookID, txID, c.Query("clientUuid"))
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	if !replayed {
		notifyDataChanged(bookID, ver)
	}
	ok(c, gin.H{"version": ver})
}

// listRecycleBin 回收站视图（WBS 4.1）：列出本账本已软删的交易（墓碑）。
//
// 路径刻意取 /books/:id/recycle 而非 /books/:id/transactions/recycle：
// 后者与已注册的 GET /books/:id/transactions/:txId 的通配段冲突，gin 注册时会 panic。
func (h *ledgerHandler) listRecycleBin(c *gin.Context) {
	bookID, good := bookID(c)
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
	list, total, err := h.txs.ListDeleted(currentUID(c), bookID, page, count)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	effPage, effCount := service.NormalizePaging(page, count)
	ok(c, gin.H{"items": list, "total": total, "page": effPage, "count": effCount})
}
