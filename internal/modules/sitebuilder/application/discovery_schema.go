package application

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// 发现页区块类型白名单。
const (
	DiscoveryTypeBanner            = "banner"
	DiscoveryTypeCardGrid          = "card_grid"
	DiscoveryTypeBusinessRecommend = "business_recommend"
	DiscoveryTypeAnnouncement      = "announcement"
	DiscoveryTypeExternalLink      = "external_link"
	DiscoveryTypeCategoryEntry     = "category_entry"
)

var discoveryTypes = map[string]struct{}{
	DiscoveryTypeBanner:            {},
	DiscoveryTypeCardGrid:          {},
	DiscoveryTypeBusinessRecommend: {},
	DiscoveryTypeAnnouncement:      {},
	DiscoveryTypeExternalLink:      {},
	DiscoveryTypeCategoryEntry:     {},
}

// IsKnownDiscoveryType 判断区块类型是否受支持。
func IsKnownDiscoveryType(t string) bool {
	_, ok := discoveryTypes[t]
	return ok
}

// DiscoveryConfig 区块配置校验接口。
type DiscoveryConfig interface {
	Validate() error
}

// bannerConfig banner 区块配置。
type bannerConfig struct {
	Image        string `json:"image"`
	LinkType     string `json:"link_type"`
	LinkValue    string `json:"link_value"`
	OpenInNewTab bool   `json:"open_in_new_tab"`
}

func (c bannerConfig) Validate() error {
	if c.Image == "" {
		return errors.New("config.image is required")
	}
	return validateLink(c.LinkType, c.LinkValue)
}

// cardGridCard 卡片格子中的单卡。
type cardGridCard struct {
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle"`
	Image        string `json:"image"`
	ActionType   string `json:"action_type"`
	ActionTarget string `json:"action_target"`
}

func (c cardGridCard) Validate() error {
	if c.Title == "" {
		return errors.New("card.title is required")
	}
	return validateAction(c.ActionType, c.ActionTarget)
}

type cardGridConfig struct {
	Cards []cardGridCard `json:"cards"`
}

func (c cardGridConfig) Validate() error {
	if len(c.Cards) == 0 {
		return errors.New("config.cards must not be empty")
	}
	if len(c.Cards) > 12 {
		return errors.New("config.cards exceeds max 12")
	}
	for i := range c.Cards {
		if err := c.Cards[i].Validate(); err != nil {
			return fmt.Errorf("cards[%d]: %w", i, err)
		}
	}
	return nil
}

type businessRecommendConfig struct {
	Title      string `json:"title"`
	ProductIDs []uint `json:"product_ids"`
}

func (c businessRecommendConfig) Validate() error {
	if len(c.ProductIDs) == 0 {
		return errors.New("config.product_ids must not be empty")
	}
	if len(c.ProductIDs) > 20 {
		return errors.New("config.product_ids exceeds max 20")
	}
	for _, id := range c.ProductIDs {
		if id == 0 {
			return errors.New("config.product_ids contains invalid id")
		}
	}
	return nil
}

type announcementConfig struct {
	Text      string `json:"text"`
	LinkType  string `json:"link_type"`
	LinkValue string `json:"link_value"`
}

func (c announcementConfig) Validate() error {
	if c.Text == "" {
		return errors.New("config.text is required")
	}
	return validateLink(c.LinkType, c.LinkValue)
}

type externalLinkConfig struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
}

func (c externalLinkConfig) Validate() error {
	if c.Label == "" {
		return errors.New("config.label is required")
	}
	return ValidateExternalURL(c.URL)
}

type categoryEntryConfig struct {
	CategoryIDs []uint `json:"category_ids"`
}

func (c categoryEntryConfig) Validate() error {
	if len(c.CategoryIDs) == 0 {
		return errors.New("config.category_ids must not be empty")
	}
	if len(c.CategoryIDs) > 20 {
		return errors.New("config.category_ids exceeds max 20")
	}
	for _, id := range c.CategoryIDs {
		if id == 0 {
			return errors.New("config.category_ids contains invalid id")
		}
	}
	return nil
}

func validateLink(linkType, linkValue string) error {
	switch linkType {
	case "", "none":
		return nil
	case "internal":
		if linkValue == "" {
			return errors.New("link_value is required for internal link")
		}
		return nil
	case "external":
		return ValidateExternalURL(linkValue)
	default:
		return errors.New("link_type must be none/internal/external")
	}
}

func validateAction(actionType, actionTarget string) error {
	switch actionType {
	case "internal":
		if !IsAllowedHomeEntryRoute(actionTarget) {
			return errors.New("action_target route not in whitelist")
		}
		return nil
	case "external":
		return ValidateExternalURL(actionTarget)
	default:
		return errors.New("action_type must be internal or external")
	}
}

// ParseDiscoveryConfig 按区块类型把 JSON 配置反序列化并校验。
func ParseDiscoveryConfig(blockType string, raw jsonmap.JSON) (DiscoveryConfig, error) {
	if !IsKnownDiscoveryType(blockType) {
		return nil, fmt.Errorf("unknown discovery block type: %s", blockType)
	}
	var config DiscoveryConfig
	switch blockType {
	case DiscoveryTypeBanner:
		config = &bannerConfig{}
	case DiscoveryTypeCardGrid:
		config = &cardGridConfig{}
	case DiscoveryTypeBusinessRecommend:
		config = &businessRecommendConfig{}
	case DiscoveryTypeAnnouncement:
		config = &announcementConfig{}
	case DiscoveryTypeExternalLink:
		config = &externalLinkConfig{}
	case DiscoveryTypeCategoryEntry:
		config = &categoryEntryConfig{}
	default:
		return nil, fmt.Errorf("unknown discovery block type: %s", blockType)
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, config); err != nil {
		return nil, fmt.Errorf("invalid config payload: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}
