package application

import "github.com/Aether-v1/hcz/internal/shared/jsonmap"

// SiteSettingsStore sitebuilder 读写站点配置的端口。
// 由 settings application.Service 适配实现，避免 sitebuilder 反向依赖 settings 模块。
type SiteSettingsStore interface {
	GetByKey(key string) (jsonmap.JSON, error)
	Update(key string, value map[string]interface{}) (jsonmap.JSON, error)
}
