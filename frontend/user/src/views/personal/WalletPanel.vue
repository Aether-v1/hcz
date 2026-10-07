<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 + 账单入口 -->
    <div class="mb-2 flex items-center justify-between">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('nav.wallet') }}</h1>
      <router-link
        to="/me/wallet/transactions"
        class="flex items-center gap-1.5 rounded-full border bg-card px-3 py-1.5 text-xs font-medium text-muted-foreground shadow-sm transition-colors hover:text-foreground"
      >
        <ReceiptText :size="14" :stroke-width="1.8" />
        {{ t('personalCenter.wallet.bills') }}
      </router-link>
    </div>

    <!-- 余额卡 -->
    <WalletBalanceCard
      :alert="walletAlert"
      :total-balance="totalBalance"
      :available-balance="availableBalance"
      :frozen-balance="frozenBalance"
      :currency="walletCurrency"
      :frozen-note="frozenNote"
      :error="walletError"
      :loading="walletLoading"
      @retry="loadWallet"
    />

    <!-- 快捷操作：余额充值 / 申请提现（单行） -->
    <div class="flex gap-2">
      <button
        type="button"
        class="flex flex-1 items-center gap-3 rounded-2xl border bg-card px-4 py-3 shadow-sm transition-colors hover:bg-accent/40"
        @click="onRechargeClick"
      >
        <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
          <Plus :size="18" :stroke-width="1.8" />
        </div>
        <span class="text-sm font-medium text-foreground">{{ t('personalCenter.wallet.rechargeTitle') }}</span>
      </button>
      <button
        type="button"
        class="flex flex-1 items-center gap-3 rounded-2xl border bg-card px-4 py-3 shadow-sm transition-colors hover:bg-accent/40"
        @click="openWithdrawSheet"
      >
        <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
          <ArrowUpRight :size="18" :stroke-width="1.8" />
        </div>
        <span class="text-sm font-medium text-foreground">{{ t('personalCenter.wallet.withdraw.formTitle') }}</span>
      </button>
    </div>

    <!-- 收款方式入口 -->
    <button
      type="button"
      class="flex w-full items-center gap-3 rounded-2xl border bg-card px-4 py-3 shadow-sm transition-colors hover:bg-accent/40"
      @click="goPaymentMethods"
    >
      <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
        <CreditCard :size="18" :stroke-width="1.8" />
      </div>
      <div class="min-w-0 flex-1 text-left">
        <div class="text-sm font-medium text-foreground">{{ t('paymentMethods.pageTitle') }}</div>
        <div class="mt-0.5 truncate text-xs text-muted-foreground">{{ t('paymentMethods.pageSubtitle') }}</div>
      </div>
      <ChevronRight :size="18" :stroke-width="1.8" class="shrink-0 text-muted-foreground" />
    </button>

    <!-- 未绑定 TRC20 提示 -->
    <div
      v-if="trc20Bound === false"
      class="flex items-center justify-between gap-3 rounded-2xl border border-amber-500/40 bg-amber-500/10 px-4 py-3"
    >
      <p class="text-sm text-amber-600">{{ t('paymentMethods.notBoundTip') }}</p>
      <button
        type="button"
        class="shrink-0 text-sm font-medium text-amber-600 hover:underline"
        @click="goPaymentMethods"
      >
        {{ t('paymentMethods.goBind') }}
      </button>
    </div>

    <!-- 充值弹窗 -->
    <Teleport to="body">
      <Transition name="sheet">
        <div v-if="showRechargeSheet" class="fixed inset-0 z-50">
          <div class="absolute inset-0 bg-black/50" @click="closeRechargeSheet"></div>
          <div class="absolute bottom-0 left-0 right-0 flex justify-center">
            <div
              class="sheet-panel w-full max-w-3xl max-h-[85vh] overflow-y-auto overscroll-contain rounded-t-3xl bg-background shadow-2xl transform-gpu sm:mb-6 sm:rounded-3xl"
              :style="sheetDragStyle"
              @touchstart="onSheetTouchStart"
              @touchmove="onSheetTouchMove"
              @touchend="onSheetTouchEnd"
              @mousedown="onSheetMouseDown"
            >
              <div class="sticky top-0 z-10 border-b bg-background">
                <div class="mx-auto mt-2 h-1 w-10 rounded-full bg-muted-foreground/20"></div>
                <div class="px-5 py-2.5 text-center text-base font-semibold text-foreground">{{ t('personalCenter.wallet.rechargeTitle') }}</div>
              </div>
              <div class="p-5 pb-8">
                <WalletRechargeForm
                  :amount="rechargeForm.amount"
                  :channel-id="rechargeForm.channelId"
                  :remark="rechargeForm.remark"
                  :currency="selectedChannelCurrency"
                  :channels="channels"
                  :has-channels="hasChannels"
                  :recharging="recharging"
                  :channel-loading="channelLoading"
                  :selected-channel="selectedChannel"
                  :fee-rate-display="selectedChannelFeeRateDisplay"
                  :fixed-fee-display="selectedChannelFixedFeeDisplay"
                  :fee-amount-display="selectedChannelFeeAmountDisplay"
                  @update:amount="rechargeForm.amount = $event"
                  @update:channel-id="rechargeForm.channelId = $event"
                  @update:remark="rechargeForm.remark = $event"
                  @submit="handleRecharge"
                />
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- 提现弹窗 -->
    <Teleport to="body">
      <Transition name="sheet">
        <div v-if="showWithdrawSheet" class="fixed inset-0 z-50">
          <div class="absolute inset-0 bg-black/50" @click="closeWithdrawSheet"></div>
          <div class="absolute bottom-0 left-0 right-0 flex justify-center">
            <div
              class="sheet-panel w-full max-w-3xl max-h-[85vh] overflow-y-auto overscroll-contain rounded-t-3xl bg-background shadow-2xl transform-gpu sm:mb-6 sm:rounded-3xl"
              :style="sheetDragStyle"
              @touchstart="onSheetTouchStart"
              @touchmove="onSheetTouchMove"
              @touchend="onSheetTouchEnd"
              @mousedown="onSheetMouseDown"
            >
              <div class="sticky top-0 z-10 border-b bg-background">
                <div class="mx-auto mt-2 h-1 w-10 rounded-full bg-muted-foreground/20"></div>
                <div class="px-5 py-2.5 text-center text-base font-semibold text-foreground">{{ t('personalCenter.wallet.withdraw.formTitle') }}</div>
              </div>
              <div class="p-5 pb-8">
                <WalletWithdrawal />
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- 未绑定 TRC20 引导弹窗 -->
    <Teleport to="body">
      <Transition name="sheet">
        <div v-if="showBindTip" class="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div class="absolute inset-0 bg-black/50" @click="showBindTip = false"></div>
          <div class="relative z-10 w-full max-w-sm rounded-3xl bg-background p-6 shadow-2xl">
            <div class="grid h-11 w-11 place-items-center rounded-2xl bg-accent text-muted-foreground">
              <Coins :size="22" :stroke-width="1.8" />
            </div>
            <h3 class="mt-4 text-base font-semibold text-foreground">{{ t('paymentMethods.bindTipTitle') }}</h3>
            <p class="mt-1.5 text-sm text-muted-foreground">{{ t('paymentMethods.bindTipDesc') }}</p>
            <div class="mt-5 flex gap-3">
              <button
                type="button"
                class="flex-1 rounded-lg border px-4 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-accent"
                @click="showBindTip = false"
              >
                {{ t('paymentMethods.bindTipCancel') }}
              </button>
              <button
                type="button"
                class="flex-1 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
                @click="goBindFromTip"
              >
                {{ t('paymentMethods.goBind') }}
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, ChevronRight, Coins, CreditCard, Plus, ReceiptText } from 'lucide-vue-next'
import { walletAPI } from '../../api'
import { paymentMethodsAPI, parsePaymentMethodList } from '../../api/paymentMethods'
import { useAppStore } from '../../stores/app'
import type { PageAlert } from '../../utils/alerts'
import { amountToCents, basisPointsToPercent, calculateFeeCents, centsToAmount, rateToBasisPoints } from '../../utils/money'
import WalletBalanceCard from '../../components/wallet/WalletBalanceCard.vue'
import WalletRechargeForm from '../../components/wallet/WalletRechargeForm.vue'
import WalletWithdrawal from './WalletWithdrawal.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const recharging = ref(false)
const showRechargeSheet = ref(false)
const showWithdrawSheet = ref(false)
const wallet = ref<any>(null)
const walletError = ref(false)
const walletLoading = ref(false)
const walletAlert = ref<PageAlert | null>(null)
const channels = ref<any[]>([])
const channelFetchTimer = ref<number | null>(null)
const channelFetchSeq = ref(0)
const channelLoading = ref(false)
const channelsResolvedAmount = ref('')

// 收款方式 / TRC20 绑定状态
const trc20Bound = ref<boolean | null>(null)
const showBindTip = ref(false)

const rechargeForm = reactive({
  amount: '',
  channelId: 0,
  remark: '',
})

const hasChannels = computed(() => {
  const amount = rechargeForm.amount.trim()
  const amountCents = amountToCents(amount)
  if (!amount || amountCents === null || amountCents <= 0) {
    return true
  }
  if (channelLoading.value) {
    return true
  }
  if (channelsResolvedAmount.value !== amount) {
    return true
  }
  return channels.value.length > 0
})

const selectedChannel = computed(() => channels.value.find((item: any) => item.id === rechargeForm.channelId) || null)

const EPAY_ALLOWED_CHANNEL_TYPES = new Set(['wechat', 'wxpay', 'alipay', 'qqpay'])

const channelLimitMeta = (channel?: any) => {
  const minCents = amountToCents(String(channel?.min_amount ?? ''))
  const maxCents = amountToCents(String(channel?.max_amount ?? ''))
  return {
    minCents,
    maxCents,
    hasMin: minCents !== null && minCents > 0,
    hasMax: maxCents !== null && maxCents > 0,
  }
}

const isSupportedEpayChannel = (channel: any) => {
  const providerType = String(channel?.provider_type || '').toLowerCase()
  if (providerType !== 'epay') return true
  const channelType = String(channel?.channel_type || '').toLowerCase()
  return EPAY_ALLOWED_CHANNEL_TYPES.has(channelType)
}

const isChannelOutOfRange = (channel: any, targetAmountCents: number) => {
  const meta = channelLimitMeta(channel)
  if (!meta.hasMin && !meta.hasMax) return false
  const lessThanMin = meta.hasMin && meta.minCents !== null && targetAmountCents < meta.minCents
  const greaterThanMax = meta.hasMax && meta.maxCents !== null && targetAmountCents > meta.maxCents
  return Boolean(lessThanMin || greaterThanMax)
}

const shouldHideChannelForAmount = (channel: any, targetAmountCents: number) => {
  if (!Boolean(channel?.hide_amount_out_range)) return false
  return isChannelOutOfRange(channel, targetAmountCents)
}

const getAllowedChannelIdSet = () => {
  const allowedIds = appStore.config?.wallet_recharge_channel_ids
  if (!Array.isArray(allowedIds) || allowedIds.length === 0) return null
  return new Set(allowedIds.map(Number))
}

const isChannelAllowedByConfig = (channel: any, allowedIdSet: Set<number> | null) => {
  if (!allowedIdSet) return true
  return allowedIdSet.has(Number(channel?.id))
}

const mapChannel = (channel: any) => ({
  id: Number(channel.id),
  name: String(channel.name || channel.channel_type || channel.id),
  channel_type: String(channel.channel_type || ''),
  fee_policy: String(channel.fee_policy || ''),
  fee_rate: String(channel.fee_rate ?? '0'),
  fixed_fee: String(channel.fixed_fee ?? '0'),
  min_amount: String(channel.min_amount ?? '0'),
  max_amount: String(channel.max_amount ?? '0'),
  hide_amount_out_range: Boolean(channel.hide_amount_out_range),
})

const normalizeChannels = (list: any[], targetAmountCents: number) => {
  const allowedIdSet = getAllowedChannelIdSet()
  return list
    .filter((channel: any) => isSupportedEpayChannel(channel))
    .filter((channel: any) => !shouldHideChannelForAmount(channel, targetAmountCents))
    .filter((channel: any) => isChannelAllowedByConfig(channel, allowedIdSet))
    .map(mapChannel)
    .filter((channel: any) => Number.isFinite(channel.id) && channel.id > 0)
}

const getSelectedChannelAmountHint = (channel: any, amountCents: number) => {
  if (!isChannelOutOfRange(channel, amountCents)) return ''

  const meta = channelLimitMeta(channel)
  if (meta.hasMin && meta.hasMax && meta.minCents !== null && meta.maxCents !== null) {
    return t('payment.channelAmountLimitHint', {
      min: formatMoney(centsToAmount(meta.minCents), selectedChannelCurrency.value),
      max: formatMoney(centsToAmount(meta.maxCents), selectedChannelCurrency.value),
    })
  }
  if (meta.hasMin && meta.minCents !== null) {
    return t('payment.channelAmountMinHint', {
      min: formatMoney(centsToAmount(meta.minCents), selectedChannelCurrency.value),
    })
  }
  if (meta.hasMax && meta.maxCents !== null) {
    return t('payment.channelAmountMaxHint', {
      max: formatMoney(centsToAmount(meta.maxCents), selectedChannelCurrency.value),
    })
  }
  return ''
}

const selectedChannelAmountHint = computed(() => {
  const channel = selectedChannel.value
  if (!channel) return ''
  const amountCents = amountToCents(rechargeForm.amount.trim())
  if (amountCents === null || amountCents <= 0) return ''
  return getSelectedChannelAmountHint(channel, amountCents)
})

const loadPaymentChannels = async (seq: number, amount: string) => {
  if (seq !== channelFetchSeq.value) return
  const amountCents = amountToCents(amount)
  if (!amount || amountCents === null || amountCents <= 0) {
    if (seq !== channelFetchSeq.value) return
    channels.value = []
    channelsResolvedAmount.value = ''
    return
  }

  try {
    const response = await walletAPI.getPaymentChannels(amount)
    if (seq !== channelFetchSeq.value) return
    const list = Array.isArray(response.data.data) ? response.data.data : []
    channels.value = normalizeChannels(list, amountCents)
  } catch {
    if (seq !== channelFetchSeq.value) return
    channels.value = []
  } finally {
    if (seq === channelFetchSeq.value) {
      channelsResolvedAmount.value = amount
      channelLoading.value = false
    }
  }
}

const scheduleLoadPaymentChannels = () => {
  const amount = rechargeForm.amount.trim()
  const amountCents = amountToCents(amount)
  if (!amount || amountCents === null || amountCents <= 0) {
    channelFetchSeq.value += 1
    if (channelFetchTimer.value) {
      window.clearTimeout(channelFetchTimer.value)
      channelFetchTimer.value = null
    }
    channelLoading.value = false
    channelsResolvedAmount.value = ''
    channels.value = []
    return
  }

  const seq = channelFetchSeq.value + 1
  channelFetchSeq.value = seq
  channelLoading.value = true

  if (channelFetchTimer.value) {
    window.clearTimeout(channelFetchTimer.value)
    channelFetchTimer.value = null
  }
  channelFetchTimer.value = window.setTimeout(() => {
    channelFetchTimer.value = null
    void loadPaymentChannels(seq, amount)
  }, 300)
}

const formatMoney = (amount?: string, currency?: string) => {
  if (amount === null || amount === undefined || amount === '') return '-'
  if (currency === null || currency === undefined || currency === '') {
    return String(amount)
  }
  return `${amount} ${currency}`
}

const selectedChannelCurrency = computed(() => String(appStore.config?.currency || 'CNY'))
const selectedChannelFeeRateDisplay = computed(() => {
  const rate = rateToBasisPoints(selectedChannel.value?.fee_rate)
  if (rate === null) return '0.00%'
  return `${basisPointsToPercent(rate)}%`
})
const selectedChannelFixedFeeDisplay = computed(() => {
  return formatMoney(String(selectedChannel.value?.fixed_fee ?? '0.00'), selectedChannelCurrency.value)
})
const selectedChannelFeeAmountDisplay = computed(() => {
  const amountCents = amountToCents(rechargeForm.amount)
  if (amountCents === null || amountCents <= 0) return formatMoney('0.00', selectedChannelCurrency.value)
  const rate = rateToBasisPoints(selectedChannel.value?.fee_rate) || 0
  const fixedFeeCents = amountToCents(selectedChannel.value?.fixed_fee) || 0
  const variableFeeCents = calculateFeeCents(amountCents, rate) || 0
  return formatMoney(centsToAmount(variableFeeCents + fixedFeeCents), selectedChannelCurrency.value)
})
// P0-2: 钱包余额本位币固定 USDT，读 API 返回的 currency，不用 site config currency。
// 三栏语义：total = available + frozen（后端已用 decimal 精确计算）
const walletCurrency = computed(() => String(wallet.value?.currency || 'USDT'))
const totalBalance = computed(() => String(wallet.value?.total_balance ?? ''))
const availableBalance = computed(() => String(wallet.value?.available_balance ?? ''))
const frozenBalance = computed(() => String(wallet.value?.frozen_balance ?? ''))
const frozenNote = computed(() =>
  String(wallet.value?.frozen_note || t('personalCenter.wallet.frozenNote'))
)

// 弹窗控制
const sheetCloseFn = ref<(() => void) | null>(null)
const lockBodyScroll = () => { document.body.style.overflow = 'hidden' }
const unlockBodyScroll = () => { document.body.style.overflow = '' }
const openRechargeSheet = () => { showRechargeSheet.value = true; lockBodyScroll(); sheetCloseFn.value = closeRechargeSheet }
const closeRechargeSheet = () => { showRechargeSheet.value = false; unlockBodyScroll(); sheetCloseFn.value = null }
const openWithdrawSheet = () => { showWithdrawSheet.value = true; lockBodyScroll(); sheetCloseFn.value = closeWithdrawSheet }
const closeWithdrawSheet = () => { showWithdrawSheet.value = false; unlockBodyScroll(); sheetCloseFn.value = null }

// 收款方式入口跳转
const goPaymentMethods = () => {
  router.push('/wallet/payment-methods')
}

// 校验是否已绑定 USDT TRC20 地址
const checkTrc20Binding = async () => {
  try {
    const res = await paymentMethodsAPI.list('USDT_TRC20')
    const items = parsePaymentMethodList(res).filter((m) => m.enabled !== false && m.status !== 'disabled')
    trc20Bound.value = items.length > 0
  } catch {
    // 校验失败不阻断充值，按已绑定处理
    trc20Bound.value = true
  }
}

// 充值点击：未绑定 TRC20 则引导先绑定
const onRechargeClick = async () => {
  if (trc20Bound.value === null) {
    await checkTrc20Binding()
  }
  if (trc20Bound.value === false) {
    showBindTip.value = true
    return
  }
  openRechargeSheet()
}

const goBindFromTip = () => {
  showBindTip.value = false
  router.push('/wallet/payment-methods')
}

// 下拉关闭手势（touch + mouse）
const sheetDrag = reactive({ startY: 0, offset: 0, dragging: false })
const sheetDragStyle = computed(() => {
  if (!sheetDrag.dragging) return undefined
  return {
    transform: `translateY(${sheetDrag.offset}px)`,
    transition: 'none',
  }
})
const sheetDragStart = (clientY: number, panel: HTMLElement) => {
  if (panel.scrollTop > 0) return false
  sheetDrag.startY = clientY
  sheetDrag.dragging = true
  sheetDrag.offset = 0
  return true
}
const sheetDragMove = (clientY: number) => {
  if (!sheetDrag.dragging) return
  const delta = clientY - sheetDrag.startY
  if (delta > 0) sheetDrag.offset = delta
}
const sheetDragEnd = () => {
  if (!sheetDrag.dragging) return
  sheetDrag.dragging = false
  if (sheetDrag.offset > 120 && sheetCloseFn.value) {
    sheetCloseFn.value()
  }
  sheetDrag.offset = 0
}
const onSheetTouchStart = (e: TouchEvent) => {
  const touch = e.touches[0]
  if (!touch) return
  sheetDragStart(touch.clientY, e.currentTarget as HTMLElement)
}
const onSheetTouchMove = (e: TouchEvent) => {
  if (!sheetDrag.dragging) return
  const touch = e.touches[0]
  if (!touch) return
  const delta = touch.clientY - sheetDrag.startY
  if (delta > 0) {
    sheetDrag.offset = delta
    e.preventDefault()
  }
}
const onSheetTouchEnd = () => { sheetDragEnd() }

// PC 鼠标拖拽
const onSheetMouseDown = (e: MouseEvent) => {
  if (!sheetDragStart(e.clientY, e.currentTarget as HTMLElement)) return
  const onMouseMove = (ev: MouseEvent) => sheetDragMove(ev.clientY)
  const onMouseUp = () => {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
    sheetDragEnd()
  }
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

// 钱包余额加载：失败时置 walletError，余额区显示"加载失败+重试"，绝不显示假 0。
// 只有真实 API 返回 0 才显示 0.00 USDT。
const loadWallet = async () => {
  walletLoading.value = true
  walletError.value = false
  try {
    const response = await walletAPI.account()
    wallet.value = response.data.data
  } catch {
    walletError.value = true
  } finally {
    walletLoading.value = false
  }
}

const handleRecharge = async () => {
  walletAlert.value = null
  const amount = rechargeForm.amount.trim()
  const amountCents = amountToCents(amount)
  if (!amount || amountCents === null || amountCents <= 0) {
    walletAlert.value = {
      level: 'warning',
      message: t('personalCenter.wallet.errors.invalidAmount'),
    }
    return
  }
  if (!Number.isFinite(rechargeForm.channelId) || rechargeForm.channelId <= 0) {
    walletAlert.value = {
      level: 'warning',
      message: t('personalCenter.wallet.errors.channelRequired'),
    }
    return
  }
  if (selectedChannelAmountHint.value) {
    walletAlert.value = {
      level: 'warning',
      message: selectedChannelAmountHint.value,
    }
    return
  }

  recharging.value = true
  try {
    const response = await walletAPI.recharge({
      amount,
      channel_id: rechargeForm.channelId,
      remark: rechargeForm.remark.trim() || undefined,
    })
    const payload = response.data.data || {}
    const rechargeNo = payload?.recharge?.recharge_no || payload?.recharge_no || ''
    rechargeForm.amount = ''
    rechargeForm.remark = ''
    if (rechargeNo) {
      router.push(`/recharge-orders/${encodeURIComponent(rechargeNo)}`)
    } else {
      walletAlert.value = {
        level: 'success',
        message: t('personalCenter.wallet.createPaymentSuccess'),
      }
    }
  } catch (err: any) {
    walletAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.wallet.errors.rechargeFailed'),
    }
  } finally {
    recharging.value = false
  }
}

// 支付网关回调可能带 recharge_no 回到 /me/wallet，重定向到充值详情页
const redirectRechargeReturn = () => {
  const query = route.query as Record<string, unknown>
  const rechargeNo = String(query.recharge_no || '').trim()
  const orderNo = String(query.order_no || '').trim()
  const targetNo = rechargeNo || (/^WR/i.test(orderNo) ? orderNo : '')
  if (targetNo) {
    router.replace(`/recharge-orders/${encodeURIComponent(targetNo)}${window.location.search}`)
  }
}

const initialize = async () => {
  walletAlert.value = null
  try {
    if (!appStore.config) {
      await appStore.loadConfig()
    }
    await loadWallet()
    redirectRechargeReturn()
    void checkTrc20Binding()
  } catch (err: any) {
    walletAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.wallet.errors.loadFailed'),
    }
  }
}

watch(() => rechargeForm.amount, () => {
  scheduleLoadPaymentChannels()
}, { immediate: true })

watch(
  channels,
  (list) => {
    if (list.length === 0) {
      rechargeForm.channelId = 0
      return
    }
    if (!list.some((item: any) => item.id === rechargeForm.channelId)) {
      const first = list[0]
      if (!first) {
        rechargeForm.channelId = 0
        return
      }
      rechargeForm.channelId = first.id
    }
  },
  { immediate: true }
)

onMounted(() => {
  void initialize()
})

onUnmounted(() => {
  channelFetchSeq.value += 1
  if (channelFetchTimer.value) {
    window.clearTimeout(channelFetchTimer.value)
    channelFetchTimer.value = null
  }
  channelLoading.value = false
})
</script>

<style>
.sheet-enter-active,
.sheet-leave-active {
  transition: opacity 0.3s ease;
}
.sheet-enter-active .sheet-panel {
  transition: transform 0.38s cubic-bezier(0.32, 0.72, 0, 1);
}
.sheet-leave-active .sheet-panel {
  transition: transform 0.25s cubic-bezier(0.4, 0, 1, 1);
}
.sheet-enter-from,
.sheet-leave-to {
  opacity: 0;
}
.sheet-enter-from .sheet-panel,
.sheet-leave-to .sheet-panel {
  transform: translateY(100%);
}
</style>

