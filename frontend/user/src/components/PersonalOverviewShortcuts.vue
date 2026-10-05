<template>
  <nav class="grid grid-cols-2 gap-3 sm:grid-cols-4" :aria-label="t('personalCenter.tabs.overview')">
    <RouterLink v-for="item in shortcuts" :key="item.to" :to="item.to" class="flex min-w-0 flex-col items-center gap-2 rounded-2xl border bg-card p-4 text-center text-foreground shadow-sm transition-colors hover:border-primary/40 hover:text-primary">
      <span class="grid h-11 w-11 place-items-center rounded-xl bg-primary/10 text-primary"><component :is="item.icon" :size="22" aria-hidden="true" /></span>
      <span class="text-sm font-semibold">{{ t(item.label) }}</span>
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { LifeBuoy, ReceiptText, Share2, Wallet } from 'lucide-vue-next'

const { t } = useI18n()
const shortcuts = [
  { to: '/me/orders', label: 'personalCenter.tabs.orders', icon: ReceiptText },
  { to: '/me/wallet', label: 'personalCenter.tabs.wallet', icon: Wallet },
  { to: '/me/invitation', label: 'personalCenter.tabs.invitation', icon: Share2 },
  { to: '/support', label: 'support.help_center', icon: LifeBuoy },
]
</script>
