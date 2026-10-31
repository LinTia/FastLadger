package instance

// N4（INFRA-04）的回归测试：bump 必须能区分"读取已有"与"静默新建"。
//
// 原缺陷：在错误的 EBK_DATA_DIR 下执行 `instance bump`，会新建一个 instance.meta
// 并递增它——真正的实例纹丝不动、也不报错，客户端从此把新实例当另一个实例。

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadOrCreateWithStatusReportsCreation 首次是新建，之后是读取已有
func TestLoadOrCreateWithStatusReportsCreation(t *testing.T) {
	dir := t.TempDir()

	first, created, err := LoadOrCreateWithStatus(dir)
	if err != nil {
		t.Fatalf("首次 LoadOrCreateWithStatus 失败：%v", err)
	}
	if !created {
		t.Fatal("目录里原本没有 instance.meta，应回报 created=true")
	}
	if first.RestoreEpoch != 1 {
		t.Fatalf("新建实例的 restore_epoch 应为 1，实际 %d", first.RestoreEpoch)
	}

	second, created2, err := LoadOrCreateWithStatus(dir)
	if err != nil {
		t.Fatalf("二次 LoadOrCreateWithStatus 失败：%v", err)
	}
	if created2 {
		t.Fatal("文件已存在，应回报 created=false")
	}
	if second.InstanceId != first.InstanceId {
		t.Fatalf("instance_id 不应变化：%q → %q", first.InstanceId, second.InstanceId)
	}
}

// TestLoadOrCreateWithStatusOnBrokenFileIsNotCreate 坏文件不是"新建"，必须报错
func TestLoadOrCreateWithStatusOnBrokenFileIsNotCreate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("准备坏文件失败：%v", err)
	}
	if _, created, err := LoadOrCreateWithStatus(dir); err == nil {
		t.Fatal("坏文件应报错，不得被当成「新建」处理")
	} else if created {
		t.Fatal("坏文件场景不得回报 created=true")
	}
}

// TestPathIsUsableForAbsJoin CLI 打绝对路径时依赖 Path 的拼装结果；保证文件名不被改
func TestPathIsUsableForAbsJoin(t *testing.T) {
	p := Path("./data")
	if base := filepath.Base(p); base != fileName {
		t.Fatalf("Path 应以 %s 结尾，实际 %q", fileName, base)
	}
}

// TestR2StaleTmpDoesNotAffectNextStart 残留的 instance.meta.tmp 不得影响下次启动。
//
// 吸收自复审探针：上一次进程在写 tmp 之后、rename 之前被 kill，会留下半截
// instance.meta.tmp；Read 只认 instance.meta，tmp 由下一次 write 的 O_TRUNC 覆盖。
func TestR2StaleTmpDoesNotAffectNextStart(t *testing.T) {
	dir := t.TempDir()
	m, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate 失败：%v", err)
	}
	tmp := Path(dir) + tmpSuffix
	if err := os.WriteFile(tmp, []byte(`{"instance_id":"zzz","restore_epoch":99,"restored_at"`), 0o600); err != nil {
		t.Fatalf("写入残留 tmp 失败：%v", err)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("残留 tmp 不应影响 Read：%v", err)
	}
	if got.RestoreEpoch != 1 || got.InstanceId != m.InstanceId {
		t.Fatalf("残留 tmp 污染了读取结果：%+v", got)
	}

	after, err := Bump(dir)
	if err != nil {
		t.Fatalf("残留 tmp 不应让 Bump 失败：%v", err)
	}
	if after.RestoreEpoch != 2 {
		t.Fatalf("Bump 应得到 epoch=2，实际 %d", after.RestoreEpoch)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("Bump 后不应残留 %s（stat err=%v）", tmp, err)
	}
}
