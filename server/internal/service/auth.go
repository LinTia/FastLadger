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
	"unicode/utf8"

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
	// ErrPasswordTooLong 口令超过 bcrypt 能接受的字节数（GPT-R45）。
	//
	// bcrypt 只取输入的前 72 字节，超出的部分会被静默丢弃——那意味着"用户以为设了一个
	// 很长的密码，实际生效的只有前 72 字节"。golang.org/x/crypto/bcrypt 为此直接返回
	// ErrPasswordTooLong，旧实现把它当成通用错误一路冒泡，最终在 HTTP 层变成 500。
	// 现在在哈希【之前】显式校验，返回 400，且不会有任何一次 DB 写入。
	ErrPasswordTooLong = errors.New("密码过长：最多 72 字节")
	// ErrUsernameTooLong 用户名超出长度上限（GPT-R45）。
	//
	// 0001_init.up.sql 里 username 是 VARCHAR(64)，但 SQLite **不强制** VARCHAR 的长度
	//（按 TEXT 亲和性存储），所以超长用户名能一路落库，之后再遇到索引、审计、日志、
	// 迁移对比都会踩到"超出 schema 预期"的坑。上限必须由应用层兜住。
	ErrUsernameTooLong = errors.New("用户名长度超限：1~64 个字符")
	// ErrAuthUnavailable 认证依赖不可用（GPT-R46）。
	//
	// 与 ErrBadCredentials 严格区分：数据库故障、登录审计写入失败、令牌签发失败都属于
	// **服务端故障**，绝不能回落成 401——否则客户端会以为"用户名或密码错了"并反复重试，
	// 而真正的故障被 401 掩盖；HTTP 层据此返回 503 并记安全日志。
	ErrAuthUnavailable = errors.New("认证服务暂时不可用")
)

const (
	// minPasswordLen 口令最小长度。**按字节计**（既有行为，不改）：
	// 8 个 ASCII 字符 = 8 字节；中文等宽字符同样按字节参与，故 3 个汉字（9 字节）也满足。
	// 这里刻意不改成按字符计——收紧会让已注册用户的既有口令"变短"，属于破坏性变更。
	minPasswordLen = 8
	// maxPasswordBytes 口令最大长度，**按字节计**：这是 bcrypt 的硬上限（72 字节），
	// 不是产品决策。字符数口径在这里没有意义——bcrypt 吃的是字节。
	maxPasswordBytes = 72
	// maxUsernameRunes 用户名最大**字符**数（Unicode 码点），对齐 schema 的 VARCHAR(64)。
	maxUsernameRunes = 64
	// maxUsernameBytes 用户名最大**字节**数（UTF-8 编码后）。
	//
	// 为什么字符数之外还要卡字节：64 个 4 字节 emoji 是 256 字节，字符数合法但字节数
	// 已经超过常见索引/日志的舒适区。取 255 作为兜底，正常中文名（64×3=192 字节）不受影响。
	maxUsernameBytes = 255
)

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
//
// 顺序即契约（GPT-R45）：**所有输入校验都在开事务之前完成**，因此任何一条校验失败都
// 是"零写入"——不存在"建了用户才发现口令超长"的中间态，也就不可能落库。
func normalizeAndHash(username, password string) (string, string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		// 归入 ErrInvalidArgument：由 mapSvcErr 映射为 400，而不是 500
		return "", "", fmt.Errorf("%w：用户名不能为空", ErrInvalidArgument)
	}
	if err := validateUsername(username); err != nil {
		return "", "", err
	}
	if len(password) < minPasswordLen {
		return "", "", ErrWeakPassword
	}
	// bcrypt 的 72 字节上限必须在哈希之前拦住：否则 bcrypt 返回它自己的
	// ErrPasswordTooLong，被当成"服务内部错误"冒到 HTTP 层变成 500，而客户端
	// 真正需要的是"请把密码改短一点"的 400。
	if len(password) > maxPasswordBytes {
		return "", "", fmt.Errorf("%w：实际 %d 字节", ErrPasswordTooLong, len(password))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("口令哈希失败：%w", err)
	}
	return username, string(hash), nil
}

// validateUsername 用户名边界校验（GPT-R45）。
//
// 口径（写死在代码里，避免"字符还是字节"含糊）：
//  1. 主口径【字符】：TrimSpace 之后 1~maxUsernameRunes(64) 个 Unicode 码点，
//     对齐 schema 的 VARCHAR(64) 与 normalizeName 的既有口径；
//  2. 兜底口径【字节】：UTF-8 编码后不超过 maxUsernameBytes(255)。
//
// 两条都按"客户端输入错误"处理 → HTTP 400，且调用点在事务之前，不会落库。
func validateUsername(username string) error {
	if n := utf8.RuneCountInString(username); n > maxUsernameRunes {
		return fmt.Errorf("%w：实际 %d 个字符", ErrUsernameTooLong, n)
	}
	if b := len(username); b > maxUsernameBytes {
		return fmt.Errorf("%w：UTF-8 编码后 %d 字节", ErrUsernameTooLong, b)
	}
	return nil
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

// Login 校验口令并签发 token（含 jti）。
//
// 错误分为两类，HTTP 层据此分岔（GPT-R46）：
//   - 凭证类（ErrBadCredentials / ErrAccountDisabled）→ 统一 401，不区分"账号不存在"与
//     "口令错误"，避免账号枚举；
//   - 依赖类（ErrAuthUnavailable）→ 503：数据库故障 / 登录审计写入失败 / 令牌签发失败。
//     这类故障必须显式标注，否则会被 401 吞掉（旧实现把所有错误一律 401）。
func (s *AuthService) Login(username, password string) (string, error) {
	var u model.User
	has, err := s.engine.Where("username = ? AND deleted = 0", strings.TrimSpace(username)).Get(&u)
	if err != nil {
		// 数据库故障 ≠ 口令错误：标成依赖不可用，由 HTTP 层返回 503 + 安全日志
		return "", fmt.Errorf("%w：查询用户失败：%w", ErrAuthUnavailable, err)
	}
	if !has {
		return "", ErrBadCredentials
	}
	if u.Disabled {
		return "", ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		// 只有"口令不匹配"才算凭证错误；哈希本身损坏（长度不对/格式非法）是数据故障，
		// 归到 503 才不会让运维误以为是用户输错密码。
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return "", ErrBadCredentials
		}
		return "", fmt.Errorf("%w：校验口令失败：%w", ErrAuthUnavailable, err)
	}
	// 审计（WBS 4.3）：登录是实例级事件，不隶属任何账本，book_id 取哨兵 0。
	// 刻意放在签发令牌【之前】：审计写失败就不签发，避免出现"已通过认证却无留痕"。
	//
	// GPT-R46 协调口径：审计写失败是**服务端故障**，不是"凭证不对"。
	// 若把它归 401，客户端会提示"用户名或密码错误"并诱导用户反复重试，
	// 而真正的故障（审计表写不进去）被彻底掩盖。
	if err := AppendAudit(s.engine, model.AuditBookInstance, u.Id,
		model.AuditActionLogin, model.AuditEntityUser, u.Id, nil, "server"); err != nil {
		return "", fmt.Errorf("%w：登录审计写入失败，已拒绝签发令牌：%w", ErrAuthUnavailable, err)
	}
	token, err := SignJWT(s.secret, u.Id, s.ttl)
	if err != nil {
		return "", fmt.Errorf("%w：签发令牌失败：%w", ErrAuthUnavailable, err)
	}
	return token, nil
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
	_, u, err := s.VerifyWithClaims(token)
	return u, err
}

// VerifyWithClaims 同 Verify，额外返回解析出的 claims。
//
// 用途（GPT-R51）：WebSocket 是长连接，握手之后不再经过任何中间件，
// 服务端必须在握手时把 JWT 的 exp 交下去，让连接有一个生命周期上限。
// 独立成方法而不是改 Verify 的签名，是为了不动既有调用点；两者判定逻辑完全同一份。
func (s *AuthService) VerifyWithClaims(token string) (*Claims, *model.User, error) {
	claims, err := ParseJWT(s.secret, token)
	if err != nil {
		return nil, nil, err
	}
	var u model.User
	has, err := s.engine.ID(claims.Uid).Where("deleted = 0").Get(&u)
	if err != nil {
		return nil, nil, err
	}
	if !has {
		return nil, nil, ErrBadCredentials
	}
	if u.Disabled {
		return nil, nil, ErrAccountDisabled
	}
	return claims, &u, nil
}
