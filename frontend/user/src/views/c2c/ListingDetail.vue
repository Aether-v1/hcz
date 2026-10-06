<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4 max-w-3xl">
      <!-- Header -->
      <div class="mb-6 mt-8 flex items-center gap-3">
        <Button variant="ghost" size="icon" @click="goBack">
          <ArrowLeft class="w-4 h-4" />
        </Button>
        <div>
          <h1 class="text-2xl font-bold tracking-tight">{{ t('c2c.listingDetail') }}</h1>
          <p class="mt-0.5 text-xs text-muted-foreground font-mono">{{ listing?.listing_no }}</p>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="rounded-2xl border bg-muted/60 h-64 animate-pulse"></div>

      <!-- Not found -->
      <EmptyState
        v-else-if="!listing"
        variant="soft"
        size="lg"
        :title="t('c2c.listingDetailNotFound')"
        :action-label="t('c2c.listingDetailBackToList')"
        @action="goBack"
      />

      <!-- Detail -->
      <template v-else>
        <!-- Main Card -->
        <Card class="mb-4">
          <CardContent class="p-6">
            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="text-sm text-muted-foreground">{{ t('c2c.listingDetailLabels.unitPrice') }}</div>
                <div class="mt-1 text-3xl font-bold font-mono">
                  {{ listing.price }}
                  <span class="text-base text-muted-foreground font-normal">{{ listing.fiat_currency }}/USDT</span>
                </div>
              </div>
              <Badge size="sm" :class="LISTING_STATUS_VARIANTS[listing.status]">
                {{ LISTING_STATUS_LABELS[listing.status] }}
              </Badge>
            </div>

            <div class="mt-6 grid grid-cols-2 sm:grid-cols-3 gap-4 pt-6 border-t border-border/60">
              <div>
                <div class="text-xs text-muted-foreground">{{ t('c2c.listingDetailLabels.availableQty') }}</div>
                <div class="mt-1 text-sm font-semibold font-mono">{{ formatUsdt(listing.available_usdt) }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('c2c.listingDetailLabels.listingTotal') }}</div>
                <div class="mt-1 text-sm font-semibold font-mono">{{ formatUsdt(listing.total_usdt) }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('c2c.listingDetailLabels.seller') }}</div>
                <div class="mt-1 text-sm font-semibold">{{ t('c2c.userPrefix') }}{{ listing.seller_user_id }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('c2c.sell.minFiat') }}</div>
                <div class="mt-1 text-sm font-semibold font-mono">{{ listing.min_fiat_amount }} {{ listing.fiat_currency }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('c2c.sell.maxFiat') }}</div>
                <div class="mt-1 text-sm font-semibold font-mono">{{ listing.max_fiat_amount }} {{ listing.fiat_currency }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">{{ t('c2c.myListings.createdAt') }}</div>
                <div class="mt-1 text-sm font-semibold">{{ formatTime(getCreatedAt(listing)) }}</div>
              </div>
            </div>

            <div v-if="listing.terms" class="mt-6 rounded-xl bg-muted/40 p-4">
              <div class="text-xs font-medium text-muted-foreground mb-1.5">{{ t('c2c.sell.terms') }}</div>
              <p class="text-sm whitespace-pre-wrap leading-relaxed">{{ listing.terms }}</p>
            </div>
          </CardContent>
        </Card>

        <!-- Action Bar -->
        <Card v-if="!isOwnListing && listing.status === 'active'" class="mb-4">
          <CardContent class="p-6">
            <Button class="w-full h-12 text-base font-bold" @click="goBuy">
              <TrendingDown class="w-5 h-5" />
              {{ t('c2c.listingDetailLabels.buyNow') }}
            </Button>
          </CardContent>
        </Card>

        <!-- Own Listing Actions -->
        <Card v-else-if="isOwnListing && listing.status !== 'closed'" class="mb-4">
          <CardContent class="p-6">
            <div class="flex flex-wrap gap-2">
              <Button variant="outline" @click="openEdit">
                <Pencil class="w-4 h-4" />
                {{ t('c2c.myListings.edit') }}
              </Button>
              <Button v-if="listing.status === 'active'" variant="outline" @click="onPause">
                <Pause class="w-4 h-4" />
                {{ t('c2c.myListings.pause') }}
              </Button>
              <Button v-if="listing.status === 'paused'" variant="outline" @click="onResume">
                <Play class="w-4 h-4" />
                {{ t('c2c.myListings.resume') }}
              </Button>
              <Button variant="ghost" class="text-destructive hover:text-destructive" @click="onClose">
                <X class="w-4 h-4" />
                {{ t('c2c.myListings.close') }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <!-- Trade Records -->
        <Card>
          <CardHeader>
            <CardTitle class="text-base">{{ t('c2c.listingDetailLabels.trades') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-sm text-muted-foreground">{{ t('c2c.listingDetailLabels.noTrades') }}</p>
          </CardContent>
        </Card>
      </template>

      <!-- Edit Modal -->
      <Teleport to="body">
        <div v-if="editModal.visible" class="fixed inset-0 z-[120] flex items-center justify-center p-4">
          <div class="fixed inset-0 bg-black/40 backdrop-blur-sm" @click="closeEdit"></div>
          <div class="relative z-10 w-full max-w-md rounded-2xl bg-card border shadow-2xl p-6">
            <h3 class="text-lg font-bold">{{ t('c2c.listingDetailLabels.editTitle') }}</h3>
            <p class="mt-1 text-xs text-muted-foreground font-mono">{{ listing?.listing_no }}</p>
            <form class="mt-5 space-y-4" @submit.prevent="submitEdit">
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.listingDetailLabels.unitPrice') }}（{{ listing?.fiat_currency || t('c2c.listingDetailLabels.fiatLabel') }}/USDT）</Label>
                <Input v-model="editForm.price" inputmode="decimal" required />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <Label class="mb-1.5 block">{{ t('c2c.sell.minFiat') }}</Label>
                  <Input v-model="editForm.min_fiat_amount" inputmode="decimal" required />
                </div>
                <div>
                  <Label class="mb-1.5 block">{{ t('c2c.sell.maxFiat') }}</Label>
                  <Input v-model="editForm.max_fiat_amount" inputmode="decimal" required />
                </div>
              </div>
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.sell.terms') }}</Label>
                <Textarea v-model="editForm.terms" rows="3" />
              </div>
              <div class="flex justify-end gap-2 pt-2">
                <Button type="button" variant="secondary" @click="closeEdit">{{ t('c2c.buyPanel.cancel') }}</Button>
                <Button type="submit" :disabled="submitting">
                  {{ submitting ? t('c2c.listingDetailLabels.saving') : t('c2c.listingDetailLabels.save') }}
                </Button>
              </div>
            </form>
          </div>
        </div>
      </Teleport>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft, TrendingDown, Pencil, Pause, Play, X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import EmptyState from '@/components/EmptyState.vue'
import { formatUsdt } from '@/utils/money'
import { c2cAPI, type C2CListing } from '@/api/c2c'
import { useUserAuthStore } from '@/stores/userAuth'
import {
  LISTING_STATUS_LABELS,
  LISTING_STATUS_VARIANTS,
} from '@/composables/useC2C'
import { useConfirmDialog } from '@/composables/useConfirmDialog'
import { toast } from '@/composables/useToast'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useUserAuthStore()
const appStore = useAppStore()
const { confirm } = useConfirmDialog()

const listing = ref<C2CListing | null>(null)
const loading = ref(true)
const submitting = ref(false)

const currentUserId = computed(() => Number(auth.user?.id) || 0)
const isOwnListing = computed(() =>
  listing.value ? listing.value.seller_user_id === currentUserId.value : false,
)

const formatTime = (iso: string) => {
  if (!iso) return ''
  const d = new Date(iso.replace(' ', 'T'))
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(appStore.locale, {
    year: 'numeric', month: 'short', day: 'numeric',
  })
}

// C2CListing 接口未声明 created_at，但后端实际返回
const getCreatedAt = (l: C2CListing): string =>
  (l as C2CListing & { created_at?: string }).created_at || ''

const loadDetail = async () => {
  const id = Number(route.params.id)
  if (!id) return
  loading.value = true
  try {
    const res = await c2cAPI.getListingDetail(id)
    listing.value = (res.data?.data || null) as C2CListing | null
  } catch {
    listing.value = null
    toast.error(t('c2c.listingDetailLabels.notExist'))
  } finally {
    loading.value = false
  }
}

const goBack = () => router.back()
const goBuy = () => {
  if (listing.value) {
    router.push({ path: '/c2c/buy', query: { listing_id: String(listing.value.id) } })
  }
}

// ── Edit Modal ──
const editModal = reactive({ visible: false })
const editForm = reactive({
  price: '',
  min_fiat_amount: '',
  max_fiat_amount: '',
  terms: '',
})

const openEdit = () => {
  if (!listing.value) return
  editForm.price = listing.value.price
  editForm.min_fiat_amount = listing.value.min_fiat_amount
  editForm.max_fiat_amount = listing.value.max_fiat_amount
  editForm.terms = listing.value.terms || ''
  editModal.visible = true
}

const closeEdit = () => {
  editModal.visible = false
}

const submitEdit = async () => {
  if (!listing.value) return
  submitting.value = true
  try {
    await c2cAPI.updateListing(listing.value.id, {
      price: editForm.price,
      min_fiat_amount: editForm.min_fiat_amount,
      max_fiat_amount: editForm.max_fiat_amount,
      terms: editForm.terms,
    })
    toast.success(t('c2c.listingDetailLabels.updated'))
    closeEdit()
    await loadDetail()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.listingDetailLabels.updateFailed'))
  } finally {
    submitting.value = false
  }
}

// ── Actions ──
const onPause = async () => {
  if (!listing.value) return
  const ok = await confirm({ title: t('c2c.listingDetailLabels.pauseTitle'), message: t('c2c.listingDetailLabels.pauseConfirm', { listingNo: listing.value.listing_no }), variant: 'default' })
  if (!ok) return
  try {
    await c2cAPI.pauseListing(listing.value.id)
    toast.success(t('c2c.listingDetailLabels.paused'))
    await loadDetail()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.listingDetailLabels.operationFailed'))
  }
}

const onResume = async () => {
  if (!listing.value) return
  try {
    await c2cAPI.resumeListing(listing.value.id)
    toast.success(t('c2c.listingDetailLabels.resumed'))
    await loadDetail()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.listingDetailLabels.operationFailed'))
  }
}

const onClose = async () => {
  if (!listing.value) return
  const ok = await confirm({ title: t('c2c.listingDetailLabels.closeTitle'), message: t('c2c.listingDetailLabels.closeConfirm', { listingNo: listing.value.listing_no }), variant: 'danger' })
  if (!ok) return
  try {
    await c2cAPI.closeListing(listing.value.id)
    toast.success(t('c2c.listingDetailLabels.closed'))
    await loadDetail()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.listingDetailLabels.operationFailed'))
  }
}

watch(() => route.params.id, () => {
  void loadDetail()
})

onMounted(() => {
  void loadDetail()
})
</script>
