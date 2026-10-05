package gormstore

import (
	"errors"

	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
	"gorm.io/gorm"
)

// DiscoveryBlockStore 发现页区块数据访问。
type DiscoveryBlockStore struct {
	db *gorm.DB
}

// NewDiscoveryBlockStore 创建 DiscoveryBlockStore。
func NewDiscoveryBlockStore(db *gorm.DB) *DiscoveryBlockStore {
	return &DiscoveryBlockStore{db: db}
}

// Create 新建区块。
func (s *DiscoveryBlockStore) Create(block *sitebuilderdomain.DiscoveryBlock) error {
	return s.db.Create(block).Error
}

// Update 更新区块。
func (s *DiscoveryBlockStore) Update(block *sitebuilderdomain.DiscoveryBlock) error {
	return s.db.Save(block).Error
}

// Delete 删除区块。
func (s *DiscoveryBlockStore) Delete(id uint) error {
	if id == 0 {
		return nil
	}
	return s.db.Delete(&sitebuilderdomain.DiscoveryBlock{}, id).Error
}

// GetByID 按 ID 查询。
func (s *DiscoveryBlockStore) GetByID(id uint) (*sitebuilderdomain.DiscoveryBlock, error) {
	if id == 0 {
		return nil, nil
	}
	var block sitebuilderdomain.DiscoveryBlock
	if err := s.db.Where("id = ?", id).First(&block).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &block, nil
}

// List 列出全部区块；enabledOnly=true 时仅返回启用项，按 sort_order 升序。
func (s *DiscoveryBlockStore) List(enabledOnly bool) ([]sitebuilderdomain.DiscoveryBlock, error) {
	query := s.db.Model(&sitebuilderdomain.DiscoveryBlock{})
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	var rows []sitebuilderdomain.DiscoveryBlock
	if err := query.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ReorderItem 批量排序单条。
type ReorderItem struct {
	ID        uint
	SortOrder int
}

// Reorder 批量更新排序。
func (s *DiscoveryBlockStore) Reorder(items []ReorderItem) error {
	if len(items) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			if err := tx.Model(&sitebuilderdomain.DiscoveryBlock{}).
				Where("id = ?", items[i].ID).
				UpdateColumn("sort_order", items[i].SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
