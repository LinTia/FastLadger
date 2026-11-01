package service

// JWT（HS256）手写实现——刻意不引入第三方库，以便严格满足 R08/R09：
//  1. 限定 HS256，拒绝 alg=none 及其他算法；
//  2. 安全读取 uid：claims 非法/缺失时返回错误，绝不 panic；
//  3. 密钥只从配置注入，禁止硬编码或占位值（见 internal/config）。
import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const jwtAlgHS256 = "HS256"

// Claims JWT 载荷
type Claims struct {
	Uid int64  `json:"uid"`
	Jti string `json:"jti"`
	Exp int64  `json:"exp"`
}

var (
	// ErrTokenMalformed token 结构或 claims 非法
	ErrTokenMalformed = errors.New("token 格式错误")
	// ErrTokenBadAlg 算法不是 HS256（含 alg=none 攻击）
	ErrTokenBadAlg = errors.New("token 算法非法：仅接受 HS256")
	// ErrTokenBadSign 签名校验失败
	ErrTokenBadSign = errors.New("token 签名校验失败")
	// ErrTokenExpired 已过期
	ErrTokenExpired = errors.New("token 已过期")
	// ErrTokenRevoked 会话已失效（R52 多端会话强校验）。
	//
	// 与 ErrTokenExpired 的语义区别面向客户端："该设备已被主动登出 / 会话不存在"，
	// HTTP 层据此给出可区分的 401 文案，让客户端清掉本地凭证重新登录，
	// 而不是当成网络抖动重试。覆盖三种情况：无会话行（存量令牌/伪造）、已撤销、
	// 会话行自身过期。
	ErrTokenRevoked = errors.New("token 对应的会话已失效")
)

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

func b64urlEncode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func b64urlDecode(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// SignJWT 签发 HS256 token，带 jti（便于后续吊销）。
func SignJWT(secret string, uid int64, ttl time.Duration) (string, error) {
	tok, _, err := signJWTWithClaims(secret, uid, ttl)
	return tok, err
}

// signJWTWithClaims 同 SignJWT，额外返回解析出的 claims（含 jti / exp）。
//
// 供登录路径把"签发令牌"与"落会话行"放进同一事务（R52）：会话行需要 jti 与 exp，
// 从签名后的 token 再解析一遍既绕又脆，直接返回构造时的 claims。
// 既有调用点（jwt_test 等）继续走 SignJWT，公开签名不变。
func signJWTWithClaims(secret string, uid int64, ttl time.Duration) (string, *Claims, error) {
	claims := &Claims{Uid: uid, Jti: newJTI(), Exp: time.Now().Add(ttl).Unix()}
	hb, err := json.Marshal(jwtHeader{Alg: jwtAlgHS256, Typ: "JWT"})
	if err != nil {
		return "", nil, err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", nil, err
	}
	signing := b64urlEncode(hb) + "." + b64urlEncode(cb)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(signing)); err != nil {
		return "", nil, err
	}
	return signing + "." + b64urlEncode(mac.Sum(nil)), claims, nil
}

// ParseJWT 校验并解析 token
func ParseJWT(secret, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}
	hb, err := b64urlDecode(parts[0])
	if err != nil {
		return nil, ErrTokenMalformed
	}
	var h jwtHeader
	if err := json.Unmarshal(hb, &h); err != nil {
		return nil, ErrTokenMalformed
	}
	if h.Alg != jwtAlgHS256 {
		return nil, ErrTokenBadAlg
	}

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(parts[0] + "." + parts[1])); err != nil {
		return nil, err
	}
	sig, err := b64urlDecode(parts[2])
	if err != nil {
		return nil, ErrTokenMalformed
	}
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, ErrTokenBadSign
	}

	cb, err := b64urlDecode(parts[1])
	if err != nil {
		return nil, ErrTokenMalformed
	}
	var c Claims
	if err := json.Unmarshal(cb, &c); err != nil {
		return nil, ErrTokenMalformed
	}
	if c.Uid <= 0 {
		return nil, ErrTokenMalformed
	}
	if c.Exp <= time.Now().Unix() {
		return nil, ErrTokenExpired
	}
	return &c, nil
}

func newJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}
