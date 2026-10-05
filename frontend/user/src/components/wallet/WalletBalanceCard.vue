<template>
  <div class="flex h-full flex-col rounded-2xl border border-sky-100 bg-[linear-gradient(145deg,#eaf5fb_0%,#f9fbfc_100%)] p-6 dark:border-slate-700 dark:bg-[linear-gradient(145deg,#263747_0%,#1f2b35_100%)] md:p-7">
    <div class="flex items-center gap-3">
      <span class="flex h-10 w-10 items-center justify-center rounded-xl bg-white/80 text-slate-700 dark:bg-slate-700 dark:text-slate-200">
        <Wallet class="h-5 w-5" aria-hidden="true" />
      </span>
      <div>
        <h1 class="text-lg font-bold text-foreground">{{ t('personalCenter.wallet.title') }}</h1>
        <p class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.subtitle') }}</p>
      </div>
    </div>

    <Alert v-if="alert" class="mt-5" :variant="pageAlertVariant(alert.level)" :class="pageAlertToneClass(alert.level)">
      <AlertDescription>{{ alert.message }}</AlertDescription>
    </Alert>

    <div class="flex flex-1 flex-col justify-center py-7">
      <p class="text-sm text-muted-foreground">{{ t('personalCenter.wallet.balanceLabel') }}</p>
      <p class="mt-2 break-words text-3xl font-bold tracking-tight tabular-nums text-foreground sm:text-4xl">{{ balanceDisplay }}</p>
    </div>

    <div class="mt-auto flex items-center justify-between border-t border-sky-200/80 pt-5 text-sm dark:border-slate-600">
      <span class="text-muted-foreground">{{ t('personalCenter.wallet.transactionsLabel') }}</span>
      <span class="font-semibold tabular-nums text-foreground">{{ totalTransactions }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Wallet } from 'lucide-vue-next'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import { Alert, AlertDescription } from '@/components/ui/alert'

defineProps<{
  alert: PageAlert | null
  balanceDisplay: string
  totalTransactions: number
}>()

const { t } = useI18n()
</script>
