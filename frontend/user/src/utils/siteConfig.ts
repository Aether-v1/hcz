/**
 * 站点装修配置的纯函数解析/兜底逻辑（不依赖 Vue / Pinia，便于单测）。
 * useSiteConfig composable 在其上包一层响应式。
 */

export const DEFAULT_BRAND_PRIMARY = '#4F46E5'

export const isHexColor = (value: unknown): value is string =>
  typeof value === 'string' && /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(value.trim())

/** 6 位 hex 追加 alpha 后缀；非法输入返回空串 */
export const withAlpha = (hex: string, alpha: string): string => {
  const v = (hex || '').trim()
  if (!isHexColor(v)) return ''
  if (v.length === 4) {
    const expanded = `#${v[1]}${v[1]}${v[2]}${v[2]}${v[3]}${v[3]}`
    return expanded + alpha
  }
  return v + alpha
}

export const asArray = <T = unknown>(value: unknown): T[] => (Array.isArray(value) ? (value as T[]) : [])

export const toBool = (value: unknown, fallback = false): boolean => {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value !== 0
  if (typeof value === 'string') {
    const v = value.trim().toLowerCase()
    return v === '1' || v === 'true' || v === 'yes' || v === 'on'
  }
  return fallback
}

/** 纯字符串 / 多语言映射取值 */
export const pickText = (value: unknown, locale: string): string => {
  if (value == null) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'object') {
    const record = value as Record<string, unknown>
    const candidates = [record[locale], record['zh-CN'], record['en-US'], Object.values(record)[0]]
    for (const c of candidates) {
      if (typeof c === 'string' && c.trim()) return c
    }
    return ''
  }
  return ''
}

export const isHttpUrl = (value: string): boolean => /^https?:\/\//i.test(value)

export interface ResolvedHomeEntry {
  id: number | string
  key: string
  title: string
  subtitle: string
  icon: string
  external: boolean
  href: string
  badge: string
  recommended: boolean
}

/** 默认业务入口（home_entries 为空时兜底） */
export const DEFAULT_HOME_ENTRIES: Array<{ key: string; title: string; subtitle: string; icon: string }> = [
  { key: 'recharge', title: '生活充值', subtitle: '话费 / 流量 / 游戏', icon: 'recharge' },
  { key: 'c2c', title: 'C2C', subtitle: '点对点交易', icon: 'c2c' },
  { key: 'wallet', title: 'Wallet', subtitle: '资产与余额', icon: 'wallet' },
  { key: 'invitation', title: '邀请中心', subtitle: '邀请有奖', icon: 'invitation' },
]

export interface RouteMap {
  [key: string]: string
}

/**
 * 归一化业务入口：过滤 enabled、按 sort_order 排序、解析内外链；
 * 空数组时返回默认 4 个入口。
 */
export const normalizeHomeEntries = (
  raw: unknown,
  locale: string,
  routeMap: RouteMap,
): ResolvedHomeEntry[] => {
  const normalized = asArray<Record<string, unknown>>(raw)
    .filter((item) => toBool(item?.enabled, true))
    .slice()
    .sort((a, b) => Number(a?.sort_order || 0) - Number(b?.sort_order || 0))
    .map((item, index) => {
      const key = String(item?.key || `entry-${index}`)
      const actionType = String(item?.action_type || 'internal') === 'external' ? 'external' : 'internal'
      let actionTarget = String(item?.action_target || item?.url || '').trim()
      if (actionType === 'internal' && (!actionTarget || !actionTarget.startsWith('/'))) {
        actionTarget = routeMap[key] || ''
      }
      return {
        id: Number(item?.id) || `entry-${index}`,
        key,
        title: pickText(item?.title, locale) || key,
        subtitle: pickText(item?.subtitle, locale),
        icon: String(item?.icon || key),
        external: actionType === 'external',
        href: actionTarget,
        badge: pickText(item?.badge, locale),
        recommended: toBool(item?.recommended, false),
      }
    })
    .filter((item) => (item.external ? isHttpUrl(item.href) : item.href.startsWith('/')))

  if (normalized.length > 0) return normalized

  return DEFAULT_HOME_ENTRIES.map((d, index) => ({
    id: `fallback-${index}`,
    key: d.key,
    title: d.title,
    subtitle: d.subtitle,
    icon: d.icon,
    external: false,
    href: routeMap[d.key] || '/',
    badge: '',
    recommended: index === 0,
  }))
}

/** 归一化 discovery blocks：过滤 enabled、按 sort_order 排序 */
export const normalizeDiscoveryBlocks = (
  raw: unknown,
  locale: string,
): Array<{ id: number | string; type: string; title: string; config: Record<string, any> }> => {
  return asArray<Record<string, unknown>>(raw)
    .filter((item) => toBool(item?.enabled, true))
    .slice()
    .sort((a, b) => Number(a?.sort_order || 0) - Number(b?.sort_order || 0))
    .map((item, index) => ({
      id: Number(item?.id) || `block-${index}`,
      type: String(item?.type || ''),
      title: pickText(item?.title, locale),
      config: (item?.config && typeof item.config === 'object' ? item.config : {}) as Record<string, any>,
    }))
}
