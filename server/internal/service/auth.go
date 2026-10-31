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
	"log"
	"net/netip"
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
	// DefaultLoginMaxAttempts 登录失败限流默认配额：15 分钟窗口内 10 次失败（A6）。
	//
	// 窗口长度的口径：**窗口 = 被打满后的瘫痪时长**（fail-closed 容量语义下，攻击者灌满
	// 的 key 要等窗口过期才回收）。邀请兑换的 1 小时窗口是"每 IP 每小时 10 次"的产品口径，
	// 直接照抄会把"爆破者被拦"放大成"该 IP 上所有用户 1 小时进不来"，故取 15 分钟。
	DefaultLoginMaxAttempts = 10
	// DefaultLoginWindowMinutes 登录失败限流默认窗口（分钟），见 DefaultLoginMaxAttempts。
	DefaultLoginWindowMinutes = 15
	// maxAttemptedUserLen 登录失败审计里 attempted 字段的最大字符数。
	// 用户名是客户端可控输入，进审计前必须截断，防止超长串把 before_json 撑爆。
	maxAttemptedUserLen = 64
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

	// 登录失败限流（A6）：**独立实例**，绝不复用 MemberService.limiter——
	// 那个实例被邀请签发/兑换共用一个 key 空间，塞进登录会让"登录被爆破"与
	// "邀请被灌爆"互相挤占容量，形成跨功能 DoS 耦合。
	loginLimiter *Limiter
	// loginMaxAttempts / loginWindow 失败配额与窗口；默认 10 次 / 15 分钟，
	// 可经 ConfigureLoginThrottle 按 EBK_LOGIN_MAX_ATTEMPTS / EBK_LOGIN_WINDOW_MINUTES 覆盖。
	loginMaxAttempts int
	loginWindow      time.Duration
}

// NewAuthService 构造认证服务（secret 由配置注入，已在 config 层 fail-fast 校验）
func NewAuthService(engine *xorm.Engine, secret string, ttl time.Duration) *AuthService {
	return &AuthService{
		engine:           engine,
		secret:           secret,
		ttl:              ttl,
		loginLimiter:     NewLimiter(),
		loginMaxAttempts: DefaultLoginMaxAttempts,
		loginWindow:      DefaultLoginWindowMinutes * time.Minute,
	}
}

// ConfigureLoginThrottle 覆盖登录失败限流参数（装配层接线用，启动期调用一次）。
//
// 参数来自 config（EBK_LOGIN_MAX_ATTEMPTS / EBK_LOGIN_WINDOW_MINUTES，已 fail-fast 校验
// 为正数）；这里再防御一次：非法值直接 panic——带着"0 次配额"或"0 时长窗口"运行
// 等于把限流整个关掉，宁可不启动。
func (s *AuthService) ConfigureLoginThrottle(maxAttempts int, window time.Duration) {
	if maxAttempts <= 0 || window <= 0 {
		panic(fmt.Sprintf("登录限流参数非法：maxAttempts=%d window=%v", maxAttempts, window))
	}
	s.loginMaxAttempts = maxAttempts
	s.loginWindow = window
}

// loginKey 登录限流的桶键：按【来源网段】聚合分桶——IPv4 按 /24、IPv6 按 /64（R-1）。
//
// 为什么按前缀聚合而不是完整地址（对抗验证 ZZ-2 实测复现）：IPv6 隐私扩展（RFC 4941）
// 与代理池让一台机器在自己 /64 内可拥有海量地址；按完整 ClientIP 分桶等于"每换一个
// 地址就是全新配额"，15 分钟 10 次的配额被 /64 内的地址数摊薄，登录限流在 IPv6 环境
// 下形同虚设。聚合口径与 member.go 的 ipBucket 一致（/24、/64），但实现各自独立、
// 互不引用（文件所有权隔离）。
//
// 代价（明示）：同一 /24（IPv4）或 /64（IPv6）内的多台设备共享一个失败配额。
// 家庭自托管场景下同一出口路由器后面的设备本来就该算同一个来源，可接受；
// 口径已写进《落地方案》§8.4.2。
//
// clientIP 来自 gin 的 ClientIP()——router.go 已 SetTrustedProxies(nil)，
// 它恒等于 TCP 对端（RemoteAddr），不可被 X-Forwarded-For 单方面操纵。
// 不可解析的值（正常不应出现）统一归 invalid 共享桶：fail-closed，
// 绝不会因为解析失败而绕过限流。
func loginKey(clientIP string) string {
	ip, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		return "login:invalid"
	}
	ip = ip.Unmap() // IPv4-mapped IPv6（::ffff:203.0.113.9）按 IPv4 口径聚合
	bits := 64
	if ip.Is4() {
		bits = 24
	}
	return "login:" + netip.PrefixFrom(ip, bits).Masked().String()
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
// 兼容入口：不携带来源 IP 的调用方（单测 / CLI 直调）统一归入 unknown 桶。
// 生产 HTTP 入口必须走 LoginFromIP（按真实 ClientIP 分桶）。
func (s *AuthService) Login(username, password string) (string, error) {
	return s.LoginFromIP(username, password, "")
}

// LoginFromIP 校验口令并签发 token（含 jti），按来源网段做失败限流（A6 / R-1）。
//
// 执行顺序即契约：
//  1. **限流判定在最前**：超限直接 ErrRateLimited（HTTP 429）——不查库、不写审计，
//     审计表的写入速率被天然封顶（审计写在限流之后，绝不能倒过来）；
//  2. **只有失败才占配额，且归还的必须是自己的占位**（R-2）：入口 AllowWithToken 占位，
//     成功路径在签发令牌前 ReleaseToken 归还【自己那次】，内部故障（DB / 审计 / 签发）
//     也归还——它们不是"尝试爆破"的信号；失败路径保留占位（计数生效）。
//     旧版 Release(key) 删最近一条，同 IP 并发下会误删他人的失败计数并留下幽灵占位，
//     已废弃；
//  3. 失败路径（限流通过后）写 AuditActionLoginFailed（A7 加固）：账号不存在记
//     entity_id=0，口令错/禁用记命中用户 id；before_json 记 attempted 用户名与 reason，
//     **绝不记口令**；响应仍保持统一 401 文案（R46：不做用户枚举）；
//  4. 成功路径的 login 审计保持在【签发令牌之前】（WBS 4.3 / R46 口径，不得倒退）。
//
// 错误分类（与 GPT-R46 一致，HTTP 层据此分岔）：
//   - 凭证类（ErrBadCredentials / ErrAccountDisabled）→ 统一 401；
//   - 限流（ErrRateLimited）→ 429——它不是凭证错误，也不是内部错误，客户端应退避重试；
//   - 依赖类（ErrAuthUnavailable）→ 503：数据库故障 / 成功审计写入失败 / 令牌签发失败。
func (s *AuthService) LoginFromIP(username, password, clientIP string) (string, error) {
	key := loginKey(clientIP)
	// ① 失败限流：先占位（失败才计数——见方法注释第 2 条）
	slot, allowed := s.loginLimiter.AllowWithToken(key, s.loginMaxAttempts, s.loginWindow)
	if !allowed {
		return "", ErrRateLimited
	}

	var u model.User
	has, err := s.engine.Where("username = ? AND deleted = 0", strings.TrimSpace(username)).Get(&u)
	if err != nil {
		// 数据库故障 ≠ 口令错误：归还占位（内部故障不计入失败配额，否则一次
		// 审计表/库故障就能把全网用户的失败配额吃光），并标成依赖不可用
		s.loginLimiter.ReleaseToken(slot)
		return "", fmt.Errorf("%w：查询用户失败：%w", ErrAuthUnavailable, err)
	}
	if !has {
		// 不区分"账号不存在"与"口令错误"（响应层统一 401），但审计要记真实原因，
		// 否则"有人在枚举账号"就不可见。账号不存在时没有 actor/entity 可指 → 0。
		s.writeLoginFailedAudit(username, 0, "no_such_user")
		return "", ErrBadCredentials
	}
	if u.Disabled {
		s.writeLoginFailedAudit(username, u.Id, "disabled")
		return "", ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		// 只有"口令不匹配"才算凭证错误；哈希本身损坏（长度不对/格式非法）是数据故障，
		// 归到 503 才不会让运维误以为是用户输错密码。
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			s.writeLoginFailedAudit(username, u.Id, "bad_password")
			return "", ErrBadCredentials
		}
		s.loginLimiter.ReleaseToken(slot)
		return "", fmt.Errorf("%w：校验口令失败：%w", ErrAuthUnavailable, err)
	}

	// ② 认证成功：归还【自己】的占位（成功不占失败配额——多设备正常登录不得被误伤；
	// 同网段并发的失败计数不受影响，R-2）
	s.loginLimiter.ReleaseToken(slot)

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

// writeLoginFailedAudit 写登录失败审计（A7 加固，best-effort）。
//
//   - book_id=AuditBookInstance(0)：失败尝试发生在"账本之外"，与成功登录同口径；
//   - actor_uid / entity_id = 命中的用户 id（账号不存在为 0）——回答"谁在被人尝试"；
//   - before_json 记 {"attempted":<用户名>,"reason":no_such_user|bad_password|disabled}，
//     **绝不记口令**（口令进审计等于把凭据落进备份与日志采集）；
//   - best-effort：失败尝试的【拒绝决定】在审计写入之前已经做出，写不进去不应把一次
//     401 变成 503（那会让审计表故障期间所有失败登录都变成 5xx，还会掩盖真实失败率）；
//     改记服务端日志供运维发现"审计管道断了"。
//     对照：成功路径的 login 审计是【安全关键】（已认证却无留痕不可接受），故保持
//     "写失败即 503、不签发令牌"的强口径，两条路径刻意不对称。
func (s *AuthService) writeLoginFailedAudit(attempted string, uid int64, reason string) {
	before := map[string]string{
		"attempted": truncateRunes(strings.TrimSpace(attempted), maxAttemptedUserLen),
		"reason":    reason,
	}
	if err := AppendAudit(s.engine, model.AuditBookInstance, uid,
		model.AuditActionLoginFailed, model.AuditEntityUser, uid, before, "server"); err != nil {
		log.Printf("[auth] 登录失败审计写入失败 attempted=%q uid=%d reason=%s err=%v",
			truncateRunes(strings.TrimSpace(attempted), maxAttemptedUserLen), uid, reason, err)
	}
}

// truncateRunes 按字符数截断（审计字段卫生：客户端可控输入先截断再落库）
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
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
