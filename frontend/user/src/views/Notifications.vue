<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <!-- Header -->
      <div class="mb-8 mt-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('notifications.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('notifications.subtitle') }}</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          :disabled="notificationStore.unreadCount === 0 || markingAll"
          @click="handleMarkAllRead">
          <CheckCheck class="w-4 h-4" />
          {{ t('notifications.markAllRead') }}
        </Button>
      </div>

      <!-- Loading skeleton -->
      <div v-if="notificationStore.loadingList && notificationStore.notifications.length === 0" class="max-w-3xl mx-auto space-y-3">
        <div v-for="i in 5" :key="i" class="rounded-2xl border bg-muted/60 h-24 animate-pulse"></div>
      </div>

      <!-- Empty state -->
      <EmptyState
        v-else-if="notificationStore.notifications.length === 0"
        variant="soft"
        size="lg"
        class="max-w-3xl mx-auto"
        :title="t('notifications.empty')">
        <template #icon>
          <Bell class="w-20 h-20 text-muted-foreground opacity-70" :stroke-width="1.5" />
        </template>
      </EmptyState>

      <!-- List -->
      <div v-else class="max-w-3xl mx-auto space-y-3">
        <article
          v-for="item in notificationStore.notifications"
          :key="item.id"
          class="group flex items-start gap-4 rounded-2xl border p-4 md:p-5 transition-colors cursor-pointer"
          :class="item.is_read
            ? 'bg-card/60 border-border/60 opacity-70'
            : 'bg-card border-primary/30 shadow-sm hover:border-primary/50'"
          @click="handleItemClick(item)">
          <!-- Type icon -->
          <div class="flex h-11 w-11 flex-none items-center justify-center rounded-xl" :class="typeMeta(item.type).box">
            <component :is="typeMeta(item.type).icon" class="h-5 w-5" />
          </div>

          <!-- Content -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h2 class="truncate text-base" :class="item.is_read ? 'font-medium text-muted-foreground' : 'font-bold text-foreground'">
                {{ item.title }}
              </h2>
              <span v-if="!item.is_read" class="mt-0.5 h-2 w-2 flex-none rounded-full bg-primary"></span>
            </div>
            <p v-if="item.body" class="mt-1 text-sm text-muted-foreground line-clamp-2">{{ item.body }}</p>
            <time class="mt-2 block text-xs text-muted-foreground/80 font-mono">
              {{ formatTime(item.created_at) }}
            </time>
          </div>

          <!-- Read badge -->
          <Badge v-if="item.is_read" variant="neutral" size="xs" class="flex-none">{{ t('notifications.read') }}</Badge>
        </article>

        <!-- Load more -->
        <div v-if="notificationStore.hasMore" class="pt-2 text-center">
          <Button variant="outline" size="sm" :disabled="notificationStore.loadingList" @click="handleLoadMore">
            <Loader2 v-if="notificationStore.loadingList" class="w-4 h-4 animate-spin" />
            {{ notificationStore.loadingList ? t('notifications.loading') : t('notifications.loadMore') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Bell, CheckCheck, CheckCircle2, BadgeDollarSign, Loader2, MessageSquareWarning,
  Package, TrendingUp, Wallet, XCircle,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import EmptyState from '../components/EmptyState.vue'
import { useNotificationStore, type NotificationItem } from '../stores/notification'
import { useAppStore } from '../stores/app'
import { toast } from '../composables/useToast'

const { t } = useI18n()
const notificationStore = useNotificationStore()
const appStore = useAppStore()
const markingAll = ref(false)

const typeMetaMap: Record<string, { icon: any; box: string }> = {
  wallet_recharge: { icon: Wallet, box: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' },
  order_processing: { icon: Package, box: 'bg-blue-500/10 text-blue-600 dark:text-blue-400' },
  order_completed: { icon: CheckCircle2, box: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' },
  refund_success: { icon: BadgeDollarSign, box: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' },
  aftersale_update: { icon: MessageSquareWarning, box: 'bg-amber-500/10 text-amber-600 dark:text-amber-400' },
  commission_confirmed: { icon: TrendingUp, box: 'bg-purple-500/10 text-purple-600 dark:text-purple-400' },
  order_canceled: { icon: XCircle, box: 'bg-zinc-500/10 text-zinc-500' },
}

const defaultMeta = { icon: Bell, box: 'bg-secondary text-muted-foreground' }
const typeMeta = (type: string) => typeMetaMap[type] || defaultMeta

const formatTime = (iso: string) => {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(appStore.locale, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const handleItemClick = async (item: NotificationItem) => {
  if (item.is_read) return
  try {
    await notificationStore.markRead(item.id)
  } catch {
    toast.error(t('notifications.markReadFailed'))
  }
}

const handleMarkAllRead = async () => {
  markingAll.value = true
  try {
    await notificationStore.markAllRead()
    toast.success(t('notifications.markAllReadSuccess'))
  } catch {
    toast.error(t('notifications.markAllReadFailed'))
  } finally {
    markingAll.value = false
  }
}

const handleLoadMore = () => {
  void notificationStore.loadMore()
}

onMounted(() => {
  void notificationStore.fetchList(1)
})
</script>

<style scoped>
.line-clamp-2 {
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}
</style>
