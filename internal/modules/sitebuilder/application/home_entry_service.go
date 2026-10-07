package application

import (
	"errors"
	"strings"

	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
)

// HomeEntryStore 首页入口存储端口。
type HomeEntryStore interface {
	Create(entry *sitebuilderdomain.HomeEntry) error
	Update(entry *sitebuilderdomain.HomeEntry) error
	Delete(id uint) error
	GetByID(id uint) (*sitebuilderdomain.HomeEntry, error)
	List(enabledOnly bool) ([]sitebuilderdomain.HomeEntry, error)
	Reorder(items []ReorderItem) error
	Count() (int64, error)
}

// MaxHomeEntries 首页核心入口上限（固定 4 槽）。
const MaxHomeEntries = 4

// HomeEntryInput 创建/更新首页入口入参。
type HomeEntryInput struct {
	Key          string
	Title        string
	Subtitle     string
	Icon         string
	Image        string
	ActionType   string
	ActionTarget string
	Badge        string
	Recommended  *bool
	Enabled      *bool
	SortOrder    int
}

// HomeEntryService 首页入口业务逻辑。
type HomeEntryService struct {
	store    HomeEntryStore
	products interface {
		GetBySlug(slug string, onlyActive bool) (*productdomain.Product, error)
	}
}

// SetProductLookup connects published catalog products to manually configured entries.
func (s *HomeEntryService) SetProductLookup(products interface {
	GetBySlug(slug string, onlyActive bool) (*productdomain.Product, error)
}) {
	s.products = products
}

// NewHomeEntryService 创建 HomeEntryService。
func NewHomeEntryService(store HomeEntryStore) *HomeEntryService {
	return &HomeEntryService{store: store}
}

// ListAdmin 列出全部入口（后台）。
func (s *HomeEntryService) ListAdmin() ([]sitebuilderdomain.HomeEntry, error) {
	return s.store.List(false)
}

// ListPublic 列出启用入口（前台）。
func (s *HomeEntryService) ListPublic() ([]sitebuilderdomain.HomeEntry, error) {
	entries, err := s.store.List(true)
	if err != nil {
		return nil, err
	}
	visible := make([]sitebuilderdomain.HomeEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.ActionType == "product" {
			if s.products == nil {
				continue
			}
			product, lookupErr := s.products.GetBySlug(entry.ActionTarget, true)
			if lookupErr != nil || product == nil {
				continue
			}
		}
		visible = append(visible, entry)
	}
	return visible, nil
}

// Get 按 ID 查询。
func (s *HomeEntryService) Get(id uint) (*sitebuilderdomain.HomeEntry, error) {
	return s.store.GetByID(id)
}

// Create 新建入口。
func (s *HomeEntryService) Create(input HomeEntryInput) (*sitebuilderdomain.HomeEntry, error) {
	count, err := s.store.Count()
	if err != nil {
		return nil, err
	}
	if count >= MaxHomeEntries {
		return nil, errors.New("home entries are limited to 4 slots")
	}
	entry := &sitebuilderdomain.HomeEntry{
		Key:       strings.TrimSpace(input.Key),
		SortOrder: input.SortOrder,
	}
	if input.Recommended != nil {
		entry.Recommended = *input.Recommended
	}
	if input.Enabled != nil {
		entry.Enabled = *input.Enabled
	} else {
		entry.Enabled = true
	}
	if err := s.applyInput(entry, input); err != nil {
		return nil, err
	}
	if err := s.store.Create(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// Update 更新入口。
func (s *HomeEntryService) Update(id uint, input HomeEntryInput) (*sitebuilderdomain.HomeEntry, error) {
	entry, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.New("home entry not found")
	}
	// Key 唯一，不允许通过更新改写。
	if err := s.applyInput(entry, input); err != nil {
		return nil, err
	}
	entry.SortOrder = input.SortOrder
	if input.Recommended != nil {
		entry.Recommended = *input.Recommended
	}
	if input.Enabled != nil {
		entry.Enabled = *input.Enabled
	}
	if err := s.store.Update(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// Delete 删除入口。
func (s *HomeEntryService) Delete(id uint) error {
	return s.store.Delete(id)
}

// SetEnabled 启用/禁用。
func (s *HomeEntryService) SetEnabled(id uint, enabled bool) (*sitebuilderdomain.HomeEntry, error) {
	entry, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.New("home entry not found")
	}
	if enabled && entry.ActionType == "product" {
		if err := s.requirePublishedProduct(entry.ActionTarget); err != nil {
			return nil, err
		}
	}
	entry.Enabled = enabled
	if err := s.store.Update(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// Reorder 批量排序。
func (s *HomeEntryService) Reorder(items []ReorderItem) error {
	return s.store.Reorder(items)
}

// applyInput 校验并填充业务字段。
func (s *HomeEntryService) applyInput(entry *sitebuilderdomain.HomeEntry, input HomeEntryInput) error {
	entry.Title = strings.TrimSpace(input.Title)
	if entry.Title == "" {
		return errors.New("title is required")
	}
	entry.Subtitle = strings.TrimSpace(input.Subtitle)
	entry.Badge = strings.TrimSpace(input.Badge)
	entry.Image = strings.TrimSpace(input.Image)
	if entry.Image != "" && !strings.HasPrefix(entry.Image, "/") {
		if err := ValidateExternalURL(entry.Image); err != nil {
			return errors.New("image must be a site path or http/https URL")
		}
	}
	if strings.HasPrefix(entry.Image, "//") {
		return errors.New("image must not be a protocol-relative URL")
	}

	entry.Icon = strings.TrimSpace(input.Icon)
	if entry.Icon != "" && !IsAllowedHomeEntryIcon(entry.Icon) {
		return errors.New("icon not in whitelist")
	}

	entry.ActionType = strings.ToLower(strings.TrimSpace(input.ActionType))
	entry.ActionTarget = strings.TrimSpace(input.ActionTarget)
	switch entry.ActionType {
	case "internal":
		if !IsAllowedHomeEntryRoute(entry.ActionTarget) {
			return errors.New("action_target route not in whitelist")
		}
	case "external":
		if err := ValidateExternalURL(entry.ActionTarget); err != nil {
			return err
		}
	case "product":
		if err := s.requirePublishedProduct(entry.ActionTarget); err != nil {
			return err
		}
	default:
		return errors.New("action_type must be internal, external or product")
	}
	return nil
}

func (s *HomeEntryService) requirePublishedProduct(slug string) error {
	if s.products == nil || slug == "" {
		return errors.New("published product is required")
	}
	product, err := s.products.GetBySlug(slug, true)
	if err != nil {
		return err
	}
	if product == nil {
		return errors.New("product is not published")
	}
	return nil
}
