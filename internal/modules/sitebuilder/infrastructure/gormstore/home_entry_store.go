package gormstore

import (
	"errors"

	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
	"gorm.io/gorm"
)

// HomeEntryStore 首页入口数据访问。
type HomeEntryStore struct {
	db *gorm.DB
}

// NewHomeEntryStore 创建 HomeEntryStore。
func NewHomeEntryStore(db *gorm.DB) *HomeEntryStore {
	return &HomeEntryStore{db: db}
}

// Create 新建首页入口。
func (s *HomeEntryStore) Create(entry *sitebuilderdomain.HomeEntry) error {
	return s.db.Create(entry).Error
}

// Update 更新首页入口。
func (s *HomeEntryStore) Update(entry *sitebuilderdomain.HomeEntry) error {
	return s.db.Save(entry).Error
}

// Delete 删除首页入口。
func (s *HomeEntryStore) Delete(id uint) error {
	if id == 0 {
		return nil
	}
	return s.db.Delete(&sitebuilderdomain.HomeEntry{}, id).Error
}

// GetByID 按 ID 查询。
func (s *HomeEntryStore) GetByID(id uint) (*sitebuilderdomain.HomeEntry, error) {
	if id == 0 {
		return nil, nil
	}
	var entry sitebuilderdomain.HomeEntry
	if err := s.db.Where("id = ?", id).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

// List 列出全部入口；enabledOnly=true 时仅返回启用项，按 sort_order 升序。
func (s *HomeEntryStore) List(enabledOnly bool) ([]sitebuilderdomain.HomeEntry, error) {
	query := s.db.Model(&sitebuilderdomain.HomeEntry{})
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	var rows []sitebuilderdomain.HomeEntry
	if err := query.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Reorder 批量更新排序。
func (s *HomeEntryStore) Reorder(items []ReorderItem) error {
	if len(items) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			if err := tx.Model(&sitebuilderdomain.HomeEntry{}).
				Where("id = ?", items[i].ID).
				UpdateColumn("sort_order", items[i].SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
