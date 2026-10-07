package publicconfigwiring

import (
	"context"

	"github.com/Aether-v1/hcz/internal/constants"
	categorydomain "github.com/Aether-v1/hcz/internal/modules/catalog/category/domain"
	contentapp "github.com/Aether-v1/hcz/internal/modules/content/application"
	sitebuilderapp "github.com/Aether-v1/hcz/internal/modules/sitebuilder/application"
	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
)

// publicConfigSiteBuilderAdapter 把 sitebuilder / content banner 服务适配为公开配置端口。
// 所有方法对 DB 错误做防御：出错时返回空切片，不抛出，保证公开配置恒为 200。
type publicConfigSiteBuilderAdapter struct {
	homeEntries        *sitebuilderapp.HomeEntryService
	discovery          *sitebuilderapp.DiscoveryBlockService
	banners            *contentapp.BannerService
	featuredCategories *sitebuilderapp.HomeFeaturedCategoryService
	categories         interface {
		ListActive() ([]categorydomain.Category, error)
	}
}

func (a publicConfigSiteBuilderAdapter) PublicHomeEntries() []map[string]interface{} {
	if a.homeEntries == nil {
		return nil
	}
	entries, err := a.homeEntries.ListPublic()
	if err != nil {
		return nil
	}
	if len(entries) == 0 {
		return make([]map[string]interface{}, 0)
	}
	result := make([]map[string]interface{}, 0, len(entries))
	for i := range entries {
		e := entries[i]
		result = append(result, map[string]interface{}{
			"key":           e.Key,
			"title":         e.Title,
			"subtitle":      e.Subtitle,
			"icon":          e.Icon,
			"image":         e.Image,
			"action_type":   e.ActionType,
			"action_target": e.ActionTarget,
			"badge":         e.Badge,
			"recommended":   e.Recommended,
			"sort_order":    e.SortOrder,
		})
	}
	return result
}

func (a publicConfigSiteBuilderAdapter) PublicDiscoveryBlocks() []map[string]interface{} {
	if a.discovery == nil {
		return nil
	}
	blocks, err := a.discovery.ListPublic()
	if err != nil || len(blocks) == 0 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(blocks))
	for i := range blocks {
		b := blocks[i]
		// 单个 block 的 config 损坏时跳过该块，不影响其它块。
		if _, validateErr := sitebuilderapp.ParseDiscoveryConfig(b.Type, b.Config); validateErr != nil {
			continue
		}
		result = append(result, discoveryBlockPublicMap(b))
	}
	return result
}

func discoveryBlockPublicMap(b sitebuilderdomain.DiscoveryBlock) map[string]interface{} {
	return map[string]interface{}{
		"id":         b.ID,
		"type":       b.Type,
		"title":      b.Title,
		"config":     map[string]interface{}(b.Config),
		"sort_order": b.SortOrder,
	}
}

func (a publicConfigSiteBuilderAdapter) PublicBanners() []map[string]interface{} {
	if a.banners == nil {
		return nil
	}
	banners, err := a.banners.ListPublic(context.Background(), contentapp.PublicBannerQuery{
		Position: constants.BannerPositionHomeHero,
		Limit:    20,
	})
	if err != nil || len(banners) == 0 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(banners))
	for i := range banners {
		b := banners[i]
		result = append(result, map[string]interface{}{
			"id":              b.ID,
			"title":           map[string]interface{}(b.TitleJSON),
			"subtitle":        map[string]interface{}(b.SubtitleJSON),
			"image":           b.Image,
			"mobile_image":    b.MobileImage,
			"link_type":       b.LinkType,
			"link_value":      b.LinkValue,
			"open_in_new_tab": b.OpenInNewTab,
			"sort_order":      b.SortOrder,
		})
	}
	return result
}

// PublicFeaturedCategories 返回首页热门推荐分类（已 join 完整分类信息）。
// DB 失败时返回空切片，不抛出。
func (a publicConfigSiteBuilderAdapter) PublicFeaturedCategories() []map[string]interface{} {
	if a.featuredCategories == nil || a.categories == nil {
		return make([]map[string]interface{}, 0)
	}
	items, err := a.featuredCategories.ListPublic()
	if err != nil || len(items) == 0 {
		return make([]map[string]interface{}, 0)
	}
	activeCats, err := a.categories.ListActive()
	if err != nil {
		return make([]map[string]interface{}, 0)
	}
	catByID := make(map[uint]categorydomain.Category, len(activeCats))
	for _, c := range activeCats {
		catByID[c.ID] = c
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		cat, ok := catByID[item.CategoryID]
		if !ok {
			continue
		}
		name := map[string]interface{}(cat.NameJSON)
		if item.Alias != "" {
			name = map[string]interface{}{"override": item.Alias}
		}
		icon := cat.Icon
		if item.IconOverride != "" {
			icon = item.IconOverride
		}
		result = append(result, map[string]interface{}{
			"id":         cat.ID,
			"slug":       cat.Slug,
			"name":       name,
			"icon":       icon,
			"sort_order": item.SortOrder,
		})
	}
	return result
}
