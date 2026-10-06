<template>
  <div class="space-y-6">
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <PanelHeading :title="t('personalCenter.affiliate.title')" :description="t('personalCenter.affiliate.subtitle')" :icon="Megaphone">
        <template #actions>
          <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.affiliate') }}</Badge>
        </template>
      </PanelHeading>

      <Alert v-if="panelAlert" class="mb-5" :variant="pageAlertVariant(panelAlert.level)" :class="pageAlertToneClass(panelAlert.level)">
        <AlertDescription>{{ panelAlert.message }}</AlertDescription>
      </Alert>

      <div v-if="loading" class="space-y-3">
        <div v-for="idx in 3" :key="idx" class="h-16 animate-pulse rounded-xl border bg-muted"></div>
      </div>

      <template v-else-if="dashboard?.opened">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div class="rounded-xl border p-4 md:col-span-2">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.affiliateCode') }}</div>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <span class="rounded-lg border border-border bg-muted/30 px-2 py-1 font-mono text-sm text-foreground">{{ dashboard?.affiliate_code || '-' }}</span>
              <Button type="button" variant="outline" size="sm" @click="copyPromotionUrl">
                {{ t('personalCenter.affiliate.copyPromotionUrl') }}
              </Button>
            </div>
            <div class="mt-3 text-xs text-muted-foreground break-all">{{ promotionUrl }}</div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.conversionRate') }}</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ conversionRateText }}</div>
            <div class="mt-2 text-xs text-muted-foreground">
              {{ t('personalCenter.affiliate.conversionDetail', { clicks: dashboard?.click_count || 0, orders: dashboard?.valid_order_count || 0 }) }}
            </div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.stats.total') }}</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ totalCommissionText }} <span class="text-xs font-normal text-muted-foreground">USDT</span></div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.affiliate.stats.pending') }}</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ dashboard?.pending_commission || '0.00' }} <span class="text-xs font-normal text-muted-foreground">USDT</span></div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">可划转</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ availableTransferBalance }} <span class="text-xs font-normal text-muted-foreground">USDT</span></div>
          </div>
          <div class="rounded-xl border p-4">
            <div class="text-xs text-muted-foreground">已划转</div>
            <div class="mt-2 text-lg font-bold text-foreground">{{ transferredAmount }} <span class="text-xs font-normal text-muted-foreground">USDT</span></div>
          </div>
          <div v-if="hasDebt" class="rounded-xl border border-amber-300 bg-amber-50 p-4">
            <div class="text-xs text-amber-700">佣金欠款</div>
            <div class="mt-2 text-lg font-bold text-amber-700">{{ debtAmount }} <span class="text-xs font-normal">USDT</span></div>
          </div>
        </div>
      </template>

      <div v-else class="rounded-xl border border-dashed p-5">
        <!-- 未申请 -->
        <template v-if="affiliateStatus === 'not_applied'">
          <p class="text-sm text-muted-foreground">{{ t('personalCenter.affiliate.notOpened') }}</p>
          <Button type="button" :disabled="applying" class="mt-4 font-bold" @click="applyAffiliate">
            {{ applying ? '申请中...' : '申请推广' }}
          </Button>
        </template>
        <!-- 审核中 -->
        <template v-else-if="affiliateStatus === 'pending'">
          <div class="flex items-center gap-2">
            <div class="h-2 w-2 animate-pulse rounded-full bg-amber-500"></div>
            <p class="text-sm font-medium text-amber-700">申请审核中</p>
          </div>
          <p class="mt-2 text-xs text-muted-foreground">您的推广申请正在等待管理员审核，审核通过后将自动开通。</p>
          <p class="mt-1 text-xs text-muted-foreground">申请时间：{{ formatDate(application?.created_at) }}</p>
        </template>
        <!-- 已拒绝 -->
        <template v-else-if="affiliateStatus === 'rejected'">
          <p class="text-sm font-medium text-red-600">申请未通过</p>
          <p v-if="application?.review_note" class="mt-1 text-xs text-muted-foreground">拒绝原因：{{ application.review_note }}</p>
          <p class="mt-1 text-xs text-muted-foreground">拒绝时间：{{ formatDate(application?.reviewed_at || undefined) }}</p>
          <Button type="button" :disabled="applying" class="mt-4 font-bold" @click="applyAffiliate">
            {{ applying ? '申请中...' : '重新申请' }}
          </Button>
        </template>
        <!-- 已禁用 -->
        <template v-else-if="affiliateStatus === 'disabled'">
          <p class="text-sm font-medium text-red-600">推广资格已禁用</p>
          <p class="mt-1 text-xs text-muted-foreground">您的推广账号已被管理员禁用，如需恢复请联系客服。</p>
        </template>
        <!-- 兜底 -->
        <template v-else>
          <p class="text-sm text-muted-foreground">{{ t('personalCenter.affiliate.notOpened') }}</p>
          <Button type="button" :disabled="applying" class="mt-4 font-bold" @click="applyAffiliate">
            {{ applying ? '申请中...' : '申请推广' }}
          </Button>
        </template>
      </div>
    </div>

    <!-- 划转表单 -->
    <div v-if="dashboard?.opened" class="rounded-2xl border bg-card p-7 shadow-sm">
      <h3 class="text-lg font-bold text-foreground">划转到钱包</h3>
      <p class="mt-1 text-sm text-muted-foreground">将推广佣金划转到主钱包，然后在「我的钱包」统一提现</p>

      <Alert v-if="hasDebt" variant="default" class="mt-4 border-amber-300 bg-amber-50 text-amber-800">
        <AlertDescription>
          佣金欠款：{{ debtAmount }} USDT（新佣金将优先抵扣欠款）
        </AlertDescription>
      </Alert>

      <div class="mt-4 grid grid-cols-1 gap-2 text-sm md:grid-cols-2">
        <div class="text-muted-foreground">可划转佣金：<span class="font-mono font-semibold text-foreground">{{ availableTransferBalance }} USDT</span></div>
        <div class="text-muted-foreground">累计已划转：<span class="font-mono font-semibold text-foreground">{{ transferredAmount }} USDT</span></div>
      </div>

      <form class="mt-5 grid grid-cols-1 gap-4 md:grid-cols-4" @submit.prevent="handleTransfer">
        <div>
          <Label class="mb-2 block">划转金额</Label>
          <Input
            v-model="transferForm.amount"
            type="text"
            inputmode="decimal"
            class="h-11"
            placeholder="输入划转金额"
          />
        </div>
        <div>
          <Label class="mb-2 block">&nbsp;</Label>
          <Button type="button" variant="outline" class="h-11 w-full" :disabled="submittingTransfer || hasDebt" @click="handleTransferAll">
            全部划转
          </Button>
        </div>
        <div class="md:col-span-2 flex items-end">
          <Button type="submit" :disabled="submittingTransfer || hasDebt" class="h-11 w-full px-5 font-bold">
            {{ submittingTransfer ? '划转中...' : '确认划转' }}
          </Button>
        </div>
      </form>
    </div>

    <div v-if="dashboard?.opened" class="rounded-2xl border bg-card p-7 shadow-sm">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-lg font-bold text-foreground">{{ t('personalCenter.affiliate.commissionTitle') }}</h3>
        <Button type="button" variant="outline" size="sm" @click="loadCommissions(commissionsPagination.page)">
          {{ t('orders.filters.refresh') }}
        </Button>
      </div>

      <div v-if="commissionsLoading" class="space-y-3">
        <div v-for="idx in 3" :key="idx" class="h-14 animate-pulse rounded-xl border bg-muted"></div>
      </div>
      <div v-else-if="commissions.length === 0" class="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
        {{ t('personalCenter.affiliate.commissionEmpty') }}
      </div>
      <div v-else class="overflow-x-auto rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted/50">
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.level') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.orderNo') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.amount') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.status') }}</TableHead>
              <TableHead class="px-4">{{ t('personalCenter.affiliate.table.createdAt') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="item in commissions" :key="item.id">
              <TableCell class="px-4">
                <Badge variant="accent" size="sm">{{ levelBadgeText(item.level) }}</Badge>
              </TableCell>
              <TableCell class="px-4 font-mono text-xs text-foreground">-</TableCell>
              <TableCell class="px-4 font-mono text-xs text-foreground">{{ formatUsdt(item.commission_amount, item.currency || 'USDT') }}</TableCell>
              <TableCell class="px-4">
                <Badge :variant="commissionStatusVariant(item.status)" size="sm">
                  {{ commissionStatusLabel(item.status) }}
                </Badge>
              </TableCell>
              <TableCell class="px-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <PaginationNav
        :current-page="commissionsPagination.page"
        :total-pages="commissionsPagination.total_page"
        :loading="commissionsLoading"
        :scroll-top="false"
        @change-page="loadCommissions"
      />
    </div>

    <!-- 划转记录 -->
    <div v-if="dashboard?.opened" class="rounded-2xl border bg-card p-7 shadow-sm">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-lg font-bold text-foreground">划转记录</h3>
        <Button type="button" variant="outline" size="sm" @click="loadTransfers(transfersPagination.page)">
          {{ t('orders.filters.refresh') }}
        </Button>
      </div>

      <div v-if="transfersLoading" class="space-y-3">
        <div v-for="idx in 3" :key="idx" class="h-14 animate-pulse rounded-xl border bg-muted"></div>
      </div>
      <div v-else-if="transfers.length === 0" class="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
        暂无划转记录
      </div>
      <div v-else class="overflow-x-auto rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted/50">
              <TableHead class="px-4">金额</TableHead>
              <TableHead class="px-4">时间</TableHead>
              <TableHead class="px-4">参考号</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="item in transfers" :key="item.id">
              <TableCell class="px-4 font-mono text-xs text-foreground">+{{ item.amount }} USDT → 钱包</TableCell>
              <TableCell class="px-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
              <TableCell class="px-4 font-mono text-xs text-muted-foreground">{{ shortReference(item.reference) }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <PaginationNav
        :current-page="transfersPagination.page"
        :total-pages="transfersPagination.total_page"
        :loading="transfersLoading"
        :scroll-top="false"
        @change-page="loadTransfers"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Megaphone } from 'lucide-vue-next'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { affiliateAPI, type AffiliateApplicationData, type AffiliateCommissionData, type AffiliateDashboardData, type AffiliateTransferRecord } from '../../api'
import {
  AFFILIATE_COMMISSION_STATUS_AVAILABLE,
  AFFILIATE_COMMISSION_STATUS_PENDING_CONFIRM,
  AFFILIATE_COMMISSION_STATUS_REJECTED,
  AFFILIATE_COMMISSION_STATUS_WITHDRAWN,
} from '../../constants/affiliate'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import { formatUsdt } from '../../utils/money'
import type { BadgeTone } from '../../utils/status'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()

const loading = ref(true)
const applying = ref(false)
const submittingTransfer = ref(false)
const commissionsLoading = ref(false)
const transfersLoading = ref(false)
const dashboard = ref<AffiliateDashboardData | null>(null)
const application = ref<AffiliateApplicationData | null>(null)
const profileStatus = ref<string | null>(null) // active / disabled / null
const panelAlert = ref<PageAlert | null>(null)

const commissions = ref<AffiliateCommissionData[]>([])
const transfers = ref<AffiliateTransferRecord[]>([])

const commissionsPagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const transfersPagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const transferForm = reactive({
  amount: '',
})

const promotionUrl = computed(() => {
  if (!dashboard.value?.affiliate_code) return '-'
  const path = dashboard.value.promotion_path || `/?aff=${dashboard.value.affiliate_code}`
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}${path}`
})

const conversionRateText = computed(() => {
  const value = Number(dashboard.value?.conversion_rate || 0)
  if (!Number.isFinite(value)) return '0.00%'
  return `${value.toFixed(2)}%`
})

// 可划转余额：优先用 available_transfer_balance，fallback 到 available_commission
const availableTransferBalance = computed(() => {
  const raw = dashboard.value?.available_transfer_balance || dashboard.value?.available_commission || '0'
  const n = Number(raw)
  return Number.isFinite(n) ? n.toFixed(2) : '0.00'
})

// 已划转金额：优先用 transferred_amount，fallback 到 withdrawn_commission
const transferredAmount = computed(() => {
  const raw = dashboard.value?.transferred_amount || dashboard.value?.withdrawn_commission || '0'
  const n = Number(raw)
  return Number.isFinite(n) ? n.toFixed(2) : '0.00'
})

// 欠款
const debtAmount = computed(() => {
  const raw = dashboard.value?.debt_amount || '0'
  const n = Number(raw)
  return Number.isFinite(n) ? n.toFixed(2) : '0.00'
})

const hasDebt = computed(() => Number(dashboard.value?.debt_amount || 0) > 0)

// Affiliate 统一状态：not_applied / pending / rejected / active / disabled
const affiliateStatus = computed<'not_applied' | 'pending' | 'rejected' | 'active' | 'disabled'>(() => {
  if (dashboard.value?.opened) return 'active'
  if (profileStatus.value === 'disabled') return 'disabled'
  if (application.value?.status === 'pending') return 'pending'
  if (application.value?.status === 'rejected') return 'rejected'
  return 'not_applied'
})

// 总返利 = 待确认 + 可划转 + 已划转
const totalCommissionText = computed(() => {
  const pending = Number(dashboard.value?.pending_commission || 0)
  const available = Number(availableTransferBalance.value || 0)
  const transferred = Number(transferredAmount.value || 0)
  const sum = [pending, available, transferred].reduce((acc, n) => acc + (Number.isFinite(n) ? n : 0), 0)
  return sum.toFixed(2)
})

const levelBadgeText = (level?: number) => `L${level || 1}`

const formatDate = (raw?: string) => {
  if (!raw) return '-'
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

const shortReference = (ref?: string) => {
  if (!ref) return '-'
  if (ref.length <= 8) return ref
  return '...' + ref.slice(-6)
}

const loadDashboard = async () => {
  try {
    const response = await affiliateAPI.dashboard()
    dashboard.value = response.data.data || null
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.affiliate.errors.loadFailed'),
    }
  }
}

const loadApplication = async () => {
  try {
    const response = await affiliateAPI.application()
    application.value = response.data.data || null
  } catch {
    application.value = null
  }
}

const loadProfile = async () => {
  try {
    const response = await affiliateAPI.profile()
    const profile = response.data.data
    profileStatus.value = profile?.status || null
  } catch {
    profileStatus.value = null
  }
}

const loadCommissions = async (page = 1) => {
  commissionsLoading.value = true
  try {
    const response = await affiliateAPI.commissions({
      page,
      page_size: commissionsPagination.page_size,
    })
    commissions.value = response.data.data || []
    Object.assign(commissionsPagination, response.data.pagination || commissionsPagination)
  } catch {
    commissions.value = []
  } finally {
    commissionsLoading.value = false
  }
}

const loadTransfers = async (page = 1) => {
  transfersLoading.value = true
  try {
    const response = await affiliateAPI.transfers({
      page,
      page_size: transfersPagination.page_size,
    })
    transfers.value = response.data.data || []
    Object.assign(transfersPagination, response.data.pagination || transfersPagination)
  } catch {
    transfers.value = []
  } finally {
    transfersLoading.value = false
  }
}

const reloadOpenedData = async () => {
  if (!dashboard.value?.opened) return
  await Promise.all([loadCommissions(1), loadTransfers(1)])
}

const initialize = async () => {
  loading.value = true
  panelAlert.value = null
  await Promise.all([loadDashboard(), loadApplication(), loadProfile()])
  await reloadOpenedData()
  loading.value = false
}

const applyAffiliate = async () => {
  applying.value = true
  panelAlert.value = null
  try {
    await affiliateAPI.apply()
    await Promise.all([loadApplication(), loadProfile()])
    panelAlert.value = {
      level: 'success',
      message: '申请已提交，等待管理员审核',
    }
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || '申请失败，请稍后重试'
    panelAlert.value = {
      level: 'error',
      message: msg,
    }
  } finally {
    applying.value = false
  }
}

const handleTransfer = async () => {
  panelAlert.value = null
  const amount = transferForm.amount.trim()
  if (!amount) {
    panelAlert.value = {
      level: 'warning',
      message: '请输入划转金额',
    }
    return
  }
  const numAmount = Number(amount)
  if (!Number.isFinite(numAmount) || numAmount <= 0) {
    panelAlert.value = {
      level: 'warning',
      message: '划转金额必须大于 0',
    }
    return
  }

  submittingTransfer.value = true
  try {
    await affiliateAPI.transferToWallet({ amount })
    transferForm.amount = ''
    panelAlert.value = {
      level: 'success',
      message: '划转成功',
    }
    await Promise.all([loadDashboard(), loadTransfers(1)])
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || '划转失败，请稍后重试',
    }
  } finally {
    submittingTransfer.value = false
  }
}

const handleTransferAll = async () => {
  panelAlert.value = null
  submittingTransfer.value = true
  try {
    await affiliateAPI.transferToWallet({ all: true })
    transferForm.amount = ''
    panelAlert.value = {
      level: 'success',
      message: '划转成功',
    }
    await Promise.all([loadDashboard(), loadTransfers(1)])
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || '划转失败，请稍后重试',
    }
  } finally {
    submittingTransfer.value = false
  }
}

const copyPromotionUrl = async () => {
  if (!dashboard.value?.affiliate_code || !promotionUrl.value || promotionUrl.value === '-') return
  try {
    await navigator.clipboard.writeText(promotionUrl.value)
    panelAlert.value = {
      level: 'success',
      message: t('personalCenter.affiliate.copySuccess'),
    }
  } catch {
    panelAlert.value = {
      level: 'error',
      message: t('personalCenter.affiliate.errors.copyFailed'),
    }
  }
}

const commissionStatusLabel = (status?: string) => {
  if (status === AFFILIATE_COMMISSION_STATUS_PENDING_CONFIRM) return t('personalCenter.affiliate.commissionStatus.pendingConfirm')
  if (status === AFFILIATE_COMMISSION_STATUS_AVAILABLE) return t('personalCenter.affiliate.commissionStatus.available')
  if (status === AFFILIATE_COMMISSION_STATUS_REJECTED) return t('personalCenter.affiliate.commissionStatus.rejected')
  if (status === AFFILIATE_COMMISSION_STATUS_WITHDRAWN) return t('personalCenter.affiliate.commissionStatus.withdrawn')
  return status || '-'
}

const commissionStatusVariant = (status?: string): BadgeTone => {
  if (status === AFFILIATE_COMMISSION_STATUS_PENDING_CONFIRM) return 'warning'
  if (status === AFFILIATE_COMMISSION_STATUS_AVAILABLE) return 'success'
  if (status === AFFILIATE_COMMISSION_STATUS_WITHDRAWN) return 'info'
  return 'neutral'
}

onMounted(() => {
  initialize()
})
</script>
