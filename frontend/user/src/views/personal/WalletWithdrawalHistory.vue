<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h2 class="text-xl font-black text-foreground">{{ t('personalCenter.wallet.withdraw.historyTitle') }}</h2>
      <router-link to="/me/wallet/withdrawal" class="text-sm text-primary underline-offset-4 hover:underline">
        {{ t('personalCenter.wallet.withdraw.newWithdrawal') }}
      </router-link>
    </div>

    <!-- Status Filter -->
    <div class="flex flex-wrap gap-2">
      <button
        v-for="s in statusFilters"
        :key="s.value"
        type="button"
        class="rounded-full px-3.5 py-1.5 text-xs font-semibold transition-colors"
        :class="filterStatus === s.value
          ? 'bg-primary text-primary-foreground'
          : 'bg-muted text-muted-foreground hover:bg-accent'"
        @click="changeStatus(s.value)"
      >
        {{ s.label }}
      </button>
    </div>

    <!-- Alert -->
    <div v-if="alert" class="rounded-xl border p-4 text-sm" :class="alertClass">
      {{ alert }}
    </div>

    <!-- List -->
    <div v-if="loading" class="rounded-2xl border bg-card p-8 text-center text-muted-foreground">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="list.length === 0" class="rounded-2xl border bg-card p-8 text-center text-muted-foreground">
      {{ t('personalCenter.wallet.withdraw.empty') }}
    </div>

    <div v-else class="space-y-3">
      <div
        v-for="item in list"
        :key="item.id"
        class="rounded-2xl border bg-card p-5 shadow-sm space-y-3"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="font-mono text-sm font-semibold text-foreground">{{ item.withdrawal_no }}</div>
            <div class="mt-1 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</div>
          </div>
          <Badge :variant="statusVariant(item.status)" size="sm">
            {{ statusLabel(item.status) }}
          </Badge>
        </div>

        <div class="grid grid-cols-2 gap-3 text-sm">
          <div>
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.addressLabel') }}</div>
            <div class="mt-0.5 break-all font-mono text-xs text-foreground">{{ item.address }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.networkLabel') }}</div>
            <div class="mt-0.5 text-foreground">{{ item.network }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.requestAmount') }}</div>
            <div class="mt-0.5 font-mono text-foreground">{{ formatUsdt(item.request_amount) }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.feeLabel') }}</div>
            <div class="mt-0.5 font-mono text-foreground">{{ formatUsdt(item.fee_amount) }}</div>
          </div>
          <div class="col-span-2">
            <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.netLabel') }}</div>
            <div class="mt-0.5 font-mono font-semibold text-primary">{{ formatUsdt(item.net_amount) }}</div>
          </div>
          <div v-if="item.txid" class="col-span-2">
            <div class="text-xs text-muted-foreground">TxID</div>
            <div class="mt-0.5 break-all font-mono text-xs text-foreground">{{ item.txid }}</div>
          </div>
          <div v-if="item.reject_reason" class="col-span-2">
            <div class="text-xs text-destructive">{{ t('personalCenter.wallet.withdraw.rejectReason') }}</div>
            <div class="mt-0.5 text-xs text-destructive">{{ item.reject_reason }}</div>
          </div>
        </div>

        <!-- Cancel button for pending -->
        <div v-if="item.status === 'pending' && cancelingId !== item.id" class="flex justify-end">
          <Button variant="outline" size="sm" @click="startCancel(item)">
            {{ t('personalCenter.wallet.withdraw.cancel') }}
          </Button>
        </div>

        <!-- Cancel confirm inline -->
        <div v-if="cancelingId === item.id" class="rounded-xl border border-destructive/30 bg-destructive/5 p-4 space-y-3">
          <p class="text-sm font-semibold text-destructive">{{ t('personalCenter.wallet.withdraw.cancelConfirmTitle') }}</p>
          <p class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.cancelConfirmDesc') }}</p>
          <div class="flex gap-2">
            <Input
              v-model="cancelTotp"
              type="text"
              inputmode="numeric"
              maxlength="6"
              :placeholder="t('personalCenter.wallet.withdraw.totpPlaceholder')"
              class="h-9 flex-1 font-mono tracking-widest"
            />
            <Button variant="destructive" size="sm" :disabled="cancelSubmitting" @click="confirmCancel(item)">
              {{ cancelSubmitting ? t('common.loading') : t('personalCenter.wallet.withdraw.cancelConfirm') }}
            </Button>
            <Button variant="outline" size="sm" :disabled="cancelSubmitting" @click="cancelCancel">
              {{ t('common.cancel') }}
            </Button>
          </div>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="totalPage > 1" class="flex items-center justify-center gap-2">
      <Button variant="outline" size="sm" :disabled="page <= 1" @click="changePage(page - 1)">
        {{ t('common.previous') }}
      </Button>
      <span class="text-sm text-muted-foreground">{{ page }} / {{ totalPage }}</span>
      <Button variant="outline" size="sm" :disabled="page >= totalPage" @click="changePage(page + 1)">
        {{ t('common.next') }}
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { walletAPI } from '../../api'
import type { Withdrawal } from '../../api/types'
import { formatUsdt } from '../../utils/money'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'

const { t } = useI18n()

const loading = ref(true)
const list = ref<Withdrawal[]>([])
const page = ref(1)
const pageSize = 20
const totalPage = ref(1)
const total = ref(0)
const filterStatus = ref('')
const alert = ref('')
const alertType = ref<'error' | 'warning' | 'success'>('error')

const cancelingId = ref<number | null>(null)
const cancelTotp = ref('')
const cancelSubmitting = ref(false)

const statusFilters = [
  { value: '', label: t('personalCenter.wallet.withdraw.statusAll') },
  { value: 'pending', label: t('personalCenter.wallet.withdraw.statusPending') },
  { value: 'approved', label: t('personalCenter.wallet.withdraw.statusApproved') },
  { value: 'processing', label: t('personalCenter.wallet.withdraw.statusProcessing') },
  { value: 'completed', label: t('personalCenter.wallet.withdraw.statusCompleted') },
  { value: 'rejected', label: t('personalCenter.wallet.withdraw.statusRejected') },
  { value: 'canceled', label: t('personalCenter.wallet.withdraw.statusCanceled') },
]

const alertClass = computed(() => {
  if (alertType.value === 'error') return 'border-destructive/50 bg-destructive/10 text-destructive'
  return 'border-green-500/50 bg-green-500/10 text-green-600'
})

const statusVariant = (status: string) => {
  switch (status) {
    case 'pending': return 'warning'
    case 'approved': return 'info'
    case 'processing': return 'accent'
    case 'completed': return 'success'
    case 'rejected': return 'danger'
    case 'canceled': return 'neutral'
    default: return 'outline'
  }
}

const statusLabel = (status: string) => {
  const map: Record<string, string> = {
    pending: t('personalCenter.wallet.withdraw.statusPending'),
    approved: t('personalCenter.wallet.withdraw.statusApproved'),
    processing: t('personalCenter.wallet.withdraw.statusProcessing'),
    completed: t('personalCenter.wallet.withdraw.statusCompleted'),
    rejected: t('personalCenter.wallet.withdraw.statusRejected'),
    canceled: t('personalCenter.wallet.withdraw.statusCanceled'),
  }
  return map[status] || status
}

const formatDate = (d?: string | null) => {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleString()
  } catch {
    return d
  }
}

const fetchList = async () => {
  loading.value = true
  try {
    const res = await walletAPI.listWithdrawals({
      page: page.value,
      page_size: pageSize,
      status: filterStatus.value || undefined,
    })
    list.value = res.data.data?.list || res.data.data || []
    const pg = res.data.pagination
    if (pg) {
      totalPage.value = pg.total_page || 1
      total.value = pg.total || 0
    }
  } catch {
    list.value = []
  } finally {
    loading.value = false
  }
}

const changeStatus = (s: string) => {
  filterStatus.value = s
  page.value = 1
  void fetchList()
}

const changePage = (p: number) => {
  if (p < 1 || p > totalPage.value) return
  page.value = p
  void fetchList()
}

const startCancel = (item: Withdrawal) => {
  cancelingId.value = item.id
  cancelTotp.value = ''
}

const cancelCancel = () => {
  cancelingId.value = null
  cancelTotp.value = ''
}

const confirmCancel = async (item: Withdrawal) => {
  const code = cancelTotp.value.trim()
  if (!/^\d{6}$/.test(code)) {
    alert.value = t('personalCenter.wallet.withdraw.errors.totpRequired')
    alertType.value = 'error'
    return
  }
  cancelSubmitting.value = true
  try {
    await walletAPI.cancelWithdrawal(item.id, code)
    alert.value = t('personalCenter.wallet.withdraw.cancelSuccess')
    alertType.value = 'success'
    cancelCancel()
    await fetchList()
  } catch (err: any) {
    alert.value = err?.message || t('personalCenter.wallet.withdraw.cancelFailed')
    alertType.value = 'error'
  } finally {
    cancelSubmitting.value = false
  }
}

onMounted(() => {
  void fetchList()
})
</script>
