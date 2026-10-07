<template>
  <nav class="hcz-bottom-nav theme-safe-bottom" :aria-label="t('coreNav.ariaLabel')">
    <div class="hcz-bottom-nav__inner">
      <router-link
        v-for="item in items"
        :key="item.key"
        :to="item.path"
        class="hcz-bottom-nav__item"
        :class="{ 'is-active': isActive(item.key) }"
        :aria-current="isActive(item.key) ? 'page' : undefined"
      >
        <span class="hcz-bottom-nav__icon">
          <component :is="item.icon" :size="24" :stroke-width="isActive(item.key) ? 2.3 : 1.9" aria-hidden="true" />
          <span v-if="item.key === 'me' && notificationStore.unreadCount > 0" class="hcz-bottom-nav__dot" />
        </span>
      </router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { House, ReceiptText, UserRound } from 'lucide-vue-next'
import { useNotificationStore } from '../stores/notification'

/**
 * 移动端底部主导航：首页 / 订单 / 我的。
 * Discovery、C2C 业务路由保留，但从底部主导航移除（可从个人中心等入口进入）。
 */
const { t } = useI18n()
const route = useRoute()
const notificationStore = useNotificationStore()

const items = computed(() => [
  { key: 'home', path: '/', label: t('coreNav.home'), icon: House },
  { key: 'orders', path: '/me/orders', label: t('coreNav.orders'), icon: ReceiptText },
  { key: 'me', path: '/me', label: t('coreNav.me'), icon: UserRound },
])

const isActive = (key: string) => {
  const path = route.path
  switch (key) {
    case 'home': return path === '/'
    case 'orders': return path === '/me/orders' || path.startsWith('/orders/') || path.startsWith('/recharge-orders/')
    case 'me': return path === '/me' || path === '/notifications' || (path.startsWith('/me/') && path !== '/me/orders')
    default: return false
  }
}
</script>
