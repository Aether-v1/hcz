import { computed, onMounted, ref, watch, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Banknote, Home, Gift, ShieldCheck, UserCircle, Megaphone, Key, Share2 } from 'lucide-vue-next'
import type { PageAlert } from '../utils/alerts'
import { useAppStore } from '../stores/app'
import { useUserProfileStore } from '../stores/userProfile'
import { useUserAuthStore } from '../stores/userAuth'
import type { PublicMemberLevel } from '../api'

export type PersonalSection = 'overview' | 'profile' | 'security' | 'giftCard' | 'affiliate' | 'invitation' | 'reseller' | 'api'

export interface PersonalSectionItem {
  key: PersonalSection
  label: string
  icon: Component
}

/**
 * 个人中心壳逻辑（classic + vault 壳共用）：账户头部、侧栏导航、概览数据、初始化。
 */
export function usePersonalCenter(sectionGetter: () => PersonalSection) {
  const router = useRouter()
  const { t, locale } = useI18n()
  const appStore = useAppStore()
  const userProfileStore = useUserProfileStore()
  const userAuthStore = useUserAuthStore()

  const sectionItems: PersonalSectionItem[] = [
    { key: 'overview', label: 'personalCenter.tabs.overview', icon: Home },
    { key: 'profile', label: 'personalCenter.tabs.profile', icon: UserCircle },
    { key: 'security', label: 'personalCenter.tabs.security', icon: ShieldCheck },
    { key: 'giftCard', label: 'personalCenter.tabs.giftCard', icon: Gift },
    { key: 'affiliate', label: 'personalCenter.tabs.affiliate', icon: Megaphone },
    { key: 'invitation', label: 'personalCenter.tabs.invitation', icon: Share2 },
    { key: 'reseller', label: 'personalCenter.tabs.reseller', icon: Banknote },
    { key: 'api', label: 'personalCenter.tabs.api', icon: Key },
  ]

  const sectionRouteMap: Record<PersonalSection, string> = {
    overview: '/me',
    profile: '/me/profile',
    security: '/me/security',
    affiliate: '/me/affiliate',
    invitation: '/me/invitation',
    reseller: '/me/reseller',
    giftCard: '/me/gift-cards',
    api: '/me/api',
  }

  const canAccessResellerConsole = computed(() => appStore.canAccessResellerConsole)
  const visibleSectionItems = computed(() => {
    return sectionItems.filter((item) => item.key !== 'reseller' || canAccessResellerConsole.value)
  })
  const currentSection = computed<PersonalSection>(() => {
    if (sectionGetter() === 'reseller' && !canAccessResellerConsole.value) {
      return 'overview'
    }
    return sectionGetter()
  })
  const globalAlert = ref<PageAlert | null>(null)

  const displayInitial = computed(() => {
    const name = userProfileStore.displayName || ''
    const normalized = name.trim()
    if (!normalized) return 'U'
    return normalized.slice(0, 1).toUpperCase()
  })

  const switchSection = (section: PersonalSection) => {
    router.push(sectionRouteMap[section])
  }

  const emailVerifiedLabel = computed(() => {
    if (userProfileStore.profile?.email_verified_at) {
      return t('personalCenter.overview.emailVerified')
    }
    return t('personalCenter.overview.emailUnverified')
  })

  const emailVerifiedVariant = computed<'success' | 'warning'>(() => {
    return userProfileStore.profile?.email_verified_at ? 'success' : 'warning'
  })

  const discountText = computed(() => {
    const lvl = userProfileStore.currentLevel
    if (lvl && lvl.discount_rate < 100) {
      return t('personalCenter.memberLevel.discountOff', { n: lvl.discount_rate / 10 })
    }
    return t('personalCenter.memberLevel.noDiscount')
  })

  const isImagePath = (icon: string | undefined | null) => icon?.startsWith('/uploads/') || icon?.startsWith('http')

  const levelName = (level: PublicMemberLevel | null | undefined) => {
    if (!level) return t('personalCenter.memberLevel.defaultLevel')
    const loc = locale.value as string
    return level.name[loc] || level.name['zh-CN'] || level.name['en'] || level.slug || t('personalCenter.memberLevel.defaultLevel')
  }

  const initialize = async () => {
    if (!userAuthStore.isAuthenticated || currentSection.value === 'overview') return
    globalAlert.value = null
    const [profileOk] = await Promise.all([
      userProfileStore.loadProfile(),
      userProfileStore.loadMemberLevels(),
    ])
    if (!profileOk) {
      globalAlert.value = {
        level: 'error',
        message: userProfileStore.profileError || t('personalCenter.common.loadFailed'),
      }
    }
  }

  onMounted(() => {
    initialize()
  })
  watch(currentSection, () => { void initialize() })

  return {
    userProfileStore,
    canAccessResellerConsole,
    visibleSectionItems,
    currentSection,
    globalAlert,
    displayInitial,
    switchSection,
    emailVerifiedLabel,
    emailVerifiedVariant,
    discountText,
    isImagePath,
    levelName,
  }
}

