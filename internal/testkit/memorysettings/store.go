// Package settingsstore provides an in-memory implementation of the settings
// persistence contract for tests outside the settings bounded context.
package memorysettings

import (
	"encoding/json"

	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// Store keeps settings values in memory.
type Store struct {
	Values map[string]jsonmap.JSON
}

// New creates an empty in-memory settings store.
func New() *Store {
	return &Store{Values: make(map[string]jsonmap.JSON)}
}

// GetByKey implements settings/contract.Store.
func (store *Store) GetByKey(key string) (jsonmap.JSON, bool, error) {
	value, found := store.Values[key]
	return value, found, nil
}

// Upsert implements settings/contract.Store.
//
// 真实数据库（gorm type:json 列）在写入/读取时会经过 JSON 序列化往返，把
// []map[string]interface{} 归一化为 []interface{}。这里同样做一次 JSON 往返，
// 保证测试替身与真实持久化的类型行为一致（否则嵌套 level_rates 等字段会因
// []map 无法断言为 []interface{} 而在 Decode 时丢失）。
func (store *Store) Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var rounded jsonmap.JSON
	if err := json.Unmarshal(raw, &rounded); err != nil {
		return nil, err
	}
	store.Values[key] = rounded
	return rounded, nil
}
