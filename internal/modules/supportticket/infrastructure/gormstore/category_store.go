package gormstore

import (
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
)

// CreateCategory 创建分类。
func (s *Store) CreateCategory(c *supportdomain.Category) error {
	return s.db.Create(c).Error
}

// UpdateCategory 更新分类。
func (s *Store) UpdateCategory(c *supportdomain.Category) error {
	return s.db.Save(c).Error
}

// GetCategoryByID 按 ID 取分类。
func (s *Store) GetCategoryByID(id uint) (*supportdomain.Category, error) {
	if id == 0 {
		return nil, nil
	}
	var c supportdomain.Category
	if err := s.db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, firstErr(err)
	}
	return &c, nil
}

// GetCategoryByCode 按 code 取分类。
func (s *Store) GetCategoryByCode(code string) (*supportdomain.Category, error) {
	if code == "" {
		return nil, nil
	}
	var c supportdomain.Category
	if err := s.db.Where("code = ?", code).First(&c).Error; err != nil {
		return nil, firstErr(err)
	}
	return &c, nil
}

// ListEnabledCategories 列出启用分类（用户侧），按 sort_order。
func (s *Store) ListEnabledCategories() ([]supportdomain.Category, error) {
	var rows []supportdomain.Category
	if err := s.db.Where("enabled = ?", true).Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAllCategories 列出全部分类（后台），按 sort_order。
func (s *Store) ListAllCategories() ([]supportdomain.Category, error) {
	var rows []supportdomain.Category
	if err := s.db.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
