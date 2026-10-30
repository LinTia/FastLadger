// Package instance 实例级元信息：恢复世代（restore_epoch）。
//
// 依据《新手开发完整指南》§26.1 与《落地方案》§8.3：
// 从一个旧备份恢复后数据库里的版本号也回到旧值，仍缓存着"恢复后状态"的客户端会继续按
// 旧游标 sync/push、sync/pull，把数据覆盖回去或产生重复，且没有任何机制让它们察觉
// "服务端整体回退了"。因此用一个**独立于数据库、不随备份恢复**的元信息文件
//
//	${EBK_DATA_DIR}/instance.meta
//	{ "instance_id": "随机固定值", "restore_epoch": 1, "restored_at": "" }
//
// 记住"这个实例被恢复过几次"。恢复流程（运维手册）：停容器 → 替换 db 文件 →
// `fastledger instance bump` → 启动。备份包必须**排除** instance.meta。
//
// 客户端处理规则（PRD §12.1 / 指南 §26.1）：发现 epoch 不一致即
// ① 停止基于旧游标的增量 push/pull；② 全量重建本地库；③ 列出离线队列让用户逐条确认后重放。
package instance

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// DefaultDir 默认数据目录（与 config.DefaultDBPath 的 ./data 保持一致）
	DefaultDir = "./data"
	// fileName 元信息文件名（固定名字，运维脚本依赖它）
	fileName = "instance.meta"
	// filePerm 权限 0600：实例标识不敏感，但按最小权限落盘（《落地方案》§5.5 文件权限受控）
	filePerm = 0o600
	// dirPerm 数据目录权限。
	//
	// 0700 而非 0755（GPT-R35）：这个目录里放的是 instance.meta 与数据库，
	// `fastledger instance bump` 往往是【第一个】创建它的命令（恢复流程在启动之前跑），
	// 若这里建成 0755，bump 到服务启动之间会有一段"同机其他用户可进"的窗口。
	// （服务启动后 database.EnsureDataDir 会再收紧一次，双重覆盖。）
	dirPerm = 0o700
	// instanceIDBytes instance_id 的随机字节数（16 字节 → 32 位十六进制）
	instanceIDBytes = 16
	// tmpSuffix 原子写入的临时文件后缀
	tmpSuffix = ".tmp"
)

var (
	// ErrMetaInvalid instance.meta 内容非法（缺字段 / epoch 非正 / 非法 JSON）
	ErrMetaInvalid = errors.New("instance.meta 内容非法")
	// ErrMetaNotFound instance.meta 不存在（Read 专用，LoadOrCreate 会自动创建）
	ErrMetaNotFound = errors.New("instance.meta 不存在")
)

// Meta 实例元信息（JSON 字段名与指南 §26.1 的示例**逐字一致**，勿改 snake_case）
type Meta struct {
	// InstanceId 实例唯一标识（随机生成后固定不变，用于识别"这是同一个实例"）
	InstanceId string `json:"instance_id"`
	// RestoreEpoch 恢复世代：每次从备份恢复后 +1，**不随备份恢复**
	RestoreEpoch int64 `json:"restore_epoch"`
	// RestoredAt 最近一次恢复的时间（RFC3339；从未恢复过为空串）
	RestoredAt string `json:"restored_at"`
}

// Path 返回 instance.meta 的完整路径（空目录回落默认目录，避免写到进程 CWD 根）
func Path(dataDir string) string {
	return filepath.Join(normalizeDir(dataDir), fileName)
}

// normalizeDir 数据目录归一化：空白 → 默认 ./data
func normalizeDir(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return DefaultDir
	}
	return dataDir
}

// Validate 校验元信息自洽性。
//
// 为什么不"读到坏文件就悄悄重建"：重建会把 instance_id 与 epoch 一起抹掉，
// 让一次真实的损坏变成"看起来正常的新实例"，反而让客户端的世代检测失效。
// 因此一律 fail-fast，由运维介入处理。
func (m *Meta) Validate() error {
	if m == nil {
		return fmt.Errorf("%w：元信息为空", ErrMetaInvalid)
	}
	if strings.TrimSpace(m.InstanceId) == "" {
		return fmt.Errorf("%w：instance_id 不能为空", ErrMetaInvalid)
	}
	if len(m.InstanceId) > 64 {
		return fmt.Errorf("%w：instance_id 过长（%d > 64）", ErrMetaInvalid, len(m.InstanceId))
	}
	if m.RestoreEpoch <= 0 {
		return fmt.Errorf("%w：restore_epoch 必须为正整数，当前=%d", ErrMetaInvalid, m.RestoreEpoch)
	}
	if m.RestoredAt != "" {
		if _, err := time.Parse(time.RFC3339, m.RestoredAt); err != nil {
			return fmt.Errorf("%w：restored_at 不是 RFC3339（%q）", ErrMetaInvalid, m.RestoredAt)
		}
	}
	return nil
}

// Read 只读加载并校验（文件不存在返回 ErrMetaNotFound）
func Read(dataDir string) (*Meta, error) {
	path := Path(dataDir)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w：%s", ErrMetaNotFound, path)
		}
		return nil, fmt.Errorf("读取 %s 失败：%w", path, err)
	}
	m := &Meta{}
	if err := json.Unmarshal(b, m); err != nil {
		return nil, fmt.Errorf("%w：%s 不是合法 JSON（%v）", ErrMetaInvalid, path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadOrCreate 加载元信息；不存在则生成（instance_id 随机、restore_epoch=1）并落盘。
//
// 服务启动时调用一次：既保证后续 /healthz 与 sync/pull 能回报世代，
// 又保证首次启动就有一个可被备份排除逻辑识别的固定文件。
func LoadOrCreate(dataDir string) (*Meta, error) {
	m, err := Read(dataDir)
	if err == nil {
		return m, nil
	}
	if !errors.Is(err, ErrMetaNotFound) {
		// 文件存在但非法：不自动修复（见 Validate 注释）
		return nil, err
	}
	id, err := newInstanceID()
	if err != nil {
		return nil, err
	}
	m = &Meta{InstanceId: id, RestoreEpoch: 1, RestoredAt: ""}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := write(dataDir, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Bump 恢复后递增世代：restore_epoch += 1 且 restored_at = 当前时间（RFC3339），落盘。
//
// 只由 `fastledger instance bump` 子命令调用（《落地方案》§8.3 的恢复流程）。
// 服务进程不要在运行时调用它：世代变化意味着"所有客户端都要全量重建"，
// 应发生在启动前，避免运行中的客户端拿到前后不一致的世代。
func Bump(dataDir string) (*Meta, error) {
	m, err := LoadOrCreate(dataDir)
	if err != nil {
		return nil, err
	}
	m.RestoreEpoch++
	m.RestoredAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := write(dataDir, m); err != nil {
		return nil, err
	}
	return m, nil
}

// write 原子落盘 + fsync（GPT-R39）。
//
// 分四步，任一步失败都必须返回错误（不做"尽力而为"）：
//  1. 写临时文件 → Sync → Close（数据本体落盘）；
//  2. Rename 覆盖目标（原子：要么旧内容要么新内容，不会半截）；
//  3. 对【父目录】Sync（目录项变更落盘）。
//
// 为什么第 3 步不能省：rename 只保证原子性，**不保证断电持久化**。
// 没有目录 fsync 时，Bump 返回成功之后立刻断电，仍可能读到旧 restore_epoch——
// 客户端就会以为数据库没恢复过，继续用旧游标 sync/push 把数据覆盖回去。
//
// （meta 文件很小，但它是"恢复世代"的唯一凭据，半截文件会让全实例客户端无法判断世代。）
func write(dataDir string, m *Meta) error {
	dir := normalizeDir(dataDir)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("创建数据目录 %s 失败：%w", dir, err)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 instance.meta 失败：%w", err)
	}
	b = append(b, '\n')

	path := Path(dataDir)
	tmp := path + tmpSuffix
	if err := writeFileSync(tmp, b); err != nil {
		// 写坏了就带走临时文件，避免残留被误认为备份包的一部分
		cleanupTmp(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanupTmp(tmp)
		return fmt.Errorf("落盘 %s 失败：%w", path, err)
	}
	// rename 已生效，但目录项本身也要落盘，否则断电后可能退回旧世代
	if err := syncDirHook(dir); err != nil {
		return fmt.Errorf("同步目录 %s 失败（rename 已生效，断电后仍可能丢失）：%w", dir, err)
	}
	return nil
}

// cleanupTmp 清理写入失败的临时文件。
//
// 只删【普通文件】：万一这个临时名字被别的东西占成了目录，
// 一次失败的写入不应该顺手删掉一个目录。
func cleanupTmp(tmp string) {
	if st, err := os.Stat(tmp); err == nil && st.IsDir() {
		return
	}
	_ = os.Remove(tmp)
}

// writeFileSync 写临时文件：Write → Sync → Close，三步都必须成功。
//
// 顺序不能颠倒：Sync 必须在 Close 之前（要拿着句柄刷），Close 的返回值也要检查
// —— 部分文件系统会在 Close 时才真正报错。
func writeFileSync(tmp string, b []byte) error {
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("创建 %s 失败：%w", tmp, err)
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return fmt.Errorf("写入 %s 失败：%w", tmp, err)
	}
	if err := syncFileHook(f); err != nil {
		_ = f.Close()
		return fmt.Errorf("同步 %s 失败（数据可能未落盘）：%w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭 %s 失败：%w", tmp, err)
	}
	return nil
}

// syncDir 对目录执行 fsync，让 rename 产生的目录项变更真正落盘。
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		// Windows 不允许对目录句柄调用 FlushFileBuffers（实测 Access is denied），
		// 目录持久化只能依赖系统缓存策略；此处显式返回 nil 而不是吞掉真实错误。
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("打开目录 %s 失败：%w", dir, err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return fmt.Errorf("同步目录 %s 失败：%w", dir, err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("关闭目录 %s 失败：%w", dir, err)
	}
	return nil
}

// 同步钩子：生产路径指向真实 fsync；测试可替换为故障注入实现（GPT-R39）。
//
// 之所以要做成可替换的变量，是因为"断电"无法在单元测试里真实发生，
// 能验证的只有【调用序列与错误处理】——见 meta_r39_test.go 的验证边界说明。
var (
	// syncFileHook 刷临时文件的数据本体
	syncFileHook = func(f *os.File) error { return f.Sync() }
	// syncDirHook 刷父目录的目录项
	syncDirHook = func(dir string) error { return syncDir(dir) }
)

// newInstanceID 生成 32 位十六进制随机实例 ID（crypto/rand，不可预测、无需时间戳）
func newInstanceID() (string, error) {
	b := make([]byte, instanceIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成 instance_id 失败：%w", err)
	}
	return hex.EncodeToString(b), nil
}
