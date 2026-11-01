package api

// Web 端静态资源服务（WBS 5.9）。
//
// 挂载策略：
//   - 不注册任何 /web 前缀路由——浏览器直接打开 http://host:port/ 即用；
//   - 走 gin 的 NoRoute 兜底：显式路由（/healthz、/api/v1/*、/ws）永远优先，
//     其余 GET 请求按内嵌文件查找，未命中回退 index.html（SPA 入口）；
//   - 非 GET 的未知路径维持 404 JSON（与既有 API 404 行为一致，不吞掉拼错的
//     POST/DELETE 请求）；
//   - 内嵌 FS 只读，不存在目录穿越：fs.FS 的路径语义本身拒绝 .. 与盘符；
//     另有 fs.ValidPath 前置校验双保险。
//
// 为什么不用 http.FileServer：它对显式 /index.html 请求会 301 到 ./，
// 与 SPA 回退语义冲突；自实现按需 ReadFile，行为完全可控。
//
// 安全口径：静态资源不携带任何敏感信息；鉴权仍由 /api/v1 的 JWTAuth 把守，
// Web 前端只是这些 API 的另一个消费者（token 存 localStorage，由 JS 携带）。
import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"fastledger/web"

	"github.com/gin-gonic/gin"
)

const webIndexHTML = "index.html"

// newWebHandler 构造静态文件处理器；dist 为空（embed 失败/空目录）时返回 nil，
// 调用方跳过挂载——服务照常运行，仅 Web 端不可用（降级不致命）。
func newWebHandler() gin.HandlerFunc {
	dist, err := web.Dist()
	if err != nil {
		return nil
	}
	serve := func(c *gin.Context, name string) {
		data, rerr := fs.ReadFile(dist, name)
		if rerr != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "not found"})
			return
		}
		ct := mime.TypeByExtension(path.Ext(name))
		if ct == "" {
			ct = "application/octet-stream"
		}
		c.Header("Content-Type", ct)
		// 静态资源带内容指纹级联（同名文件=同内容），短缓存即可；
		// index.html 不缓存，保证发版后浏览器立刻拿到新入口引用。
		if name == webIndexHTML {
			c.Header("Cache-Control", "no-cache")
		} else {
			c.Header("Cache-Control", "public, max-age=300")
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, ct, data)
	}
	return func(c *gin.Context) {
		// 仅 GET/HEAD 做静态服务：拼错的 POST/DELETE 保持 404 JSON，
		// 绝不被 SPA 回退吞掉（客户端会把 HTML 当 JSON 解析）。
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "not found"})
			return
		}
		// API 与 WS 前缀绝不回退到静态页：拼错的 API 路径保持 404 JSON，
		// 否则客户端会把 HTML 当 JSON 解析，掩盖路由错误。
		p := path.Clean("/" + c.Request.URL.Path)
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/healthz") || strings.HasPrefix(p, "/ws") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "not found"})
			return
		}
		name := strings.TrimPrefix(p, "/")
		if name == "" {
			serve(c, webIndexHTML)
			return
		}
		if !fs.ValidPath(name) { // 二次防线：..、前导 / 等一律拒绝
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "bad path"})
			return
		}
		if _, err := fs.Stat(dist, name); err != nil {
			// 文件不存在 → SPA 回退 index.html（兼容 history 路由与刷新）
			serve(c, webIndexHTML)
			return
		}
		serve(c, name)
	}
}
