package api

// 长期 API Token（PAT）HTTP 适配层（WBS 6.6 / PRD §187 §296）。
//
// 契约（全部挂在 JWTAuth 组内，PAT 与 JWT 均可调用——PAT 也能管理 PAT）：
//
//	GET    /api/v1/users/me/tokens        列出本人全部 Token（含已撤销；绝不含哈希）
//	POST   /api/v1/users/me/tokens        生成新 Token；明文**仅在本响应出现一次**
//	DELETE /api/v1/users/me/tokens/:id    撤销本人指定 Token（跨用户/不存在 → 404）
//
// 安全口径：
//   - 明文一次性下发（同 GitHub PAT 语义），前端必须提示"关闭后无法再查看"；
//   - 撤销立即生效：下一个请求经 JWTAuth 的哈希查找自然 401；WS 长连接在
//     下一次握手时被拒（存量连接不受影响，与"撤销会话踢 WS"不同——PAT 没有
//     jti，无法定位在线连接；快捷指令场景可接受，见 CHANGELOG [1.53]）；
//   - 请求体 name 仅展示用途，不参与任何鉴权决策。

import (
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// patHandler 长期 API Token handler
type patHandler struct {
	svc *service.PATService
}

// list GET /api/v1/users/me/tokens
func (h *patHandler) list(c *gin.Context) {
	items, err := h.svc.List(currentUID(c))
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, items)
}

// create POST /api/v1/users/me/tokens  {"name": "快捷指令"}
func (h *patHandler) create(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &req) {
		return
	}
	plaintext, item, err := h.svc.Create(currentUID(c), req.Name)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{
		"token": plaintext, // 仅此一次；此后库里只有哈希
		"item":  item,
	})
}

// revoke DELETE /api/v1/users/me/tokens/:id
func (h *patHandler) revoke(c *gin.Context) {
	id, good := pathID(c, "tokenId", "Token 不存在")
	if !good {
		return
	}
	if err := h.svc.Revoke(currentUID(c), id); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"revoked": true})
}
