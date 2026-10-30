package api

// 可见性开关的 HTTP 适配层：PUT /api/v1/books/:id/visibility
//
// 契约（PRD §7.3 / §12.1 决策 2~4）：
//   - 仅 owner / admin 可改（service 层判定，member → 403）；
//   - **不递增 sync_version**（可见性不是数据变更）；
//   - 变更后先刷新在线连接的可见性缓存，再广播 visibility_changed +
//     forceFullResync=true，客户端以 afterVersion=0 定向全量重拉该账本（不清空整个本地库）；
//   - 广播失败不影响已提交结果（notifyXxx 内部只记日志）。
import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// visibilityRequest 请求体；用指针以区分"未传字段"与"显式传 false"
type visibilityRequest struct {
	MembersCanViewOthers *bool `json:"membersCanViewOthers"`
}

func (h *bookHandler) setVisibility(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req visibilityRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.MembersCanViewOthers == nil {
		// 缺字段不是"设为 false"：显式拒绝，避免误把私账开关关掉/打开
		fail(c, http.StatusBadRequest, "缺少 membersCanViewOthers 字段")
		return
	}
	canView := *req.MembersCanViewOthers
	changed, err := h.books.SetVisibility(currentUID(c), bookID, canView)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 只有**实际变化**才通知（幂等重复提交不该让所有在线端白做一次全量重拉）：
	// 先 RefreshVisibility 重算缓存，再广播 visibility_changed + forceFullResync=true。
	if changed {
		notifyVisibilityChanged(bookID, currentBookVersion(h.books, bookID))
	}
	ok(c, gin.H{"id": bookID, "membersCanViewOthers": canView, "changed": changed})
}
