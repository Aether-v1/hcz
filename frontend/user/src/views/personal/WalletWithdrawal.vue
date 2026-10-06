<template>
  <div class="space-y-6">
    <!-- Balance Card -->
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <PanelHeading
        eyebrow="USDT"
        :title="t('personalCenter.wallet.withdraw.balanceTitle')"
        :description="t('personalCenter.wallet.withdraw.balanceSubtitle')"
        :icon="Wallet"
      />
      <div class="mt-2 text-3xl font-black text-foreground">
        {{ balanceDisplay }}
      </div>
    </div>

    <!-- Alert -->
    <div v-if="alert" class="rounded-xl border p-4 text-sm" :class="alertClass">
      {{ alert }}
    </div>

    <!-- Withdrawal Form -->
    <form class="rounded-2xl border bg-card p-7 shadow-sm space-y-5" @submit.prevent="handleSubmit">
      <PanelHeading
        :title="t('personalCenter.wallet.withdraw.formTitle')"
        :description="t('personalCenter.wallet.withdraw.formSubtitle')"
        :icon="ArrowUpRight"
      />

      <!-- Network (fixed TRC20) -->
      <div>
        <Label class="mb-2 block">{{ t('personalCenter.wallet.withdraw.networkLabel') }}</Label>
        <Input value="TRC20" disabled class="h-11 bg-muted" />
        <p class="mt-1 text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.networkFixedHint') }}</p>
      </div>

      <!-- Saved Addresses Dropdown -->
      <div v-if="savedAddresses.length > 0">
        <Label class="mb-2 block">{{ t('personalCenter.wallet.withdraw.savedAddressLabel') }}</Label>
        <Select v-model="selectedSavedAddressId" @update:model-value="(val: any) => onSelectSavedAddress(String(val ?? ''))">
          <SelectTrigger class="h-11 w-full">
            <SelectValue :placeholder="t('personalCenter.wallet.withdraw.savedAddressPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">{{ t('personalCenter.wallet.withdraw.savedAddressManual') }}</SelectItem>
            <SelectItem v-for="addr in savedAddresses" :key="addr.id" :value="String(addr.id)">
              {{ addr.label || addr.address }} {{ addr.is_default ? '★' : '' }}
            </SelectItem>
          </SelectContent>
        </Select>

        <!-- Address Management -->
        <div class="mt-3 rounded-xl border border-border bg-muted/30 p-3">
          <div class="mb-2 text-xs font-semibold text-muted-foreground">
            {{ t('personalCenter.wallet.withdraw.addressManageTitle') }}
          </div>
          <div class="space-y-2">
            <div
              v-for="addr in savedAddresses"
              :key="addr.id"
              class="flex items-center justify-between gap-2 rounded-lg border bg-card px-3 py-2"
            >
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-foreground">{{ addr.label || t('personalCenter.wallet.withdraw.addressLabel') }}</span>
                  <span v-if="addr.is_default" class="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-semibold text-primary">★</span>
                </div>
                <div class="mt-0.5 font-mono text-xs text-muted-foreground">{{ abbreviateAddress(addr.address) }}</div>
              </div>
              <div class="flex shrink-0 items-center gap-1">
                <Button
                  v-if="!addr.is_default"
                  type="button"
                  size="sm"
                  variant="outline"
                  class="h-7 text-xs"
                  @click="handleSetDefaultAddress(addr.id)"
                >
                  {{ t('personalCenter.wallet.withdraw.setDefault') }}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  class="h-7 text-xs text-destructive hover:text-destructive"
                  @click="handleDeleteAddress(addr.id)"
                >
                  {{ t('personalCenter.wallet.withdraw.deleteAddress') }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Address Input -->
      <div>
        <Label class="mb-2 block">{{ t('personalCenter.wallet.withdraw.addressLabel') }}</Label>
        <Input
          v-model="address"
          type="text"
          :placeholder="t('personalCenter.wallet.withdraw.addressPlaceholder')"
          class="h-11 font-mono"
          autocomplete="off"
        />
      </div>

      <!-- Save Address -->
      <div class="flex items-center gap-2">
        <Checkbox id="saveAddress" v-model="saveAddress" />
        <Label for="saveAddress" class="text-sm font-normal cursor-pointer">
          {{ t('personalCenter.wallet.withdraw.saveAddressLabel') }}
        </Label>
        <Input
          v-if="saveAddress"
          v-model="addressLabel"
          type="text"
          :placeholder="t('personalCenter.wallet.withdraw.addressLabelPlaceholder')"
          class="h-9 w-48"
        />
      </div>

      <!-- Amount Input -->
      <div>
        <Label class="mb-2 block">{{ t('personalCenter.wallet.withdraw.amountLabel') }}</Label>
        <div class="relative">
          <Input
            v-model="amount"
            type="text"
            inputmode="decimal"
            :placeholder="t('personalCenter.wallet.withdraw.amountPlaceholder')"
            class="h-11 pr-16 font-mono"
          />
          <span class="absolute right-3 top-1/2 -translate-y-1/2 text-sm font-semibold text-muted-foreground">USDT</span>
        </div>
      </div>

      <!-- Fee / Net Preview -->
      <div v-if="quote" class="grid grid-cols-2 gap-3">
        <div class="rounded-xl border border-border p-4">
          <div class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.withdraw.feeLabel') }}</div>
          <div class="mt-1 font-semibold font-mono text-foreground">{{ formatUsdt(quote.fee_amount, 'USDT') }}</div>
        </div>
        <div class="rounded-xl border border-primary/40 bg-primary/5 p-4">
          <div class="text-xs text-primary">{{ t('personalCenter.wallet.withdraw.netLabel') }}</div>
          <div class="mt-1 font-semibold font-mono text-primary">{{ formatUsdt(quote.net_amount, 'USDT') }}</div>
        </div>
      </div>
      <p v-else-if="quoteLoading" class="text-xs text-muted-foreground">
        {{ t('personalCenter.wallet.withdraw.quoteLoading') }}
      </p>

      <!-- TOTP Code -->
      <div>
        <Label class="mb-2 block">{{ t('personalCenter.wallet.withdraw.totpLabel') }}</Label>
        <Input
          v-model="totpCode"
          type="text"
          inputmode="numeric"
          maxlength="6"
          :placeholder="t('personalCenter.wallet.withdraw.totpPlaceholder')"
          class="h-11 font-mono tracking-[0.3em]"
          autocomplete="one-time-code"
        />
      </div>

      <!-- Submit Button -->
      <Button
        type="submit"
        :disabled="submitting || !canSubmit"
        class="h-12 w-full font-bold text-base"
      >
        {{ submitting ? t('personalCenter.wallet.withdraw.submitting') : t('personalCenter.wallet.withdraw.submit') }}
      </Button>
    </form>

    <!-- History Link -->
    <div class="text-center">
      <router-link to="/me/wallet/withdrawal-history" class="text-sm text-primary underline-offset-4 hover:underline">
        {{ t('personalCenter.wallet.withdraw.viewHistory') }}
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Wallet, ArrowUpRight } from 'lucide-vue-next'
import { walletAPI } from '../../api'
import type { WithdrawalAddress, WithdrawalQuote } from '../../api/types'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { formatUsdt } from '../../utils/money'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const { t } = useI18n()
const router = useRouter()

const wallet = ref<any>(null)
const savedAddresses = ref<WithdrawalAddress[]>([])
const selectedSavedAddressId = ref<string>('')
const address = ref('')
const saveAddress = ref(false)
const addressLabel = ref('')
const amount = ref('')
const totpCode = ref('')
const submitting = ref(false)
const alert = ref<string>('')
const alertType = ref<'error' | 'warning' | 'success'>('error')
const quote = ref<WithdrawalQuote | null>(null)
const quoteLoading = ref(false)
let quoteTimer: number | null = null
let quoteSeq = 0

// 可提现余额：走 formatUsdt。钱包未加载（null）时显示 '--'，不显示假 0.00。
const balanceDisplay = computed(() => {
  const ccy = String(wallet.value?.currency || 'USDT')
  return formatUsdt(wallet.value?.available_balance, ccy)
})

const alertClass = computed(() => {
  if (alertType.value === 'error') return 'border-destructive/50 bg-destructive/10 text-destructive'
  if (alertType.value === 'warning') return 'border-warning/50 bg-warning/10 text-warning'
  return 'border-green-500/50 bg-green-500/10 text-green-600'
})

const canSubmit = computed(() => {
  return address.value.trim().length > 0 &&
    amount.value.trim().length > 0 &&
    totpCode.value.trim().length === 6
})

const showAlert = (msg: string, type: 'error' | 'warning' | 'success' = 'error') => {
  alert.value = msg
  alertType.value = type
}

const loadWallet = async () => {
  try {
    const res = await walletAPI.account()
    wallet.value = res.data.data
  } catch {
    // ignore
  }
}

const loadAddresses = async () => {
  try {
    const res = await walletAPI.listWithdrawalAddresses()
    savedAddresses.value = res.data.data?.list || res.data.data || []
    // auto-select default
    const def = savedAddresses.value.find(a => a.is_default)
    if (def) {
      selectedSavedAddressId.value = String(def.id)
      address.value = def.address
    }
  } catch {
    // ignore
  }
}

const onSelectSavedAddress = (val: string) => {
  selectedSavedAddressId.value = val
  if (!val) {
    address.value = ''
    return
  }
  const addr = savedAddresses.value.find(a => String(a.id) === val)
  if (addr) {
    address.value = addr.address
  }
}

const abbreviateAddress = (addr: string) => {
  if (!addr || addr.length <= 12) return addr
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`
}

const handleSetDefaultAddress = async (id: number) => {
  try {
    await walletAPI.setDefaultWithdrawalAddress(id)
    showAlert(t('personalCenter.wallet.withdraw.setDefaultSuccess'), 'success')
    await loadAddresses()
  } catch (err: any) {
    showAlert(err?.message || t('personalCenter.wallet.withdraw.errors.submitFailed'))
  }
}

const handleDeleteAddress = async (id: number) => {
  if (!window.confirm(t('personalCenter.wallet.withdraw.deleteAddressConfirm'))) return
  try {
    await walletAPI.deleteWithdrawalAddress(id)
    showAlert(t('personalCenter.wallet.withdraw.deleteAddressSuccess'), 'success')
    // If deleted address was currently selected, clear address input
    if (selectedSavedAddressId.value === String(id)) {
      selectedSavedAddressId.value = ''
      address.value = ''
    }
    await loadAddresses()
  } catch (err: any) {
    showAlert(err?.message || t('personalCenter.wallet.withdraw.errors.submitFailed'))
  }
}

const fetchQuote = async () => {
  const amt = amount.value.trim()
  if (!amt || isNaN(Number(amt)) || Number(amt) <= 0) {
    quote.value = null
    return
  }
  const seq = ++quoteSeq
  quoteLoading.value = true
  try {
    const res = await walletAPI.quoteWithdrawal({ network: 'TRC20', amount: amt })
    if (seq === quoteSeq) {
      quote.value = res.data.data
    }
  } catch {
    if (seq === quoteSeq) quote.value = null
  } finally {
    if (seq === quoteSeq) quoteLoading.value = false
  }
}

watch(amount, () => {
  if (quoteTimer) window.clearTimeout(quoteTimer)
  quoteTimer = window.setTimeout(() => void fetchQuote(), 400)
})

const handleSubmit = async () => {
  alert.value = ''
  const addr = address.value.trim()
  const amt = amount.value.trim()
  const code = totpCode.value.trim()

  if (!addr) {
    showAlert(t('personalCenter.wallet.withdraw.errors.addressRequired'))
    return
  }
  if (!amt || isNaN(Number(amt)) || Number(amt) <= 0) {
    showAlert(t('personalCenter.wallet.withdraw.errors.invalidAmount'))
    return
  }
  if (!/^\d{6}$/.test(code)) {
    showAlert(t('personalCenter.wallet.withdraw.errors.totpRequired'))
    return
  }

  submitting.value = true
  try {
    // Optionally save address first
    if (saveAddress.value && addressLabel.value.trim()) {
      try {
        await walletAPI.createWithdrawalAddress({
          network: 'TRC20',
          address: addr,
          label: addressLabel.value.trim(),
        })
      } catch {
        // non-blocking
      }
    }
    const idemKey = crypto.randomUUID()
    await walletAPI.createWithdrawal(
      { network: 'TRC20', address: addr, amount: amt, totp_code: code },
      idemKey,
    )
    showAlert(t('personalCenter.wallet.withdraw.submitSuccess'), 'success')
    amount.value = ''
    totpCode.value = ''
    quote.value = null
    // Refresh balance
    await loadWallet()
    // Navigate to history
    router.push('/me/wallet/withdrawal-history')
  } catch (err: any) {
    showAlert(err?.message || t('personalCenter.wallet.withdraw.errors.submitFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void loadWallet()
  void loadAddresses()
})
</script>
