package application

import (
	"strings"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	"github.com/Aether-v1/hcz/internal/modules/supportticket/statemachine"
)

// ==================== 分类管理用例 ====================

// ListEnabledCategories 用户侧启用分类。
func (s *Service) ListEnabledCategories() ([]supportdomain.Category, error) {
	return s.repo.ListEnabledCategories()
}

// ListAllCategories 后台全部分类。
func (s *Service) ListAllCategories() ([]supportdomain.Category, error) {
	return s.repo.ListAllCategories()
}

// CreateCategory 创建分类。
func (s *Service) CreateCategory(in supportcontract.CreateCategoryInput) (*supportdomain.Category, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return nil, supportcontract.ErrCategoryNotFound
	}
	if in.DefaultPriority == "" {
		in.DefaultPriority = statemachine.PriorityNormal
	}
	if !statemachine.ValidPriority(in.DefaultPriority) {
		return nil, supportcontract.ErrInvalidPriority
	}
	// code 唯一校验。
	existing, err := s.repo.GetCategoryByCode(in.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, supportcontract.ErrDuplicateCategoryCode
	}
	c := &supportdomain.Category{
		Code:            in.Code,
		Name:            in.Name,
		Enabled:         in.Enabled,
		SortOrder:       in.SortOrder,
		DefaultPriority: in.DefaultPriority,
	}
	if err := s.repo.CreateCategory(c); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateCategory 更新分类。
func (s *Service) UpdateCategory(id uint, in supportcontract.UpdateCategoryInput) (*supportdomain.Category, error) {
	c, err := s.repo.GetCategoryByID(id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, supportcontract.ErrCategoryNotFound
	}
	if !statemachine.ValidPriority(in.DefaultPriority) {
		return nil, supportcontract.ErrInvalidPriority
	}
	c.Name = strings.TrimSpace(in.Name)
	c.Enabled = in.Enabled
	c.SortOrder = in.SortOrder
	c.DefaultPriority = in.DefaultPriority
	if err := s.repo.UpdateCategory(c); err != nil {
		return nil, err
	}
	return c, nil
}

// DisableCategory 软禁用分类（保留历史工单引用完整性）。
func (s *Service) DisableCategory(id uint) error {
	c, err := s.repo.GetCategoryByID(id)
	if err != nil {
		return err
	}
	if c == nil {
		return supportcontract.ErrCategoryNotFound
	}
	c.Enabled = false
	return s.repo.UpdateCategory(c)
}
