package application

import (
	"errors"
	"strconv"

	categorydomain "github.com/Aether-v1/hcz/internal/modules/catalog/category/domain"
	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
)

// MaxFeaturedCategories 首页热门推荐分类上限。
const MaxFeaturedCategories = 6

// HomeFeaturedCategoryStore 首页推荐分类存储端口。
type HomeFeaturedCategoryStore interface {
	Create(item *sitebuilderdomain.HomeFeaturedCategory) error
	Update(item *sitebuilderdomain.HomeFeaturedCategory) error
	Delete(id uint) error
	GetByID(id uint) (*sitebuilderdomain.HomeFeaturedCategory, error)
	List(enabledOnly bool) ([]sitebuilderdomain.HomeFeaturedCategory, error)
	Reorder(items []ReorderItem) error
	Count() (int64, error)
}

// CategoryLookup 分类查询窄接口，用于校验 category_id 存在性与启用状态。
type CategoryLookup interface {
	GetByID(id string) (*categorydomain.Category, error)
}

// HomeFeaturedCategoryService 首页推荐分类业务逻辑。
type HomeFeaturedCategoryService struct {
	store    HomeFeaturedCategoryStore
	category CategoryLookup
}

// NewHomeFeaturedCategoryService 创建服务。
func NewHomeFeaturedCategoryService(store HomeFeaturedCategoryStore) *HomeFeaturedCategoryService {
	return &HomeFeaturedCategoryService{store: store}
}

// SetCategoryLookup 注入正式分类查询服务。
func (s *HomeFeaturedCategoryService) SetCategoryLookup(category CategoryLookup) {
	s.category = category
}

// ListAdmin 列出全部配置（后台）。
func (s *HomeFeaturedCategoryService) ListAdmin() ([]sitebuilderdomain.HomeFeaturedCategory, error) {
	return s.store.List(false)
}

// ListPublic 列出启用项（前台），按 sort_order 排序，最多 6 个。
func (s *HomeFeaturedCategoryService) ListPublic() ([]sitebuilderdomain.HomeFeaturedCategory, error) {
	items, err := s.store.List(true)
	if err != nil {
		return nil, err
	}
	if len(items) > MaxFeaturedCategories {
		items = items[:MaxFeaturedCategories]
	}
	return items, nil
}

// Get 按 ID 查询。
func (s *HomeFeaturedCategoryService) Get(id uint) (*sitebuilderdomain.HomeFeaturedCategory, error) {
	return s.store.GetByID(id)
}

// FeaturedCategoryInput 创建/更新推荐分类入参。
type FeaturedCategoryInput struct {
	CategoryID   uint
	Alias        string
	IconOverride string
	Enabled      *bool
	SortOrder    int
}

// Create 新建推荐分类。
func (s *HomeFeaturedCategoryService) Create(input FeaturedCategoryInput) (*sitebuilderdomain.HomeFeaturedCategory, error) {
	if input.CategoryID == 0 {
		return nil, errors.New("category_id is required")
	}
	if err := s.validateCategory(input.CategoryID); err != nil {
		return nil, err
	}
	count, err := s.store.Count()
	if err != nil {
		return nil, err
	}
	if count >= MaxFeaturedCategories {
		return nil, errors.New("featured categories are limited to 6")
	}
	item := &sitebuilderdomain.HomeFeaturedCategory{
		CategoryID:   input.CategoryID,
		Alias:        input.Alias,
		IconOverride: input.IconOverride,
		SortOrder:    input.SortOrder,
		Enabled:      true,
	}
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	}
	if err := s.store.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

// Update 更新。
func (s *HomeFeaturedCategoryService) Update(id uint, input FeaturedCategoryInput) (*sitebuilderdomain.HomeFeaturedCategory, error) {
	item, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("featured category not found")
	}
	if input.CategoryID != 0 && input.CategoryID != item.CategoryID {
		if err := s.validateCategory(input.CategoryID); err != nil {
			return nil, err
		}
		item.CategoryID = input.CategoryID
	}
	item.Alias = input.Alias
	item.IconOverride = input.IconOverride
	item.SortOrder = input.SortOrder
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	}
	if err := s.store.Update(item); err != nil {
		return nil, err
	}
	return item, nil
}

// Delete 删除。
func (s *HomeFeaturedCategoryService) Delete(id uint) error {
	return s.store.Delete(id)
}

// SetEnabled 启用/禁用。
func (s *HomeFeaturedCategoryService) SetEnabled(id uint, enabled bool) (*sitebuilderdomain.HomeFeaturedCategory, error) {
	item, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("featured category not found")
	}
	item.Enabled = enabled
	if err := s.store.Update(item); err != nil {
		return nil, err
	}
	return item, nil
}

// Reorder 批量排序。
func (s *HomeFeaturedCategoryService) Reorder(items []ReorderItem) error {
	return s.store.Reorder(items)
}

// validateCategory 校验分类存在且已启用。
func (s *HomeFeaturedCategoryService) validateCategory(categoryID uint) error {
	if s.category == nil {
		return errors.New("category lookup service is not configured")
	}
	cat, err := s.category.GetByID(strconv.FormatUint(uint64(categoryID), 10))
	if err != nil {
		return err
	}
	if cat == nil {
		return errors.New("category does not exist")
	}
	if !cat.IsActive {
		return errors.New("category is disabled")
	}
	return nil
}
