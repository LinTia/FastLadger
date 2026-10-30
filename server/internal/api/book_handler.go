package api

import (
	"net/http"
	"strconv"

	"fastledger/internal/model"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

type bookHandler struct {
	books   *service.BookService
	members *service.MemberService
}

// bookID 解析路径参数；非法返回 400
func bookID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "账本 ID 非法")
		return 0, false
	}
	return id, true
}

func (h *bookHandler) create(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		BookType int8   `json:"bookType"`
	}
	if !bindJSON(c, &req) {
		return
	}
	b, err := h.books.Create(currentUID(c), req.Name, req.BookType)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, b)
}

func (h *bookHandler) list(c *gin.Context) {
	books, err := h.books.List(currentUID(c))
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, books)
}

func (h *bookHandler) get(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	// 服务端判定成员资格：非成员看不到账本
	if _, err := h.members.RoleOf(id, currentUID(c)); err != nil {
		mapSvcErr(c, err)
		return
	}
	b, err := h.books.Get(id)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, b)
}

func (h *bookHandler) update(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	var req struct {
		Name                 string `json:"name"`
		MembersCanViewOthers *bool  `json:"membersCanViewOthers"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.books.Update(currentUID(c), id, req.Name, req.MembersCanViewOthers); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"id": id})
}

func (h *bookHandler) remove(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	if err := h.books.Delete(currentUID(c), id); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"id": id})
}

func (h *bookHandler) listMembers(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	if _, err := h.members.RoleOf(id, currentUID(c)); err != nil {
		mapSvcErr(c, err)
		return
	}
	list, err := h.members.List(id)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, list)
}

func (h *bookHandler) createInvite(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	var req struct {
		Role          int8   `json:"role"`
		BoundUsername string `json:"boundUsername"`
	}
	if !bindJSON(c, &req) {
		return
	}
	inv, err := h.members.CreateInvite(currentUID(c), id, req.Role, req.BoundUsername)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{
		"code":      inv.Code,
		"role":      inv.Role,
		"expiresAt": inv.ExpiresAt,
	})
}

func (h *bookHandler) join(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	var req struct {
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	// 传入 path 上的 bookID 与真实客户端 IP：
	// bookID 用于校验邀请确实属于该账本；IP 用于按 IP 限流（PRD §12.1，WB-R01）。
	u, err := h.members.JoinViaInvite(id, req.Code, req.Username, req.Password, c.ClientIP())
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, currentUserDTO(u))
}

func (h *bookHandler) changeRole(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		fail(c, http.StatusBadRequest, "成员 ID 非法")
		return
	}
	var req struct {
		Role int8 `json:"role"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.members.ChangeRole(currentUID(c), id, uid, req.Role); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"uid": uid, "role": req.Role})
}

func (h *bookHandler) removeMember(c *gin.Context) {
	id, good := bookID(c)
	if !good {
		return
	}
	uid, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil || uid <= 0 {
		fail(c, http.StatusBadRequest, "成员 ID 非法")
		return
	}
	if err := h.members.Remove(currentUID(c), id, uid); err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"uid": uid})
}

// 编译期引用，确保模型包被使用（避免误删 import）
var _ = model.RoleOwner
