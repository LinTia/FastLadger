package api

// 增量同步的 HTTP 适配层（阶段 3.1 / 3.2，契约见《落地方案》§5.1）。
//
//   - GET  /api/v1/books/:id/sync/pull?afterVersion=&limit=
//   - POST /api/v1/books/:id/sync/push  body {"items":[{...CreateRequest...}]}
//
// 约定：
//   - uid 只来自认证上下文，bookID 只来自可信路径参数；
//   - pull 失败一律经 mapSvcErr 映射（非成员 403、未初始化 400、内部错误 500）；
//   - push 的**每条结果**独立：整体 HTTP 200 + results 数组，客户端据此逐条重试，
//     这样"一条坏命令"不会卡住整个离线队列（WBS 阶段 3.2 验收点）。
import (
	"github.com/gin-gonic/gin"

	"fastledger/internal/service"
)

// syncHandler 同步 handler 集合
type syncHandler struct {
	svc *service.SyncService
}

func (h *syncHandler) pull(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	afterVersion, good := queryInt64(c, "afterVersion")
	if !good {
		return
	}
	limit, good := queryInt(c, "limit")
	if !good {
		return
	}
	batch, err := h.svc.Pull(currentUID(c), bookID, afterVersion, limit)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, batch)
}

// pushRequest push 请求体
//
// items 用 service.PushItem（与 CreateRequest 字段一致但**不带 binding 标签**），
// 否则 gin 的 `binding:"required"` 会把"某一条缺 clientUuid"升级成整批 400，
// 与"单项失败不影响后续项"的契约冲突。
type pushRequest struct {
	Items []service.PushItem `json:"items"`
}

func (h *syncHandler) push(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req pushRequest
	if !bindJSON(c, &req) {
		return
	}
	results, err := h.svc.Push(c.Request.Context(), currentUID(c), bookID, req.Items)
	if err != nil {
		// 只有"整批非法"（items 为空/超长/服务未初始化）会走到这里 → 400
		mapSvcErr(c, err)
		return
	}

	// 补传成功也要通知**其他**在线成员（否则对方只能等周期性 pull 才发现）。
	// 只取成功项里的最高版本发**一条** data_changed：WS 只表达"有新版本"，客户端收到后
	// 走一次授权 pull 就能拿到全部增量；逐条广播会把一次离线补传放大成上百条通知。
	var maxVersion int64
	for _, r := range results {
		if r.Status == service.PushStatusCreated && r.Version > maxVersion {
			maxVersion = r.Version
		}
	}
	if maxVersion > 0 {
		// 广播失败只记日志，绝不影响已经提交的补传结果
		notifyDataChanged(bookID, maxVersion)
	}

	ok(c, gin.H{"results": results})
}
