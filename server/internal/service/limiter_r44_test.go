package service

// GPT-R44：限流器内存必须有界。
//
// 这些用例【不依赖数据库】，本机（无 gcc / CGO 不可用）可以直接跑，且刻意用
// **注入的固定时钟**而不是 sleep 来推进时间——过期回收必须是确定性的：
// sleep 只会让用例在慢机器上偶发失败，掩盖真实的时序问题。

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock 确定性时钟：只能由测试显式推进，杜绝 sleep 带来的偶发失败。
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// TestR44ExpiredKeysAreReclaimed 过期 key 必须被回收（主动 + 惰性两条路径都要验证）。
func TestR44ExpiredKeysAreReclaimed(t *testing.T) {
	c := newFakeClock()
	l := newLimiter(1000, c.now)
	const max = 3
	window := time.Minute

	// 灌入 200 个不同 IP，各自只打 1 次（模拟"打一枪就换 IP"）
	const n = 200
	for i := 0; i < n; i++ {
		if !l.Allow("ip:"+string(rune('A'+i%26))+itoa(i), max, window) {
			t.Fatalf("第 %d 个 key 的首次请求应被允许", i)
		}
	}
	if got := l.Size(); got != n {
		t.Fatalf("刚灌入时应有 %d 个 key，实际 %d", n, got)
	}

	// 推进到窗口之外：这些 key 之后不会再被访问，只能靠【主动清理】回收
	c.advance(window + time.Second)
	if freed := l.Sweep(); freed != n {
		t.Fatalf("主动清理应回收全部 %d 个过期 key，实际 %d", n, freed)
	}
	if got := l.Size(); got != 0 {
		t.Fatalf("清理后应剩 0 个 key，实际 %d（map 无界增长，GPT-R44 未修复）", got)
	}

	// 惰性清理路径：过期 key 再次被访问时，旧时间戳不得累积（不能被误判为已超限）
	for i := 0; i < max; i++ {
		if !l.Allow("ip:revisit", max, window) {
			t.Fatalf("窗口内第 %d 次应被允许", i+1)
		}
	}
	if l.Allow("ip:revisit", max, window) {
		t.Fatal("窗口内第 max+1 次应被限流")
	}
	c.advance(window + time.Second)
	if !l.Allow("ip:revisit", max, window) {
		t.Fatal("窗口过期后应重新允许（惰性清理必须把过期时间戳裁掉）")
	}
	if got := l.Size(); got != 1 {
		t.Fatalf("惰性清理不应留下多余 key，实际 %d", got)
	}
}

// TestR44KeyCountBoundedByMaxKeys 大量不同 IP 不得让 map 无界增长。
func TestR44KeyCountBoundedByMaxKeys(t *testing.T) {
	c := newFakeClock()
	const cap = 32
	l := newLimiter(cap, c.now)

	// 先把 0 号 IP 用满额度（5 次），作为"正常用户"的基线
	const maxHits = 5
	first := "ip:" + itoa(0)
	for i := 0; i < maxHits; i++ {
		if !l.Allow(first, maxHits, time.Hour) {
			t.Fatalf("0 号 IP 的第 %d 次应被允许", i+1)
		}
	}

	// 再灌入远超容量的不同 IP（旧实现下这里会让 map 涨到 5000 条）
	const total = 5000
	rejected := 0
	for i := 0; i < total; i++ {
		if !l.Allow("ip:"+itoa(i), maxHits, time.Hour) {
			rejected++
		}
	}
	if got := l.Size(); got > cap {
		t.Fatalf("key 数量必须有界（上限 %d），实际 %d", cap, got)
	}
	if rejected == 0 {
		t.Fatal("容量打满后应有新 key 被拒绝（否则说明容量保护没生效）")
	}
	if got := l.Size(); got != cap {
		t.Fatalf("容量打满时应稳定在上限 %d，实际 %d", cap, got)
	}

	// 已存在的 key 不得因为"后来者"被清掉（拒绝新 key，而不是淘汰旧 key）：
	// 0 号 IP 已用满 5 次，第 6 次必须仍被限流——它的计数没有被冲掉。
	if l.Allow(first, maxHits, time.Hour) {
		t.Fatal("已用满额度的 key 必须仍被限流（说明它的计数没被后来者冲掉）")
	}
	if got := l.Size(); got != cap {
		t.Fatalf("已有 key 继续访问不应改变容量，实际 %d", got)
	}
}

// TestR44CleanupDoesNotWeakenRateLimit 清理不得让"本该被限流的请求"放行。
func TestR44CleanupDoesNotWeakenRateLimit(t *testing.T) {
	c := newFakeClock()
	l := newLimiter(1000, c.now)
	const max = 5
	window := time.Minute

	for i := 0; i < max; i++ {
		if !l.Allow("invite:create:book:1", max, window) {
			t.Fatalf("第 %d 次应被允许", i+1)
		}
	}
	// 反复清理：只要还在窗口内，计数就不能被清掉
	for i := 0; i < 10; i++ {
		l.Sweep()
		c.advance(time.Second)
	}
	if l.Allow("invite:create:book:1", max, window) {
		t.Fatal("窗口内已用满额度，清理后不得放行（清理削弱了限流）")
	}
	// 跨过窗口后才允许
	c.advance(window)
	if !l.Allow("invite:create:book:1", max, window) {
		t.Fatal("窗口过期后应允许")
	}
}

// TestR44ConcurrentAllowIsRaceFreeAndBounded 并发访问：无数据竞争（-race）且容量仍有界。
//
// 不用 sleep：所有 goroutine 由 WaitGroup 汇合，断言只依赖"全部结束后"的状态。
func TestR44ConcurrentAllowIsRaceFreeAndBounded(t *testing.T) {
	const (
		goroutines = 16
		perG       = 200
		maxKeys    = 128
		maxHits    = 10
	)
	l := newLimiter(maxKeys, time.Now)

	var allowed int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start // 同时起跑，尽量制造真实的并发交错
			for i := 0; i < perG; i++ {
				key := "ip:" + itoa(g) + ":" + itoa(i%50)
				if l.Allow(key, maxHits, time.Hour) {
					atomic.AddInt64(&allowed, 1)
				}
			}
			// 并发读写 Size / Sweep 也不得有竞争
			_ = l.Size()
			l.Sweep()
		}(g)
	}
	close(start)
	wg.Wait()

	if got := l.Size(); got > maxKeys {
		t.Fatalf("并发下 key 数量仍必须有界（上限 %d），实际 %d", maxKeys, got)
	}
	// 每个 key 最多被允许 maxHits 次：goroutines*perG 次请求分布在 goroutines*50 个 key 上，
	// 这里只断言"被允许的总数不超过 上限×maxHits"，防止清理把计数清没了导致超额放行。
	if allowed > int64(maxKeys*maxHits) {
		t.Fatalf("被允许的次数 %d 超过容量上限允许的最大次数 %d（存在越权放行）",
			allowed, maxKeys*maxHits)
	}
	if allowed == 0 {
		t.Fatal("并发下不应全部被拒")
	}
}

// TestR44ZeroWindowKeyDoesNotLeak 零长度窗口等退化参数也不得留下空壳 key。
func TestR44ZeroWindowKeyDoesNotLeak(t *testing.T) {
	c := newFakeClock()
	l := newLimiter(1000, c.now)
	// max<=0：一律拒绝（fail-closed），且不得在 map 里留下条目
	for i := 0; i < 100; i++ {
		if l.Allow("bad:"+itoa(i), 0, time.Minute) {
			t.Fatal("max<=0 时必须拒绝（不得退化成不限流）")
		}
	}
	if got := l.Size(); got != 0 {
		t.Fatalf("max<=0 的调用不得留下 key，实际 %d", got)
	}
	// 正常参数仍可用（防御分支没有污染状态）
	if !l.Allow("good", 1, time.Minute) {
		t.Fatal("正常参数应被允许")
	}
	c.advance(time.Minute)
	l.Sweep()
	if got := l.Size(); got != 0 {
		t.Fatalf("过期后应被清理，实际 %d", got)
	}
}

// itoa 小工具：避免每个用例都引 strconv（仅测试用）
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	p := len(buf)
	for i > 0 {
		p--
		buf[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		buf[p] = '-'
	}
	return string(buf[p:])
}
