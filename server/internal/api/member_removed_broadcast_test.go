package api

// GPT-R24（P1）回归：移除成员时的清理通知必须**同时**满足两条方向相反的约束：
//   1. 目标（在线且仍连着）**必须收到一次** `member_removed`，否则客户端不会清除该账本本地数据；
//   2. 目标此后**不得再收到**该账本任何事件（订阅已被摘除）。
//
// 修复前：notifyMemberRemoved 先 RefreshVisibility（摘订阅）再普通广播，目标因 `!sub`
// 被过滤，永远收不到清理事件。本用例在修复前应当失败。

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"fastledger/internal/ws"
)

type wsEnvelope struct {
	Event           string                 `json:"event"`
	BookID          int64                  `json:"bookId"`
	Version         int64                  `json:"version"`
	ForceFullResync bool                   `json:"forceFullResync"`
	Data            map[string]interface{} `json:"data"`
}

// nextEvent 读一条事件；超时视为"没有消息"。
func nextEvent(t *testing.T, c *ws.Client, wait time.Duration) (wsEnvelope, bool) {
	t.Helper()
	select {
	case raw := <-c.Send:
		var env wsEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("事件不是合法 JSON：%v（原文 %s）", err, raw)
		}
		return env, true
	case <-time.After(wait):
		return wsEnvelope{}, false
	}
}

func expectNoEvent(t *testing.T, c *ws.Client, why string) {
	t.Helper()
	if env, ok := nextEvent(t, c, 80*time.Millisecond); ok {
		t.Fatalf("%s：不应收到事件，却收到 %s", why, env.Event)
	}
}

func TestNotifyMemberRemovedReachesTargetAndStopsFurtherEvents(t *testing.T) {
	hub := ws.DefaultHub

	const bookID, targetUID, otherUID int64 = 9001, 77, 88
	allowed := map[int64]bool{targetUID: true, otherUID: true}
	hub.SetChecker(func(uid, bID int64) (bool, error) {
		if bID != bookID {
			return false, errors.New("非本账本")
		}
		if !allowed[uid] {
			return false, errors.New("已不是该账本成员")
		}
		return true, nil
	})

	target := ws.NewClient()
	target.UID = targetUID
	other := ws.NewClient()
	other.UID = otherUID
	hub.Register(target)
	hub.Register(other)
	defer hub.Unregister(target)
	defer hub.Unregister(other)

	hub.Subscribe(target, []int64{bookID}, targetUID)
	hub.Subscribe(other, []int64{bookID}, otherUID)
	if !target.Subscribed(bookID) || !other.Subscribed(bookID) {
		t.Fatal("前置条件失败：两个连接都应已订阅该账本")
	}

	// 模拟"目标已被移除"：从此刻起 checker 不再认可它
	delete(allowed, targetUID)

	notifyMemberRemoved(bookID, 12345, targetUID)

	// 约束 1：目标必须收到清理通知（这正是修复前丢失的那条）
	env, ok := nextEvent(t, target, time.Second)
	if !ok {
		t.Fatal("目标未收到 member_removed —— GPT-R24 未修复（摘订阅发生在广播之前）")
	}
	if env.Event != "member_removed" {
		t.Fatalf("目标收到 %s，期望 member_removed", env.Event)
	}
	if !env.ForceFullResync {
		t.Fatal("member_removed 必须带 forceFullResync=true")
	}
	if env.BookID != bookID || env.Version != 12345 {
		t.Fatalf("信封字段不符：bookId=%d version=%d", env.BookID, env.Version)
	}
	if got, _ := env.Data["targetUid"].(float64); int64(got) != targetUID {
		t.Fatalf("data.targetUid = %v，期望 %d", env.Data["targetUid"], targetUID)
	}
	// 目标只应收到一次，不能重复
	expectNoEvent(t, target, "目标重复收到清理通知")

	// 其他仍然有效的成员也要收到（普通账本广播不能被这条定向路径挤掉）
	envOther, ok := nextEvent(t, other, time.Second)
	if !ok {
		t.Fatal("其他成员未收到 member_removed")
	}
	if envOther.Event != "member_removed" {
		t.Fatalf("其他成员收到 %s", envOther.Event)
	}

	// 约束 2：目标订阅已被摘除
	if target.Subscribed(bookID) {
		t.Fatal("移除后目标不应再订阅该账本")
	}

	// 约束 2 的实证：后续 data_changed 不再到达目标，但其他成员照常收到
	notifyDataChanged(bookID, 12346)
	expectNoEvent(t, target, "被移除后仍收到 data_changed（订阅未摘除）")
	if env2, ok := nextEvent(t, other, time.Second); !ok || env2.Event != "data_changed" {
		t.Fatalf("其他成员应收到 data_changed，实际 %v/%q", ok, env2.Event)
	}
}

// 多连接场景：同一用户的两台设备都要收到清理通知（否则另一台会残留本地数据）。
func TestNotifyMemberRemovedReachesAllConnectionsOfTarget(t *testing.T) {
	hub := ws.DefaultHub

	const bookID, targetUID int64 = 9002, 99
	allowed := map[int64]bool{targetUID: true}
	hub.SetChecker(func(uid, bID int64) (bool, error) {
		if bID != bookID || !allowed[uid] {
			return false, errors.New("非成员")
		}
		return true, nil
	})

	phone := ws.NewClient()
	phone.UID = targetUID
	desktop := ws.NewClient()
	desktop.UID = targetUID
	hub.Register(phone)
	hub.Register(desktop)
	defer hub.Unregister(phone)
	defer hub.Unregister(desktop)

	hub.Subscribe(phone, []int64{bookID}, targetUID)
	hub.Subscribe(desktop, []int64{bookID}, targetUID)
	delete(allowed, targetUID)

	notifyMemberRemoved(bookID, 1, targetUID)

	for name, c := range map[string]*ws.Client{"phone": phone, "desktop": desktop} {
		env, ok := nextEvent(t, c, time.Second)
		if !ok || env.Event != "member_removed" {
			t.Fatalf("%s 未收到 member_removed（ok=%v event=%q）", name, ok, env.Event)
		}
	}
}
