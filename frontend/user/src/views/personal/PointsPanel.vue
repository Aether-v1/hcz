<template>
  <div class="space-y-4 pb-8">
    <!-- 页面标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('personalCenter.points.balance') }}</h1>
    </div>

    <!-- 积分账户卡 -->
    <div class="rounded-2xl border bg-card p-5 shadow-sm">
      <div class="flex items-center gap-3">
        <div class="grid h-12 w-12 place-items-center rounded-xl bg-accent text-muted-foreground">
          <Coins :size="24" :stroke-width="1.8" />
        </div>
        <div>
          <p class="text-xs text-muted-foreground">{{ t('personalCenter.points.balance') }}</p>
          <p class="mt-0.5 text-2xl font-bold tracking-tight tabular-nums text-foreground">{{ account.balance }}</p>
        </div>
      </div>
      <div class="mt-4 grid grid-cols-2 gap-2 border-t pt-3 text-sm">
        <div>
          <p class="text-xs text-muted-foreground">{{ t('personalCenter.points.totalEarned') }}</p>
          <p class="mt-0.5 font-semibold tabular-nums text-foreground">+{{ account.total_earned }}</p>
        </div>
        <div>
          <p class="text-xs text-muted-foreground">{{ t('personalCenter.points.totalSpent') }}</p>
          <p class="mt-0.5 font-semibold tabular-nums text-foreground">-{{ account.total_spent }}</p>
        </div>
      </div>
    </div>

    <!-- 每日签到卡 -->
    <div v-if="checkinStatus?.enabled" class="rounded-2xl border bg-card p-5 shadow-sm">
      <div class="flex items-center gap-3">
        <div class="grid h-12 w-12 place-items-center rounded-xl bg-accent text-muted-foreground">
          <CalendarCheck :size="24" :stroke-width="1.8" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-foreground">{{ t('personalCenter.points.checkin') }}</p>
          <p class="mt-0.5 text-xs text-muted-foreground">
            {{ t('personalCenter.points.consecutiveDays', { n: checkinStatus.consecutive_days }) }}
            <span v-if="!checkinStatus.checked_in_today" class="ml-2">
              {{ t('personalCenter.points.rewardPreview', { points: checkinStatus.next_reward || checkinStatus.today_reward }) }}
            </span>
          </p>
        </div>
        <Button
          v-if="!checkinStatus.checked_in_today"
          size="sm"
          class="h-9 shrink-0 px-4 font-semibold"
          :disabled="checkinSubmitting"
          @click="doCheckin"
        >
          {{ checkinSubmitting ? t('personalCenter.points.checkingIn') : t('personalCenter.points.checkinButton') }}
        </Button>
        <Badge v-else variant="success" size="sm">
          <Check class="h-3 w-3" />
          {{ t('personalCenter.points.checkedIn') }}
        </Badge>
      </div>
      <div v-if="checkinHistory.length" class="mt-4 flex flex-wrap gap-1.5 border-t pt-3">
        <span
          v-for="date in checkinHistory"
          :key="date"
          class="rounded-md bg-accent px-2 py-1 text-[11px] font-medium tabular-nums text-muted-foreground"
        >
          {{ formatCheckinDay(date) }}
        </span>
        <span class="px-1 py-1 text-[11px] text-muted-foreground">{{ t('personalCenter.points.monthTotal', { n: checkinHistory.length }) }}</span>
      </div>
    </div>

    <!-- 商城 / 兑换记录入口 -->
    <div class="grid grid-cols-2 gap-3">
      <RouterLink
        v-for="entry in entryShortcuts"
        :key="entry.to"
        :to="entry.to"
        class="flex min-w-0 items-center gap-2.5 rounded-2xl border bg-card px-4 py-3.5 text-foreground shadow-sm transition-colors hover:border-primary/40 hover:text-primary"
      >
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <component :is="entry.icon" :size="18" aria-hidden="true" />
        </span>
        <span class="min-w-0 flex-1 truncate text-sm font-semibold">{{ t(entry.label) }}</span>
        <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
      </RouterLink>
    </div>

    <!-- 积分流水 -->
    <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
      <div class="border-b px-5 py-3">
        <p class="text-sm font-semibold text-foreground">{{ t('personalCenter.points.history') }}</p>
      </div>

      <div v-if="ledgerLoading" class="space-y-3 px-5 py-4">
        <div v-for="i in 3" :key="i" class="h-12 animate-pulse rounded-xl bg-muted/60"></div>
      </div>

      <div v-else-if="ledger.length === 0" class="px-5 py-8 text-center text-sm text-muted-foreground">
        {{ t('personalCenter.points.noRecords') }}
      </div>

      <div v-else class="divide-y divide-border">
        <div v-for="entry in ledger" :key="entry.id" class="flex items-center justify-between px-5 py-3">
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium text-foreground">{{ pointsLedgerLabel(t, entry.action_type, entry.reason) }}</p>
            <p class="mt-0.5 text-xs text-muted-foreground">{{ formatDate(entry.created_at) }}</p>
          </div>
          <span
            class="shrink-0 font-mono text-sm font-semibold tabular-nums"
            :class="entry.amount >= 0 ? 'text-success' : 'text-muted-foreground'"
          >
            {{ entry.amount >= 0 ? '+' : '' }}{{ entry.amount }}
          </span>
        </div>
      </div>

      <div v-if="!ledgerLoading && ledgerPagination.total_page > 1" class="border-t px-5 py-2.5">
        <PaginationNav
          :current-page="ledgerPagination.page"
          :total-pages="ledgerPagination.total_page"
          :loading="ledgerLoading"
          :scroll-top="false"
          @change-page="loadLedger"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, CalendarCheck, Coins, ShoppingBag, ReceiptText, ChevronRight } from 'lucide-vue-next'
import { checkinAPI, pointsAPI, type PointsAccountData, type PointsLedgerEntry, type CheckinStatusData } from '../../api'
import { pointsLedgerLabel } from '../../utils/status'
import { toast } from '../../composables/useToast'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()

const entryShortcuts = [
  { to: '/me/points/mall', label: 'personalCenter.points.mallEntry', icon: ShoppingBag },
  { to: '/me/points/orders', label: 'personalCenter.points.ordersEntry', icon: ReceiptText },
]

const account = ref<PointsAccountData>({ balance: 0, total_earned: 0, total_spent: 0 })

const checkinStatus = ref<CheckinStatusData | null>(null)
const checkinHistory = ref<string[]>([])
const checkinSubmitting = ref(false)

const ledger = ref<PointsLedgerEntry[]>([])
const ledgerLoading = ref(true)
const ledgerPagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const loadAccount = async () => {
  try {
    const response = await pointsAPI.account()
    account.value = response.data.data || account.value
  } catch {
    /* 静默：概览数据失败不阻塞其余面板 */
  }
}

const loadCheckin = async () => {
  try {
    const response = await checkinAPI.status()
    checkinStatus.value = response.data.data || null
    if (checkinStatus.value?.enabled) {
      const historyResponse = await checkinAPI.history()
      checkinHistory.value = historyResponse.data.data?.checked_dates || []
    }
  } catch {
    checkinStatus.value = null
  }
}

const loadLedger = async (page = 1) => {
  ledgerLoading.value = true
  try {
    const response = await pointsAPI.ledger({ page, page_size: ledgerPagination.value.page_size })
    ledger.value = response.data.data || []
    ledgerPagination.value = response.data.pagination || ledgerPagination.value
  } catch {
    ledger.value = []
  } finally {
    ledgerLoading.value = false
  }
}

const doCheckin = async () => {
  if (checkinSubmitting.value) return
  checkinSubmitting.value = true
  try {
    const response = await checkinAPI.checkIn()
    const result = response.data.data
    if (result && !result.already_checked_in) {
      toast.success(t('personalCenter.points.checkinSuccess', { points: result.points_awarded }))
    }
    account.value = { ...account.value, balance: result?.current_balance ?? account.value.balance }
    await Promise.all([loadCheckin(), loadLedger(1)])
  } catch (err: any) {
    toast.error(err?.message || t('personalCenter.points.checkinFailed'))
  } finally {
    checkinSubmitting.value = false
  }
}

const formatCheckinDay = (date: string) => date.slice(8)

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

onMounted(() => {
  void Promise.all([loadAccount(), loadCheckin(), loadLedger(1)])
})
</script>
