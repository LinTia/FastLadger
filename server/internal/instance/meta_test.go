package instance

// instance.meta（restore_epoch 恢复世代）的纯逻辑测试。
//
// 该文件不碰数据库，因此在本机（无 gcc / CGO 不可用）可以完整验证：
// 首次生成、往返读取、Bump 递增、坏文件不被静默重建、权限 0600。
import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestLoadOrCreateRoundTrip 首次生成 → 再次读取必须完全一致（instance_id 固定不变）
func TestLoadOrCreateRoundTrip(t *testing.T) {
	dir := t.TempDir()

	first, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("首次生成失败：%v", err)
	}
	if len(first.InstanceId) != instanceIDBytes*2 {
		t.Fatalf("instance_id 长度应为 %d，实际 %q", instanceIDBytes*2, first.InstanceId)
	}
	if first.RestoreEpoch != 1 {
		t.Fatalf("首次 restore_epoch 应为 1，实际 %d", first.RestoreEpoch)
	}
	if first.RestoredAt != "" {
		t.Fatalf("从未恢复过时 restored_at 应为空串，实际 %q", first.RestoredAt)
	}

	second, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("再次加载失败：%v", err)
	}
	if second.InstanceId != first.InstanceId {
		t.Fatalf("instance_id 不稳定：%q → %q", first.InstanceId, second.InstanceId)
	}
	if second.RestoreEpoch != first.RestoreEpoch {
		t.Fatalf("重复加载不应改变 epoch：%d → %d", first.RestoreEpoch, second.RestoreEpoch)
	}

	// 落盘位置必须是 ${dataDir}/instance.meta（不在数据库里，指南 §26.1）
	if _, statErr := os.Stat(Path(dir)); statErr != nil {
		t.Fatalf("instance.meta 未落盘：%v", statErr)
	}
	// 文件内容字段名必须是 snake_case（运维脚本与文档逐字一致）
	raw, rerr := os.ReadFile(Path(dir))
	if rerr != nil {
		t.Fatalf("读取文件失败：%v", rerr)
	}
	for _, key := range []string{"instance_id", "restore_epoch", "restored_at"} {
		if !strings.Contains(string(raw), `"`+key+`"`) {
			t.Fatalf("instance.meta 缺少字段 %q：%s", key, raw)
		}
	}
	var decoded map[string]interface{}
	if jerr := json.Unmarshal(raw, &decoded); jerr != nil {
		t.Fatalf("instance.meta 不是合法 JSON：%v", jerr)
	}
}

// TestBump increments epoch and stamps RFC3339 time
func TestBump(t *testing.T) {
	dir := t.TempDir()

	before, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	after, err := Bump(dir)
	if err != nil {
		t.Fatalf("Bump 失败：%v", err)
	}
	if after.RestoreEpoch != before.RestoreEpoch+1 {
		t.Fatalf("epoch 应 +1：%d → %d", before.RestoreEpoch, after.RestoreEpoch)
	}
	if after.InstanceId != before.InstanceId {
		t.Fatalf("Bump 不得改变 instance_id：%q → %q", before.InstanceId, after.InstanceId)
	}
	if _, perr := time.Parse(time.RFC3339, after.RestoredAt); perr != nil {
		t.Fatalf("restored_at 不是 RFC3339：%q", after.RestoredAt)
	}

	// 落盘后重新读取必须是新值（不是只在内存里改）
	reloaded, err := Read(dir)
	if err != nil {
		t.Fatalf("重新读取失败：%v", err)
	}
	if reloaded.RestoreEpoch != after.RestoreEpoch || reloaded.RestoredAt != after.RestoredAt {
		t.Fatalf("Bump 未落盘：%+v != %+v", reloaded, after)
	}

	// 连续 Bump 必须单调递增
	third, err := Bump(dir)
	if err != nil {
		t.Fatalf("第二次 Bump 失败：%v", err)
	}
	if third.RestoreEpoch != after.RestoreEpoch+1 {
		t.Fatalf("epoch 应继续 +1：%d → %d", after.RestoreEpoch, third.RestoreEpoch)
	}
}

// TestReadMissing 不存在时返回 ErrMetaNotFound（LoadOrCreate 才负责创建）
func TestReadMissing(t *testing.T) {
	if _, err := Read(t.TempDir()); !errors.Is(err, ErrMetaNotFound) {
		t.Fatalf("应返回 ErrMetaNotFound，实际 %v", err)
	}
}

// TestBrokenFileIsNotSilentlyRebuilt 坏文件必须报错，不能被"悄悄重建"洗掉世代信息
func TestBrokenFileIsNotSilentlyRebuilt(t *testing.T) {
	dir := t.TempDir()
	path := Path(dir)
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("准备坏文件失败：%v", err)
	}
	if _, err := LoadOrCreate(dir); !errors.Is(err, ErrMetaInvalid) {
		t.Fatalf("坏 JSON 应返回 ErrMetaInvalid，实际 %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败：%v", err)
	}
	if string(raw) != "not-json" {
		t.Fatalf("坏文件不应被覆盖，实际内容 %q", raw)
	}
}

// TestValidate 校验规则表（缺 id / epoch 非正 / 非法时间）
func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		meta Meta
		ok   bool
	}{
		{"合法-未恢复过", Meta{InstanceId: "abc", RestoreEpoch: 1}, true},
		{"合法-已恢复", Meta{InstanceId: "abc", RestoreEpoch: 3, RestoredAt: "2026-10-29T10:00:00Z"}, true},
		{"缺 id", Meta{RestoreEpoch: 1}, false},
		{"id 全空白", Meta{InstanceId: "   ", RestoreEpoch: 1}, false},
		{"epoch=0", Meta{InstanceId: "abc"}, false},
		{"epoch 负数", Meta{InstanceId: "abc", RestoreEpoch: -1}, false},
		{"时间格式错", Meta{InstanceId: "abc", RestoreEpoch: 1, RestoredAt: "2026/10/29"}, false},
		{"id 过长", Meta{InstanceId: strings.Repeat("a", 65), RestoreEpoch: 1}, false},
	}
	for _, c := range cases {
		err := c.meta.Validate()
		if c.ok && err != nil {
			t.Errorf("%s：期望通过，实际 %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s：期望被拒绝", c.name)
		}
		if !c.ok && err != nil && !errors.Is(err, ErrMetaInvalid) {
			t.Errorf("%s：错误应包装 ErrMetaInvalid，实际 %v", c.name, err)
		}
	}
}

// TestFilePermissionIsPrivate 0600（Windows 上权限语义不同，跳过）
func TestFilePermissionIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不实现 POSIX 文件权限位")
	}
	dir := t.TempDir()
	if _, err := LoadOrCreate(dir); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	st, err := os.Stat(Path(dir))
	if err != nil {
		t.Fatalf("stat 失败：%v", err)
	}
	if perm := st.Mode().Perm(); perm != filePerm {
		t.Fatalf("instance.meta 权限应为 %o，实际 %o", filePerm, perm)
	}
}

// TestPathDefaultDir 空目录回落 ./data（避免写到进程 CWD 根）
func TestPathDefaultDir(t *testing.T) {
	want := filepath.Join(DefaultDir, fileName)
	if got := Path(""); got != want {
		t.Fatalf("空目录应回落默认目录，实际 %q（期望 %q）", got, want)
	}
	if got := Path("  "); got != want {
		t.Fatalf("空白目录应回落默认目录，实际 %q（期望 %q）", got, want)
	}
	if got := Path("/srv/fl"); got != filepath.Join("/srv/fl", fileName) {
		t.Fatalf("显式目录应用传入值，实际 %q", got)
	}
}
