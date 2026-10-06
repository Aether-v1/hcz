import { computed } from 'vue'
import { useAppStore } from '../stores/app'
import { useI18n } from 'vue-i18n'
import { getImageUrl } from '../utils/image'
import { HOME_ENTRY_ROUTE_MAP } from '../components/home/homeEntryIcons'
import {
  DEFAULT_BRAND_PRIMARY,
  asArray,
  isHexColor,
  normalizeDiscoveryBlocks,
  normalizeHomeEntries,
  pickText,
  toBool,
  withAlpha,
  type ResolvedHomeEntry,
} from '../utils/siteConfig'
import type {
  AnnouncementConfig,
  BannerItem,
  DiscoveryBlock,
  SocialLinks,
} from '../types/siteConfig'

export { DEFAULT_BRAND_PRIMARY }

/**
 * 把 config.brand.primary_color 注入到 :root 的 CSS variable。
 * 在 config 加载完成后调用一次即可（stores/app.ts）。
 */
export const applyBrandColor = (raw: unknown): void => {
  if (typeof document === 'undefined') return
  const primary = isHexColor(raw) ? raw.trim() : DEFAULT_BRAND_PRIMARY
  const light = withAlpha(primary, '1A')
  document.documentElement.style.setProperty('--brand-primary', primary)
  document.documentElement.style.setProperty(
    '--brand-primary-light',
    light || 'rgba(79, 70, 229, 0.1)',
  )
}

export function useSiteConfig() {
  const appStore = useAppStore()
  const { locale } = useI18n()

  const config = computed(() => appStore.config || {})

  const siteName = computed(() => {
    const v = String(config.value?.brand?.site_name || '').trim()
    return v || 'HCZ'
  })

  const siteLogo = computed(() => {
    const raw = String(config.value?.brand?.site_logo || '').trim()
    return raw ? getImageUrl(raw) : '/hcz1_logo.png'
  })

  const primaryColor = computed(() => {
    const raw = String(config.value?.brand?.primary_color || '').trim()
    return isHexColor(raw) ? raw : DEFAULT_BRAND_PRIMARY
  })

  /** 导航配置：优先 config.navigation，兼容既有 config.nav_config */
  const navigation = computed(() => config.value?.navigation || config.value?.nav_config || {})

  const footer = computed(() => config.value?.footer || {})

  const copyright = computed(() =>
    String(footer.value?.copyright || config.value?.brand?.copyright || '').trim(),
  )

  const footerLinks = computed(() => {
    const raw = asArray<Record<string, unknown>>(
      footer.value?.footer_links ?? config.value?.footer_links,
    )
    return raw
      .map((item) => ({
        name: pickText(item?.name, locale.value),
        url: String(item?.url || '').trim(),
      }))
      .filter((item) => item.name && /^(https?:\/\/|\/(?!\/)|mailto:|tel:)/i.test(item.url))
  })

  const socialLinks = computed<SocialLinks>(() => {
    const raw = (config.value?.social_links || config.value?.social || {}) as Record<string, unknown>
    const out: SocialLinks = {}
    for (const [k, v] of Object.entries(raw)) {
      if (typeof v === 'string' && v.trim()) out[k] = v.trim()
    }
    if (!out.telegram && config.value?.contact?.telegram) out.telegram = String(config.value.contact.telegram)
    if (!out.whatsapp && config.value?.contact?.whatsapp) out.whatsapp = String(config.value.contact.whatsapp)
    return out
  })

  /** 业务入口：过滤 enabled、按 sort_order 排序；为空时给默认 4 个 */
  const homeEntries = computed<ResolvedHomeEntry[]>(() =>
    normalizeHomeEntries(config.value?.home_entries, locale.value, HOME_ENTRY_ROUTE_MAP),
  )

  /** 装修 Banner：过滤 enabled、按 sort_order 排序；为空则不展示 */
  const banners = computed<BannerItem[]>(() => {
    const raw = asArray<Record<string, unknown>>(config.value?.banners)
    return raw
      .filter((item) => toBool(item?.enabled, true))
      .slice()
      .sort((a, b) => Number(a?.sort_order || 0) - Number(b?.sort_order || 0))
  })

  /** 公告条：config.announcement（与既有弹窗公告同源） */
  const announcement = computed<AnnouncementConfig | null>(() => {
    const raw = config.value?.announcement as AnnouncementConfig | null | undefined
    if (!raw || typeof raw !== 'object') return null
    const title = pickText(raw.title, locale.value)
    if (!title) return null
    return { ...raw, title }
  })

  /** 发现页 blocks：过滤 enabled、按 sort_order 排序 */
  const discoveryBlocks = computed<DiscoveryBlock[]>(() =>
    normalizeDiscoveryBlocks(config.value?.discovery_blocks, locale.value).map((b) => ({
      ...b,
      enabled: true,
      sort_order: 0,
    })) as DiscoveryBlock[],
  )

  return {
    config,
    siteName,
    siteLogo,
    primaryColor,
    navigation,
    footer,
    copyright,
    footerLinks,
    socialLinks,
    homeEntries,
    banners,
    announcement,
    discoveryBlocks,
  }
}
