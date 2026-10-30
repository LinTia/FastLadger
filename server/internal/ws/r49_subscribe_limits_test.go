package ws

// GPT-R49【P2】WebSocket 订阅消息没有账本 ID 数量上限或去重 —— 定向回归。
//
// 背景：客户端一帧可以塞进数千个账本 ID（64 KB 的 readLimit 只挡字节数，挡不住条数），
// 也可以反复塞同一个 ID；**每个 ID 都独立触发一次 SQLite 鉴权查询**，
// 而连接池 MaxOpenConns=1 —— 一帧 N 个 ID 就是 N 次串行查询，
// 期间所有广播和其它请求的查库全被堵住（表现为"所有人收不到通知"）。
//
// 本文件的断言口径：
//  1. 超限必须**返回明确错误**（ErrTooManyBookIDs / ErrSubscriptionLimit* /
//     ErrSubscribeRateLimited），绝不静默截断 —— 截断会让客户端以为订阅成功了；
//  2. "不会触发 N 次鉴权查询"用 **checker 调用次数**做硬断言，而不是"看起来没查"；
//  3. 限制必须发生在查库【之前】：被拒的帧对应的 checker 调用次数为 0。

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// r49AllAllowChecker 放行一切账本（用于测"上限"本身，排除成员校验的干扰）
func r49AllAllowChecker(uid, bookID int64) (bool, error) { return true, nil }

// r49IDs 生成 [from, from+n) 的账本 ID
func r49IDs(from int64, n int) []int64 {
	ids := make([]int64, 0, n)
	for i := int64(0); i < int64(n); i++ {
		ids = append(ids, from+i)
	}
	return ids
}

// TestR49RejectsTooManyBookIDsInOneFrame 单帧账本 ID 数超上限 → 明确错误 + 一次库都不查。
func TestR49RejectsTooManyBookIDsInOneFrame(t *testing.T) {
	chk := newFakeChecker()
	for _, b := range r49IDs(1, MaxBookIDsPerFrame+100) {
		chk.allow(b, 100, true)
	}
	hub := NewHub(chk.check)
	c := NewClient()
	hub.Register(c)

	// 正好等于上限：应当放行（查库次数 = 上限值，说明没重复查）
	err := hub.Subscribe(c, r49IDs(1, MaxBookIDsPerFrame), 100)
	if err != nil {
		t.Fatalf("恰好等于上限（%d）的帧不应被拒：%v", MaxBookIDsPerFrame, err)
	}
	if got := chk.callCount(); got != MaxBookIDsPerFrame {
		t.Fatalf("去重后应只查库 %d 次，实际 %d（说明存在重复/多余查询）",
			MaxBookIDsPerFrame, got)
	}

	// 超过上限：拒绝 + 不查库
	before := chk.callCount()
	err = hub.Subscribe(c, r49IDs(1, MaxBookIDsPerFrame+1), 100)
	if !errors.Is(err, ErrTooManyBookIDs) {
		t.Fatalf("超量帧应返回 ErrTooManyBookIDs，实际 %v", err)
	}
	if got := chk.callCount(); got != before {
		t.Fatalf("被拒的帧不得查库：之前 %d 次，之后 %d 次", before, got)
	}
	if !strings.Contains(err.Error(), "上限") {
		t.Fatalf("错误信息必须写明上限，便于客户端提示用户：%q", err.Error())
	}
}

// TestR49RejectsHugeFrameWithoutAnyAuthQuery 数千个 ID 的帧：0 次鉴权查询。
//
// 这是报告里点名的放大路径（"每个 ID 都独立触发一次 SQLite 鉴权查询"）。
// 修复前会让 checker 被调用数千次；修复后一次都不该发生。
func TestR49RejectsHugeFrameWithoutAnyAuthQuery(t *testing.T) {
	chk := newFakeChecker()
	hub := NewHub(chk.check)
	c := NewClient()
	hub.Register(c)

	huge := r49IDs(1, 5000)
	err := hub.Subscribe(c, huge, 100)
	if !errors.Is(err, ErrTooManyBookIDs) {
		t.Fatalf("5000 个 ID 的单帧应被拒，实际 %v", err)
	}
	if got := chk.callCount(); got != 0 {
		t.Fatalf("被拒的 5000-ID 帧触发了 %d 次鉴权查询，应为 0（限制必须在查库之前）", got)
	}
	if c.SubscriptionCount() != 0 {
		t.Fatalf("被拒的帧不得留下任何订阅，实际 %d", c.SubscriptionCount())
	}
}

// TestR49DeduplicatesBookIDsBeforeAuth 重复 ID 先去重：查库次数 == 去重后的条数。
func TestR49DeduplicatesBookIDsBeforeAuth(t *testing.T) {
	chk := newFakeChecker()
	chk.allow(1, 100, true)
	chk.allow(2, 100, false)
	hub := NewHub(chk.check)
	c := NewClient()
	hub.Register(c)

	// 1 出现 400 次、2 出现 400 次 → 去重后只有 2 个 ID
	frame := make([]int64, 0, 800)
	for i := 0; i < 400; i++ {
		frame = append(frame, 1, 2)
	}
	if err := hub.Subscribe(c, frame, 100); err != nil {
		t.Fatalf("去重后未超限，不应被拒：%v", err)
	}
	if got := chk.callCount(); got != 2 {
		t.Fatalf("重复 ID 必须先去重再查库：期望 2 次（每账本 1 次），实际 %d 次", got)
	}
	if c.SubscriptionCount() != 2 {
		t.Fatalf("去重后应订阅 2 个账本，实际 %d", c.SubscriptionCount())
	}

	// 同一账本重复订阅：幂等，不新增条目、也不重复查库
	before := chk.callCount()
	if err := hub.Subscribe(c, []int64{1, 1, 1}, 100); err != nil {
		t.Fatalf("重复订阅不应报错：%v", err)
	}
	if got := chk.callCount(); got != before+1 {
		t.Fatalf("重订阅单个账本应只查 1 次库，实际 %d 次", got-before)
	}
	if c.SubscriptionCount() != 2 {
		t.Fatalf("重复订阅不得新增条目，实际 %d", c.SubscriptionCount())
	}
}

// TestR49RejectsBeyondPerConnectionSubscriptionLimit 单连接订阅总数上限。
//
// 多帧逐步逼近上限：允许到 200，第 4 帧（会到 256）必须被拒且不留任何新增。
func TestR49RejectsBeyondPerConnectionSubscriptionLimit(t *testing.T) {
	hub := NewHub(r49AllAllowChecker)
	c := NewClient()
	hub.Register(c)

	frames := MaxSubscriptionsPerClient / MaxBookIDsPerFrame // 3 帧 × 64 = 192
	for i := 0; i < frames; i++ {
		if err := hub.Subscribe(c, r49IDs(int64(i*MaxBookIDsPerFrame+1), MaxBookIDsPerFrame), 100); err != nil {
			t.Fatalf("第 %d 帧（累计 %d）不应超限：%v", i+1, (i+1)*MaxBookIDsPerFrame, err)
		}
	}
	got := c.SubscriptionCount()
	if got != frames*MaxBookIDsPerFrame {
		t.Fatalf("累计订阅数应为 %d，实际 %d", frames*MaxBookIDsPerFrame, got)
	}

	err := hub.Subscribe(c, r49IDs(int64(frames*MaxBookIDsPerFrame+1), MaxBookIDsPerFrame), 100)
	if !errors.Is(err, ErrSubscriptionLimitPerConnection) {
		t.Fatalf("超出单连接上限应返回 ErrSubscriptionLimitPerConnection，实际 %v", err)
	}
	if after := c.SubscriptionCount(); after != got {
		t.Fatalf("超限帧必须整帧拒绝（不得部分写入/静默截断）：之前 %d，之后 %d", got, after)
	}
}

// TestR49RejectsBeyondPerUserSubscriptionLimit 单用户（多设备合计）订阅总数上限。
//
// 每个连接自己都远低于单连接上限（64 < 200），但同一 uid 的合计越过 600 时
// 必须被拒 —— 否则开 N 台设备就能把订阅总量放大到无界。
func TestR49RejectsBeyondPerUserSubscriptionLimit(t *testing.T) {
	hub := NewHub(r49AllAllowChecker)
	perConn := MaxBookIDsPerFrame
	clients := MaxSubscriptionsPerUser/perConn + 1 // 10 个连接 × 64 = 640 > 600
	cs := make([]*Client, 0, clients)
	for i := 0; i < clients; i++ {
		c := NewClient()
		hub.Register(c)
		cs = append(cs, c)
	}

	for i := 0; i < clients-1; i++ {
		if err := hub.Subscribe(cs[i], r49IDs(1, perConn), 100); err != nil {
			t.Fatalf("第 %d 个连接（用户累计 %d）不应超限：%v", i+1, (i+1)*perConn, err)
		}
	}
	last := cs[clients-1]
	err := hub.Subscribe(last, r49IDs(1, perConn), 100)
	if !errors.Is(err, ErrSubscriptionLimitPerUser) {
		t.Fatalf("超出单用户上限应返回 ErrSubscriptionLimitPerUser，实际 %v", err)
	}
	if last.SubscriptionCount() != 0 {
		t.Fatalf("被拒的连接不得留下订阅，实际 %d", last.SubscriptionCount())
	}

	// 另一个用户不受影响（上限按 uid 分别计算）
	other := NewClient()
	hub.Register(other)
	if err := hub.Subscribe(other, r49IDs(1, perConn), 200); err != nil {
		t.Fatalf("其它用户不应被别人的订阅量挡住：%v", err)
	}
	if other.SubscriptionCount() != perConn {
		t.Fatalf("其它用户应能订阅 %d 个，实际 %d", perConn, other.SubscriptionCount())
	}
}

// TestR49RateLimitsSubscribeFrames 订阅消息频率限制：连续爆发超过桶容量即被限流。
//
// 判据是**结构性**的（令牌桶不可能给出超过容量的令牌），不依赖 sleep：
// 连续 48 帧里最多 subscribeBurst 帧通过，其余必须返回 ErrSubscribeRateLimited。
func TestR49RateLimitsSubscribeFrames(t *testing.T) {
	hub := NewHub(r49AllAllowChecker)
	c := NewClient()
	hub.Register(c)

	total := subscribeBurst + 16
	ok, limited := 0, 0
	for i := 0; i < total; i++ {
		if err := hub.Subscribe(c, []int64{int64(i + 1)}, 100); err != nil {
			if !errors.Is(err, ErrSubscribeRateLimited) {
				t.Fatalf("第 %d 帧被拒的原因不是限流：%v", i+1, err)
			}
			limited++
			continue
		}
		ok++
	}
	if ok != subscribeBurst {
		t.Fatalf("连续爆发最多应放行 %d 帧，实际放行 %d", subscribeBurst, ok)
	}
	if limited != total-subscribeBurst {
		t.Fatalf("应有 %d 帧被限流，实际 %d", total-subscribeBurst, limited)
	}

	// 令牌按时间补充：等够 3 个令牌的时间后应能再次订阅（证明不是一次性封死）
	time.Sleep(300*time.Millisecond + 50*time.Millisecond)
	if err := hub.Subscribe(c, []int64{9999}, 100); err != nil {
		t.Fatalf("令牌补充后应能继续订阅（限流是速率限制而非永久封禁）：%v", err)
	}
}

// TestR49RateLimitRejectsBeforeAnyAuthQuery 被限流的帧一次库都不查。
//
// 限流器如果放在鉴权之后就失去意义：那一帧已经把唯一的 SQLite 连接占满了。
func TestR49RateLimitRejectsBeforeAnyAuthQuery(t *testing.T) {
	chk := newFakeChecker()
	hub := NewHub(chk.check)
	c := NewClient()
	hub.Register(c)

	// 先耗尽令牌桶
	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(c, []int64{1}, 100); err != nil {
			t.Fatalf("第 %d 帧不应被拒：%v", i+1, err)
		}
	}
	before := chk.callCount()
	if err := hub.Subscribe(c, r49IDs(1, 50), 100); !errors.Is(err, ErrSubscribeRateLimited) {
		t.Fatalf("令牌耗尽后应被限流，实际 %v", err)
	}
	if got := chk.callCount(); got != before {
		t.Fatalf("被限流的帧不得查库：之前 %d 次，之后 %d 次", before, got)
	}
}

// TestR49ServeWSReturnsExplicitErrorFrameOnOversizedSubscribe 真实连接端到端：
// 超量订阅必须收到一条显式 error 帧，而不是被静默忽略。
//
// "静默截断"是最坏结果：客户端以为订阅成功，之后永远等不到该账本的事件。
func TestR49ServeWSReturnsExplicitErrorFrameOnOversizedSubscribe(t *testing.T) {
	chk := newQAChecker()
	chk.set(7, 1, true)
	old := DefaultHub
	DefaultHub = NewHub(chk.check)
	t.Cleanup(func() { DefaultHub = old })

	srv := newQATestServer(t)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket 握手失败：%v", err)
	}
	defer func() { _ = conn.Close() }()
	waitCount(t, DefaultHub, 1)

	// 合法帧：静默成功（不回 error）
	if err := conn.WriteJSON(map[string]interface{}{"action": "subscribe", "books": []int64{1}}); err != nil {
		t.Fatalf("发送订阅失败：%v", err)
	}
	// 超量帧：必须回一条 error
	if err := conn.WriteJSON(map[string]interface{}{
		"action": "subscribe",
		"books":  r49IDs(1, MaxBookIDsPerFrame+1),
	}); err != nil {
		t.Fatalf("发送超量订阅失败：%v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("超量订阅未收到任何响应（静默忽略 = 客户端以为订阅成功）：%v", err)
	}
	var got wsErrorMsg
	if err := json.Unmarshal(msg, &got); err != nil {
		t.Fatalf("响应不是可解析的 JSON：%v（原文 %s）", err, msg)
	}
	if got.Event != "error" || got.Code != "subscribe_rejected" {
		t.Fatalf("应收到 event=error / code=subscribe_rejected，实际 %+v", got)
	}
	if !strings.Contains(got.Msg, "上限") {
		t.Fatalf("错误帧应说明被拒原因（含上限字样），实际 %q", got.Msg)
	}
}
