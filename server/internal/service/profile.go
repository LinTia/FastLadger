package service

// 个人资料与账号安全（WBS 5.x 客户端支撑 / PRD §4.12「我的 / 设置」）。
//
// 范围口径（如实说明）：
//   - 邮箱可更新（唯一约束由 idx_users_email 部分唯一索引兜底，此处先查后写
//     给出可读的 409）；
//   - **用户名不可改**——它是登录标识与审计/同步的归属锚点，MVP 不开放
//     （PRD §4.12 只说"个人资料"，未承诺改用户名；改名的涟漪面见下）；
//   - 密码修改：验证旧口令 → 新口令沿用注册强度规则（normalizeAndHash 同一套）
//     → 更新哈希 → **撤销该用户全部会话**。改密不撤销会话等于"改了锁没换钥匙"：
//     已泄露的旧令牌在 exp 内仍然有效，这是账号安全的基本要求。
//
// 长期 API Token（PAT）不受改密影响：它按设计"不随会话/口令失效"
// （PRD §187 长期凭证语义），用户可随时在设置页单独撤销。

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fastledger/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// ErrEmailTaken 邮箱已被其他账号占用（HTTP 409）
var ErrEmailTaken = errors.New("该邮箱已被其他账号使用")

// maxEmailBytes 与 0001 列定义（VARCHAR(128)）一致
const maxEmailBytes = 128

// UpdateEmail 更新本人邮箱。email 允许为空串（清除邮箱）——
// idx_users_email 是部分唯一索引（WHERE email <> ”），空值不参与唯一约束。
func (s *AuthService) UpdateEmail(uid int64, email string) (*model.User, error) {
	email = strings.TrimSpace(email)
	if email != "" {
		if len(email) > maxEmailBytes {
			return nil, fmt.Errorf("%w：邮箱不超过 %d 字节", ErrInvalidArgument, maxEmailBytes)
		}
		// 宽松格式校验：必须含一个 @ 且不含空白（严格 RFC 校验交给发送验证，MVP 不做）
		if strings.Count(email, "@") != 1 || strings.ContainsAny(email, " \t\r\n") {
			return nil, fmt.Errorf("%w：邮箱格式不正确", ErrInvalidArgument)
		}
		// 先查后写：唯一索引兜底之外给出可读错误（不区分"占用者是谁"）
		var dup model.User
		has, err := s.engine.Where("email = ? AND id <> ? AND deleted = 0", email, uid).Get(&dup)
		if err != nil {
			return nil, err
		}
		if has {
			return nil, ErrEmailTaken
		}
	}
	if _, err := s.engine.ID(uid).Cols("email").Update(&model.User{Email: email}); err != nil {
		return nil, err
	}
	return s.GetByID(uid)
}

// ChangePassword 修改本人口令：验证旧口令 → 新口令走注册同款校验 →
// 更新哈希 → 撤销全部会话（含当前会话——客户端应引导重新登录）。
// 返回撤销的会话数（供响应如实告知"已下线 N 台设备"）。
func (s *AuthService) ChangePassword(uid int64, oldPassword, newPassword string) (int, error) {
	var u model.User
	has, err := s.engine.ID(uid).Where("deleted = 0").Get(&u)
	if err != nil {
		return 0, err
	}
	if !has {
		return 0, ErrBadCredentials
	}
	// 旧口令校验（bcrypt.CompareHashAndPassword 恒定时间比较语义由库保证）
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
		return 0, ErrBadCredentials
	}
	// 新口令强度：与注册同一套口径（最短长度 / bcrypt 72 字节上限）
	if len(newPassword) < minPasswordLen {
		return 0, ErrWeakPassword
	}
	if len(newPassword) > maxPasswordBytes {
		return 0, fmt.Errorf("%w：实际 %d 字节", ErrPasswordTooLong, len(newPassword))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("口令哈希失败：%w", err)
	}
	if _, err := s.engine.ID(uid).Cols("password_hash").Update(&model.User{PasswordHash: string(hash)}); err != nil {
		return 0, err
	}
	// 撤销全部会话（keepJti 为空 = 全撤，含当前会话——改密后统一重新登录）
	now := time.Now().Unix()
	revoked, err := s.RevokeOtherSessions(uid, "", now)
	if err != nil {
		return 0, err
	}
	return len(revoked), nil
}
