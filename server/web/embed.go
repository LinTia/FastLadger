// Package web 内嵌 Web 端静态资源（WBS 5.9）。
//
// 为什么单独一个包：go:embed 不允许 ../ 相对路径，embed 指令必须落在被嵌目录
// 自身或其祖先包内——静态文件在 server/web/dist/，因此声明只能写在这里，
// 由 api / main 引用（embed.FS 只读，服务端零落盘）。
//
// Web 端定位（《新手开发完整指南》§23 / PRD §5.2）：
// 浏览器打开即可浏览 / 检索 / 批量编辑 / 对账；不做 OCR（OCR 交给客户端）。
// dist/ 内为手写原生 HTML/JS/CSS，无构建工具链（自托管单二进制卖点，不引入 npm）。
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// Dist 返回内嵌的 dist 目录（子文件系统，路径不含 dist/ 前缀）。
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
