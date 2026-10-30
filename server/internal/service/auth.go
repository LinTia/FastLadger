package service

// 认证：注册 / 登录 / 口令哈希。
//
// 约束（R08/R09 与 PRD §12.1）：
//   - 口令用 bcrypt（自带盐，不另设 salt 列）；
//   - 登录须校验账号是否已删除 / 禁用；
//   - 首个创建的用户自动成为实例管理员（实例管理员 ≠ 账本 owner）。
import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fastledger/internal/model"

	"golang.org/x/crypto/bcrypt"
	"xorm.io/xorm"
)

var (
	// ErrBadCredentials 用户名或口令错误（不区分，避免枚举）
	ErrBadCredentials = errors.New("用户名或密码错误")
	// ErrAccountDisabled 账号已禁用
	ErrAccountDisabled = errors.New("账号已禁用")
	// ErrUserExists 用户名已存在
	ErrUserExists = errors.New("用户名已存在")
	// ErrWeakPassword 口令强度不足
	ErrWeakPassword = errors.New("密码强度不足：至少 8 位")
)

const minPasswordLen = 8

// AuthService 认证服务
type AuthService struct {
	engine *xorm.Engine
	secret string
	ttl    time.Duration
}

// NewAuthService 构造认证服务（secret 由配置注入，已在 config 层 fail-fast 校验）
func NewAuthService(engine *xorm.Engine, secret string, ttl time.Duration) *AuthService {
	return &AuthService{engine: engine, secret: secret, ttl: ttl}
}

// Register 注册；若是系统首个用户，自动授予实例管理员
func (s *AuthService) Register(username, password string) (*model.User, error) {
	uname, hash, err := normalizeAndHash(username, password)
	if err != nil {
		return nil, err
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	u, err := s.insertUserInTx(sess, uname, hash)
	if err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return u, nil
}

// normalizeAndHash 校验用户名/口令并生成 bcrypt 哈希。
//
// 刻意放在事务之外：bcrypt 是纯 CPU 开销（默认成本约数十毫秒），
// 若放进事务会长时间占住唯一的数据库连接（MaxOpenConns=1），拖慢所有并发请求。
func normalizeAndHash(username, password string) (string, string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		// 归入 ErrInvalidArgument：由 mapSvcErr 映射为 400，而不是 500
		return "", "", fmt.Errorf("%w：用户名不能为空", ErrInvalidArgument)
	}
	if len(password) < minPasswordLen {
		return "", "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("口令哈希失败：%w", err)
	}
	return username, string(hash), nil
}

// insertUserInTx 在**调用方的事务**内建用户，不自行 Begin/Commit。
//
// JoinViaInvite 复用它，使「建 user + 写 book_members + 标记邀请已用」落在同一事务内
// （D-R07：原实现调用会自行提交的 Register，第二步失败时留下孤儿用户，无法回滚）。
func (s *AuthService) insertUserInTx(sess *xorm.Session, username, passwordHash string) (*model.User, error) {
	var exist int64
	if _, err := sess.SQL("SELECT COUNT(1) FROM users WHERE username = ? AND deleted = 0", username).
		Get(&exist); err != nil {
		return nil, err
	}
	if exist > 0 {
		return nil, ErrUserExists
	}

	// 首个用户 → 实例管理员（之后由实例管理员转移，见 PRD §12.1）
	var total int64
	if _, err := sess.SQL("SELECT COUNT(1) FROM users").Get(&total); err != nil {
		return nil, err
	}

	u := &model.User{
		Username:        username,
		PasswordHash:    passwordHash,
		IsInstanceAdmin: total == 0,
	}
	if _, err := sess.Insert(u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login 校验口令并签发 token（含 jti）
func (s *AuthService) Login(username, password string) (string, error) {
	var u model.User
	has, err := s.engine.Where("username = ? AND deleted = 0", strings.TrimSpace(username)).Get(&u)
	if err != nil {
		return "", err
	}
	if !has {
		return "", ErrBadCredentials
	}
	if u.Disabled {
		return "", ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", ErrBadCredentials
	}
	return SignJWT(s.secret, u.Id, s.ttl)
}

// GetByID 按 ID 取有效用户（已删除返回 ErrBadCredentials）
func (s *AuthService) GetByID(uid int64) (*model.User, error) {
	var u model.User
	has, err := s.engine.ID(uid).Where("deleted = 0").Get(&u)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrBadCredentials
	}
	return &u, nil
}

// Verify 校验 token 且确认账号仍有效（删除/禁用即失效）
func (s *AuthService) Verify(token string) (*model.User, error) {
	claims, err := ParseJWT(s.secret, token)
	if err != nil {
		return nil, err
	}
	var u model.User
	has, err := s.engine.ID(claims.Uid).Where("deleted = 0").Get(&u)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrBadCredentials
	}
	if u.Disabled {
		return nil, ErrAccountDisabled
	}
	return &u, nil
}
