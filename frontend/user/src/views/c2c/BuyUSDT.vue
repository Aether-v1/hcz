<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <!-- Header -->
      <div class="mb-6 mt-8">
        <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('c2c.market.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('c2c.market.subtitle') }}</p>
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

      <!-- Filters -->
      <div class="mt-6 flex flex-wrap items-end gap-3 max-w-3xl">
        <div class="w-32">
          <Label class="mb-2 block">{{ t('c2c.market.filterFiat') }}</Label>
          <Input v-model="fiatFilter" class="h-10" @keyup.enter="applyFilter" />
        </div>
        <div class="flex-1 min-w-40">
          <Label class="mb-2 block">{{ t('c2c.market.filterAmount') }}</Label>
          <Input v-model="amountFilter" inputmode="decimal" class="h-10" :placeholder="t('c2c.market.filterAmount')" />
        </div>
        <Button variant="outline" class="h-10" @click="applyFilter">
          {{ t('c2c.trade.refresh') }}
        </Button>
      </div>

      <!-- Loading -->
      <div v-if="c2cStore.marketLoading && filteredListings.length === 0" class="mt-6 space-y-3">
        <div v-for="i in 5" :key="i" class="h-24 rounded-2xl border bg-muted/60 animate-pulse"></div>
      </div>

      <!-- Empty -->
      <EmptyState
        v-else-if="filteredListings.length === 0"
        icon="inbox"
        variant="soft"
        size="lg"
        class="mt-6"
        :title="t('c2c.market.empty')"
      />

      <!-- List: PC table header (desktop) + rows -->
      <div v-else class="mt-6">
        <!-- desktop header -->
        <div class="hidden md:grid grid-cols-12 gap-3 px-4 py-2 text-xs font-medium text-muted-foreground">
          <div class="col-span-2">{{ t('c2c.market.seller') }}</div>
          <div class="col-span-2">{{ t('c2c.market.price') }}</div>
          <div class="col-span-2">{{ t('c2c.market.available') }}</div>
          <div class="col-span-3">{{ t('c2c.market.minMax') }}</div>
          <div class="col-span-2">{{ t('c2c.market.terms') }}</div>
          <div class="col-span-1 text-right"></div>
        </div>

        <div
          v-for="listing in filteredListings"
          :key="listing.id"
          class="mb-3 rounded-2xl border bg-card p-4 shadow-sm md:grid md:grid-cols-12 md:gap-3 md:items-center"
        >
          <!-- seller -->
          <div class="md:col-span-2 flex items-center gap-2">
            <span class="font-medium text-sm">{{ t('c2c.userPrefix') }}{{ listing.seller_user_id }}</span>
            <Badge v-if="isOwnListing(listing)" size="sm" class="bg-primary/10 text-primary">{{ t('c2c.market.myListing') }}</Badge>
          </div>
          <!-- price -->
          <div class="md:col-span-2 mt-2 md:mt-0">
            <div class="md:hidden text-xs text-muted-foreground">{{ t('c2c.market.price') }}</div>
            <div class="font-mono font-semibold">{{ listing.price }} {{ listing.fiat_currency }}</div>
          </div>
          <!-- available -->
          <div class="md:col-span-2 mt-2 md:mt-0">
            <div class="md:hidden text-xs text-muted-foreground">{{ t('c2c.market.available') }}</div>
            <div class="font-mono text-sm">{{ formatUsdt(listing.available_usdt) }}</div>
          </div>
          <!-- min/max -->
          <div class="md:col-span-3 mt-2 md:mt-0">
            <div class="md:hidden text-xs text-muted-foreground">{{ t('c2c.market.minMax') }}</div>
            <div class="font-mono text-sm">
              {{ listing.min_fiat_amount }} ~ {{ listing.max_fiat_amount }} {{ listing.fiat_currency }}
            </div>
          </div>
          <!-- terms -->
          <div class="md:col-span-2 mt-2 md:mt-0">
            <div class="md:hidden text-xs text-muted-foreground">{{ t('c2c.market.terms') }}</div>
            <div class="text-xs text-muted-foreground truncate max-w-[160px]">{{ listing.terms || '-' }}</div>
          </div>
          <!-- action -->
          <div class="md:col-span-1 mt-3 md:mt-0 md:text-right">
            <Button size="sm" class="w-full md:w-auto" :disabled="isOwnListing(listing)" @click="openBuyPanel(listing)">
              {{ t('c2c.market.buyButton') }}
            </Button>
          </div>
        </div>

        <!-- Load more -->
        <div v-if="hasMore" class="mt-4 text-center">
          <Button variant="outline" :disabled="loadingMore" @click="loadMore">
            {{ loadingMore ? '...' : t('c2c.market.loadMore') }}
          </Button>
        </div>
      </div>
    </div>

    <!-- Buy panel modal -->
    <div v-if="selectedListing" class="fixed inset-0 z-50 flex items-end md:items-center justify-center bg-black/50 p-0 md:p-4" @click.self="closeBuyPanel">
      <div class="w-full md:max-w-md rounded-t-2xl md:rounded-2xl bg-card p-6 shadow-xl">
        <h3 class="text-base font-semibold">{{ t('c2c.buyPanel.title') }}</h3>

        <div class="mt-4 space-y-2 text-sm rounded-xl bg-muted/50 p-4">
          <div class="flex justify-between"><span class="text-muted-foreground">{{ t('c2c.market.seller') }}</span><span class="font-medium">{{ t('c2c.userPrefix') }}{{ selectedListing.seller_user_id }}</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">{{ t('c2c.buyPanel.price') }}</span><span class="font-mono font-medium">{{ selectedListing.price }} {{ selectedListing.fiat_currency }}</span></div>
          <div class="flex justify-between"><span class="text-muted-foreground">{{ t('c2c.market.minMax') }}</span><span class="font-mono text-xs">{{ selectedListing.min_fiat_amount }} ~ {{ selectedListing.max_fiat_amount }}</span></div>
        </div>

        <div class="mt-4 space-y-3">
          <div>
            <Label class="mb-2 block">{{ t('c2c.buyPanel.fiatAmount') }}（{{ selectedListing.fiat_currency }}）</Label>
            <Input v-model="buyFiat" inputmode="decimal" class="h-11 font-mono" @input="onFiatInput" />
          </div>
          <div>
            <Label class="mb-2 block">{{ t('c2c.buyPanel.usdtAmount') }}</Label>
            <Input v-model="buyUsdt" inputmode="decimal" class="h-11 font-mono" @input="onUsdtInput" />
          </div>
          <p class="text-xs text-muted-foreground">{{ t('c2c.buyPanel.previewNote') }}</p>
        </div>

        <div class="mt-5 flex gap-3">
          <Button variant="outline" class="flex-1" @click="closeBuyPanel">{{ t('c2c.buyPanel.cancel') }}</Button>
          <Button class="flex-1" :disabled="!canConfirmBuy || submitting" @click="confirmBuy">
            {{ submitting ? '...' : t('c2c.buyPanel.confirm') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useC2CStore } from '@/stores/c2c'
import { useC2CTradeActions } from '@/composables/useC2C'
import { useUserAuthStore } from '@/stores/userAuth'
import { c2cAPI, type C2CListing } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import EmptyState from '@/components/EmptyState.vue'
import { formatUsdt } from '@/utils/money'

const { t } = useI18n()
const router = useRouter()
const c2cStore = useC2CStore()
const tradeActions = useC2CTradeActions()
const auth = useUserAuthStore()

const wallet = computed(() => c2cStore.wallet)

const fiatFilter = ref('CNY')
const amountFilter = ref('')
const loadingMore = ref(false)

const selectedListing = ref<C2CListing | null>(null)
const buyFiat = ref('')
const buyUsdt = ref('')
const submitting = ref(false)

// ─── Front-end amount filter ───
const filteredListings = computed(() => {
  let list = c2cStore.marketListings
  const amt = amountFilter.value.trim()
  if (amt && !isNaN(Number(amt))) {
    const n = Number(amt)
    list = list.filter((l) => Number(l.min_fiat_amount) <= n && Number(l.max_fiat_amount) >= n)
  }
  return list
})

const hasMore = computed(() => c2cStore.marketPagination.page < c2cStore.marketPagination.total_page)

const isOwnListing = (listing: C2CListing) => {
  const uid = Number(auth.user?.id)
  return Boolean(uid) && listing.seller_user_id === uid
}

const applyFilter = () => {
  void c2cStore.fetchMarketListings({
    page: 1,
    fiat_currency: fiatFilter.value.trim().toUpperCase() || undefined,
  })
}

const loadMore = async () => {
  if (!hasMore.value || loadingMore.value) return
  loadingMore.value = true
  try {
    const nextPage = c2cStore.marketPagination.page + 1
    const res = await c2cAPI.listMarketListings({
      page: nextPage,
      page_size: c2cStore.marketPagination.page_size,
      fiat_currency: fiatFilter.value.trim().toUpperCase() || undefined,
    })
    const more = (res.data?.data || []) as C2CListing[]
    c2cStore.marketListings = [...c2cStore.marketListings, ...more]
    const pg = res.data?.pagination
    if (pg) {
      c2cStore.marketPagination = {
        page: Number(pg.page) || nextPage,
        page_size: Number(pg.page_size) || c2cStore.marketPagination.page_size,
        total: Number(pg.total) || c2cStore.marketPagination.total,
        total_page: Number(pg.total_page) || c2cStore.marketPagination.total_page,
      }
    }
  } finally {
    loadingMore.value = false
  }
}

// ─── Buy panel amount linkage ───
const openBuyPanel = (listing: C2CListing) => {
  selectedListing.value = listing
  buyFiat.value = ''
  buyUsdt.value = ''
}

const closeBuyPanel = () => {
  selectedListing.value = null
}

const onFiatInput = () => {
  const price = Number(selectedListing.value?.price || 0)
  const fiat = Number(buyFiat.value)
  if (!isNaN(fiat) && price > 0) {
    buyUsdt.value = (fiat / price).toFixed(6)
  }
}

const onUsdtInput = () => {
  const price = Number(selectedListing.value?.price || 0)
  const usdt = Number(buyUsdt.value)
  if (!isNaN(usdt) && price > 0) {
    buyFiat.value = (usdt * price).toFixed(2)
  }
}

const canConfirmBuy = computed(() => {
  const usdt = Number(buyUsdt.value)
  return !isNaN(usdt) && usdt > 0 && Boolean(selectedListing.value)
})

const confirmBuy = async () => {
  if (!selectedListing.value || !canConfirmBuy.value) return
  submitting.value = true
  try {
    const trade = await tradeActions.createTrade(selectedListing.value.id, Number(buyUsdt.value).toFixed(6))
    if (trade?.id) {
      closeBuyPanel()
      await router.push(`/c2c/trades/${trade.id}`)
    }
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void c2cStore.fetchWallet()
  void c2cStore.fetchMarketListings({ page: 1, fiat_currency: fiatFilter.value.trim().toUpperCase() || undefined })
})
</script>
