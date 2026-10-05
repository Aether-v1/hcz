import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Compass, House, ReceiptText, UserRound } from 'lucide-vue-next'

/** classic、vault 与移动底栏共用的四个主入口。 */
export function useCoreNavigation() {
  const route = useRoute()
  const { t } = useI18n()

  const items = computed(() => [
    { key: 'home', path: '/', label: t('coreNav.home'), icon: House },
    { key: 'orders', path: '/me/orders', label: t('coreNav.orders'), icon: ReceiptText },
    { key: 'discover', path: '/discovery', label: t('coreNav.discover'), icon: Compass },
    { key: 'me', path: '/me', label: t('coreNav.me'), icon: UserRound },
  ])

  const isActive = (key: string) => {
    const path = route.path
    switch (key) {
      case 'home': return path === '/'
      case 'orders': return path === '/me/orders' || path.startsWith('/orders/') || path.startsWith('/recharge-orders/')
      case 'discover': return path === '/discovery'
      case 'me': return path === '/me' || path === '/notifications' || path.startsWith('/c2c') || (path.startsWith('/me/') && path !== '/me/orders')
      default: return false
    }
  }

  return { items, isActive }
}
