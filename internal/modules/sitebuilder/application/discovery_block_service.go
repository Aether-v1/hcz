package application

import (
	"errors"
	"strings"

	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// ReorderItem 批量排序单条。
type ReorderItem struct {
	ID        uint
	SortOrder int
}

// DiscoveryBlockStore 发现页区块存储端口。
type DiscoveryBlockStore interface {
	Create(block *sitebuilderdomain.DiscoveryBlock) error
	Update(block *sitebuilderdomain.DiscoveryBlock) error
	Delete(id uint) error
	GetByID(id uint) (*sitebuilderdomain.DiscoveryBlock, error)
	List(enabledOnly bool) ([]sitebuilderdomain.DiscoveryBlock, error)
	Reorder(items []ReorderItem) error
}

// DiscoveryBlockInput 创建/更新区块入参。
type DiscoveryBlockInput struct {
	Type      string
	Title     string
	Config    jsonmap.JSON
	Enabled   *bool
	SortOrder int
}

// DiscoveryBlockService 发现页区块业务逻辑。
type DiscoveryBlockService struct {
	store DiscoveryBlockStore
}

// NewDiscoveryBlockService 创建 DiscoveryBlockService。
func NewDiscoveryBlockService(store DiscoveryBlockStore) *DiscoveryBlockService {
	return &DiscoveryBlockService{store: store}
}

// ListAdmin 列出全部区块（后台）。
func (s *DiscoveryBlockService) ListAdmin() ([]sitebuilderdomain.DiscoveryBlock, error) {
	return s.store.List(false)
}

// ListPublic 列出启用区块（前台）。损坏的 config 块在聚合层过滤，这里只返回启用项。
func (s *DiscoveryBlockService) ListPublic() ([]sitebuilderdomain.DiscoveryBlock, error) {
	return s.store.List(true)
}

// Get 按 ID 查询。
func (s *DiscoveryBlockService) Get(id uint) (*sitebuilderdomain.DiscoveryBlock, error) {
	return s.store.GetByID(id)
}

// Create 新建区块。
func (s *DiscoveryBlockService) Create(input DiscoveryBlockInput) (*sitebuilderdomain.DiscoveryBlock, error) {
	if err := validateBlockType(input.Type); err != nil {
		return nil, err
	}
	if _, err := ParseDiscoveryConfig(input.Type, input.Config); err != nil {
		return nil, err
	}
	block := &sitebuilderdomain.DiscoveryBlock{
		Type:      input.Type,
		Title:     strings.TrimSpace(input.Title),
		Config:    input.Config,
		SortOrder: input.SortOrder,
		Enabled:   true,
	}
	if input.Enabled != nil {
		block.Enabled = *input.Enabled
	}
	if err := s.store.Create(block); err != nil {
		return nil, err
	}
	return block, nil
}

// Update 更新区块。
func (s *DiscoveryBlockService) Update(id uint, input DiscoveryBlockInput) (*sitebuilderdomain.DiscoveryBlock, error) {
	block, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, errors.New("discovery block not found")
	}
	blockType := strings.TrimSpace(input.Type)
	if blockType == "" {
		blockType = block.Type
	}
	if err := validateBlockType(blockType); err != nil {
		return nil, err
	}
	config := input.Config
	if config == nil {
		config = jsonmap.JSON(block.Config)
	}
	if _, err := ParseDiscoveryConfig(blockType, config); err != nil {
		return nil, err
	}
	block.Type = blockType
	block.Title = strings.TrimSpace(input.Title)
	block.Config = config
	block.SortOrder = input.SortOrder
	if input.Enabled != nil {
		block.Enabled = *input.Enabled
	}
	if err := s.store.Update(block); err != nil {
		return nil, err
	}
	return block, nil
}

// Delete 删除区块。
func (s *DiscoveryBlockService) Delete(id uint) error {
	return s.store.Delete(id)
}

// SetEnabled 启用/禁用。
func (s *DiscoveryBlockService) SetEnabled(id uint, enabled bool) (*sitebuilderdomain.DiscoveryBlock, error) {
	block, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, errors.New("discovery block not found")
	}
	block.Enabled = enabled
	if err := s.store.Update(block); err != nil {
		return nil, err
	}
	return block, nil
}

// Reorder 批量排序。
func (s *DiscoveryBlockService) Reorder(items []ReorderItem) error {
	return s.store.Reorder(items)
}

func validateBlockType(blockType string) error {
	if !IsKnownDiscoveryType(strings.TrimSpace(blockType)) {
		return errors.New("invalid discovery block type")
	}
	return nil
}
