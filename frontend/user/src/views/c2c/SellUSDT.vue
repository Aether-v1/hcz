<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <!-- Header -->
      <div class="mb-6 mt-8">
        <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('c2c.sell.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('c2c.sell.subtitle') }}</p>
      </div>

      <!-- Wallet -->
      <div class="grid grid-cols-3 gap-3 max-w-2xl">
        <div class="rounded-2xl border bg-card p-4 shadow-sm">
          <div class="text-xs text-muted-foreground">{{ t('c2c.wallet.available') }}</div>
          <div class="mt-1 text-lg font-bold font-mono">{{ wallet?.available_balance ?? '0' }}</div>
        </div>
        <div class="rounded-2xl border bg-card p-4 shadow-sm">
          <div class="text-xs text-muted-foreground">{{ t('c2c.wallet.frozen') }}</div>
          <div class="mt-1 text-lg font-bold font-mono">{{ wallet?.frozen_balance ?? '0' }}</div>
        </div>
        <div class="rounded-2xl border bg-card p-4 shadow-sm">
          <div class="text-xs text-muted-foreground">{{ t('c2c.wallet.total') }}</div>
          <div class="mt-1 text-lg font-bold font-mono">{{ wallet?.total_balance ?? '0' }}</div>
        </div>
      </div>

      <!-- No payment method guide -->
      <div
        v-if="!c2cStore.hasEnabledPaymentMethod"
        class="mt-6 max-w-2xl rounded-2xl border border-amber-500/40 bg-amber-500/10 p-6"
      >
        <p class="text-sm font-medium text-amber-700">{{ t('c2c.sell.noPaymentMethod') }}</p>
        <Button class="mt-4" @click="goPaymentMethods">{{ t('c2c.sell.goAddPayment') }}</Button>
      </div>

      <!-- Form -->
      <form
        v-else
        class="mt-6 max-w-2xl rounded-2xl border bg-card p-6 shadow-sm space-y-5"
        @submit.prevent="handleSubmit"
      >
        <div>
          <Label class="mb-2 block">{{ t('c2c.sell.fiatCurrency') }}</Label>
          <Input v-model="form.fiat_currency" class="h-11" placeholder="CNY" />
        </div>

        <div>
          <Label class="mb-2 block">{{ t('c2c.sell.price') }}</Label>
          <Input v-model="form.price" inputmode="decimal" class="h-11 font-mono" />
        </div>

        <div>
          <Label class="mb-2 block">{{ t('c2c.sell.totalUsdt') }}</Label>
          <Input v-model="form.total_usdt" inputmode="decimal" class="h-11 font-mono" />
          <p class="mt-1 text-xs text-muted-foreground">{{ t('c2c.wallet.available') }}：{{ wallet?.available_balance ?? '0' }} USDT</p>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <Label class="mb-2 block">{{ t('c2c.sell.minFiat') }}</Label>
            <Input v-model="form.min_fiat_amount" inputmode="decimal" class="h-11 font-mono" />
          </div>
          <div>
            <Label class="mb-2 block">{{ t('c2c.sell.maxFiat') }}</Label>
            <Input v-model="form.max_fiat_amount" inputmode="decimal" class="h-11 font-mono" />
          </div>
        </div>

        <div>
          <Label class="mb-2 block">{{ t('c2c.sell.terms') }}</Label>
          <Textarea v-model="form.terms" class="min-h-[80px]" />
        </div>

        <p v-if="errorMsg" class="text-sm text-destructive">{{ errorMsg }}</p>

        <Button type="submit" class="h-12 w-full font-semibold" :disabled="submitting">
          {{ submitting ? '...' : t('c2c.sell.submit') }}
        </Button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useC2CStore } from '@/stores/c2c'
import { c2cAPI } from '@/api/c2c'
import { toast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

const { t } = useI18n()
const router = useRouter()
const c2cStore = useC2CStore()

const wallet = computed(() => c2cStore.wallet)

const form = reactive({
  fiat_currency: 'CNY',
  price: '',
  total_usdt: '',
  min_fiat_amount: '',
  max_fiat_amount: '',
  terms: '',
})

const submitting = ref(false)
const errorMsg = ref('')

const goPaymentMethods = () => {
  void router.push('/c2c/payment-methods')
}

const validate = (): string => {
  const price = Number(form.price)
  const total = Number(form.total_usdt)
  const min = Number(form.min_fiat_amount)
  const max = Number(form.max_fiat_amount)
  const available = Number(wallet.value?.available_balance ?? 0)

  if (!form.fiat_currency.trim()) return t('c2c.sell.fiatCurrency') + ' 不能为空'
  if (!(price > 0)) return t('c2c.sell.price') + ' 必须大于 0'
  if (!(total > 0)) return t('c2c.sell.totalUsdt') + ' 必须大于 0'
  if (!(min > 0)) return t('c2c.sell.minFiat') + ' 必须大于 0'
  if (!(max >= min)) return t('c2c.sell.maxFiat') + ' 不能小于最小成交额'
  if (total > available) return t('c2c.errors.insufficientBalance')
  return ''
}

const handleSubmit = async () => {
  errorMsg.value = ''
  const err = validate()
  if (err) {
    errorMsg.value = err
    return
  }
  submitting.value = true
  try {
    await c2cAPI.createListing({
      fiat_currency: form.fiat_currency.trim().toUpperCase(),
      price: form.price,
      total_usdt: form.total_usdt,
      min_fiat_amount: form.min_fiat_amount,
      max_fiat_amount: form.max_fiat_amount,
      terms: form.terms.trim() || undefined,
    })
    toast.success(t('c2c.sell.success'))
    void router.push('/c2c/my-listings')
  } catch (e) {
    errorMsg.value = e instanceof Error ? e.message : String(e)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void c2cStore.fetchWallet()
  void c2cStore.fetchPaymentMethods()
})
</script>
