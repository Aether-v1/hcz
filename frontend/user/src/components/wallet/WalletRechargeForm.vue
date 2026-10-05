<template>
  <div class="rounded-2xl border bg-card p-6 shadow-sm md:p-7">
    <div class="mb-6">
      <h2 class="text-lg font-bold text-foreground">{{ t('personalCenter.wallet.rechargeTitle') }}</h2>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('personalCenter.wallet.rechargeSubtitle') }}</p>
    </div>
    <form class="grid grid-cols-1 gap-4 sm:grid-cols-2" @submit.prevent="$emit('submit')">
      <div class="sm:col-span-2">
        <Label class="mb-2 block text-sm font-semibold">{{ t('personalCenter.wallet.amountLabel') }} · {{ currency }}</Label>
        <Input
          :model-value="amount"
          @update:model-value="(v) => $emit('update:amount', String(v).trim())"
          type="text"
          inputmode="decimal"
          :placeholder="t('personalCenter.wallet.amountPlaceholder')"
          class="h-12 text-base"
        />
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <span class="mr-1 text-xs text-muted-foreground">{{ t('personalCenter.wallet.quickAmounts') }}</span>
          <button v-for="value in quickAmounts" :key="value" type="button" :disabled="recharging"
            :aria-pressed="amount === String(value)"
            class="rounded-full border px-3 py-1.5 text-xs font-semibold tabular-nums transition-colors disabled:opacity-50"
            :class="amount === String(value) ? 'border-slate-900 bg-slate-900 text-white dark:border-slate-200 dark:bg-slate-200 dark:text-slate-900' : 'bg-card text-muted-foreground hover:border-foreground hover:text-foreground'"
            @click="$emit('update:amount', String(value))">{{ value }}</button>
        </div>
      </div>
      <div>
        <Label class="mb-2 block">{{ t('personalCenter.wallet.channelLabel') }}</Label>
        <Select
          :model-value="String(channelId)"
          @update:model-value="(v) => $emit('update:channelId', Number(v))"
          :disabled="!hasChannels || channelLoading || recharging"
        >
          <SelectTrigger class="h-11 w-full">
            <SelectValue :placeholder="t('personalCenter.wallet.channelPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="0">{{ t('personalCenter.wallet.channelPlaceholder') }}</SelectItem>
            <SelectItem v-for="channel in channels" :key="channel.id" :value="String(channel.id)">
              {{ channel.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div>
        <Label class="mb-2 block">{{ t('personalCenter.wallet.remarkLabel') }}</Label>
        <Input
          :model-value="remark"
          @update:model-value="(v) => $emit('update:remark', String(v).trim())"
          type="text"
          :placeholder="t('personalCenter.wallet.remarkPlaceholder')"
          class="h-11"
        />
      </div>
      <div class="sm:col-span-2">
        <Button
          type="submit"
          :disabled="recharging || channelLoading || !hasChannels"
          class="h-11 w-full bg-slate-900 font-bold text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
        >
          {{ recharging ? t('personalCenter.wallet.recharging') : t('personalCenter.wallet.rechargeSubmit') }}
        </Button>
      </div>
    </form>
    <div v-if="selectedChannel?.fee_policy === 'customer_surcharge'" class="mt-4 grid grid-cols-1 gap-3 text-sm md:grid-cols-3">
      <div class="rounded-xl border border-warning/40 p-4">
        <div class="text-xs text-warning">{{ t('payment.feeRateLabel') }}</div>
        <div class="mt-1 font-semibold text-foreground">{{ feeRateDisplay }}</div>
      </div>
      <div class="rounded-xl border border-warning/40 p-4">
        <div class="text-xs text-warning">{{ t('payment.fixedFeeLabel') }}</div>
        <div class="mt-1 font-semibold text-foreground">{{ fixedFeeDisplay }}</div>
      </div>
      <div class="rounded-xl border border-warning/40 p-4">
        <div class="text-xs text-warning">{{ t('payment.feeAmountLabel') }}</div>
        <div class="mt-1 font-semibold text-foreground">{{ feeAmountDisplay }}</div>
      </div>
    </div>
    <p v-if="channelLoading" class="mt-3 text-xs text-muted-foreground">
      {{ t('common.loading') }}
    </p>
    <p v-else-if="!hasChannels" class="mt-3 text-xs text-amber-600">
      {{ t('payment.channelEmpty') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

defineProps<{
  amount: string
  currency: string
  channelId: number
  remark: string
  channels: Array<{ id: number; name: string }>
  hasChannels: boolean
  channelLoading: boolean
  recharging: boolean
  selectedChannel: { id: number; name: string; fee_policy?: string } | null
  feeRateDisplay: string
  fixedFeeDisplay: string
  feeAmountDisplay: string
}>()

defineEmits<{
  (e: 'submit'): void
  (e: 'update:amount', value: string): void
  (e: 'update:channelId', value: number): void
  (e: 'update:remark', value: string): void
}>()

const { t } = useI18n()
const quickAmounts = [50, 100, 200, 500]
</script>

