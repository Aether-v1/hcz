<template>
  <div class="pb-8 text-foreground">
    <!-- Header -->
    <div class="mb-4">
      <h1 class="text-xl font-bold tracking-tight md:text-2xl">{{ t('c2c.title') }}</h1>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('c2c.subtitle') }}</p>
    </div>

    <!-- Wallet Card -->
    <div class="mb-4 rounded-2xl border bg-card p-5 shadow-sm">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2 text-sm text-muted-foreground">
          <div class="grid h-9 w-9 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Wallet :size="18" :stroke-width="1.8" />
          </div>
          <span>{{ t('c2c.wallet.available') }}</span>
        </div>
        <span class="font-mono text-xs text-muted-foreground">{{ wallet?.currency || 'USDT' }}</span>
      </div>
      <div class="mt-3 font-mono text-2xl font-bold tracking-tight">
        {{ formatUsdt(wallet?.available_balance, wallet?.currency) }}
      </div>
      <div class="mt-4 grid grid-cols-2 gap-4 border-t border-border/60 pt-4">
        <div>
          <div class="text-xs text-muted-foreground">{{ t('c2c.wallet.frozen') }}</div>
          <div class="mt-1 font-mono text-sm font-semibold">{{ formatUsdt(wallet?.frozen_balance, wallet?.currency) }}</div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">{{ t('c2c.wallet.total') }}</div>
          <div class="mt-1 font-mono text-sm font-semibold">{{ formatUsdt(wallet?.total_balance, wallet?.currency) }}</div>
        </div>
      </div>
    </div>

    <!-- Entry Cards 2x2 -->
    <div class="mb-4 grid grid-cols-2 gap-3">
      <router-link
        v-for="entry in entries"
        :key="entry.to"
        :to="entry.to"
        class="flex items-center gap-2.5 rounded-2xl border bg-card px-3 py-3 shadow-sm transition-colors hover:bg-accent/40"
      >
        <div class="grid h-8 w-8 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
          <component :is="entry.icon" :size="16" :stroke-width="1.8" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-sm font-semibold leading-tight">{{ entry.title }}</div>
          <div class="mt-1 truncate text-xs leading-snug text-muted-foreground">{{ entry.desc }}</div>
        </div>
      </router-link>
    </div>

    <!-- Recent Trades Summary -->
    <div v-if="recentTrades.length > 0" class="rounded-2xl border bg-card shadow-sm">
      <div class="flex items-center justify-between border-b px-5 py-3">
        <h2 class="text-sm font-semibold">{{ t('c2c.myTrades.title') }}</h2>
        <router-link to="/c2c/trades" class="text-xs text-primary hover:underline">
          {{ t('c2c.viewAll') }}
        </router-link>
      </div>
      <div class="divide-y divide-border/60">
        <router-link
          v-for="trade in recentTrades"
          :key="trade.id"
          :to="`/c2c/trades/${trade.id}`"
          class="flex items-center justify-between gap-3 px-5 py-3 transition-colors hover:bg-accent/40"
        >
          <div class="min-w-0 flex-1">
            <div class="font-mono text-xs text-muted-foreground">{{ trade.trade_no }}</div>
            <div class="mt-0.5 truncate text-sm font-medium">
              {{ formatUsdt(trade.usdt_amount) }} · {{ trade.fiat_amount }} {{ trade.fiat_currency }}
            </div>
          </div>
          <Badge size="sm" :class="TRADE_STATUS_VARIANTS[trade.status]">
            {{ TRADE_STATUS_LABELS[trade.status] }}
          </Badge>
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  TrendingDown,
  TrendingUp,
  Receipt,
  ListOrdered,
  Wallet,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { useC2CStore } from '@/stores/c2c'
import {
  TRADE_STATUS_LABELS,
  TRADE_STATUS_VARIANTS,
} from '@/composables/useC2C'
import { formatUsdt } from '@/utils/money'

const { t } = useI18n()
const c2cStore = useC2CStore()

const wallet = computed(() => c2cStore.wallet)
const recentTrades = computed(() => c2cStore.myTrades.slice(0, 3))

const entries = [
  {
    to: '/c2c/buy',
    icon: TrendingDown,
    title: t('c2c.buy'),
    desc: t('c2c.entries.buyDesc'),
  },
  {
    to: '/c2c/sell',
    icon: TrendingUp,
    title: t('c2c.sell.title'),
    desc: t('c2c.entries.sellDesc'),
  },
  {
    to: '/c2c/trades',
    icon: Receipt,
    title: t('c2c.myTrades.title'),
    desc: t('c2c.entries.tradesDesc'),
  },
  {
    to: '/c2c/my-listings',
    icon: ListOrdered,
    title: t('c2c.myListings.title'),
    desc: t('c2c.entries.listingsDesc'),
  },
] as const

onMounted(() => {
  void c2cStore.fetchWallet()
  void c2cStore.fetchMyTrades({ page: 1, page_size: 3 })
})
</script>
