import { api } from './client'

/**
 * Site Builder（站点装修）API 封装
 * 后端 Phase 9 提供的 /admin/site/* 接口。
 * 响应统一走 client 的 ApiResponse 信封，业务数据在 res.data.data。
 */

export type LocalizedText = Record<string, string>

// ─────────────────────────── 常量白名单 ───────────────────────────

/** 首页入口图标白名单 */
export const HOME_ENTRY_ICONS = [
  'recharge',
  'c2c',
  'wallet',
  'withdrawal',
  'invitation',
  'support',
  'orders',
  'gift',
  'ticket',
  'discovery',
] as const

/** 首页入口 internal 跳转路由白名单 */
export const HOME_ENTRY_ROUTES = [
  'recharge',
  'c2c',
  'wallet',
  'withdrawal',
  'invitation',
  'support',
  'orders',
] as const

/** 社交链接预置 key */
export const SOCIAL_LINK_KEYS = [
  'telegram',
  'whatsapp',
  'x',
  'discord',
  'email',
  'custom',
] as const

/** 发现页区块类型 */
export const DISCOVERY_BLOCK_TYPES = [
  'banner',
  'card_grid',
  'business_recommend',
  'announcement',
  'external_link',
  'category_entry',
] as const

// ─────────────────────────── 类型定义 ───────────────────────────

export interface SocialLink {
  key: string
  label: string
  value: string
  enabled: boolean
}

export interface BrandConfig {
  site_name: string
  site_logo: string
  site_icon: string
  primary_color: string
  copyright: string
  site_description: LocalizedText
  social_links: SocialLink[]
}

export type HomeEntryActionType = 'internal' | 'external' | 'product'

export interface HomeEntry {
  id?: number
  key: string
  title: string
  subtitle: string
  icon: string
  image: string
  action_type: HomeEntryActionType
  action_target: string
  badge: string
  recommended: boolean
  enabled: boolean
  sort_order: number
}

export type DiscoveryBlockType = (typeof DISCOVERY_BLOCK_TYPES)[number]

export interface DiscoveryBlock {
  id?: number
  type: DiscoveryBlockType
  title: string
  enabled: boolean
  sort_order: number
  config: Record<string, any>
}

export type StorefrontTemplate = 'classic' | 'vault'

export interface TemplateConfig {
  storefront_template: StorefrontTemplate
}

// ─────────────────────────── 品牌设置 ───────────────────────────

export const getBrand = () => api.get('/admin/site/brand')
export const updateBrand = (data: BrandConfig) => api.put('/admin/site/brand', data)

// ─────────────────────────── 首页入口 ───────────────────────────

export const getHomeEntries = () => api.get('/admin/site/home-entries')
export const createHomeEntry = (data: HomeEntry) => api.post('/admin/site/home-entries', data)
export const updateHomeEntry = (id: number, data: HomeEntry) => api.put(`/admin/site/home-entries/${id}`, data)
export const deleteHomeEntry = (id: number) => api.delete(`/admin/site/home-entries/${id}`)
export const toggleHomeEntry = (id: number) => api.patch(`/admin/site/home-entries/${id}/toggle`)
export const reorderHomeEntries = (ids: number[]) =>
  api.post('/admin/site/home-entries/reorder', { ids })

// ─────────────────────────── 发现页区块 ───────────────────────────

export const getDiscoveryBlocks = () => api.get('/admin/site/discovery-blocks')
export const createDiscoveryBlock = (data: DiscoveryBlock) => api.post('/admin/site/discovery-blocks', data)
export const updateDiscoveryBlock = (id: number, data: DiscoveryBlock) =>
  api.put(`/admin/site/discovery-blocks/${id}`, data)
export const deleteDiscoveryBlock = (id: number) => api.delete(`/admin/site/discovery-blocks/${id}`)
export const toggleDiscoveryBlock = (id: number) => api.patch(`/admin/site/discovery-blocks/${id}/toggle`)
export const reorderDiscoveryBlocks = (ids: number[]) =>
  api.post('/admin/site/discovery-blocks/reorder', { ids })

// ─────────────────────────── 模板 ───────────────────────────

export const getTemplate = () => api.get('/admin/site/template')
export const updateTemplate = (data: TemplateConfig) => api.put('/admin/site/template', data)

// ─────────────────────────── 校验工具 ───────────────────────────

/** 仅允许 http/https 外链 */
export const isHttpUrl = (value: string): boolean => {
  if (!value) return false
  return /^https?:\/\/.+/i.test(value.trim())
}

/** HEX 颜色校验 */
export const isHexColor = (value: string): boolean =>
  /^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$/.test(String(value || '').trim())

/** email 校验 */
export const isEmail = (value: string): boolean =>
  /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(String(value || '').trim())
