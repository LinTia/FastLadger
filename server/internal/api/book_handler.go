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

// update 修改账本设置（名称 / 可见性开关，`PUT /books/:id`）——文档既有入口。
//
// 可见性写入与 `PUT /books/:id/visibility` 共用**同一个** BookService.SetVisibility
// （单一实现、两个入口，见《落地方案》§5.2 不变量）。本 handler 只负责：
// 把"是否真的变了"（res.VisibilityChanged）翻译成一次广播——
// **只在条件分支内广播**，值没变不播（否则所有在线端白做一次 afterVersion=0 全量重拉）。
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
	res, err := h.books.Update(currentUID(c), id, req.Name, req.MembersCanViewOthers)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	if res.VisibilityChanged {
		notifyVisibilityChanged(id, currentBookVersion(h.books, id))
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
	// 已提交 → 通知在线成员清空该账本本地数据并停止订阅（失败只记日志）
	// 版本从 book_versions 读（账本是软删，版本水位仍在，信封要求带 version）
	notifyBookDeleted(id, currentBookVersion(h.books, id))
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
	// GPT-R41：boundEmail 与 boundUsername 二选一（都为空 = 不绑定）。
	// 两者同时传入由 service 层 resolveInviteBinding 显式拒绝（400），此处不做取舍。
	var req struct {
		Role          int8   `json:"role"`
		BoundUsername string `json:"boundUsername"`
		BoundEmail    string `json:"boundEmail"`
	}
	if !bindJSON(c, &req) {
		return
	}
	inv, err := h.members.CreateInvite(currentUID(c), id, req.Role, req.BoundUsername, req.BoundEmail)
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
	// GPT-R41：email 是邀请绑定邮箱时的兑换凭据；未绑定邮箱的邀请可以留空，
	// 一旦提供了就会写进用户档案（缺失会造成"校验通过但档案无邮箱"的半截状态）。
	var req struct {
		Code     string `json:"code"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	// 传入 path 上的 bookID 与真实客户端 IP：
	// bookID 用于校验邀请确实属于该账本；IP 用于按 IP 限流（PRD §12.1，WB-R01）。
	u, err := h.members.JoinViaInvite(id, req.Code, req.Username, req.Email, req.Password, c.ClientIP())
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
	// 已提交 → 角色变化可能改变可见范围，要求定向全量重拉（失败只记日志）
	notifyMemberRoleChanged(id, currentBookVersion(h.books, id), uid, req.Role)
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
	// 已提交 → 被移除成员应主动清除该账本本地数据（失败只记日志）
	notifyMemberRemoved(id, currentBookVersion(h.books, id), uid)
	ok(c, gin.H{"uid": uid})
}

// 编译期引用，确保模型包被使用（避免误删 import）
var _ = model.RoleOwner
