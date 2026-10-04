<template>
  <div>
    <PanelHeading :title="t('personalCenter.wallet.title')" :description="t('personalCenter.wallet.subtitle')" :icon="Wallet">
      <template #actions>
        <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.wallet') }}</Badge>
      </template>
    </PanelHeading>

    <Alert v-if="alert" class="mb-5" :variant="pageAlertVariant(alert.level)" :class="pageAlertToneClass(alert.level)">
      <AlertDescription>{{ alert.message }}</AlertDescription>
    </Alert>

    <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
      <StatCard :label="t('wallet.availableBalance')" :value="availableBalanceDisplay" :icon="Banknote" tone="accent" mono />
      <StatCard :label="t('wallet.frozenBalance')" :value="frozenBalanceDisplay" :icon="Lock" tone="warning" mono />
      <StatCard :label="t('wallet.totalBalance')" :value="totalBalanceDisplay" :icon="Wallet" tone="info" mono />
    </div>

    <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
      <StatCard :label="t('personalCenter.wallet.transactionsLabel')" :value="totalTransactions" :icon="ReceiptText" tone="neutral" mono />
      <StatCard
        :label="t('personalCenter.wallet.currentPageLabel')"
        :value="t('orders.pageInfo', { page: currentPage, total: totalPages })"
        :icon="Layers"
        tone="neutral"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Wallet, Banknote, ReceiptText, Layers, Lock } from 'lucide-vue-next'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import PanelHeading from '../shared/PanelHeading.vue'
import StatCard from '../shared/StatCard.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'

defineProps<{
  alert: PageAlert | null
  availableBalanceDisplay: string
  frozenBalanceDisplay: string
  totalBalanceDisplay: string
  totalTransactions: number
  currentPage: number
  totalPages: number
}>()

const { t } = useI18n()
</script>

