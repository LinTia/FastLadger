package api

// GPT-R41（邀请码邮箱绑定闭环）的 **HTTP 适配层**回归测试。
//
// 本机无 gcc（CGO 不可用），无法起真实 SQLite，因此这里只覆盖【不碰数据库】的契约：
// 请求体必须真的接收 boundEmail / email、handler 必须把它透传给 service。
// "服务端实现了、接口参数没接"是本 Finding 的原始形态，属于必须钉死的回归点。
//
// service 不能反向 import api（会成环），故包归属在这侧。

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// readInviteAPIFile 读取本包内的源文件（源码级契约守卫用）
func readInviteAPIFile(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试文件路径")
	}
	sep := "/"
	if strings.Contains(file, "\\") {
		sep = "\\"
	}
	dir := file[:strings.LastIndex(file, sep)+1]
	b, err := os.ReadFile(dir + name)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", name, err)
	}
	return string(b)
}

// fnBodyOfInviteAPI 截取某个 handler 的函数体源码（到下一个顶层 `}` 结束为止）
func fnBodyOfInviteAPI(t *testing.T, src, fn string) string {
	t.Helper()
	idx := strings.Index(src, fn)
	if idx < 0 {
		t.Fatalf("未找到 %s", fn)
	}
	body := src[idx:]
	end := strings.Index(body, "\n}\n")
	if end < 0 {
		t.Fatalf("%s 函数体解析失败（未找到结束符）", fn)
	}
	return body[:end]
}

// TestR41CreateInviteAcceptsBoundEmail 签发邀请的请求体必须带 boundEmail 且透传给 service。
func TestR41CreateInviteAcceptsBoundEmail(t *testing.T) {
	src := readInviteAPIFile(t, "book_handler.go")
	body := fnBodyOfInviteAPI(t, src, "func (h *bookHandler) createInvite")

	if !strings.Contains(body, `json:"boundEmail"`) {
		t.Fatal("createInvite 缺少 boundEmail 入参：邀请无法绑定邮箱（GPT-R41 回归）")
	}
	if !strings.Contains(body, "req.BoundEmail") {
		t.Fatal("createInvite 读取了 boundEmail 却没有使用：等于字段可用但无效")
	}
	if !strings.Contains(body,
		"h.members.CreateInvite(currentUID(c), id, req.Role, req.BoundUsername, req.BoundEmail)") {
		t.Fatal("createInvite 必须把 boundEmail 透传给 MemberService.CreateInvite")
	}
	// 用户名绑定必须继续透传（不能被邮箱改造顶掉）
	if !strings.Contains(body, "req.BoundUsername") {
		t.Fatal("createInvite 丢失了 boundUsername 透传")
	}
}

// TestR41JoinAcceptsEmail 兑换邀请的请求体必须带 email 并透传，
// 否则"邀请绑了邮箱但接口无处传邮箱"，闭环依旧是断的。
func TestR41JoinAcceptsEmail(t *testing.T) {
	src := readInviteAPIFile(t, "book_handler.go")
	body := fnBodyOfInviteAPI(t, src, "func (h *bookHandler) join")

	if !strings.Contains(body, `json:"email"`) {
		t.Fatal("join 缺少 email 入参：邀请绑定邮箱时兑换方无从提供凭据（GPT-R41 回归）")
	}
	if !strings.Contains(body, "req.Email") {
		t.Fatal("join 读取了 email 却没有使用：等于字段可用但无效")
	}
	if !strings.Contains(body,
		"h.members.JoinViaInvite(id, req.Code, req.Username, req.Email, req.Password, c.ClientIP())") {
		t.Fatal("join 必须把 email 按正确顺序透传给 JoinViaInvite（顺序错位会编译不过或用错字段）")
	}
}
