package service

// 简单内存限流器（滑动窗口）。
// 用于邀请码的生成与使用限流（PRD §12.1：每账本每分钟 ≤5 生成、同实例每小时 ≤10 尝试）。
// 单机自托管场景足够；若未来多实例部署需换成共享存储。
import (
	"sync"
	"time"
)

// Limiter 滑动窗口限流器
type Limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

// NewLimiter 构造限流器
func NewLimiter() *Limiter { return &Limiter{hits: make(map[string][]time.Time)} }

// Allow 在 window 内允许 max 次；超出返回 false
func (l *Limiter) Allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	kept := make([]time.Time, 0, len(l.hits[key])+1)
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
