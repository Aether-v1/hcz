<template>
  <div class="space-y-4 pb-8">
    <!-- Header -->
    <div class="mb-6 mt-8">
      <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('c2c.myTrades.title') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('c2c.myTrades.subtitle') }}</p>
    </div>

    <!-- Status Filter Tabs -->
    <div class="mb-6 flex flex-wrap gap-2">
      <button
        v-for="tab in statusTabs"
        :key="tab.key"
        class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
        :class="activeStatus === tab.key
          ? 'bg-primary text-primary-foreground'
          : 'bg-card border text-muted-foreground hover:text-foreground'"
        @click="onFilterChange(tab.key)"
      >
        {{ tab.label }}
      </button>
    </div>

    <!-- Loading skeleton -->
    <div v-if="c2cStore.myTradesLoading && list.length === 0" class="space-y-3">
      <div v-for="i in 4" :key="i" class="rounded-2xl border bg-muted/60 h-28 animate-pulse"></div>
    </div>

    <!-- Empty state -->
    <EmptyState
      v-else-if="list.length === 0"
      variant="soft"
      size="lg"
      :title="t('c2c.myTrades.empty')"
    />

    <!-- PC Table -->
    <div v-else class="hidden md:block rounded-2xl border bg-card shadow-sm overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow class="bg-muted/40">
            <TableHead>{{ t('c2c.myTrades.tradeNo') }}</TableHead>
            <TableHead>{{ t('c2c.myTrades.myRole') }}</TableHead>
            <TableHead>{{ t('c2c.myTrades.counterparty') }}</TableHead>
            <TableHead>{{ t('c2c.myTrades.fiatAmount') }}</TableHead>
            <TableHead>{{ t('c2c.myTrades.usdtAmount') }}</TableHead>
            <TableHead>{{ t('c2c.myTrades.unitPrice') }}</TableHead>
            <TableHead>{{ t('c2c.myTrades.status') }}</TableHead>
            <TableHead>{{ t('c2c.myTrades.createdAt') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="trade in list" :key="trade.id" class="cursor-pointer" @click="goDetail(trade.id)">
            <TableCell class="font-mono text-xs">{{ trade.trade_no }}</TableCell>
            <TableCell>
              <Badge size="xs" :class="myRole(trade) === 'buyer' ? 'bg-blue-500/10 text-blue-600' : 'bg-purple-500/10 text-purple-600'">
                {{ myRole(trade) === 'buyer' ? t('c2c.myTrades.buyer') : t('c2c.myTrades.seller') }}
              </Badge>
            </TableCell>
            <TableCell class="text-sm text-muted-foreground">{{ t('c2c.userPrefix') }}{{ counterpartyId(trade) }}</TableCell>
            <TableCell class="font-mono text-sm">
              {{ trade.fiat_amount }} <span class="text-xs text-muted-foreground">{{ trade.fiat_currency }}</span>
            </TableCell>
            <TableCell class="font-mono text-sm">{{ trade.usdt_amount }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">{{ trade.price }}</TableCell>
            <TableCell>
              <Badge size="sm" :class="TRADE_STATUS_VARIANTS[trade.status]">
                {{ TRADE_STATUS_LABELS[trade.status] }}
              </Badge>
            </TableCell>
            <TableCell class="text-xs text-muted-foreground">{{ formatTime(trade.created_at) }}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- Mobile Cards -->
    <div class="md:hidden space-y-3">
      <div
        v-for="trade in list"
        :key="trade.id"
        class="rounded-2xl border bg-card p-4 shadow-sm cursor-pointer"
        @click="goDetail(trade.id)"
      >
        <div class="flex items-start justify-between gap-2">
          <div class="font-mono text-xs text-muted-foreground">{{ trade.trade_no }}</div>
          <Badge size="sm" :class="TRADE_STATUS_VARIANTS[trade.status]">
            {{ TRADE_STATUS_LABELS[trade.status] }}
          </Badge>
        </div>
        <div class="mt-3 flex items-end justify-between">
          <div>
            <div class="text-lg font-bold font-mono">{{ formatUsdt(trade.usdt_amount) }}</div>
            <div class="mt-0.5 text-xs text-muted-foreground">
              {{ trade.fiat_amount }} {{ trade.fiat_currency }} · @ {{ trade.price }}
            </div>
          </div>
          <Badge size="xs" :class="myRole(trade) === 'buyer' ? 'bg-blue-500/10 text-blue-600' : 'bg-purple-500/10 text-purple-600'">
            {{ myRole(trade) === 'buyer' ? t('c2c.myTrades.buyer') : t('c2c.myTrades.seller') }}
          </Badge>
        </div>
        <div class="mt-3 flex items-center justify-between text-xs text-muted-foreground">
          <span>{{ t('c2c.myTrades.counterparty') }} {{ t('c2c.userPrefix') }}{{ counterpartyId(trade) }}</span>
          <span>{{ formatTime(trade.created_at) }}</span>
        </div>
      </div>
    </div>

    <!-- Load more -->
    <div v-if="hasMore" class="mt-6 text-center">
      <Button variant="outline" :disabled="loadingMore" @click="loadMore">
        <Loader2 v-if="loadingMore" class="w-4 h-4 animate-spin" />
        {{ loadingMore ? t('c2c.myTrades.loadingMore') : t('c2c.market.loadMore') }}
      </Button>
    </div>
</div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import EmptyState from '@/components/EmptyState.vue'
import { formatUsdt } from '@/utils/money'
import { useC2CStore } from '@/stores/c2c'
import { useUserAuthStore } from '@/stores/userAuth'
import { c2cAPI, type C2CTrade, type C2CTradeStatus } from '@/api/c2c'
import {
  TRADE_STATUS_LABELS,
  TRADE_STATUS_VARIANTS,
} from '@/composables/useC2C'
import { toast } from '@/composables/useToast'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const router = useRouter()
const c2cStore = useC2CStore()
const auth = useUserAuthStore()
const appStore = useAppStore()

type StatusFilter = '' | C2CTradeStatus

const activeStatus = ref<StatusFilter>('')
const loadingMore = ref(false)

const statusTabs: { key: StatusFilter; label: string }[] = [
  { key: '', label: t('c2c.myTrades.all') },
  { key: 'pending_payment', label: TRADE_STATUS_LABELS.pending_payment },
  { key: 'paid', label: TRADE_STATUS_LABELS.paid },
  { key: 'disputed', label: TRADE_STATUS_LABELS.disputed },
  { key: 'completed', label: TRADE_STATUS_LABELS.completed },
  { key: 'canceled', label: TRADE_STATUS_LABELS.canceled },
  { key: 'expired', label: TRADE_STATUS_LABELS.expired },
]

const list = computed(() => c2cStore.myTrades)

const hasMore = computed(() => {
  const pg = c2cStore.myTradesPagination
  return pg.page < pg.total_page
})

const currentUserId = computed(() => Number(auth.user?.id) || 0)

const myRole = (trade: C2CTrade): 'buyer' | 'seller' => {
  if (trade.buyer_user_id === currentUserId.value) return 'buyer'
  return 'seller'
}

const counterpartyId = (trade: C2CTrade): number => {
  return myRole(trade) === 'buyer' ? trade.seller_user_id : trade.buyer_user_id
}

const formatTime = (iso: string) => {
  if (!iso) return ''
  const d = new Date(iso.replace(' ', 'T'))
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(appStore.locale, {
    year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

const goDetail = (id: number) => router.push(`/c2c/trades/${id}`)

const onFilterChange = (status: StatusFilter) => {
  activeStatus.value = status
  void c2cStore.fetchMyTrades({ page: 1, status: status || undefined })
}

const loadMore = async () => {
  if (loadingMore.value) return
  loadingMore.value = true
  try {
    const nextPage = c2cStore.myTradesPagination.page + 1
    const res = await c2cAPI.listMyTrades({
      page: nextPage,
      page_size: c2cStore.myTradesPagination.page_size,
      status: activeStatus.value || undefined,
    })
    const data = (res.data?.data || []) as C2CTrade[]
    c2cStore.myTrades.push(...data)
    const pg = res.data?.pagination
    if (pg) {
      c2cStore.myTradesPagination.page = Number(pg.page) || nextPage
    }
  } catch {
    toast.error(t('c2c.myTrades.loadMoreFailed'))
  } finally {
    loadingMore.value = false
  }
}

onMounted(() => {
  void c2cStore.fetchMyTrades({ page: 1 })
})
</script>
