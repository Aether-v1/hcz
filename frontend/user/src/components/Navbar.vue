<template>
  <nav
    class="fixed top-0 left-0 right-0 z-50 bg-gradient-to-b from-white/70 to-white/0 backdrop-blur-sm dark:from-black/50 dark:to-black/0"
    :style="{ transitionDuration: 'var(--ui-duration-normal)' }">
    <div class="hcz-shell-container flex h-14 items-center justify-between gap-4">
      <!-- Back-only 模式：返回按钮 -->
      <template v-if="backOnly">
        <button type="button" class="flex items-center gap-1 rounded-lg p-2 text-foreground transition-colors hover:bg-accent/50" @click="handleBack">
          <ChevronLeft :size="20" :stroke-width="2" />
        </button>
      </template>

      <!-- 正常模式：Logo + 铃铛 -->
      <template v-else>
        <router-link to="/" class="flex min-w-0 shrink-0 items-center gap-3" :title="brandSiteName">
          <img
            v-if="brandLogo"
            :src="brandLogo"
            :alt="brandSiteName"
            class="h-8 max-w-[140px] shrink-0 object-contain"
          />
          <span v-else class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary text-sm font-black text-primary-foreground">
            {{ brandInitial }}
          </span>
          <span class="max-w-[140px] truncate text-sm font-bold tracking-tight text-foreground">{{ brandSiteName }}</span>
        </router-link>

        <div class="flex shrink-0 items-center gap-0.5">
          <router-link to="/notifications" class="relative inline-flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground" :aria-label="t('notifications.title')" :title="t('notifications.title')">
            <Bell class="h-4 w-4" />
            <span v-if="notificationStore.unreadCount" class="absolute -right-1 -top-1 rounded-full bg-rose-500 px-1 text-[10px] font-bold text-white">{{ notificationStore.badgeText }}</span>
          </router-link>
        </div>
      </template>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '../stores/app'
import { useNotificationStore } from '../stores/notification'
import { getImageUrl } from '../utils/image'
import { Bell, ChevronLeft } from 'lucide-vue-next'

withDefaults(defineProps<{
  backOnly?: boolean
}>(), {
  backOnly: false,
})

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const notificationStore = useNotificationStore()

const handleBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/')
  }
}

const brandSiteName = computed(() => {
  const text = String(appStore.config?.brand?.site_name || '').trim()
  return text !== '' ? text : 'HCZ'
})

const brandInitial = computed(() => brandSiteName.value.charAt(0).toUpperCase())

const brandLogo = computed(() => {
  const raw = String(appStore.config?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : '/hcz1_logo.png'
})
</script>
