<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AdminWithdrawal } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import ComplianceGuardWrapper from '@/components/ComplianceGuardWrapper.vue'
import { formatDate } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const id = computed(() => Number(route.params.id))
const loading = ref(true)
const withdrawal = ref<AdminWithdrawal | null>(null)
const actionLoading = ref(false)
const alert = ref('')
const alertType = ref<'error' | 'success'>('error')

// Action form state
const adminNote = ref('')
const rejectReason = ref('')
const txid = ref('')

const fetchDetail = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getWalletWithdrawal(id.value)
    withdrawal.value = res.data.data
  } catch {
    withdrawal.value = null
  } finally {
    loading.value = false
  }
}

const statusClass = (status?: string) => {
  switch (status) {
    case 'pending': return 'border-warning/30 bg-warning/10 text-warning'
    case 'approved': return 'border-info/30 bg-info/10 text-info'
    case 'processing': return 'border-primary/30 bg-primary/10 text-primary'
    case 'completed': return 'border-success/30 bg-success/10 text-success'
    case 'rejected': return 'border-destructive/30 bg-destructive/10 text-destructive'
    case 'canceled': return 'border-border bg-muted text-muted-foreground'
    default: return 'border-border bg-muted text-muted-foreground'
  }
}

const statusLabel = (status?: string) => {
  const map: Record<string, string> = {
    pending: t('admin.walletWithdrawals.status.pending'),
    approved: t('admin.walletWithdrawals.status.approved'),
    processing: t('admin.walletWithdrawals.status.processing'),
    completed: t('admin.walletWithdrawals.status.completed'),
    rejected: t('admin.walletWithdrawals.status.rejected'),
    canceled: t('admin.walletWithdrawals.status.canceled'),
  }
  return status ? (map[status] || status) : '-'
}

const showAlert = (msg: string, type: 'error' | 'success' = 'error') => {
  alert.value = msg
  alertType.value = type
}

const canApprove = computed(() => withdrawal.value?.status === 'pending')
const canReject = computed(() => ['pending', 'approved'].includes(withdrawal.value?.status || ''))
const canProcess = computed(() => withdrawal.value?.status === 'approved')
const canComplete = computed(() => withdrawal.value?.status === 'processing')
const isTerminal = computed(() => ['completed', 'rejected', 'canceled'].includes(withdrawal.value?.status || ''))

const doApprove = async () => {
  actionLoading.value = true
  alert.value = ''
  try {
    const idemKey = crypto.randomUUID()
    await adminAPI.approveWalletWithdrawal(id.value, adminNote.value.trim() || undefined, idemKey)
    showAlert(t('admin.walletWithdrawals.approveSuccess'), 'success')
    await fetchDetail()
  } catch (err: any) {
    showAlert(err?.message || t('admin.walletWithdrawals.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

const doReject = async () => {
  if (!rejectReason.value.trim()) {
    showAlert(t('admin.walletWithdrawals.rejectReasonRequired'))
    return
  }
  actionLoading.value = true
  alert.value = ''
  try {
    const idemKey = crypto.randomUUID()
    await adminAPI.rejectWalletWithdrawal(id.value, rejectReason.value.trim(), adminNote.value.trim() || undefined, idemKey)
    showAlert(t('admin.walletWithdrawals.rejectSuccess'), 'success')
    await fetchDetail()
  } catch (err: any) {
    showAlert(err?.message || t('admin.walletWithdrawals.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

const doProcessing = async () => {
  actionLoading.value = true
  alert.value = ''
  try {
    const idemKey = crypto.randomUUID()
    await adminAPI.processingWalletWithdrawal(id.value, idemKey)
    showAlert(t('admin.walletWithdrawals.processingSuccess'), 'success')
    await fetchDetail()
  } catch (err: any) {
    showAlert(err?.message || t('admin.walletWithdrawals.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

const doComplete = async () => {
  if (!txid.value.trim()) {
    showAlert(t('admin.walletWithdrawals.txidRequired'))
    return
  }
  actionLoading.value = true
  alert.value = ''
  try {
    const idemKey = crypto.randomUUID()
    await adminAPI.completeWalletWithdrawal(id.value, txid.value.trim(), adminNote.value.trim() || undefined, idemKey)
    showAlert(t('admin.walletWithdrawals.completeSuccess'), 'success')
    await fetchDetail()
  } catch (err: any) {
    showAlert(err?.message || t('admin.walletWithdrawals.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

const backToList = () => {
  router.push('/wallet-withdrawals')
}

onMounted(() => {
  void fetchDetail()
})
</script>

<template>
  <ComplianceGuardWrapper>
    <div class="space-y-6">
      <div class="flex items-center gap-3">
        <Button variant="outline" size="sm" @click="backToList">
          ← {{ t('admin.walletWithdrawals.backToList') }}
        </Button>
        <h1 class="text-2xl font-semibold">{{ t('admin.walletWithdrawals.detailTitle') }}</h1>
      </div>

      <div v-if="loading" class="rounded-xl border border-border bg-card p-8 text-center text-muted-foreground">
        {{ t('admin.common.loading') }}
      </div>

      <template v-else-if="withdrawal">
        <!-- Alert -->
        <div v-if="alert" class="rounded-xl border p-4 text-sm" :class="alertType === 'success' ? 'border-success/50 bg-success/10 text-success' : 'border-destructive/50 bg-destructive/10 text-destructive'">
          {{ alert }}
        </div>

        <!-- Detail Card -->
        <div class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-5">
          <div class="flex items-center justify-between">
            <div>
              <div class="font-mono text-lg font-semibold text-foreground">{{ withdrawal.withdrawal_no }}</div>
              <div class="mt-1 text-xs text-muted-foreground">{{ formatDate(withdrawal.created_at) }}</div>
            </div>
            <span class="inline-flex rounded-full border px-3 py-1 text-sm" :class="statusClass(withdrawal.status)">
              {{ statusLabel(withdrawal.status) }}
            </span>
          </div>

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <div class="text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.table.user') }}</div>
              <div class="mt-1 text-sm text-foreground">
                #{{ withdrawal.user_id }}
                <span v-if="withdrawal.user?.email" class="ml-2 text-muted-foreground">{{ withdrawal.user.email }}</span>
              </div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.table.network') }}</div>
              <div class="mt-1 text-sm text-foreground">{{ withdrawal.network }}</div>
            </div>
            <div class="md:col-span-2">
              <div class="text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.table.address') }}</div>
              <div class="mt-1 break-all font-mono text-sm text-foreground">{{ withdrawal.address }}</div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.requestAmount') }}</div>
              <div class="mt-1 font-mono text-sm text-foreground">{{ withdrawal.request_amount }} USDT</div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.feeAmount') }}</div>
              <div class="mt-1 font-mono text-sm text-foreground">{{ withdrawal.fee_amount }} USDT</div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.netAmount') }}</div>
              <div class="mt-1 font-mono text-sm font-semibold text-primary">{{ withdrawal.net_amount }} USDT</div>
            </div>
            <div v-if="withdrawal.txid">
              <div class="text-xs text-muted-foreground">TxID</div>
              <div class="mt-1 break-all font-mono text-sm text-foreground">{{ withdrawal.txid }}</div>
            </div>
            <div v-if="withdrawal.reject_reason" class="md:col-span-2">
              <div class="text-xs text-destructive">{{ t('admin.walletWithdrawals.rejectReason') }}</div>
              <div class="mt-1 text-sm text-destructive">{{ withdrawal.reject_reason }}</div>
            </div>
            <div v-if="withdrawal.admin_note" class="md:col-span-2">
              <div class="text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.adminNote') }}</div>
              <div class="mt-1 text-sm text-foreground">{{ withdrawal.admin_note }}</div>
            </div>
          </div>

          <!-- Timeline -->
          <div class="border-t border-border pt-4">
            <div class="text-xs font-semibold uppercase text-muted-foreground">{{ t('admin.walletWithdrawals.timeline') }}</div>
            <div class="mt-3 space-y-2 text-xs">
              <div class="flex justify-between"><span>{{ t('admin.walletWithdrawals.createdAt') }}</span><span>{{ formatDate(withdrawal.created_at) }}</span></div>
              <div v-if="withdrawal.approved_at" class="flex justify-between"><span>{{ t('admin.walletWithdrawals.approvedAt') }}</span><span>{{ formatDate(withdrawal.approved_at) }}</span></div>
              <div v-if="withdrawal.processing_at" class="flex justify-between"><span>{{ t('admin.walletWithdrawals.processingAt') }}</span><span>{{ formatDate(withdrawal.processing_at) }}</span></div>
              <div v-if="withdrawal.completed_at" class="flex justify-between"><span>{{ t('admin.walletWithdrawals.completedAt') }}</span><span>{{ formatDate(withdrawal.completed_at) }}</span></div>
              <div v-if="withdrawal.rejected_at" class="flex justify-between"><span>{{ t('admin.walletWithdrawals.rejectedAt') }}</span><span>{{ formatDate(withdrawal.rejected_at) }}</span></div>
              <div v-if="withdrawal.canceled_at" class="flex justify-between"><span>{{ t('admin.walletWithdrawals.canceledAt') }}</span><span>{{ formatDate(withdrawal.canceled_at) }}</span></div>
            </div>
          </div>
        </div>

        <!-- Action Panel -->
        <div v-if="!isTerminal" class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-5">
          <h2 class="text-lg font-semibold">{{ t('admin.walletWithdrawals.actions') }}</h2>

          <!-- Admin Note (shared) -->
          <div>
            <Label class="mb-2 block text-sm">{{ t('admin.walletWithdrawals.adminNoteLabel') }}</Label>
            <Textarea v-model="adminNote" :placeholder="t('admin.walletWithdrawals.adminNotePlaceholder')" rows="2" />
          </div>

          <!-- Approve -->
          <div v-if="canApprove" class="flex items-center gap-3">
            <Button :disabled="actionLoading" @click="doApprove">
              {{ t('admin.walletWithdrawals.approve') }}
            </Button>
          </div>

          <!-- Reject -->
          <div v-if="canReject" class="space-y-3 border-t border-border pt-4">
            <Label class="block text-sm font-semibold text-destructive">{{ t('admin.walletWithdrawals.reject') }}</Label>
            <div>
              <Label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.walletWithdrawals.rejectReasonLabel') }}</Label>
              <Textarea v-model="rejectReason" :placeholder="t('admin.walletWithdrawals.rejectReasonPlaceholder')" rows="2" />
            </div>
            <Button variant="destructive" :disabled="actionLoading || !rejectReason.trim()" @click="doReject">
              {{ t('admin.walletWithdrawals.confirmReject') }}
            </Button>
          </div>

          <!-- Processing -->
          <div v-if="canProcess" class="space-y-3 border-t border-border pt-4">
            <Label class="block text-sm font-semibold">{{ t('admin.walletWithdrawals.markProcessing') }}</Label>
            <Button :disabled="actionLoading" @click="doProcessing">
              {{ t('admin.walletWithdrawals.confirmProcessing') }}
            </Button>
          </div>

          <!-- Complete -->
          <div v-if="canComplete" class="space-y-3 border-t border-border pt-4">
            <Label class="block text-sm font-semibold">{{ t('admin.walletWithdrawals.complete') }}</Label>
            <div>
              <Label class="mb-1 block text-xs text-muted-foreground">TxID</Label>
              <Input v-model="txid" :placeholder="t('admin.walletWithdrawals.txidPlaceholder')" class="font-mono" />
            </div>
            <Button :disabled="actionLoading || !txid.trim()" @click="doComplete">
              {{ t('admin.walletWithdrawals.confirmComplete') }}
            </Button>
          </div>
        </div>
      </template>
    </div>
  </ComplianceGuardWrapper>
</template>
