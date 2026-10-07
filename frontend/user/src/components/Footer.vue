<template>
  <footer class="hidden border-t border-border/50 bg-card/80 backdrop-blur-md lg:block">
    <div class="hcz-shell-container flex items-center justify-center gap-2 py-3">
      <router-link
        v-for="item in footerItems"
        :key="item.key"
        :to="item.path"
        class="flex min-w-[80px] flex-col items-center gap-1 rounded-xl px-4 py-2 text-xs text-muted-foreground transition-all hover:text-foreground"
        :class="{ 'bg-accent-soft text-accent font-semibold': isActive(item.key) }">
        <component :is="item.icon" :size="20" :stroke-width="isActive(item.key) ? 2.2 : 1.8" />
        <span>{{ t(item.label) }}</span>
      </router-link>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Home, ClipboardList, User } from 'lucide-vue-next'

const { t } = useI18n()
const route = useRoute()

const footerItems = [
  { key: 'home', path: '/', label: 'coreNav.home', icon: Home },
  { key: 'orders', path: '/me/orders', label: 'coreNav.orders', icon: ClipboardList },
  { key: 'me', path: '/me', label: 'coreNav.me', icon: User },
]

const isActive = (key: string) => {
  if (key === 'home') return route.name === 'home'
  if (key === 'orders') return route.name === 'personal-center-orders'
  if (key === 'me') return route.name === 'personal-center'
  return false
}
</script>
