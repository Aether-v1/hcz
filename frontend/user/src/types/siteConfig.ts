/**
 * 站点装修配置类型（Phase 9 前端动态化）。
 *
 * 后端 /public/config 在原有 brand/nav_config/announcement/scripts 等字段基础上，
 * 新增 home_entries / banners / discovery_blocks / social_links / footer / primary_color。
 * 所有字段均为可选（后端尚未全量下发），组件侧通过 useSiteConfig 做 fallback，
 * 保证配置缺失时不白屏。
 */

/** 多语言文本：可能是 "zh-CN": "..." 映射，也可能是纯字符串 */
export type LocalizedText = string | Record<string, string> | null | undefined

export interface BrandConfig {
  site_name?: string
  site_logo?: string
  site_icon?: string
  site_description?: LocalizedText
  /** 品牌主色，hex，默认 #4F46E5 */
  primary_color?: string
  copyright?: string
}

export interface NavCustomItem {
  id?: number
  title?: LocalizedText
  name?: LocalizedText
  link_type?: 'internal' | 'external' | string
  url?: string
  target?: string
  icon?: string
  enabled?: boolean
  sort_order?: number
}

export interface NavConfig {
  builtin?: {
    blog?: boolean
    notice?: boolean
    about?: boolean
    [key: string]: boolean | undefined
  }
  custom_items?: NavCustomItem[]
}

export interface FooterLinkItem {
  name?: LocalizedText
  url?: string
}

export interface FooterConfig {
  copyright?: string
  footer_links?: FooterLinkItem[]
}

/** 社交链接：key 为渠道类型，value 为 url（email 为 mailto:） */
export interface SocialLinks {
  telegram?: string
  whatsapp?: string
  x?: string
  twitter?: string
  discord?: string
  email?: string
  [key: string]: string | undefined
}

export type HomeEntryActionType = 'internal' | 'external'

export interface HomeEntry {
  id: number
  key: string
  title: string
  subtitle: string
  /** icon 白名单 key：recharge/c2c/wallet/withdrawal/invitation/support/orders/gift/ticket/discovery */
  icon: string
  action_type: HomeEntryActionType
  /** internal 时为路由路径，external 时为完整 URL */
  action_target: string
  badge: string
  recommended: boolean
  enabled: boolean
  sort_order: number
}

export interface BannerItem {
  id?: number
  image?: string
  mobile_image?: string
  title?: LocalizedText
  subtitle?: LocalizedText
  link_type?: 'none' | 'internal' | 'external' | string
  link_value?: string
  open_in_new_tab?: boolean
  sort_order?: number
  enabled?: boolean
}

export interface AnnouncementConfig {
  type?: string
  title?: LocalizedText
  content?: LocalizedText
  version?: string
  link?: string
  [key: string]: unknown
}

export type DiscoveryBlockType =
  | 'banner'
  | 'card_grid'
  | 'business_recommend'
  | 'announcement'
  | 'external_link'
  | 'category_entry'

export interface DiscoveryBlock {
  id: number
  type: string
  title?: string
  config?: Record<string, any>
  enabled: boolean
  sort_order: number
}

export interface SeoConfig {
  title?: LocalizedText
  description?: LocalizedText
  keywords?: LocalizedText
}

export interface SiteConfig {
  brand: BrandConfig
  template: 'classic' | 'vault'
  navigation: NavConfig
  footer: FooterConfig
  social_links: SocialLinks
  home_entries: HomeEntry[]
  banners: BannerItem[]
  announcement: AnnouncementConfig | null
  discovery_blocks: DiscoveryBlock[]
  seo: SeoConfig
}
