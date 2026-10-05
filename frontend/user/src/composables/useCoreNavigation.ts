import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Bell, Compass, House, ReceiptText, UserRound } from 'lucide-vue-next'

/** 前台统一的五个主入口；旧路由保持不变，服务发现暂由现有服务列表承载。 */
export function useCoreNavigation() {
  const route = useRoute()
  const { t } = useI18n()

  const items = computed(() => [
    { key: 'home', path: '/', label: t('coreNav.home'), icon: House },
    { key: 'orders', path: '/me/orders', label: t('coreNav.orders'), icon: ReceiptText },
    { key: 'discover', path: '/products', label: t('coreNav.discover'), icon: Compass },
    { key: 'messages', path: '/notifications', label: t('coreNav.messages'), icon: Bell },
    { key: 'me', path: '/me', label: t('coreNav.me'), icon: UserRound },
  ])

  const isActive = (key: string) => {
    const path = route.path
    switch (key) {
      case 'home': return path === '/'
      case 'orders': return path === '/me/orders' || path.startsWith('/orders/') || path.startsWith('/recharge-orders/')
      case 'discover': return path === '/products' || path.startsWith('/products/') || path.startsWith('/categories/')
      case 'messages': return path === '/notifications'
      case 'me': return path === '/me' || (path.startsWith('/me/') && path !== '/me/orders')
      default: return false
    }
  }

  return { items, isActive }
}
