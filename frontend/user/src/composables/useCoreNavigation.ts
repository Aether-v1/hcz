import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowLeftRight, Compass, House, ReceiptText, UserRound } from 'lucide-vue-next'

/** Desktop 顶部主导航与历史引用共用的五个核心入口。 */
export function useCoreNavigation() {
  const route = useRoute()
  const { t } = useI18n()

  const items = computed(() => [
    { key: 'home', path: '/', label: t('coreNav.home'), icon: House },
    { key: 'orders', path: '/me/orders', label: t('coreNav.orders'), icon: ReceiptText },
    { key: 'c2c', path: '/c2c', label: t('coreNav.c2c'), icon: ArrowLeftRight },
    { key: 'discover', path: '/discovery', label: t('coreNav.discover'), icon: Compass },
    { key: 'me', path: '/me', label: t('coreNav.me'), icon: UserRound },
  ])

  const isActive = (key: string) => {
    const path = route.path
    switch (key) {
      case 'home': return path === '/'
      case 'orders': return path === '/me/orders' || path.startsWith('/orders/') || path.startsWith('/recharge-orders/')
      case 'c2c': return path.startsWith('/c2c')
      case 'discover': return path === '/discovery'
      case 'me': return path === '/me' || path === '/notifications' || (path.startsWith('/me/') && path !== '/me/orders')
      default: return false
    }
  }

  return { items, isActive }
}
