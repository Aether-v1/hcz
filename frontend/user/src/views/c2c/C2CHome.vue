<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <!-- Header -->
      <div class="mb-8 mt-8">
        <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('c2c.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">P2P USDT 场外交易，安全托管</p>
      </div>

      <!-- Wallet Card -->
      <Card class="mb-6 overflow-hidden">
        <CardContent class="p-6">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 text-sm text-muted-foreground">
              <Wallet class="w-4 h-4" />
              <span>{{ t('c2c.wallet.available') }}</span>
            </div>
            <span class="text-xs text-muted-foreground font-mono">{{ wallet?.currency || 'USDT' }}</span>
          </div>
          <div class="mt-2 text-3xl font-bold tracking-tight font-mono">
            {{ wallet?.available_balance || '0.00' }}
          </div>
          <div class="mt-4 grid grid-cols-2 gap-4 pt-4 border-t border-border/60">
            <div>
              <div class="text-xs text-muted-foreground">{{ t('c2c.wallet.frozen') }}</div>
              <div class="mt-1 text-sm font-semibold font-mono">{{ wallet?.frozen_balance || '0.00' }}</div>
            </div>
            <div>
              <div class="text-xs text-muted-foreground">{{ t('c2c.wallet.total') }}</div>
              <div class="mt-1 text-sm font-semibold font-mono">{{ wallet?.total_balance || '0.00' }}</div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Entry Cards 2x2 -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-8">
        <router-link
          v-for="entry in entries"
          :key="entry.to"
          :to="entry.to"
          class="group rounded-2xl border bg-card p-5 shadow-sm hover:shadow-md hover:border-primary/40 transition-all"
        >
          <div class="flex items-start justify-between">
            <div
              class="flex h-11 w-11 items-center justify-center rounded-xl"
              :class="entry.box"
            >
              <component :is="entry.icon" class="h-5 w-5" />
            </div>
            <ArrowRight class="w-4 h-4 text-muted-foreground group-hover:text-primary group-hover:translate-x-0.5 transition-all" />
          </div>
          <div class="mt-4 text-base font-semibold">{{ entry.title }}</div>
          <div class="mt-1 text-xs text-muted-foreground">{{ entry.desc }}</div>
        </router-link>
      </div>

      <!-- Recent Trades Summary -->
      <div v-if="recentTrades.length > 0" class="rounded-2xl border bg-card shadow-sm">
        <div class="flex items-center justify-between px-6 py-4 border-b border-border/60">
          <h2 class="text-base font-semibold">{{ t('c2c.myTrades.title') }}</h2>
          <router-link to="/c2c/trades" class="text-xs text-primary hover:underline">
            查看全部
          </router-link>
        </div>
        <div class="divide-y divide-border/60">
          <router-link
            v-for="trade in recentTrades"
            :key="trade.id"
            :to="`/c2c/trades/${trade.id}`"
            class="flex items-center justify-between gap-3 px-6 py-3.5 hover:bg-muted/40 transition-colors"
          >
            <div class="min-w-0 flex-1">
              <div class="font-mono text-xs text-muted-foreground">{{ trade.trade_no }}</div>
              <div class="mt-0.5 text-sm font-medium truncate">
                {{ trade.usdt_amount }} USDT · {{ trade.fiat_amount }} {{ trade.fiat_currency }}
              </div>
            </div>
            <Badge size="sm" :class="TRADE_STATUS_VARIANTS[trade.status]">
              {{ TRADE_STATUS_LABELS[trade.status] }}
            </Badge>
          </router-link>
        </div>
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
  ArrowRight,
} from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useC2CStore } from '@/stores/c2c'
import {
  TRADE_STATUS_LABELS,
  TRADE_STATUS_VARIANTS,
} from '@/composables/useC2C'

const { t } = useI18n()
const c2cStore = useC2CStore()

const wallet = computed(() => c2cStore.wallet)
const recentTrades = computed(() => c2cStore.myTrades.slice(0, 3))

const entries = [
  {
    to: '/c2c/buy',
    icon: TrendingDown,
    title: t('c2c.buy'),
    desc: '从其他用户的挂单中购买 USDT',
    box: 'bg-emerald-500/10 text-emerald-600',
  },
  {
    to: '/c2c/sell',
    icon: TrendingUp,
    title: t('c2c.sell.title'),
    desc: '发布 SELL 挂单，卖出你的 USDT',
    box: 'bg-blue-500/10 text-blue-600',
  },
  {
    to: '/c2c/trades',
    icon: Receipt,
    title: t('c2c.myTrades.title'),
    desc: '查看进行中与历史交易',
    box: 'bg-amber-500/10 text-amber-600',
  },
  {
    to: '/c2c/my-listings',
    icon: ListOrdered,
    title: t('c2c.myListings.title'),
    desc: '管理你发布的卖单',
    box: 'bg-purple-500/10 text-purple-600',
  },
] as const

onMounted(() => {
  void c2cStore.fetchWallet()
  void c2cStore.fetchMyTrades({ page: 1, page_size: 3 })
})
</script>
