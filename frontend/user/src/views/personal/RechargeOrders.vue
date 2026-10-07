<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('orders.rechargeRecords') }}</h1>
    </div>

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
        class="block rounded-2xl border bg-card p-4 shadow-sm transition-all hover:border-primary/30">
        <div class="flex items-start gap-3">
          <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Wallet :size="20" :stroke-width="1.8" />
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Wallet } from 'lucide-vue-next'
import { walletAPI } from '../../api/wallet'
import type { BadgeTone } from '../../utils/status'
import { debounceAsync } from '../../utils/debounce'
import { Badge } from '@/components/ui/badge'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()

const rechargeLoading = ref(true)
const rechargeOrders = ref<any[]>([])
const rechargePagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const rechargeFilters = reactive({ status: '' })

const rechargeStatusTabs = computed(() => [
  { value: '', label: t('orders.filters.statusAll') },
  { value: 'pending', label: t('personalCenter.wallet.rechargeStatus.pending') },
  { value: 'success', label: t('personalCenter.wallet.rechargeStatus.success') },
  { value: 'failed', label: t('personalCenter.wallet.rechargeStatus.failed') },
])

const setRechargeStatus = (status: string) => {
  rechargeFilters.status = status
  loadRechargeOrders(1)
}

const loadRechargeOrders = async (page = 1) => {
  rechargeLoading.value = true
  try {
    const response = await walletAPI.rechargeOrders({
      page,
      page_size: rechargePagination.value.page_size,
      status: rechargeFilters.status || undefined,
    })
    rechargeOrders.value = response.data.data || []
    rechargePagination.value = response.data.pagination || rechargePagination.value
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
  debouncedLoadRechargeOrders(1)
})

onUnmounted(() => {
  debouncedLoadRechargeOrders.cancel()
})
</script>
