package service

// 保留期 / 有效期的时间换算（GPT-R27）。
//
// 为什么单独成文件：所有"天数 → 秒数"的换算必须走同一处且【带溢出检查】。
// 裸写 days*86400 的问题不止是溢出本身——溢出后 now-span 会绕成一个
// 【未来】时间戳，清理任务会把"过期"判成真，进而误删全表；这是不可逆的数据丢失，
// 且表现为"清理任务跑得很成功"，极难事后发现。
//
// 因此本文件的三条铁律：
//  1. 天数必须为正整数，且不得超过 maxRetentionDays（上限即溢出防护）；
//  2. 截止时间不得回绕到未来或 Unix 纪元之前，出现即显式报错；
//  3. 任一非法输入都【不返回可用的截止时间】，调用方拿不到值就不会删数据。

import (
	"fmt"
	"math"
)

const (
	// secondsPerDay 一天的秒数（全项目唯一定义，勿在别处再写 24*60*60）
	secondsPerDay = int64(24 * 60 * 60)
	// maxRetentionDays 保留天数上限（100 年）。
	// 取这个量级的意义：int64(days)*86400 在此上限下约 3.15e9，
	// 距 int64 上限（9.22e18）有 9 个数量级余量，【不可能溢出】。
	// 同时它也拦住了"配错成 999999999"这类明显越界的输入。
	maxRetentionDays = 36500
)

// daysToSeconds 天数转秒数，带非法值与溢出上限检查。
func daysToSeconds(days int) (int64, error) {
	if days <= 0 {
		return 0, fmt.Errorf("%w：天数必须为正整数，当前=%d", ErrInvalidArgument, days)
	}
	if int64(days) > maxRetentionDays {
		return 0, fmt.Errorf("%w：天数 %d 超过上限 %d 天", ErrInvalidArgument, days, maxRetentionDays)
	}
	return int64(days) * secondsPerDay, nil
}

// retentionCutoff 计算清理截止时刻：时间戳【小于等于】该值的数据可被清理。
//
// 与 daysToSeconds 的区别：它还负责阻止"截止时间落到 Unix 纪元之前"——
// 那种值要么什么都清不掉，要么（若列里存 0）反而匹配上未设置的行。
func retentionCutoff(days int, now int64) (int64, error) {
	if now <= 0 {
		return 0, fmt.Errorf("%w：当前时间非法（now=%d）", ErrInvalidArgument, now)
	}
	span, err := daysToSeconds(days)
	if err != nil {
		return 0, err
	}
	if now < span {
		return 0, fmt.Errorf("%w：保留 %d 天使截止时间早于 Unix 纪元（now=%d）",
			ErrInvalidArgument, days, now)
	}
	return now - span, nil
}

// expiryAfter 计算"从 now 起 days 天后"的过期时刻（前向 TTL，如邀请码有效期）。
func expiryAfter(days int, now int64) (int64, error) {
	if now <= 0 {
		return 0, fmt.Errorf("%w：当前时间非法（now=%d）", ErrInvalidArgument, now)
	}
	span, err := daysToSeconds(days)
	if err != nil {
		return 0, err
	}
	if now > math.MaxInt64-span {
		return 0, fmt.Errorf("%w：天数 %d 使过期时间超出 int64 上限", ErrInvalidArgument, days)
	}
	return now + span, nil
}
