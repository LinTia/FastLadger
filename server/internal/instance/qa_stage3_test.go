package instance

// QA 独立验收（阶段 3.5：restore_epoch）—— 只补工程师用例未覆盖的边界。
//
// 关注点：世代文件是"恢复后唯一能让客户端察觉回退"的凭据，因此：
//   - 写盘必须是原子的（否则断电留下半个 JSON → 全实例客户端无法判断世代）；
//   - 坏文件绝不静默重建（重建会把 epoch 抹回 1，等于让客户端以为"没回退"）；
//   - Bump 未落盘不得返回成功（内存改了、盘上没改，重启即回退）。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQAS3BumpLeavesNoTempFile 原子写入不得残留 *.tmp（备份脚本依赖目录内容干净）
func TestQAS3BumpLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadOrCreate(dir); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	if _, err := Bump(dir); err != nil {
		t.Fatalf("Bump 失败：%v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败：%v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), tmpSuffix) {
			t.Fatalf("原子写入残留临时文件：%s", e.Name())
		}
		if e.Name() != fileName {
			t.Fatalf("数据目录出现意外文件：%s（备份排除逻辑按 %s 精确匹配）", e.Name(), fileName)
		}
	}
}

// TestQAS3BumpRefusesCorruptFile 坏文件下 Bump 必须失败且不改写文件
// （否则运维一次 bump 就把"曾经恢复过几次"洗成 1，客户端失去回退检测能力）
func TestQAS3BumpRefusesCorruptFile(t *testing.T) {
	dir := t.TempDir()
	broken := []byte(`{"instance_id": "x", "restore_epoch": 0}`) // epoch 非正 → 非法
	if err := os.WriteFile(Path(dir), broken, 0o600); err != nil {
		t.Fatalf("准备坏文件失败：%v", err)
	}
	if _, err := Bump(dir); !errors.Is(err, ErrMetaInvalid) {
		t.Fatalf("坏文件下 Bump 应返回 ErrMetaInvalid，实际 %v", err)
	}
	raw, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("读取文件失败：%v", err)
	}
	if string(raw) != string(broken) {
		t.Fatalf("坏文件不应被改写，实际 %q", raw)
	}
}

// TestQAS3ReadNeverCreatesDirectory Read 是只读操作，不得产生数据目录副作用。
func TestQAS3ReadNeverCreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "not-created")
	if _, err := Read(dir); !errors.Is(err, ErrMetaNotFound) {
		t.Fatalf("应返回 ErrMetaNotFound，实际 %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Read 不得创建目录，实际 stat err=%v", err)
	}
}

// TestQAS3EpochMonotonicAcrossManyBumps 多次 Bump 必须严格单调 +1（世代只能前进）
func TestQAS3EpochMonotonicAcrossManyBumps(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	cur := first.RestoreEpoch
	for i := 0; i < 5; i++ {
		m, err := Bump(dir)
		if err != nil {
			t.Fatalf("第 %d 次 Bump 失败：%v", i+1, err)
		}
		if m.RestoreEpoch != cur+1 {
			t.Fatalf("第 %d 次 Bump：期望 epoch=%d，实际 %d", i+1, cur+1, m.RestoreEpoch)
		}
		if m.InstanceId != first.InstanceId {
			t.Fatalf("Bump 不得改变 instance_id")
		}
		// 每次都必须是"已落盘"的值（重启后不倒退）
		disk, err := Read(dir)
		if err != nil {
			t.Fatalf("重读失败：%v", err)
		}
		if disk.RestoreEpoch != m.RestoreEpoch {
			t.Fatalf("落盘值不一致：内存 %d / 磁盘 %d", m.RestoreEpoch, disk.RestoreEpoch)
		}
		cur = m.RestoreEpoch
	}
}

// TestQAS3MetaFileIsNotInDBPath 世代信息必须在文件系统（不随数据库备份恢复）。
// 这里用"同一数据目录下不存在任何 db 文件被写入"来固化该结构决策。
func TestQAS3MetaFileIsNotInDBPath(t *testing.T) {
	dir := t.TempDir()
	if _, err := Bump(dir); err != nil {
		t.Fatalf("Bump 失败：%v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败：%v", err)
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		for _, dbSuffix := range []string{".db", ".sqlite", ".sqlite3", "-wal", "-shm"} {
			if strings.HasSuffix(name, dbSuffix) {
				t.Fatalf("instance.meta 不得与数据库文件耦合：发现 %s", e.Name())
			}
		}
	}
}
