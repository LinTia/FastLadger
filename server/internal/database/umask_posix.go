//go:build !windows

package database

import (
	"log"
	"syscall"
)

// ApplyStrictUmask 把进程 umask 收紧为 0077，使本进程此后新建的文件默认 0600、
// 目录默认 0700（GPT-R35 的第二道防线）。
//
// 为什么需要它：HardenDBFilePerms 只能收紧【调用时已存在】的文件，而 WAL 的
// -wal sidecar 是在首次写入时才生成的——那一刻如果 umask 是 022，新 sidecar
// 就是 0644。umask 是进程级的，能覆盖"之后才出现的文件"。
//
// 定位是【补充】而非唯一手段：umask 对"已存在的宽松文件"无能为力，
// 那部分由 EnsureDataDir / HardenDBFilePerms 的显式 Chmod 负责。
//
// 只应在进程启动早期调用一次：umask 是进程全局状态，改晚了对之前创建的文件无效。
func ApplyStrictUmask() {
	old := syscall.Umask(0o077)
	if old != 0o077 {
		log.Printf("已收紧进程 umask：%04o → 0077（新建数据文件默认 0600）", old)
	}
}
