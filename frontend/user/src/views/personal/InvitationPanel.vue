<template>
  <div class="space-y-4 pb-8">
    <!-- 统一页头：邀请中心（唯一 Page Header / Page Container） -->
    <div class="mb-4">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('personalCenter.invitation.title') }}</h1>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('personalCenter.invitation.subtitle') }}</p>
    </div>

    <!-- 操作 / 加载反馈 -->
    <Alert v-if="panelAlert" :variant="pageAlertVariant(panelAlert.level)" :class="pageAlertToneClass(panelAlert.level)">
      <AlertDescription>{{ panelAlert.message }}</AlertDescription>
    </Alert>

    <!-- 加载中 -->
    <div v-if="loading" class="space-y-3">
      <div v-for="idx in 3" :key="idx" class="h-16 animate-pulse rounded-2xl border bg-muted"></div>
    </div>

    <!-- 加载失败（邀请主数据是页面地基，失败时提供重试） -->
    <Alert v-else-if="inviteError" variant="destructive">
      <AlertDescription>{{ inviteError }}</AlertDescription>
      <div class="mt-3">
        <Button size="sm" variant="outline" @click="initialize">
          {{ t('personalCenter.common.loadRetry') }}
        </Button>
      </div>
    </Alert>

    <template v-else>
      <!-- 1. 邀请卡：邀请码 / 邀请链接 / 分享 -->
      <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
        <div class="flex min-h-[56px] items-center gap-3 border-b px-5 py-3">
          <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Hash :size="18" :stroke-width="1.8" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{{ t('personalCenter.invitation.myCode') }}</p>
            <p class="mt-0.5 text-lg font-black tracking-[0.15em] text-foreground">{{ inviteCodeText || '—' }}</p>
          </div>
          <button type="button" class="grid h-9 w-9 shrink-0 place-items-center rounded-xl border text-muted-foreground transition-colors hover:bg-accent hover:text-foreground" @click="copy('code', inviteCodeText)">
            <component :is="copiedKey === 'code' ? Check : Copy" :size="16" :stroke-width="1.8" />
          </button>
        </div>

        <div class="flex min-h-[56px] items-center gap-3 border-b px-5 py-3">
          <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Link :size="18" :stroke-width="1.8" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{{ t('personalCenter.invitation.inviteUrl') }}</p>
            <p class="mt-0.5 truncate font-mono text-xs text-foreground">{{ inviteUrlText || '—' }}</p>
          </div>
          <button type="button" class="grid h-9 w-9 shrink-0 place-items-center rounded-xl border text-muted-foreground transition-colors hover:bg-accent hover:text-foreground" @click="copy('url', inviteUrlText)">
            <component :is="copiedKey === 'url' ? Check : Copy" :size="16" :stroke-width="1.8" />
          </button>
          <button type="button" class="grid h-9 w-9 shrink-0 place-items-center rounded-xl border text-muted-foreground transition-colors hover:bg-accent hover:text-foreground" @click="shareInvite">
            <Share2 :size="16" :stroke-width="1.8" />
          </button>
        </div>

        <div v-if="promotionUrl" class="flex min-h-[56px] items-center gap-3 px-5 py-3">
          <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Megaphone :size="18" :stroke-width="1.8" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{{ t('personalCenter.affiliate.affiliateCode') }}</p>
            <p class="mt-0.5 truncate font-mono text-xs text-foreground">{{ dashboard?.affiliate_code || '—' }} · {{ promotionUrl }}</p>
          </div>
          <button type="button" class="grid h-9 w-9 shrink-0 place-items-center rounded-xl border text-muted-foreground transition-colors hover:bg-accent hover:text-foreground" @click="copy('promo', promotionUrl)">
            <component :is="copiedKey === 'promo' ? Check : Copy" :size="16" :stroke-width="1.8" />
          </button>
        </div>
      </div>

      <!-- 2. 数据概览：邀请 + 返利统一展示（所有状态用户都可见真实数据） -->
      <div class="rounded-2xl border bg-card p-5 shadow-sm">
        <h2 class="text-sm font-bold text-foreground">数据概览</h2>
        <div class="mt-4 grid grid-cols-2 gap-2 md:grid-cols-4">
          <div class="rounded-xl border p-3 text-center">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{{ t('personalCenter.invitation.directCount') }}</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums text-foreground">{{ directInviteCount }}</p>
          </div>
          <div class="rounded-xl border p-3 text-center">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">有效订单</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums text-foreground">{{ dashboard?.valid_order_count ?? 0 }}</p>
          </div>
          <div class="rounded-xl border p-3 text-center">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{{ t('personalCenter.affiliate.stats.pending') }}</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums text-foreground">{{ pendingCommission }}<span class="ml-0.5 text-[10px] font-normal text-muted-foreground">USDT</span></p>
          </div>
          <div class="rounded-xl border p-3 text-center">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">可划转</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums text-foreground">{{ availableTransferBalance }}<span class="ml-0.5 text-[10px] font-normal text-muted-foreground">USDT</span></p>
          </div>
          <div class="rounded-xl border p-3 text-center">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">已划转</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums text-foreground">{{ transferredAmount }}<span class="ml-0.5 text-[10px] font-normal text-muted-foreground">USDT</span></p>
          </div>
          <div class="rounded-xl border p-3 text-center">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">总返利</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums text-foreground">{{ totalCommissionText }}<span class="ml-0.5 text-[10px] font-normal text-muted-foreground">USDT</span></p>
          </div>
          <div class="rounded-xl border p-3 text-center" :class="hasDebt ? 'border-amber-300 bg-amber-50' : ''">
            <p class="text-[10px] font-semibold uppercase tracking-wider" :class="hasDebt ? 'text-amber-700' : 'text-muted-foreground'">欠款</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums" :class="hasDebt ? 'text-amber-700' : 'text-foreground'">{{ debtAmount }}<span class="ml-0.5 text-[10px] font-normal text-muted-foreground">USDT</span></p>
          </div>
          <div class="rounded-xl border p-3 text-center">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{{ t('personalCenter.affiliate.conversionRate') }}</p>
            <p class="mt-1.5 text-lg font-bold tabular-nums text-foreground">{{ conversionRateText }}</p>
          </div>
        </div>
      </div>

      <!-- 3. 推广资格：仅未批准时显示（APPROVED+active 时该区域整体退出页面） -->
      <div v-if="!transferEnabled" class="rounded-2xl border bg-card p-5 shadow-sm">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-sm font-bold text-foreground">推广资格</h2>
          <Badge :variant="qualificationBadgeVariant" size="sm">{{ qualificationBadgeText }}</Badge>
        </div>

        <!-- 未申请 -->
        <template v-if="appStatus === 'not_applied' && profileStatus !== 'disabled'">
          <p class="text-sm text-muted-foreground">审核通过后，可将推广返利划转至 USDT 钱包。</p>
          <Button type="button" :disabled="applying" class="mt-4 font-bold" @click="applyAffiliate">
            {{ applying ? '申请中...' : '申请推广资格' }}
          </Button>
        </template>
        <!-- 审核中 -->
        <template v-else-if="appStatus === 'pending'">
          <div class="flex items-center gap-2">
            <span class="h-2 w-2 animate-pulse rounded-full bg-amber-500"></span>
            <p class="text-sm font-medium text-amber-700">推广资格审核中</p>
          </div>
          <p class="mt-2 text-xs text-muted-foreground">返利将继续累计，审核通过后即可划转。</p>
          <p class="mt-1 text-xs text-muted-foreground">申请时间：{{ formatDate(application?.created_at) }}</p>
        </template>
        <!-- 已拒绝 -->
        <template v-else-if="appStatus === 'rejected'">
          <p class="text-sm font-medium text-red-600">申请未通过</p>
          <p v-if="application?.review_note" class="mt-1 text-xs text-muted-foreground">拒绝原因：{{ application.review_note }}</p>
          <p class="mt-1 text-xs text-muted-foreground">已有返利不会受到影响。</p>
          <Button type="button" :disabled="applying" class="mt-4 font-bold" @click="applyAffiliate">
            {{ applying ? '申请中...' : '重新申请' }}
          </Button>
        </template>
        <!-- 已禁用（推广资格暂停；历史数据保留） -->
        <template v-else-if="profileStatus === 'disabled'">
          <p class="text-sm font-medium text-red-600">推广资格已暂停</p>
          <p class="mt-1 text-xs text-muted-foreground">您的推广账号已被管理员暂停，如需恢复请联系客服。历史邀请与返利数据仍可查看。</p>
        </template>
      </div>

      <!-- 4. 资金操作：划转到 USDT 钱包 -->
      <div v-if="transferEnabled" class="rounded-2xl border bg-card p-5 shadow-sm">
        <h2 class="text-sm font-bold text-foreground">划转到 USDT 钱包</h2>
        <p class="mt-1 text-xs text-muted-foreground">将推广佣金划转到主钱包，然后在「我的钱包」统一提现。</p>

        <Alert v-if="hasDebt" variant="default" class="mt-4 border-amber-300 bg-amber-50 text-amber-800">
          <AlertDescription>佣金欠款：{{ debtAmount }} USDT（新佣金将优先抵扣欠款，还清前暂停划转）</AlertDescription>
        </Alert>

        <div class="mt-4 grid grid-cols-1 gap-2 text-sm md:grid-cols-2">
          <div class="text-muted-foreground">可划转佣金：<span class="font-mono font-semibold text-foreground">{{ availableTransferBalance }} USDT</span></div>
          <div class="text-muted-foreground">累计已划转：<span class="font-mono font-semibold text-foreground">{{ transferredAmount }} USDT</span></div>
        </div>

        <form class="mt-5 grid grid-cols-1 gap-4 md:grid-cols-4" @submit.prevent="handleTransfer">
          <div>
            <Label class="mb-2 block">划转金额</Label>
            <Input v-model="transferForm.amount" type="text" inputmode="decimal" class="h-11" placeholder="输入划转金额" />
          </div>
          <div>
            <Label class="mb-2 block">&nbsp;</Label>
            <Button type="button" variant="outline" class="h-11 w-full" :disabled="submittingTransfer || hasDebt" @click="handleTransferAll">全部划转</Button>
          </div>
          <div class="md:col-span-2 flex items-end">
            <Button type="submit" :disabled="submittingTransfer || hasDebt" class="h-11 w-full px-5 font-bold">
              {{ submittingTransfer ? '划转中...' : '确认划转' }}
            </Button>
          </div>
        </form>
      </div>

      <!-- 5. 返利明细：佣金记录 -->
      <div class="rounded-2xl border bg-card p-5 shadow-sm">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-sm font-bold text-foreground">返利明细</h2>
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
                <TableCell class="px-4 font-mono text-xs text-foreground">{{ formatUsdt(item.commission_amount, item.currency || 'USDT') }}</TableCell>
                <TableCell class="px-4">
                  <Badge :variant="commissionStatusVariant(item.status)" size="sm">{{ commissionStatusLabel(item.status) }}</Badge>
                </TableCell>
                <TableCell class="px-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <PaginationNav
            :current-page="commissionsPagination.page"
            :total-pages="commissionsPagination.total_page"
            :loading="commissionsLoading"
            :scroll-top="false"
            @change-page="loadCommissions"
          />
        </div>
      </div>

      <!-- 6. 划转记录 -->
      <div class="rounded-2xl border bg-card p-5 shadow-sm">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-sm font-bold text-foreground">划转记录</h2>
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
          <PaginationNav
            :current-page="transfersPagination.page"
            :total-pages="transfersPagination.total_page"
            :loading="transfersLoading"
            :scroll-top="false"
            @change-page="loadTransfers"
          />
        </div>
      </div>

      <!-- 7. 我的邀请列表（后端 /invitation/invitees 接入后在此展示，不再新建独立页面） -->
      <div class="rounded-2xl border bg-card p-5 shadow-sm">
        <h2 class="text-sm font-bold text-foreground">我的邀请列表</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('personalCenter.invitation.myInviter') }}：{{ invitation?.inviter_display_name || t('personalCenter.invitation.noInviter') }}</p>
        <p class="mt-3 text-xs text-muted-foreground">下级成员明细即将上线，敬请期待。</p>
      </div>

      <!-- 8. 规则 -->
      <div class="rounded-2xl border bg-card p-5 shadow-sm">
        <h2 class="text-sm font-bold text-foreground">规则说明</h2>
        <div class="mt-3 space-y-3 text-xs leading-relaxed text-muted-foreground">
          <div>
            <p class="font-semibold text-foreground">邀请规则</p>
            <p class="mt-1">分享你的邀请码或邀请链接，好友通过链接注册即与你绑定上下级关系。所有注册用户均可邀请好友并累计推广返利。</p>
          </div>
          <div>
            <p class="font-semibold text-foreground">返利规则</p>
            <p class="mt-1">下级完成有效订单后，返利按推广层级自动累计到你的返利账户，待订单确认后变为可划转。提交推广资格并通过审核后，即可将可划转返利一键划转至 USDT 钱包；资格审核为一次性开通，通过后无需重复申请。</p>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, Check, Hash, Link, Share2, Megaphone } from 'lucide-vue-next'
import { invitationAPI, type MyInvitationData } from '../../api'
import {
  affiliateAPI,
  type AffiliateApplicationData,
  type AffiliateCommissionData,
  type AffiliateDashboardData,
  type AffiliateTransferRecord,
} from '../../api'
import {
  AFFILIATE_COMMISSION_STATUS_AVAILABLE,
  AFFILIATE_COMMISSION_STATUS_PENDING_CONFIRM,
  AFFILIATE_COMMISSION_STATUS_REJECTED,
  AFFILIATE_COMMISSION_STATUS_WITHDRAWN,
} from '../../constants/affiliate'
import { copyText } from '../../utils/clipboard'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import { formatUsdt } from '../../utils/money'
import type { BadgeTone } from '../../utils/status'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()

const loading = ref(true)
const panelAlert = ref<PageAlert | null>(null)
const copiedKey = ref<'' | 'code' | 'url' | 'promo'>('')

const invitation = ref<MyInvitationData | null>(null)
const inviteError = ref('')

const dashboard = ref<AffiliateDashboardData | null>(null)
const application = ref<AffiliateApplicationData | null>(null)
const profileStatus = ref<string | null>(null) // active / disabled / null
const applying = ref(false)
const submittingTransfer = ref(false)

const commissions = ref<AffiliateCommissionData[]>([])
const commissionsLoading = ref(false)
const transfers = ref<AffiliateTransferRecord[]>([])
const transfersLoading = ref(false)

const commissionsPagination = reactive({ page: 1, page_size: 20, total: 0, total_page: 1 })
const transfersPagination = reactive({ page: 1, page_size: 20, total: 0, total_page: 1 })
const transferForm = reactive({ amount: '' })

// ---- 邀请派生 ----
const inviteCodeText = computed(() => invitation.value?.invite_code || '')
const inviteUrlText = computed(() => {
  const raw = invitation.value?.invite_url
  if (!raw) return ''
  try {
    return new URL(raw, window.location.origin).href
  } catch {
    return raw
  }
})
const directInviteCount = computed(() => invitation.value?.direct_invite_count ?? 0)

// ---- 返利派生 ----
const appStatus = computed<'not_applied' | 'pending' | 'approved' | 'rejected'>(
  () => (dashboard.value?.application_status as 'not_applied' | 'pending' | 'approved' | 'rejected') || 'not_applied'
)
const transferEnabled = computed(() => Boolean(dashboard.value?.transfer_enabled))

const moneyText = (raw: string | undefined) => {
  const n = Number(raw)
  return Number.isFinite(n) ? n.toFixed(2) : '0.00'
}
const availableTransferBalance = computed(() =>
  moneyText(dashboard.value?.available_transfer_balance || dashboard.value?.available_commission)
)
const transferredAmount = computed(() =>
  moneyText(dashboard.value?.transferred_amount || dashboard.value?.withdrawn_commission)
)
const pendingCommission = computed(() => moneyText(dashboard.value?.pending_commission))
const debtAmount = computed(() => moneyText(dashboard.value?.debt_amount))
const hasDebt = computed(() => Number(dashboard.value?.debt_amount || 0) > 0)
const totalCommissionText = computed(() => {
  const sum = [pendingCommission.value, availableTransferBalance.value, transferredAmount.value]
    .map(Number)
    .reduce((acc, n) => acc + (Number.isFinite(n) ? n : 0), 0)
  return sum.toFixed(2)
})
const conversionRateText = computed(() => {
  const value = Number(dashboard.value?.conversion_rate || 0)
  return Number.isFinite(value) ? `${value.toFixed(2)}%` : '0.00%'
})
const promotionUrl = computed(() => {
  const code = dashboard.value?.affiliate_code
  if (!code) return ''
  const path = dashboard.value?.promotion_path || `/?aff=${code}`
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}${path}`
})

const qualificationBadgeText = computed(() => {
  if (profileStatus.value === 'disabled') return '已暂停'
  if (appStatus.value === 'pending') return '审核中'
  if (appStatus.value === 'rejected') return '未通过'
  return '未申请'
})
const qualificationBadgeVariant = computed<BadgeTone>(() => {
  if (profileStatus.value === 'disabled') return 'danger'
  if (appStatus.value === 'pending') return 'warning'
  if (appStatus.value === 'rejected') return 'danger'
  return 'neutral'
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
  return ref.length <= 8 ? ref : '...' + ref.slice(-6)
}

// ---- 数据加载 ----
const loadInvitation = async () => {
  inviteError.value = ''
  try {
    const response = await invitationAPI.me()
    invitation.value = (response.data.data || null) as MyInvitationData | null
  } catch (err: any) {
    invitation.value = null
    inviteError.value = err?.message || t('personalCenter.invitation.loadFailed')
  }
}

const loadDashboard = async () => {
  try {
    const response = await affiliateAPI.dashboard()
    dashboard.value = response.data.data || null
  } catch {
    // 返利数据加载失败保持非阻断，页面仍展示邀请信息
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
    profileStatus.value = response.data.data?.status || null
  } catch {
    profileStatus.value = null
  }
}

const loadCommissions = async (page = 1) => {
  commissionsLoading.value = true
  try {
    const response = await affiliateAPI.commissions({ page, page_size: commissionsPagination.page_size })
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
    const response = await affiliateAPI.transfers({ page, page_size: transfersPagination.page_size })
    transfers.value = response.data.data || []
    Object.assign(transfersPagination, response.data.pagination || transfersPagination)
  } catch {
    transfers.value = []
  } finally {
    transfersLoading.value = false
  }
}

const initialize = async () => {
  loading.value = true
  panelAlert.value = null
  await loadInvitation()
  await Promise.all([loadDashboard(), loadApplication(), loadProfile()])
  if (dashboard.value?.opened) {
    await Promise.all([loadCommissions(1), loadTransfers(1)])
  }
  loading.value = false
}

// ---- 交互 ----
const copy = async (key: 'code' | 'url' | 'promo', value: string) => {
  if (!value) return
  try {
    await copyText(value)
    copiedKey.value = key
    window.setTimeout(() => {
      if (copiedKey.value === key) copiedKey.value = ''
    }, 1600)
  } catch {
    panelAlert.value = { level: 'error', message: t('personalCenter.affiliate.errors.copyFailed') }
  }
}

const shareInvite = async () => {
  const url = inviteUrlText.value
  if (!url) return
  if (typeof navigator !== 'undefined' && navigator.share) {
    try {
      await navigator.share({ title: t('personalCenter.invitation.title'), text: t('personalCenter.invitation.inviteUrl'), url })
      return
    } catch {
      // 用户取消或环境不支持时回退到复制链接
    }
  }
  await copy('url', url)
}

const applyAffiliate = async () => {
  applying.value = true
  panelAlert.value = null
  try {
    await affiliateAPI.apply()
    await Promise.all([loadApplication(), loadProfile()])
    panelAlert.value = { level: 'success', message: '申请已提交，等待管理员审核' }
  } catch (err: any) {
    panelAlert.value = { level: 'error', message: err?.response?.data?.message || err?.message || '申请失败，请稍后重试' }
  } finally {
    applying.value = false
  }
}

const runTransfer = async (payload: { amount?: string; all?: boolean }) => {
  panelAlert.value = null
  submittingTransfer.value = true
  try {
    await affiliateAPI.transferToWallet(payload)
    transferForm.amount = ''
    panelAlert.value = { level: 'success', message: '划转成功' }
    await Promise.all([loadDashboard(), loadTransfers(1)])
  } catch (err: any) {
    panelAlert.value = { level: 'error', message: err?.message || '划转失败，请稍后重试' }
  } finally {
    submittingTransfer.value = false
  }
}

const handleTransfer = async () => {
  const amount = transferForm.amount.trim()
  if (!amount) {
    panelAlert.value = { level: 'warning', message: '请输入划转金额' }
    return
  }
  const numAmount = Number(amount)
  if (!Number.isFinite(numAmount) || numAmount <= 0) {
    panelAlert.value = { level: 'warning', message: '划转金额必须大于 0' }
    return
  }
  await runTransfer({ amount })
}

const handleTransferAll = async () => {
  await runTransfer({ all: true })
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

onMounted(initialize)
</script>
