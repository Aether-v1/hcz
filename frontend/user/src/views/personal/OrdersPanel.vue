<template>
  <div class="space-y-5">
    <!-- 标题 + 类型切换 -->
    <div class="flex items-center justify-between">
      <h1 class="text-lg font-bold tracking-tight text-foreground sm:text-xl">{{ t('orders.title') }}</h1>
      <div class="flex shrink-0 rounded-full bg-secondary/60 p-1">
        <button
          type="button"
          class="rounded-full px-3 py-1.5 text-xs font-semibold transition-colors sm:text-sm"
          :class="activeTab === 'product' ? 'bg-card text-primary shadow-sm' : 'text-muted-foreground hover:text-foreground'"
          @click="switchTab('product')">
          {{ t('orders.tabs.product') }}
        </button>
        <button
          type="button"
          class="rounded-full px-3 py-1.5 text-xs font-semibold transition-colors sm:text-sm"
          :class="activeTab === 'recharge' ? 'bg-card text-primary shadow-sm' : 'text-muted-foreground hover:text-foreground'"
          @click="switchTab('recharge')">
          {{ t('orders.tabs.recharge') }}
        </button>
      </div>
    </div>

    <!-- 普通订单 -->
    <template v-if="activeTab === 'product'">
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
          class="block rounded-2xl border border-border/60 bg-card p-4 transition-all hover:border-primary/30 hover:shadow-sm">
          <div class="flex items-start gap-3">
            <!-- 服务图标 -->
            <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <ReceiptText class="h-5 w-5" />
            </div>
            <!-- 订单信息 -->
            <div class="min-w-0 flex-1">
              <div class="flex items-start justify-between gap-2">
                <span class="truncate text-sm font-semibold text-foreground">{{ getLocalizedText(order.items?.[0]?.title) || t('orders.serviceFallback') }}</span>
                <span class="shrink-0 text-sm font-bold tabular-nums text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</span>
              </div>
              <p class="mt-0.5 truncate text-xs text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</p>
              <div class="mt-2 flex items-center justify-between">
                <span class="text-xs text-muted-foreground">{{ formatDate(order.created_at) }}</span>
                <div class="flex items-center gap-2">
                  <Badge :variant="statusVariant(order.status) === 'info' ? 'neutral' : statusVariant(order.status)" size="sm">
                    {{ statusLabel(order.status) }}
                  </Badge>
                  <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="h-7 rounded-full px-3 text-xs">
                    <router-link :to="`/pay?order_no=${order.order_no}`" @click.stop>{{ t('orders.payNow') }}</router-link>
                  </Button>
                </div>
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
    </template>

    <!-- 充值订单 -->
    <template v-else>
      <!-- 状态 Tab 栏 -->
      <div class="flex flex-wrap gap-3 border-b border-border/60 pb-2">
        <button
          v-for="tab in rechargeStatusTabs"
          :key="tab.value"
          type="button"
          class="relative shrink-0 pb-1.5 text-sm font-medium transition-colors"
          :class="rechargeFilters.status === tab.value ? 'text-primary' : 'text-muted-foreground hover:text-foreground'"
          @click="setRechargeStatus(tab.value)">
          {{ tab.label }}
          <span v-if="rechargeFilters.status === tab.value" class="absolute -bottom-[9px] left-0 right-0 h-0.5 rounded-full bg-primary"></span>
        </button>
      </div>

      <!-- 搜索（折叠） -->
      <div v-if="showRechargeSearch" class="flex gap-2">
        <Input
          v-model="rechargeFilters.rechargeNo"
          type="text"
          :placeholder="t('orders.rechargeFilters.rechargeNoPlaceholder')"
          class="h-10 flex-1"
          @keyup.enter="applyRechargeFilters"
        />
        <Button type="button" size="sm" class="h-10" @click="applyRechargeFilters">{{ t('orders.filters.search') }}</Button>
        <Button type="button" variant="ghost" size="sm" class="h-10" @click="resetRechargeFilters; showRechargeSearch = false">{{ t('orders.filters.reset') }}</Button>
      </div>

      <!-- Loading -->
      <div v-if="rechargeLoading" class="space-y-3">
        <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-2xl bg-muted/60"></div>
      </div>

      <!-- Empty -->
      <EmptyState
        v-else-if="rechargeOrders.length === 0"
        icon="order"
        :description="t('orders.rechargeEmpty')"
        :action-label="t('orders.rechargeEmptyAction')"
        action-to="/me/wallet"
      />

      <!-- 充值订单列表 -->
      <div v-else class="space-y-3">
        <router-link
          v-for="ro in rechargeOrders"
          :key="ro.recharge_no"
          :to="`/recharge-orders/${ro.recharge_no}`"
          class="block rounded-2xl border border-border/60 bg-card p-4 transition-all hover:border-primary/30 hover:shadow-sm">
          <div class="flex items-start gap-3">
            <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-emerald-500/10 text-emerald-500">
              <Wallet class="h-5 w-5" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-start justify-between gap-2">
                <span class="truncate text-sm font-semibold text-foreground">{{ t('personalCenter.wallet.rechargeNoLabel') }}：{{ ro.recharge_no }}</span>
                <span class="shrink-0 text-sm font-bold tabular-nums text-foreground">{{ formatMoney(ro.amount, ro.currency) }}</span>
              </div>
              <div class="mt-2 flex items-center justify-between">
                <span class="text-xs text-muted-foreground">{{ formatDate(ro.created_at) }}</span>
                <Badge :variant="rechargeStatusVariant(ro.status)" size="sm">
                  {{ rechargeStatusText(ro.status) }}
                </Badge>
              </div>
            </div>
          </div>
        </router-link>
      </div>

      <PaginationNav
        :current-page="rechargePagination.page"
        :total-pages="rechargePagination.total_page"
        :loading="rechargeLoading"
        :scroll-top="false"
        @change-page="changeRechargePage"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ReceiptText, Wallet } from 'lucide-vue-next'
import { userOrderAPI } from '../../api'
import { walletAPI } from '../../api/wallet'
import { orderStatusVariant, orderStatusLabel, type BadgeTone } from '../../utils/status'
import { debounceAsync } from '../../utils/debounce'
import { useLocalized } from '../../composables/useProduct'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()
const { getLocalizedText } = useLocalized()

// ========== Tab 状态 ==========
const activeTab = ref<'product' | 'recharge'>('product')

const switchTab = (tab: 'product' | 'recharge') => {
  if (activeTab.value === tab) return
  activeTab.value = tab
  if (tab === 'product' && !orderLoaded.value) {
    loadOrders(1)
  }
  if (tab === 'recharge' && !rechargeLoaded.value) {
    loadRechargeOrders(1)
  }
}

// 搜索折叠
const showOrderSearch = ref(false)
const showRechargeSearch = ref(false)

// 状态 Tab
const orderStatusTabs = computed(() => [
  { value: '', label: t('orders.filters.statusAll') },
  { value: 'pending_payment', label: t('order.status.pending_payment') },
  { value: 'paid', label: t('order.status.paid') },
  { value: 'fulfilling', label: t('order.status.fulfilling') },
  { value: 'completed', label: t('order.status.completed') },
  { value: 'canceled', label: t('order.status.canceled') },
])

const rechargeStatusTabs = computed(() => [
  { value: '', label: t('orders.filters.statusAll') },
  { value: 'pending', label: t('personalCenter.wallet.rechargeStatus.pending') },
  { value: 'success', label: t('personalCenter.wallet.rechargeStatus.success') },
  { value: 'failed', label: t('personalCenter.wallet.rechargeStatus.failed') },
])

const setOrderStatus = (status: string) => {
  orderFilters.status = status
  loadOrders(1)
}

const setRechargeStatus = (status: string) => {
  rechargeFilters.status = status
  loadRechargeOrders(1)
}

// ========== 普通订单 ==========
const orderLoading = ref(true)
const orderLoaded = ref(false)
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
    orderLoaded.value = true
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

// ========== 充值订单 ==========
const rechargeLoading = ref(false)
const rechargeLoaded = ref(false)
const rechargeOrders = ref<any[]>([])
const rechargePagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const rechargeFilters = reactive({ rechargeNo: '', status: '' })

const loadRechargeOrders = async (page = 1) => {
  rechargeLoading.value = true
  try {
    const response = await walletAPI.rechargeOrders({
      page,
      page_size: rechargePagination.value.page_size,
      status: rechargeFilters.status || undefined,
      recharge_no: rechargeFilters.rechargeNo || undefined,
    })
    rechargeOrders.value = response.data.data || []
    rechargePagination.value = response.data.pagination || rechargePagination.value
    rechargeLoaded.value = true
  } catch {
    rechargeOrders.value = []
  } finally {
    rechargeLoading.value = false
  }
}

const debouncedLoadRechargeOrders = debounceAsync(loadRechargeOrders, 300)

const changeRechargePage = (page: number) => {
  if (page < 1 || page > rechargePagination.value.total_page) return
  debouncedLoadRechargeOrders(page)
}
const applyRechargeFilters = () => loadRechargeOrders(1)
const resetRechargeFilters = () => {
  rechargeFilters.rechargeNo = ''
  rechargeFilters.status = ''
  loadRechargeOrders(1)
}

const rechargeStatusText = (status?: string) => {
  const normalized = String(status || '').toLowerCase()
  const key = `personalCenter.wallet.rechargeStatus.${normalized}`
  const translated = t(key)
  if (translated === key) return normalized || '-'
  return translated
}

const rechargeStatusVariant = (status?: string): BadgeTone => {
  const normalized = String(status || '').toLowerCase()
  if (normalized === 'success') return 'success'
  if (normalized === 'failed' || normalized === 'expired') return 'danger'
  return 'warning'
}

// ========== 共用工具 ==========
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
  debouncedLoadRechargeOrders.cancel()
})
</script>

