<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('personalCenter.points.ordersTitle') }}</h1>
    </div>

    <!-- 状态 Tab 栏（复用充值订单列表的 Tab 语言） -->
    <div class="flex flex-wrap gap-3 border-b border-border/60 pb-2">
      <button
        v-for="tab in statusTabs"
        :key="tab.value"
        type="button"
        class="relative shrink-0 pb-1.5 text-sm font-medium transition-colors"
        :class="status === tab.value ? 'text-primary' : 'text-muted-foreground hover:text-foreground'"
        @click="setStatus(tab.value)"
      >
        {{ tab.label }}
        <span v-if="status === tab.value" class="absolute -bottom-[9px] left-0 right-0 h-0.5 rounded-full bg-primary"></span>
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-2xl bg-muted/60"></div>
    </div>

    <!-- Empty -->
    <EmptyState
      v-else-if="orders.length === 0"
      icon="order"
      :description="t('personalCenter.points.ordersEmpty')"
      :action-label="t('personalCenter.points.mallEntry')"
      action-to="/me/points/mall"
    />

    <!-- 订单列表（复用现有订单卡语言） -->
    <div v-else class="space-y-3">
      <div
        v-for="order in orders"
        :key="order.id"
        class="rounded-2xl border bg-card p-4 shadow-sm transition-all hover:border-primary/30"
      >
        <div class="flex items-start gap-3">
          <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <ReceiptText :size="20" :stroke-width="1.8" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-start justify-between gap-2">
              <span class="truncate text-sm font-semibold text-foreground">{{ order.product_name_snapshot }}</span>
              <span class="shrink-0 text-sm font-bold tabular-nums text-foreground">-{{ order.total_points }}</span>
            </div>
            <div class="mt-2 flex items-center justify-between gap-2">
              <span class="text-xs text-muted-foreground">{{ formatDate(order.created_at) }}</span>
              <div class="flex items-center gap-2">
                <Badge :variant="statusVariant(order.status)" size="sm">{{ statusLabel(order.status) }}</Badge>
                <Button
                  v-if="canCancel(order)"
                  variant="ghost"
                  size="sm"
                  class="h-7 px-2 text-xs font-semibold text-muted-foreground hover:text-destructive"
                  @click="cancelOrder(order)"
                >
                  {{ t('personalCenter.points.cancelExchange') }}
                </Button>
              </div>
            </div>
          </div>
        </div>

        <!-- 详情行（复用现有 Detail Row 语言；订单详情 API 仅返回单个订单对象，展开详情不额外请求） -->
        <div v-if="expandedId === order.id" class="mt-3 space-y-2 rounded-xl bg-muted/50 p-4 text-sm">
          <div class="flex justify-between">
            <span class="text-muted-foreground">{{ t('personalCenter.points.orderNo') }}</span>
            <span class="font-mono text-xs text-foreground">{{ order.order_no }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">{{ t('personalCenter.points.unitPoints') }}</span>
            <span class="font-medium tabular-nums text-foreground">{{ order.unit_points }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">{{ t('personalCenter.points.quantity') }}</span>
            <span class="font-medium tabular-nums text-foreground">{{ order.quantity }}</span>
          </div>
          <div v-if="order.completed_at" class="flex justify-between">
            <span class="text-muted-foreground">{{ t('personalCenter.points.completedAt') }}</span>
            <span class="text-foreground">{{ formatDate(order.completed_at) }}</span>
          </div>
          <div v-if="order.reason" class="flex justify-between gap-4">
            <span class="shrink-0 text-muted-foreground">{{ t('personalCenter.points.reason') }}</span>
            <span class="text-right text-foreground">{{ order.reason }}</span>
          </div>
          <div v-if="fulfillmentHint(order)" class="rounded-lg border bg-card px-3 py-2.5 text-xs leading-5 text-muted-foreground">
            {{ fulfillmentHint(order) }}
          </div>
          <div v-if="canCancel(order)" class="pt-1 text-xs text-muted-foreground">
            {{ t('personalCenter.points.pendingHint') }}
          </div>
        </div>

        <button
          type="button"
          class="mt-2 flex w-full items-center justify-center gap-1 rounded-lg py-1 text-xs font-medium text-muted-foreground transition-colors hover:text-primary"
          @click="expandedId = expandedId === order.id ? null : order.id"
        >
          {{ expandedId === order.id ? t('personalCenter.points.collapseDetail') : t('personalCenter.points.viewDetail') }}
          <ChevronDown :size="14" :class="expandedId === order.id ? 'rotate-180 transition-transform' : 'transition-transform'" />
        </button>
      </div>
    </div>

    <PaginationNav
      :current-page="pagination.page"
      :total-pages="pagination.total_page"
      :loading="loading"
      :scroll-top="false"
      @change-page="loadOrders"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ReceiptText } from 'lucide-vue-next'
import { pointsMallAPI, POINTS_EXCHANGE_STATUS, type PointsExchangeOrder } from '../../api'
import { pointsExchangeStatusLabel, pointsExchangeStatusVariant } from '../../utils/status'
import { debounceAsync } from '../../utils/debounce'
import { toast } from '../../composables/useToast'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()
const { confirm } = useConfirmDialog()

const statusTabs = computed(() => [
  { value: '', label: t('personalCenter.points.statusAll') },
  { value: POINTS_EXCHANGE_STATUS.PENDING, label: pointsExchangeStatusLabel(t, POINTS_EXCHANGE_STATUS.PENDING) },
  { value: POINTS_EXCHANGE_STATUS.PROCESSING, label: pointsExchangeStatusLabel(t, POINTS_EXCHANGE_STATUS.PROCESSING) },
  { value: POINTS_EXCHANGE_STATUS.COMPLETED, label: pointsExchangeStatusLabel(t, POINTS_EXCHANGE_STATUS.COMPLETED) },
  { value: POINTS_EXCHANGE_STATUS.FAILED, label: pointsExchangeStatusLabel(t, POINTS_EXCHANGE_STATUS.FAILED) },
  { value: POINTS_EXCHANGE_STATUS.CANCELLED, label: pointsExchangeStatusLabel(t, POINTS_EXCHANGE_STATUS.CANCELLED) },
])

const orders = ref<PointsExchangeOrder[]>([])
const loading = ref(true)
const status = ref('')
const expandedId = ref<number | null>(null)
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const setStatus = (value: string) => {
  status.value = value
  void debouncedLoadOrders(1)
}

const loadOrders = async (page = 1) => {
  loading.value = true
  try {
    const response = await pointsMallAPI.exchangeOrders({
      page,
      page_size: pagination.value.page_size,
      status: status.value || undefined,
    })
    orders.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch {
    orders.value = []
  } finally {
    loading.value = false
  }
}

const debouncedLoadOrders = debounceAsync(loadOrders, 300)

const cancelOrder = async (order: PointsExchangeOrder) => {
  const ok = await confirm({
    title: t('personalCenter.points.cancelConfirmTitle'),
    message: t('personalCenter.points.cancelConfirm', { name: order.product_name_snapshot }),
    variant: 'danger',
    confirmText: t('personalCenter.points.cancelExchange'),
  })
  if (!ok) return
  try {
    const response = await pointsMallAPI.cancelExchangeOrder(order.id)
    const result = response.data.data
    const next = result?.order
    if (next) {
      const index = orders.value.findIndex((item) => item.id === next.id)
      if (index >= 0) orders.value[index] = next
    }
    toast.success(t('personalCenter.points.cancelSuccess'))
  } catch (err: any) {
    toast.error(err?.message || t('personalCenter.points.cancelFailed'))
  }
}

// MANUAL 履约：兑换成功后按商品说明由人工发放，此处回显用户需知的指引。
const fulfillmentHint = (order: PointsExchangeOrder) => {
  if (order.fulfillment_type_snapshot !== 'MANUAL') return ''
  if (order.status === POINTS_EXCHANGE_STATUS.COMPLETED
    || order.status === POINTS_EXCHANGE_STATUS.PROCESSING
    || order.status === POINTS_EXCHANGE_STATUS.PENDING) {
    return t('personalCenter.points.manualHint')
  }
  return ''
}

const canCancel = (order: PointsExchangeOrder) => order.status === POINTS_EXCHANGE_STATUS.PENDING

const statusLabel = (value: string) => pointsExchangeStatusLabel(t, value)
const statusVariant = (value: string) => pointsExchangeStatusVariant(value)

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

onMounted(() => {
  void debouncedLoadOrders(1)
})
</script>
