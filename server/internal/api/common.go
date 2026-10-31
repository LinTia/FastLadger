// Package api HTTP 层：路由、中间件与 handler。
//
// 统一响应：{ "code": 0, "msg": "ok", "data": {...} }；
// 错误时 code 与 HTTP 状态码语义一致（401/403/404/409/422/500），且不外露内部错误细节。
package api

import (
	"errors"
	"net/http"

	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// ok 成功响应
func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

// fail 失败响应（不回显内部错误）
func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "msg": msg})
}

// bindJSON 绑定并显式校验，绝不吞掉错误（D-R04 同类问题）
func bindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误")
		return false
	}
	return true
}

// mapSvcErr 把服务层错误映射为 HTTP 状态码
//
// 阶段 2 追加记账核心映射（《转账入口裁决》§2）：
// 缺键/非法命令 400、幂等键冲突或并发冲突 409、无成员权限 403、资源不存在 404。
func mapSvcErr(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, service.ErrForbidden),
		errors.Is(err, service.ErrNotMember),
		errors.Is(err, service.ErrCreateForbidden):
		fail(c, http.StatusForbidden, "无权限")
	case errors.Is(err, service.ErrCannotOperateOwner):
		fail(c, http.StatusForbidden, "不能对账本所有者执行该操作")
	case errors.Is(err, service.ErrCreateConflict):
		fail(c, http.StatusConflict, "同一 clientUuid 已用于不同内容，请检查请求或更换 clientUuid")
	case errors.Is(err, service.ErrEntityDeleted):
		// WBS 4.5：目标已删 → 409 ENTITY_DELETED，绝不静默复活
		fail(c, http.StatusConflict, "目标已被删除，无法对该数据进行操作")
	case errors.Is(err, service.ErrTxDirectionMissing):
		// 失败关闭：宁可报错也不静默算错余额。
		// GPT-C02 后该错误既覆盖"转账缺方向"也覆盖"单笔被污染方向"，
		// 文案必须中性，不得暗示一定是转账。
		fail(c, http.StatusConflict, "该交易记录的方向标记异常，无法删除或恢复，请联系实例管理员修复数据")
	case errors.Is(err, service.ErrTransferGroupIncomplete):
		// GPT-R26：转账配对损坏 → 整笔拒绝，绝不退化成单边操作
		fail(c, http.StatusConflict, "该转账记录配对不完整，无法整笔删除或恢复，请联系实例管理员修复数据")
	case errors.Is(err, service.ErrIncompleteReceipt):
		fail(c, http.StatusConflict, "该 clientUuid 已有不完整回执，请更换 clientUuid 或联系实例管理员处理")
	case errors.Is(err, service.ErrConcurrentConflict):
		fail(c, http.StatusConflict, "数据已被并发修改，请重试")
	case errors.Is(err, service.ErrBookNotFound):
		fail(c, http.StatusNotFound, "账本不存在")
	case errors.Is(err, service.ErrAccountNotFound):
		fail(c, http.StatusNotFound, "账户不存在")
	case errors.Is(err, service.ErrCategoryNotFound):
		fail(c, http.StatusNotFound, "分类不存在")
	case errors.Is(err, service.ErrTransactionNotFound):
		fail(c, http.StatusNotFound, "交易不存在")
	case errors.Is(err, service.ErrInvalidCreate),
		errors.Is(err, service.ErrInvalidArgument),
		errors.Is(err, service.ErrAccountInvalid),
		errors.Is(err, service.ErrCategoryInvalid):
		fail(c, http.StatusBadRequest, "请求参数非法")
	case errors.Is(err, service.ErrAccountCycle):
		// GPT-R47：任意长度的层级环（含"上级指向自身"）→ 400，客户端应换一个上级
		fail(c, http.StatusBadRequest, "账户层级不能成环：上级不能是自己的下级")
	case errors.Is(err, service.ErrAccountInUse):
		fail(c, http.StatusConflict, "账户仍存在交易记录，无法删除")
	case errors.Is(err, service.ErrAccountHasChildren):
		// GPT-R53：删父账户会留下悬空 parent_id
		fail(c, http.StatusConflict, "该账户下仍有子账户，无法删除")
	case errors.Is(err, service.ErrCategoryInUse):
		fail(c, http.StatusConflict, "分类仍存在交易记录，无法删除")
	case errors.Is(err, service.ErrCategoryHasChildren):
		fail(c, http.StatusConflict, "该分类下仍有子分类，无法删除")
	case errors.Is(err, service.ErrUserExists):
		fail(c, http.StatusConflict, "用户名已存在")
	case errors.Is(err, service.ErrInviteInvalid):
		fail(c, http.StatusBadRequest, "邀请码无效或已过期")
	case errors.Is(err, service.ErrInviteBoundMismatch):
		fail(c, http.StatusBadRequest, "邀请码已绑定指定用户，信息不匹配")
	case errors.Is(err, service.ErrBookFull):
		// PRD §12 决策 6：成员上限 20，属于"当前状态不允许"而非"请求非法" → 409
		fail(c, http.StatusConflict, "该账本成员已达上限（20）")
	case errors.Is(err, service.ErrRateLimited):
		fail(c, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
	case errors.Is(err, service.ErrWeakPassword):
		fail(c, http.StatusBadRequest, "密码至少 8 位")
	case errors.Is(err, service.ErrPasswordTooLong):
		// GPT-R45：bcrypt 最多接受 72 字节。这是客户端输入问题，必须 400 + 可行动文案，
		// 绝不能落到 default 变成 500（用户只会看到"服务内部错误"，不知道该改短密码）。
		fail(c, http.StatusBadRequest, "密码过长：最多 72 字节（超出部分 bcrypt 会丢弃）")
	case errors.Is(err, service.ErrUsernameTooLong):
		// GPT-R45：用户名按字符计 1~64，且 UTF-8 字节不超过 255
		fail(c, http.StatusBadRequest, "用户名长度超限：1~64 个字符")
	case errors.Is(err, service.ErrAuthUnavailable):
		// GPT-R46：认证依赖不可用（DB / 审计 / 令牌签发）→ 503，可重试
		fail(c, http.StatusServiceUnavailable, "服务暂时不可用，请稍后重试")
	default:
		// 不回显内部错误，避免泄露实现细节
		fail(c, http.StatusInternalServerError, "服务内部错误")
	}
}
