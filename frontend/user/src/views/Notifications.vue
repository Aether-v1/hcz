<template>
  <div class="pb-8 text-foreground">
    <!-- Header -->
    <div class="mb-4 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold tracking-tight md:text-2xl">{{ t('notifications.title') }}</h1>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('notifications.subtitle') }}</p>
      </div>
      <Button
        variant="outline"
        size="sm"
        :disabled="notificationStore.unreadCount === 0 || markingAll"
        @click="handleMarkAllRead">
        <CheckCheck class="h-4 w-4" />
        {{ t('notifications.markAllRead') }}
      </Button>
    </div>

    <!-- Loading skeleton -->
    <div v-if="notificationStore.loadingList && notificationStore.notifications.length === 0" class="space-y-3">
      <div v-for="i in 5" :key="i" class="h-20 animate-pulse rounded-2xl border bg-muted/60"></div>
    </div>

    <!-- Empty state -->
    <EmptyState
      v-else-if="notificationStore.notifications.length === 0"
      variant="soft"
      size="lg"
      :title="t('notifications.empty')">
      <template #icon>
        <Bell class="h-16 w-16 text-muted-foreground opacity-50" :stroke-width="1.5" />
      </template>
    </EmptyState>

    <!-- List -->
    <div v-else class="space-y-3">
      <article
        v-for="item in notificationStore.notifications"
        :key="item.id"
        class="flex cursor-pointer items-start gap-3 rounded-2xl border p-4 transition-colors"
        :tabindex="0"
        :role="notificationTarget(item) ? 'link' : 'button'"
        :class="item.is_read
          ? 'border-border/60 bg-card/60 opacity-70'
          : 'border-primary/30 bg-card shadow-sm hover:border-primary/50'"
        @click="handleItemClick(item)"
        @keydown.enter="handleItemClick(item)"
        @keydown.space.prevent="handleItemClick(item)">
        <!-- Type icon -->
        <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl" :class="typeMeta(item.type).box">
          <component :is="typeMeta(item.type).icon" :size="18" :stroke-width="1.8" />
        </div>

        <!-- Content -->
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <h2 class="truncate text-sm" :class="item.is_read ? 'font-medium text-muted-foreground' : 'font-semibold text-foreground'">
              {{ item.title }}
            </h2>
            <span v-if="!item.is_read" class="h-2 w-2 shrink-0 rounded-full bg-primary"></span>
          </div>
          <p v-if="item.body" class="mt-1 line-clamp-2 text-xs text-muted-foreground">{{ item.body }}</p>
          <time class="mt-1.5 block font-mono text-[11px] text-muted-foreground/80">
            {{ formatTime(item.created_at) }}
          </time>
        </div>

        <!-- Read badge -->
        <Badge v-if="item.is_read" variant="neutral" size="xs" class="shrink-0">{{ t('notifications.read') }}</Badge>
      </article>

      <!-- Load more -->
      <div v-if="notificationStore.hasMore" class="pt-2 text-center">
        <Button variant="outline" size="sm" :disabled="notificationStore.loadingList" @click="handleLoadMore">
          <Loader2 v-if="notificationStore.loadingList" class="h-4 w-4 animate-spin" />
          {{ notificationStore.loadingList ? t('notifications.loading') : t('notifications.loadMore') }}
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
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
import { resolveNotificationTarget } from '../utils/notificationTarget'

const { t } = useI18n()
const notificationStore = useNotificationStore()
const router = useRouter()
const appStore = useAppStore()
const markingAll = ref(false)
const notificationTarget = (item: NotificationItem) => resolveNotificationTarget(item)

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
  if (!item.is_read) {
    try {
      await notificationStore.markRead(item.id)
    } catch {
      toast.error(t('notifications.markReadFailed'))
    }
  }
  const target = notificationTarget(item)
  if (target) await router.push(target)
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
