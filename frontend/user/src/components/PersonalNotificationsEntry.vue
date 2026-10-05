<template>
  <RouterLink to="/notifications" class="flex min-h-14 items-center gap-3 rounded-xl border bg-card px-4 py-3 text-foreground transition-colors hover:border-primary/40 hover:text-primary">
    <span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary"><Bell :size="19" aria-hidden="true" /></span>
    <span class="min-w-0 flex-1 font-semibold">{{ t('notifications.entryTitle') }}</span>
    <span v-if="notificationStore.unreadCount > 0" class="rounded-full bg-destructive px-2 py-0.5 text-xs font-bold text-white" :aria-label="`${notificationStore.unreadCount} ${t('notifications.unread')}`">{{ notificationStore.badgeText }}</span>
    <ChevronRight :size="18" class="shrink-0 text-muted-foreground" aria-hidden="true" />
  </RouterLink>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronRight } from 'lucide-vue-next'
import { useNotificationStore } from '../stores/notification'

const { t } = useI18n()
const notificationStore = useNotificationStore()
</script>
