package service

// 简单内存限流器（滑动窗口），**内存有界**（GPT-R44）。
// 用于邀请码的生成与使用限流（PRD §12.1：每账本每分钟 ≤5 生成、同实例每小时 ≤10 尝试）。
// 单机自托管场景足够；若未来多实例部署需换成共享存储。
//
// 为什么必须有界（GPT-R44 的根因）：
// 旧实现把 `hits map[string][]time.Time` 当成"永久账本"——只有同一个 key 再次被访问
// 时才顺手裁掉窗口外的时间戳，而裁剪后的空切片也仍然留在 map 里。于是**只要不断换
// 新的 key（不同 IP 访问邀请入户接口）就能让 map 无限增大**，最终 OOM。
// 内存里放的是"谁在什么时候打过接口"，属于可再生的缓存，不是业务数据，必须可丢弃。
//
// 本实现给出三重约束（任一条单独都不足以封住增长）：
//  1. 惰性清理：每次 Allow 都裁掉该 key 窗口外的时间戳；
//  2. 主动清理：距上次全量扫描超过 limiterSweepInterval 就扫一遍，回收"之后再也没被
//     访问过"的过期 key（惰性清理碰不到它们）；
//  3. 容量上限 maxLimiterKeys：达到上限时【拒绝新 key】而不是淘汰旧 key。
//
// 取舍说明（为什么拒绝而不是 LRU 淘汰最旧）：
//   - 淘汰最旧 = 攻击者用大量新 IP 就能把正常用户的计数冲掉，限流被绕过，这与限流器
//     的存在意义直接冲突；
//   - 拒绝新 key = 最坏结果是"容量打满期间新来的 key 被临时限流"（429），属于 fail-closed
//     ——宁可多拦，也绝不放行。且容量会被第 2 条的主动清理持续回收，不是永久拒绝。
//
// 容量耗尽后的恢复策略（INFRA-04 反向，复审要求明确）：
//   1. 恢复的唯一途径是【过期】：Allow 在容量满时会**先强制全量清扫**，窗口已过期的
//      条目立即回收（见 Allow 步骤②）——因此攻击者的 key 一旦过窗，容量即刻自愈，
//      不需要任何人工干预或重启；
//   2. 恢复时长的上界 = 打满容量的那些 key 的窗口长度（登录 15 分钟 / 邀请 1 小时）；
//   3. 曾考虑"对只命中一次的 key 用更短的最小保留期"，**否决**：提前删掉一个仍在
//      窗口内的计数 = 该 IP 可以立刻再来一次，等于把限流改成 fail-open，直接违反
//      R44 的不变量"清理不得影响限流正确性"；
//   4. 真正的根因修复在别处：router.go 已 SetTrustedProxies(nil)，ClientIP 恒取 TCP
//      对端——"1 万个一次性伪造 IP"从此需要 1 万个真实来源，不再是改个请求头的事。
//
// 清理不影响限流正确性：被删掉的条目一定是"最新一次命中也已超出窗口"的条目，
// 重新记录一次不会让窗口内的命中数超过 max；仍在窗口内的条目一个都不会被删。
import (
	"sync"
	"time"
)

const (
	// maxLimiterKeys 单个限流器最多保留的 key 数量（内存上限）。
	//
	// 量级依据：本限流器只服务"邀请码生成（按账本）"与"邀请入户（按 IP）"两个入口。
	// 家庭自托管场景下账本数与来访 IP 都是十~百量级；取 10000 留出两个数量级余量，
	// 单条开销按 (key 字符串 + 时间戳切片) 粗估 < 200B，满载内存 < 2MB。
	maxLimiterKeys = 10000

	// limiterSweepInterval 主动清理的最小间隔：摊还掉全量扫描的成本。
	//
	// 取 1 分钟而不是更长：最长的窗口是"邀请入户"的 1 小时，间隔越短过期 key 回收越快；
	// 但扫描是 O(key 数)，满载 10000 条每分钟一次对单机可忽略。
	limiterSweepInterval = time.Minute
)

// limiterEntry 单个 key 的滑动窗口状态。
//
// hits 按时间升序（Allow 只在末尾追加），故 hits[len-1] 就是最近一次命中，
// 主动清理据此判断整条是否已过期，无需另存 lastSeen。
type limiterEntry struct {
	hits   []time.Time
	window time.Duration // 该 key 最近一次 Allow 使用的窗口长度
}

// Limiter 滑动窗口限流器（内存有界）
type Limiter struct {
	mu        sync.Mutex
	hits      map[string]*limiterEntry
	maxKeys   int
	lastSweep time.Time
	// now 注入时钟：测试用固定时钟做确定性的过期回收断言（不依赖 sleep）。
	now func() time.Time
}

// NewLimiter 构造限流器（容量取 maxLimiterKeys）
func NewLimiter() *Limiter { return newLimiter(maxLimiterKeys, time.Now) }

// NewBoundedLimiter 按指定容量构造限流器。
//
// 导出原因：容量是有意的部署参数（GIN-复审 INFRA-04 反向）——单机自托管默认
// maxLimiterKeys 足够；若某入口的 key 空间天然更小/更大，可在装配层显式给定，
// 不必改全局常量。窗口/容量语义与 NewLimiter 完全一致。
func NewBoundedLimiter(maxKeys int) *Limiter { return newLimiter(maxKeys, time.Now) }

// newLimiter 按指定容量与时钟构造（生产走 NewLimiter；测试可注入小容量与固定时钟）
func newLimiter(maxKeys int, now func() time.Time) *Limiter {
	if maxKeys <= 0 {
		maxKeys = maxLimiterKeys
	}
	if now == nil {
		now = time.Now
	}
	return &Limiter{hits: make(map[string]*limiterEntry), maxKeys: maxKeys, now: now}
}

// Allow 在 window 内允许 max 次；超出返回 false。
//
// 返回 false 有三种含义，调用方一律按"限流"处理（HTTP 429）：
//  1. 窗口内命中数已达 max（正常限流）；
//  2. 该 key 是全新 key，但容量已打满且清理后仍满（容量保护，fail-closed）；
//  3. max <= 0（防御：调用方传了非法的 0，绝不退化成"不限流"）。
func (l *Limiter) Allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock()()

	// ① 主动清理（摊还）：回收"之后再也没被访问"的过期 key。
	//    只有惰性清理的话，打一枪就换 IP 的攻击者留下的 key 永远不会被访问，也就永远不会被回收。
	if now.Sub(l.lastSweep) >= limiterSweepInterval {
		l.sweepLocked(now)
	}

	e, ok := l.hits[key]
	if !ok {
		if len(l.hits) >= l.maxKeys {
			// ② 容量保护：先强制扫一遍（可能有大量 key 只是"还没轮到被扫"而实际已过期），
			//    仍满才拒绝新 key——绝不淘汰仍然有效的旧 key 来腾位置（见文件头取舍）。
			l.sweepLocked(now)
			if len(l.hits) >= l.maxKeys {
				return false
			}
		}
		e = &limiterEntry{window: window}
		l.hits[key] = e
	}
	e.window = window

	// ③ 惰性清理：裁掉窗口外的时间戳（原地过滤，复用底层数组）
	cutoff := now.Add(-window)
	kept := e.hits[:0]
	for _, t := range e.hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	e.hits = kept

	if len(kept) >= max {
		if len(kept) == 0 {
			// 全过期且本次不记录（max<=0）：删掉空壳 key，避免空条目长期占位
			delete(l.hits, key)
		}
		return false
	}
	e.hits = append(kept, now)
	return true
}

// Size 当前保留的 key 数量（限流器内存占用的直接度量；测试与巡检用）
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.hits)
}

// Release 撤销最近一次 Allow 为 key 记下的命中。
//
// 用途：两段式"失败才计数"——入口先 Allow 占位，业务判定为【成功】后归还占位，
// 使成功不占用失败配额（登录场景：多设备正常登录不会被误伤，A6）。
//
// 并发语义：同一 key 有并发请求时可能误删其他请求刚记下的占位（失败计数偏差 ±1）。
// 对限流而言这是可接受的小偏差；换来的是"成功绝不占配额"的确定语义。
// 若 key 无记录（已被惰性清理）则为幂等空操作。
func (l *Limiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.hits[key]
	if !ok || len(e.hits) == 0 {
		return
	}
	e.hits = e.hits[:len(e.hits)-1]
	if len(e.hits) == 0 {
		delete(l.hits, key) // 不留空壳 key（R44：内存有界）
	}
}

// Sweep 立即清理所有过期 key，返回被清理的条数。
//
// 正常路径由 Allow 按间隔自动触发；暴露出来是为了让巡检/运维（未来若接定时任务）
// 能在"刚经历一波突发流量之后"强制回收，也便于测试直接断言回收结果。
func (l *Limiter) Sweep() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	before := len(l.hits)
	l.sweepLocked(l.clock()())
	return before - len(l.hits)
}

// sweepLocked 全量清理过期条目；调用方必须已持锁。
//
// 过期判定：最近一次命中距现在已超过该 key 的窗口（hits 升序 ⇒ 末位即最近一次）。
// 仍在窗口内的条目一律保留——删掉它们才会让"本该被限流的请求放行"。
func (l *Limiter) sweepLocked(now time.Time) {
	l.lastSweep = now
	for k, e := range l.hits {
		if len(e.hits) == 0 || now.Sub(e.hits[len(e.hits)-1]) >= e.window {
			delete(l.hits, k)
		}
	}
}

// clock 返回可用的取时函数（零值 Limiter 兜底，避免 nil 调用 panic）
func (l *Limiter) clock() func() time.Time {
	if l.now == nil {
		return time.Now
	}
	return l.now
}
