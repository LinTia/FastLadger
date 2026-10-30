//go:build windows

package database

import "log"

// ApplyStrictUmask Windows 无 POSIX umask 语义，此处为空实现（GPT-R35）。
//
// 同样地，Windows 没有 POSIX 权限位（os.Chmod(0600/0700) 是空操作，
// st.Mode().Perm() 恒为 0666/0777），因此 EnsureDataDir / HardenDBFilePerms
// 在 Windows 上也不执行 Chmod，权限保证由部署形态（容器/单用户目录/ACL）承担。
func ApplyStrictUmask() {
	log.Printf("Windows 平台不支持 POSIX umask：文件权限依赖部署形态（容器/单用户目录）")
}
