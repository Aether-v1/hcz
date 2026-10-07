package gormstore

import (
	"errors"

	sitebuilderapp "github.com/Aether-v1/hcz/internal/modules/sitebuilder/application"
	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
	"gorm.io/gorm"
)

// HomeFeaturedCategoryStore 首页推荐分类数据访问。
type HomeFeaturedCategoryStore struct {
	db *gorm.DB
}

// NewHomeFeaturedCategoryStore 创建 store。
func NewHomeFeaturedCategoryStore(db *gorm.DB) *HomeFeaturedCategoryStore {
	return &HomeFeaturedCategoryStore{db: db}
}

func (s *HomeFeaturedCategoryStore) Create(item *sitebuilderdomain.HomeFeaturedCategory) error {
	return s.db.Create(item).Error
}

func (s *HomeFeaturedCategoryStore) Update(item *sitebuilderdomain.HomeFeaturedCategory) error {
	return s.db.Save(item).Error
}

func (s *HomeFeaturedCategoryStore) Delete(id uint) error {
	if id == 0 {
		return nil
	}
	return s.db.Delete(&sitebuilderdomain.HomeFeaturedCategory{}, id).Error
}

func (s *HomeFeaturedCategoryStore) GetByID(id uint) (*sitebuilderdomain.HomeFeaturedCategory, error) {
	if id == 0 {
		return nil, nil
	}
	var item sitebuilderdomain.HomeFeaturedCategory
	if err := s.db.Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *HomeFeaturedCategoryStore) List(enabledOnly bool) ([]sitebuilderdomain.HomeFeaturedCategory, error) {
	query := s.db.Model(&sitebuilderdomain.HomeFeaturedCategory{})
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	var rows []sitebuilderdomain.HomeFeaturedCategory
	if err := query.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *HomeFeaturedCategoryStore) Reorder(items []sitebuilderapp.ReorderItem) error {
	if len(items) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			if err := tx.Model(&sitebuilderdomain.HomeFeaturedCategory{}).
				Where("id = ?", items[i].ID).
				UpdateColumn("sort_order", items[i].SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Count 统计推荐分类总数。
func (s *HomeFeaturedCategoryStore) Count() (int64, error) {
	var count int64
	if err := s.db.Model(&sitebuilderdomain.HomeFeaturedCategory{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
