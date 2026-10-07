<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('orders.title') }}</h1>
    </div>

    <!-- 状态 Tab 栏 -->
    <div class="flex flex-wrap gap-3 border-b border-border/60 pb-2">
      <button
        v-for="tab in orderStatusTabs"
        :key="tab.value"
        type="button"
        class="relative shrink-0 pb-1.5 text-sm font-medium transition-colors"
        :class="orderFilters.status === tab.value ? 'text-primary' : 'text-muted-foreground hover:text-foreground'"
        @click="setOrderStatus(tab.value)">
        {{ tab.label }}
        <span v-if="orderFilters.status === tab.value" class="absolute -bottom-[9px] left-0 right-0 h-0.5 rounded-full bg-primary"></span>
      </button>
    </div>

    <!-- 搜索（折叠） -->
    <div v-if="showOrderSearch" class="flex gap-2">
      <Input
        v-model="orderFilters.orderNo"
        type="text"
        :placeholder="t('orders.filters.orderNoPlaceholder')"
        class="h-10 flex-1"
        @keyup.enter="applyOrderFilters"
      />
      <Button type="button" size="sm" class="h-10" @click="applyOrderFilters">{{ t('orders.filters.search') }}</Button>
      <Button type="button" variant="ghost" size="sm" class="h-10" @click="resetOrderFilters; showOrderSearch = false">{{ t('orders.filters.reset') }}</Button>
    </div>

    <!-- Loading -->
    <div v-if="orderLoading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-2xl bg-muted/60"></div>
    </div>

    <!-- Empty -->
    <EmptyState
      v-else-if="orders.length === 0"
      icon="order"
      :description="hasOrderActiveFilters ? t('orders.emptyFiltered') : t('orders.empty')"
      :action-label="t('orders.emptyAction')"
      action-to="/products"
    />

    <!-- 订单列表 -->
    <div v-else class="space-y-3">
      <router-link
        v-for="order in orders"
        :key="order.order_no"
        :to="`/orders/${order.order_no}`"
        class="block rounded-2xl border bg-card p-4 shadow-sm transition-all hover:border-primary/30">
        <div class="flex items-start gap-3">
          <!-- 服务图标 -->
          <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <ReceiptText :size="20" :stroke-width="1.8" />
          </div>
          <!-- 订单信息 -->
          <div class="min-w-0 flex-1">
            <div class="flex items-start justify-between gap-2">
              <span class="truncate text-sm font-semibold text-foreground">{{ getLocalizedText(order.product?.name) || order.order_no }}</span>
              <span class="shrink-0 text-sm font-bold tabular-nums text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</span>
            </div>
            <div class="mt-2 flex items-center justify-between">
              <span class="text-xs text-muted-foreground">{{ formatDate(order.created_at) }}</span>
              <Badge :variant="statusVariant(order.status)" size="sm">
                {{ statusLabel(order.status) }}
              </Badge>
            </div>
          </div>
        </div>
      </router-link>
    </div>

    <PaginationNav
      :current-page="orderPagination.page"
      :total-pages="orderPagination.total_page"
      :loading="orderLoading"
      :scroll-top="false"
      @change-page="changeOrderPage"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ReceiptText } from 'lucide-vue-next'
import { userOrderAPI } from '../../api'
import { orderStatusVariant, orderStatusLabel } from '../../utils/status'
import { debounceAsync } from '../../utils/debounce'
import { useLocalized } from '../../composables/useProduct'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()
const { getLocalizedText } = useLocalized()

const showOrderSearch = ref(false)

const orderStatusTabs = computed(() => [
  { value: '', label: t('orders.filters.statusAll') },
  { value: 'pending_payment', label: t('order.status.pending_payment') },
  { value: 'paid', label: t('order.status.paid') },
  { value: 'fulfilling', label: t('order.status.fulfilling') },
  { value: 'completed', label: t('order.status.completed') },
  { value: 'canceled', label: t('order.status.canceled') },
])

const setOrderStatus = (status: string) => {
  orderFilters.status = status
  loadOrders(1)
}

const orderLoading = ref(true)
const orders = ref<any[]>([])
const orderPagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const orderFilters = reactive({ orderNo: '', status: '' })

const hasOrderActiveFilters = computed(() => Boolean(orderFilters.orderNo || orderFilters.status))

const loadOrders = async (page = 1) => {
  orderLoading.value = true
  try {
    const response = await userOrderAPI.list({
      page,
      page_size: orderPagination.value.page_size,
      status: orderFilters.status || undefined,
      order_no: orderFilters.orderNo || undefined,
    })
    orders.value = response.data.data || []
    orderPagination.value = response.data.pagination || orderPagination.value
  } catch {
    orders.value = []
  } finally {
    orderLoading.value = false
  }
}

const debouncedLoadOrders = debounceAsync(loadOrders, 300)

const changeOrderPage = (page: number) => {
  if (page < 1 || page > orderPagination.value.total_page) return
  debouncedLoadOrders(page)
}
const applyOrderFilters = () => loadOrders(1)
const resetOrderFilters = () => {
  orderFilters.orderNo = ''
  orderFilters.status = ''
  loadOrders(1)
}

const statusLabel = (status: string) => orderStatusLabel(t, status)
const statusVariant = (status: string) => orderStatusVariant(status)

const formatMoney = (amount?: string, currency?: string) => {
  if (amount === null || amount === undefined || amount === '') return '-'
  if (currency === null || currency === undefined || currency === '') return String(amount)
  return `${amount} ${currency}`
}

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

onMounted(() => {
  debouncedLoadOrders(1)
})

onUnmounted(() => {
  debouncedLoadOrders.cancel()
})
</script>
