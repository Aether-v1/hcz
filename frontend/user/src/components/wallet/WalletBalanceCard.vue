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

    <!-- 加载失败：显示错误 + 重试，禁止假 0 -->
    <div v-if="error" class="flex flex-1 flex-col items-center justify-center gap-3 py-10 text-center">
      <p class="text-sm font-medium text-destructive">{{ t('personalCenter.wallet.errors.loadFailed') }}</p>
      <Button type="button" variant="outline" size="sm" :disabled="loading" @click="$emit('retry')">
        {{ loading ? t('common.loading') : t('personalCenter.wallet.retry') }}
      </Button>
    </div>

    <template v-else>
      <!-- 总资产 -->
      <div class="flex flex-1 flex-col justify-center py-6">
        <p class="text-sm text-muted-foreground">{{ t('personalCenter.wallet.totalBalanceLabel') }}</p>
        <p class="mt-2 break-words text-3xl font-bold tracking-tight tabular-nums text-foreground sm:text-4xl">{{ totalBalanceDisplay }}</p>
      </div>

      <!-- 可用 / 冻结 两栏 -->
      <div class="grid grid-cols-2 gap-4 border-t border-sky-200/80 pt-5 dark:border-slate-600">
        <div>
          <p class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.availableLabel') }}</p>
          <p class="mt-1 text-lg font-semibold tabular-nums text-foreground">{{ availableBalanceDisplay }}</p>
        </div>
        <div>
          <div class="flex items-center gap-1">
            <p class="text-xs text-muted-foreground">{{ t('personalCenter.wallet.frozenLabel') }}</p>
            <span
              class="inline-flex cursor-help items-center rounded-full bg-slate-200/60 px-1.5 text-[10px] font-medium text-slate-500 dark:bg-slate-600/60 dark:text-slate-300"
              :title="frozenNote"
            >?</span>
          </div>
          <p class="mt-1 text-lg font-semibold tabular-nums text-amber-600 dark:text-amber-400">{{ frozenBalanceDisplay }}</p>
        </div>
      </div>
    </template>

    <div class="mt-4 flex items-center justify-between border-t border-sky-200/80 pt-4 text-sm dark:border-slate-600">
      <span class="text-muted-foreground">{{ t('personalCenter.wallet.transactionsLabel') }}</span>
      <span class="font-semibold tabular-nums text-foreground">{{ totalTransactions }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Wallet } from 'lucide-vue-next'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import { formatUsdt } from '../../utils/money'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  alert: PageAlert | null
  totalBalance: string
  availableBalance: string
  frozenBalance: string
  currency: string
  frozenNote: string
  totalTransactions: number
  error?: boolean
  loading?: boolean
}>()

defineEmits<{
  (e: 'retry'): void
}>()

const { t } = useI18n()

const totalBalanceDisplay = computed(() => formatUsdt(props.totalBalance, props.currency))
const availableBalanceDisplay = computed(() => formatUsdt(props.availableBalance, props.currency))
const frozenBalanceDisplay = computed(() => formatUsdt(props.frozenBalance, props.currency))
</script>
