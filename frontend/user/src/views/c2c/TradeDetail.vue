<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <!-- Header -->
      <div class="mb-6 mt-8 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('c2c.trade.tradeDetail') }}</h1>
          <p v-if="trade" class="mt-1 text-sm text-muted-foreground font-mono">{{ trade.trade_no }}</p>
        </div>
        <div v-if="trade" class="flex items-center gap-2">
          <Badge size="default" :class="TRADE_STATUS_VARIANTS[trade.status]">
            {{ TRADE_STATUS_LABELS[trade.status] }}
          </Badge>
          <Button variant="outline" size="sm" :disabled="loading" @click="refresh">
            <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
            {{ t('c2c.trade.refresh') }}
          </Button>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading && !trade" class="max-w-4xl mx-auto space-y-3">
        <div v-for="i in 4" :key="i" class="h-24 rounded-2xl border bg-muted/60 animate-pulse"></div>
      </div>

      <!-- Not found -->
      <EmptyState
        v-else-if="!trade"
        icon="alert"
        variant="soft"
        size="lg"
        :title="t('c2c.errors.notFound')"
      />

      <!-- Content -->
      <div v-else class="grid gap-6 lg:grid-cols-5 max-w-6xl mx-auto">
        <!-- Left: trade info -->
        <div class="lg:col-span-3 rounded-2xl border bg-card p-6 shadow-sm">
          <h2 class="text-base font-semibold">{{ t('c2c.trade.tradeInfo') }}</h2>
          <dl class="mt-4 divide-y divide-border text-sm">
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.myTrades.tradeNo') }}</dt>
              <dd class="font-mono font-medium">{{ trade.trade_no }}</dd>
            </div>
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.myTrades.role') }}</dt>
              <dd class="font-medium">{{ roleLabel }}</dd>
            </div>
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.myTrades.counterparty') }}</dt>
              <dd class="font-medium">{{ counterpartyLabel }}</dd>
            </div>
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.buyPanel.price') }}</dt>
              <dd class="font-mono font-medium">{{ trade.price }} {{ trade.fiat_currency }}</dd>
            </div>
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.myTrades.fiatAmount') }}</dt>
              <dd class="font-mono font-medium">{{ trade.fiat_amount }} {{ trade.fiat_currency }}</dd>
            </div>
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.myTrades.usdtAmount') }}</dt>
              <dd class="font-mono font-medium">{{ formatUsdt(trade.usdt_amount) }}</dd>
            </div>
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.myTrades.status') }}</dt>
              <dd><Badge size="sm" :class="TRADE_STATUS_VARIANTS[trade.status]">{{ TRADE_STATUS_LABELS[trade.status] }}</Badge></dd>
            </div>
            <div class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.myListings.createdAt') }}</dt>
              <dd class="font-mono text-xs text-muted-foreground">{{ formatTime(trade.created_at) }}</dd>
            </div>
            <div v-if="isActive" class="flex justify-between py-2.5">
              <dt class="text-muted-foreground">{{ t('c2c.trade.countdown') }}</dt>
              <dd class="font-mono font-semibold text-amber-600">{{ countdown.formatted }}</dd>
            </div>
          </dl>
        </div>

        <!-- Right: action panel -->
        <div class="lg:col-span-2 space-y-4">
          <!-- pending_payment: buyer sees seller payment info -->
          <template v-if="trade.status === 'pending_payment'">
            <div v-if="role === 'buyer'" class="rounded-2xl border bg-card p-6 shadow-sm space-y-4">
              <h3 class="text-base font-semibold">{{ t('c2c.trade.paymentInfo') }}</h3>
              <div v-if="parsedSnapshot" class="space-y-3 text-sm">
                <div v-if="parsedSnapshot.type" class="flex justify-between">
                  <span class="text-muted-foreground">{{ t('c2c.paymentMethod.type') }}</span>
                  <span class="font-medium">{{ paymentTypeLabel(parsedSnapshot.type) }}</span>
                </div>
                <div v-if="parsedSnapshot.account_name" class="flex justify-between">
                  <span class="text-muted-foreground">{{ t('c2c.paymentMethod.accountName') }}</span>
                  <span class="font-medium">{{ parsedSnapshot.account_name }}</span>
                </div>
                <div v-if="parsedSnapshot.account_identifier" class="flex justify-between">
                  <span class="text-muted-foreground">{{ t('c2c.paymentMethod.accountIdentifier') }}</span>
                  <span class="font-mono font-medium">{{ parsedSnapshot.account_identifier }}</span>
                </div>
                <div v-if="parsedSnapshot.bank_name" class="flex justify-between">
                  <span class="text-muted-foreground">{{ t('paymentMethods.fields.bankName') }}</span>
                  <span class="font-medium">{{ parsedSnapshot.bank_name }}</span>
                </div>
                <div v-if="parsedSnapshot.bank_account" class="flex justify-between">
                  <span class="text-muted-foreground">{{ t('paymentMethods.fields.bankAccount') }}</span>
                  <span class="font-mono font-medium">{{ parsedSnapshot.bank_account }}</span>
                </div>
                <div v-if="parsedSnapshot.branch_name" class="flex justify-between">
                  <span class="text-muted-foreground">{{ t('paymentMethods.fields.branchOptional') }}</span>
                  <span class="font-medium">{{ parsedSnapshot.branch_name }}</span>
                </div>
                <div v-if="parsedSnapshot.address" class="flex justify-between">
                  <span class="text-muted-foreground">{{ t('paymentMethods.fields.address') }}</span>
                  <span class="font-mono font-medium">{{ parsedSnapshot.address }}</span>
                </div>
                <div v-if="parsedSnapshot.instructions" class="rounded-lg bg-muted/50 p-3 text-xs text-muted-foreground whitespace-pre-wrap">
                  {{ parsedSnapshot.instructions }}
                </div>
                <img v-if="qrImageSrc" :src="qrImageSrc" alt="QR" class="mx-auto max-h-48 rounded-lg border" />
                <div v-if="rawSnapshotText" class="rounded-lg bg-muted/50 p-3 text-xs text-muted-foreground whitespace-pre-wrap">{{ rawSnapshotText }}</div>
              </div>
              <p v-else class="text-sm text-muted-foreground">{{ t('c2c.errors.noPaymentMethod') }}</p>

              <div class="rounded-xl border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-center text-sm font-semibold text-amber-600 font-mono">
                {{ countdown.formatted }}
              </div>

              <div>
                <Label class="mb-2 block">{{ t('c2c.trade.paymentReference') }}</Label>
                <Input v-model="paymentReference" :placeholder="t('c2c.trade.paymentReferencePlaceholder')" class="h-11" />
              </div>

              <Button class="h-11 w-full" :disabled="submitting" @click="handleMarkPaid">
                {{ submitting ? '...' : t('c2c.trade.markPaid') }}
              </Button>
              <Button variant="outline" class="h-11 w-full text-destructive hover:text-destructive" :disabled="submitting" @click="handleCancel">
                {{ t('c2c.trade.cancel') }}
              </Button>
            </div>

            <!-- seller waiting -->
            <div v-else class="rounded-2xl border bg-card p-6 shadow-sm space-y-4">
              <h3 class="text-base font-semibold">{{ t('c2c.trade.actionPanel') }}</h3>
              <div class="rounded-xl border border-blue-500/40 bg-blue-500/10 p-4 text-center text-sm font-medium text-blue-600">
                {{ t('c2c.trade.waitingBuyer') }}
              </div>
              <div class="rounded-xl border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-center text-sm font-semibold text-amber-600 font-mono">
                {{ countdown.formatted }}
              </div>
            </div>
          </template>

          <!-- paid -->
          <div v-else-if="trade.status === 'paid'" class="rounded-2xl border bg-card p-6 shadow-sm space-y-4">
            <h3 class="text-base font-semibold">{{ t('c2c.trade.actionPanel') }}</h3>
            <div v-if="role === 'buyer'" class="rounded-xl border border-blue-500/40 bg-blue-500/10 p-4 text-center text-sm font-medium text-blue-600">
              {{ t('c2c.trade.waitingSeller') }}
            </div>
            <template v-else>
              <div class="rounded-xl border border-emerald-500/40 bg-emerald-500/10 p-4 text-sm text-emerald-600">
                {{ t('c2c.trade.confirmReceived') }}
              </div>
              <Button variant="destructive" class="h-11 w-full" :disabled="submitting" @click="handleConfirmReceived">
                {{ submitting ? '...' : t('c2c.trade.confirmReceived') }}
              </Button>
            </template>
            <Button variant="outline" class="h-11 w-full" :disabled="submitting" @click="openDispute">
              {{ t('c2c.trade.dispute') }}
            </Button>
          </div>

          <!-- disputed -->
          <div v-else-if="trade.status === 'disputed'" class="rounded-2xl border bg-card p-6 shadow-sm">
            <div class="rounded-xl border border-rose-500/40 bg-rose-500/10 p-4 text-center text-sm font-medium text-rose-600">
              {{ t('c2c.trade.disputedInfo') }}
            </div>
          </div>

          <!-- completed -->
          <div v-else-if="trade.status === 'completed'" class="rounded-2xl border bg-card p-6 shadow-sm">
            <div class="rounded-xl border border-emerald-500/40 bg-emerald-500/10 p-4 text-center text-sm font-medium text-emerald-600">
              {{ role === 'buyer' ? t('c2c.trade.completedBuyer') : t('c2c.trade.completedSeller') }}
            </div>
          </div>

          <!-- canceled -->
          <div v-else-if="trade.status === 'canceled'" class="rounded-2xl border bg-card p-6 shadow-sm">
            <div class="rounded-xl border border-zinc-500/40 bg-zinc-500/10 p-4 text-center text-sm text-zinc-500">
              {{ t('c2c.trade.canceledInfo') }}
            </div>
          </div>

          <!-- expired -->
          <div v-else-if="trade.status === 'expired'" class="rounded-2xl border bg-card p-6 shadow-sm">
            <div class="rounded-xl border border-zinc-500/40 bg-zinc-500/10 p-4 text-center text-sm text-zinc-500">
              {{ t('c2c.trade.expiredInfo') }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Dispute modal -->
    <div v-if="disputeOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="disputeOpen = false">
      <div class="w-full max-w-md rounded-2xl bg-card p-6 shadow-xl">
        <h3 class="text-base font-semibold">{{ t('c2c.trade.dispute') }}</h3>
        <div class="mt-4 space-y-4">
          <div>
            <Label class="mb-2 block">{{ t('c2c.trade.disputeReason') }}</Label>
            <Input v-model="disputeReason" :placeholder="t('c2c.trade.disputeReason')" class="h-11" />
          </div>
          <div>
            <Label class="mb-2 block">{{ t('c2c.trade.disputeDescription') }}</Label>
            <Textarea v-model="disputeDescription" :placeholder="t('c2c.trade.disputeDescription')" class="min-h-[80px]" />
          </div>
          <div>
            <Label class="mb-2 block">{{ t('c2c.trade.disputeEvidence') }}</Label>
            <Input v-model="disputeEvidence" :placeholder="t('c2c.trade.disputeEvidence')" class="h-11" />
          </div>
        </div>
        <div class="mt-5 flex gap-3">
          <Button variant="outline" class="flex-1" @click="disputeOpen = false">{{ t('c2c.buyPanel.cancel') }}</Button>
          <Button class="flex-1" :disabled="!disputeReason.trim() || submitting" @click="handleDisputeSubmit">
            {{ t('c2c.trade.disputeSubmit') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { RefreshCw } from 'lucide-vue-next'
import { useC2CStore } from '@/stores/c2c'
import {
  useCountdown,
  useTradePolling,
  useC2CTradeActions,
  TRADE_STATUS_LABELS,
  TRADE_STATUS_VARIANTS,
  PAYMENT_METHOD_TYPE_LABELS,
} from '@/composables/useC2C'
import { useConfirmDialog } from '@/composables/useConfirmDialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import EmptyState from '@/components/EmptyState.vue'
import { formatUsdt } from '@/utils/money'
import { C2C_TRADE_POLLING_STATUSES } from '@/api/c2c'

const route = useRoute()
const { t } = useI18n()
const c2cStore = useC2CStore()
const tradeActions = useC2CTradeActions()
const { confirm } = useConfirmDialog()

const tradeId = Number(route.params.id)

const trade = computed(() => c2cStore.currentTrade)
const loading = computed(() => c2cStore.currentTradeLoading)
const role = computed(() => c2cStore.currentUserRole)

const paymentReference = ref('')
const submitting = ref(false)

const disputeOpen = ref(false)
const disputeReason = ref('')
const disputeDescription = ref('')
const disputeEvidence = ref('')

// ─── Countdown ───
const countdown = useCountdown(() => trade.value?.expired_at)

// ─── Polling ───
const polling = useTradePolling(tradeId, () => {
  // expired callback: nothing extra needed, polling already refreshed
})

const isActive = computed(() =>
  trade.value ? !['completed', 'canceled', 'expired'].includes(trade.value.status) : false,
)

const roleLabel = computed(() => {
  if (role.value === 'buyer') return t('c2c.myTrades.buyer')
  if (role.value === 'seller') return t('c2c.myTrades.seller')
  return '-'
})

const counterpartyId = computed(() => {
  if (!trade.value) return '-'
  return role.value === 'buyer' ? trade.value.seller_user_id : trade.value.buyer_user_id
})
const counterpartyLabel = computed(() => `#${counterpartyId.value}`)

// ─── payment_method_snapshot JSON 解析 ───
const parsedSnapshot = computed<Record<string, any> | null>(() => {
  const raw = trade.value?.payment_method_snapshot
  if (!raw) return null
  try {
    return JSON.parse(raw) as Record<string, any>
  } catch {
    return null
  }
})

const rawSnapshotText = computed(() => {
  const raw = trade.value?.payment_method_snapshot
  if (!raw) return ''
  try {
    JSON.parse(raw)
    return ''
  } catch {
    return raw
  }
})

// 兼容旧 qr_image 与新版 qr_code_url
const qrImageSrc = computed(() => {
  const snap = parsedSnapshot.value
  const url = String(snap?.qr_code_url || snap?.qr_image || '')
  if (!url) return ''
  if (/^https?:\/\//i.test(url) || url.startsWith('data:')) return url
  return `${import.meta.env.VITE_API_BASE_URL || ''}${url}`
})

const paymentTypeLabel = (type: string) => PAYMENT_METHOD_TYPE_LABELS[type] || type

const formatTime = (s: string) => {
  if (!s) return '-'
  return s.replace('T', ' ').slice(0, 19)
}

// ─── Actions ───
const refresh = () => {
  void c2cStore.fetchCurrentTrade(tradeId)
}

const handleMarkPaid = async () => {
  const ok = await confirm({
    title: t('c2c.trade.markPaid'),
    message: t('c2c.trade.markPaidConfirm'),
    confirmText: t('c2c.trade.markPaid'),
  })
  if (!ok) return
  submitting.value = true
  try {
    await tradeActions.markPaid(tradeId, paymentReference.value.trim() || undefined)
  } finally {
    submitting.value = false
  }
}

const handleCancel = async () => {
  const ok = await confirm({
    title: t('c2c.trade.cancel'),
    message: t('c2c.trade.cancelConfirm'),
    confirmText: t('c2c.trade.cancel'),
    variant: 'danger',
  })
  if (!ok) return
  submitting.value = true
  try {
    await tradeActions.cancelTrade(tradeId)
  } finally {
    submitting.value = false
  }
}

const handleConfirmReceived = async () => {
  const ok = await confirm({
    title: t('c2c.trade.confirmReceived'),
    message: t('c2c.trade.confirmReceivedConfirm'),
    confirmText: t('c2c.trade.confirmReceived'),
    variant: 'danger',
  })
  if (!ok) return
  submitting.value = true
  try {
    await tradeActions.confirmTrade(tradeId)
  } finally {
    submitting.value = false
  }
}

const openDispute = () => {
  disputeReason.value = ''
  disputeDescription.value = ''
  disputeEvidence.value = ''
  disputeOpen.value = true
}

const handleDisputeSubmit = async () => {
  if (!disputeReason.value.trim()) return
  submitting.value = true
  try {
    await tradeActions.disputeTrade(
      tradeId,
      disputeReason.value.trim(),
      disputeDescription.value.trim() || undefined,
      disputeEvidence.value.trim() || undefined,
    )
    disputeOpen.value = false
  } finally {
    submitting.value = false
  }
}

// countdown expired → refresh trade
watch(countdown.isExpired, (v) => {
  if (v) polling.handleCountdownExpired()
})

onMounted(async () => {
  await c2cStore.fetchCurrentTrade(tradeId)
  countdown.start()
  polling.init()
  if (trade.value && C2C_TRADE_POLLING_STATUSES.includes(trade.value.status)) {
    polling.startPolling()
  }
})
</script>
