package model

// 同步墓碑的 JSON 契约测试（阶段 3 返工 P2-2）。
//
// 背景：sync/pull 故意下发墓碑行（不过滤 deleted），客户端必须能识别 `deleted=true`
// 并覆盖本地行。若 `Deleted` 仍是 `json:"-"`，墓碑会被当活行处理（"删掉的数据又回来了"）。
// REST 读路径本就只查 `deleted=0`，因此对外暴露该字段没有副作用。
import (
	"encoding/json"
	"strings"
	"testing"
)

// decodeMap 序列化后转成 map，便于断言字段存在性
func decodeMap(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("反序列化失败：%s", raw)
	}
	return m
}

// TestTombstoneFieldsAreExposed 四类同步实体的 deleted / deletedAt 必须出现在 JSON 里
func TestTombstoneFieldsAreExposed(t *testing.T) {
	entities := map[string]func(bool) interface{}{
		"transaction": func(deleted bool) interface{} {
			return Transaction{Id: 1, BookId: 2, Deleted: deleted}
		},
		"account": func(deleted bool) interface{} {
			return Account{Id: 1, BookId: 2, Deleted: deleted}
		},
		"category": func(deleted bool) interface{} {
			return Category{Id: 1, BookId: 2, Deleted: deleted}
		},
		"holding": func(deleted bool) interface{} {
			return Holding{Id: 1, BookId: 2, Deleted: deleted}
		},
	}
	for name, build := range entities {
		// 活行：deleted=false 必须显式出现（客户端要能区分"没这个字段"与"未删除"）
		alive := decodeMap(t, build(false))
		if v, ok := alive["deleted"]; !ok || v != false {
			t.Errorf("%s：活行必须输出 deleted=false，实际 %v", name, alive)
		}
		// deletedAt=0 时不输出（omitempty），避免客户端误判墓碑时间
		if _, ok := alive["deletedAt"]; ok {
			t.Errorf("%s：deletedAt=0 不应输出，实际 %v", name, alive["deletedAt"])
		}

		// 墓碑：deleted=true 必须可见
		dead := decodeMap(t, build(true))
		if v, ok := dead["deleted"]; !ok || v != true {
			t.Fatalf("%s：墓碑行必须输出 deleted=true（否则客户端把删除当活行），实际 %v", name, dead)
		}
	}
}

// TestTombstoneDeletedAtOmitEmpty deletedAt 有值时必须输出，且为数字（Unix 秒）
func TestTombstoneDeletedAtOmitEmpty(t *testing.T) {
	m := decodeMap(t, Transaction{Id: 1, Deleted: true, DeletedAt: 1793260800})
	if got, ok := m["deletedAt"]; !ok {
		t.Fatalf("墓碑时间必须下发给客户端：%v", m)
	} else if got != float64(1793260800) {
		t.Fatalf("deletedAt 期望 1793260800，实际 %v", got)
	}
}

// TestTombstoneFieldIsNotEmptyString 防止有人把字段改成 `json:"-"` 或用空字符串占位
func TestTombstoneFieldIsNotEmptyString(t *testing.T) {
	raw, err := json.Marshal(Transaction{Deleted: true})
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	if !strings.Contains(string(raw), `"deleted":true`) {
		t.Fatalf("Transaction JSON 缺少可见的 deleted 标记：%s", raw)
	}
}
