<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <!-- Header -->
      <div class="mb-6 mt-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('c2c.myListings.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('c2c.myListings.subtitle') }}</p>
        </div>
        <Button @click="goSell">
          <Plus class="w-4 h-4" />
          {{ t('c2c.myListings.newListing') }}
        </Button>
      </div>

      <!-- Status Filter Tabs -->
      <div class="mb-6 flex flex-wrap gap-2">
        <button
          v-for="tab in statusTabs"
          :key="tab.key"
          class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
          :class="activeStatus === tab.key
            ? 'bg-primary text-primary-foreground'
            : 'bg-card border text-muted-foreground hover:text-foreground'"
          @click="onFilterChange(tab.key)"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Loading skeleton -->
      <div v-if="c2cStore.myListingsLoading && list.length === 0" class="space-y-3">
        <div v-for="i in 4" :key="i" class="rounded-2xl border bg-muted/60 h-28 animate-pulse"></div>
      </div>

      <!-- Empty state -->
      <EmptyState
        v-else-if="list.length === 0"
        variant="soft"
        size="lg"
        :title="t('c2c.myListings.empty')"
        :action-label="t('c2c.myListings.newListing')"
        @action="goSell"
      />

      <!-- PC Table -->
      <div v-else class="hidden md:block rounded-2xl border bg-card shadow-sm overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted/40">
              <TableHead>{{ t('c2c.myListings.listingNo') }}</TableHead>
              <TableHead>{{ t('c2c.myListings.availableTotal') }}</TableHead>
              <TableHead>{{ t('c2c.market.price') }}</TableHead>
              <TableHead>{{ t('c2c.myTrades.status') }}</TableHead>
              <TableHead>{{ t('c2c.myListings.createdAt') }}</TableHead>
              <TableHead class="text-right">{{ t('c2c.trade.actionPanel') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="item in list" :key="item.id" class="cursor-pointer" @click="goDetail(item.id)">
              <TableCell class="font-mono text-xs">{{ item.listing_no }}</TableCell>
              <TableCell>
                <div class="text-sm font-medium">{{ item.available_usdt }} USDT</div>
                <div class="text-xs text-muted-foreground">/ {{ item.total_usdt }}</div>
              </TableCell>
              <TableCell class="font-mono text-sm">
                {{ item.price }} <span class="text-xs text-muted-foreground">{{ item.fiat_currency }}</span>
              </TableCell>
              <TableCell>
                <Badge size="sm" :class="LISTING_STATUS_VARIANTS[item.status]">
                  {{ LISTING_STATUS_LABELS[item.status] }}
                </Badge>
              </TableCell>
              <TableCell class="text-xs text-muted-foreground">{{ formatTime(getCreatedAt(item)) }}</TableCell>
              <TableCell class="text-right" @click.stop>
                <div class="flex items-center justify-end gap-1.5">
                  <Button
                    v-if="item.status !== 'closed'"
                    variant="ghost"
                    size="sm"
                    class="h-7 text-xs"
                    @click="openEdit(item)"
                  >
                    {{ t('c2c.myListings.edit') }}
                  </Button>
                  <Button
                    v-if="item.status === 'active'"
                    variant="outline"
                    size="sm"
                    class="h-7 text-xs"
                    @click="onPause(item)"
                  >
                    {{ t('c2c.myListings.pause') }}
                  </Button>
                  <Button
                    v-if="item.status === 'paused'"
                    variant="outline"
                    size="sm"
                    class="h-7 text-xs"
                    @click="onResume(item)"
                  >
                    {{ t('c2c.myListings.resume') }}
                  </Button>
                  <Button
                    v-if="item.status !== 'closed'"
                    variant="ghost"
                    size="sm"
                    class="h-7 text-xs text-destructive hover:text-destructive"
                    @click="onClose(item)"
                  >
                    {{ t('c2c.myListings.close') }}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Mobile Cards -->
      <div class="md:hidden space-y-3">
        <div
          v-for="item in list"
          :key="item.id"
          class="rounded-2xl border bg-card p-4 shadow-sm"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="font-mono text-xs text-muted-foreground">{{ item.listing_no }}</div>
            <Badge size="sm" :class="LISTING_STATUS_VARIANTS[item.status]">
              {{ LISTING_STATUS_LABELS[item.status] }}
            </Badge>
          </div>
          <div class="mt-3 flex items-end justify-between">
            <div>
              <div class="text-lg font-bold font-mono">{{ item.price }} <span class="text-xs text-muted-foreground">{{ item.fiat_currency }}</span></div>
              <div class="mt-1 text-xs text-muted-foreground">
                {{ t('c2c.market.available') }} {{ item.available_usdt }} / {{ item.total_usdt }} USDT
              </div>
            </div>
          </div>
          <div class="mt-3 flex items-center justify-between gap-2 flex-wrap">
            <span class="text-xs text-muted-foreground">{{ formatTime(getCreatedAt(item)) }}</span>
            <div class="flex items-center gap-1.5">
              <Button size="xs" variant="outline" @click="goDetail(item.id)">{{ t('c2c.myListings.detail') }}</Button>
              <Button
                v-if="item.status !== 'closed'"
                size="xs"
                variant="ghost"
                @click="openEdit(item)"
              >
                {{ t('c2c.myListings.edit') }}
              </Button>
              <Button
                v-if="item.status === 'active'"
                size="xs"
                variant="outline"
                @click="onPause(item)"
              >
                {{ t('c2c.myListings.pause') }}
              </Button>
              <Button
                v-if="item.status === 'paused'"
                size="xs"
                variant="outline"
                @click="onResume(item)"
              >
                {{ t('c2c.myListings.resume') }}
              </Button>
              <Button
                v-if="item.status !== 'closed'"
                size="xs"
                variant="ghost"
                class="text-destructive hover:text-destructive"
                @click="onClose(item)"
              >
                {{ t('c2c.myListings.close') }}
              </Button>
            </div>
          </div>
        </div>
      </div>

      <!-- Load more -->
      <div v-if="hasMore" class="mt-6 text-center">
        <Button variant="outline" :disabled="loadingMore" @click="loadMore">
          <Loader2 v-if="loadingMore" class="w-4 h-4 animate-spin" />
          {{ loadingMore ? t('c2c.myListings.loadingMore') : t('c2c.market.loadMore') }}
        </Button>
      </div>

      <!-- Edit Modal -->
      <Teleport to="body">
        <div v-if="editModal.visible" class="fixed inset-0 z-[120] flex items-center justify-center p-4">
          <div class="fixed inset-0 bg-black/40 backdrop-blur-sm" @click="closeEdit"></div>
          <div class="relative z-10 w-full max-w-md rounded-2xl bg-card border shadow-2xl p-6">
            <h3 class="text-lg font-bold">{{ t('c2c.myListings.editTitle') }}</h3>
            <p class="mt-1 text-xs text-muted-foreground font-mono">{{ editingItem?.listing_no }}</p>
            <form class="mt-5 space-y-4" @submit.prevent="submitEdit">
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.myListings.unitPrice') }}（{{ editingItem?.fiat_currency || t('c2c.myListings.fiatLabel') }}/USDT）</Label>
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
                  {{ submitting ? t('c2c.myListings.saving') : t('c2c.myListings.save') }}
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
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Plus, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import EmptyState from '@/components/EmptyState.vue'
import { useC2CStore } from '@/stores/c2c'
import { c2cAPI, type C2CListing, type C2CListingStatus } from '@/api/c2c'
import {
  LISTING_STATUS_LABELS,
  LISTING_STATUS_VARIANTS,
} from '@/composables/useC2C'
import { useConfirmDialog } from '@/composables/useConfirmDialog'
import { toast } from '@/composables/useToast'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const router = useRouter()
const c2cStore = useC2CStore()
const appStore = useAppStore()
const { confirm } = useConfirmDialog()

type StatusFilter = '' | C2CListingStatus

const activeStatus = ref<StatusFilter>('')
const loadingMore = ref(false)
const submitting = ref(false)

const statusTabs: { key: StatusFilter; label: string }[] = [
  { key: '', label: t('c2c.myListings.all') },
  { key: 'active', label: LISTING_STATUS_LABELS.active ?? '' },
  { key: 'paused', label: LISTING_STATUS_LABELS.paused ?? '' },
  { key: 'closed', label: LISTING_STATUS_LABELS.closed ?? '' },
]

const list = computed(() => c2cStore.myListings)

const hasMore = computed(() => {
  const pg = c2cStore.myListingsPagination
  return pg.page < pg.total_page
})

const formatTime = (iso: string) => {
  if (!iso) return ''
  const d = new Date(iso.replace(' ', 'T'))
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(appStore.locale, {
    year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

// C2CListing 接口未声明 created_at，但后端实际返回，这里安全读取
const getCreatedAt = (item: C2CListing): string =>
  (item as C2CListing & { created_at?: string }).created_at || ''

const goSell = () => router.push('/c2c/sell')
const goDetail = (id: number) => router.push(`/c2c/listings/${id}`)

const onFilterChange = (status: StatusFilter) => {
  activeStatus.value = status
  void c2cStore.fetchMyListings({ page: 1, status: status || undefined })
}

const loadMore = async () => {
  if (loadingMore.value) return
  loadingMore.value = true
  try {
    const nextPage = c2cStore.myListingsPagination.page + 1
    const res = await c2cAPI.listMyListings({
      page: nextPage,
      page_size: c2cStore.myListingsPagination.page_size,
      status: activeStatus.value || undefined,
    })
    const data = (res.data?.data || []) as C2CListing[]
    c2cStore.myListings.push(...data)
    const pg = res.data?.pagination
    if (pg) {
      c2cStore.myListingsPagination.page = Number(pg.page) || nextPage
    }
  } catch {
    toast.error(t('c2c.myListings.loadMoreFailed'))
  } finally {
    loadingMore.value = false
  }
}

// ── Edit Modal ──
const editingItem = ref<C2CListing | null>(null)
const editModal = reactive({ visible: false })
const editForm = reactive({
  price: '',
  min_fiat_amount: '',
  max_fiat_amount: '',
  terms: '',
})

const openEdit = (item: C2CListing) => {
  editingItem.value = item
  editForm.price = item.price
  editForm.min_fiat_amount = item.min_fiat_amount
  editForm.max_fiat_amount = item.max_fiat_amount
  editForm.terms = item.terms || ''
  editModal.visible = true
}

const closeEdit = () => {
  editModal.visible = false
  editingItem.value = null
}

const submitEdit = async () => {
  if (!editingItem.value) return
  submitting.value = true
  try {
    await c2cAPI.updateListing(editingItem.value.id, {
      price: editForm.price,
      min_fiat_amount: editForm.min_fiat_amount,
      max_fiat_amount: editForm.max_fiat_amount,
      terms: editForm.terms,
    })
    toast.success(t('c2c.myListings.updated'))
    closeEdit()
    await c2cStore.fetchMyListings({ page: c2cStore.myListingsPagination.page })
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.myListings.updateFailed'))
  } finally {
    submitting.value = false
  }
}

// ── Actions ──
const onPause = async (item: C2CListing) => {
  const ok = await confirm({ title: t('c2c.myListings.pauseTitle'), message: t('c2c.myListings.pauseConfirm', { listingNo: item.listing_no }), variant: 'default' })
  if (!ok) return
  try {
    await c2cAPI.pauseListing(item.id)
    toast.success(t('c2c.myListings.paused'))
    await c2cStore.fetchMyListings({ page: 1, status: activeStatus.value || undefined })
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.myListings.operationFailed'))
  }
}

const onResume = async (item: C2CListing) => {
  try {
    await c2cAPI.resumeListing(item.id)
    toast.success(t('c2c.myListings.resumed'))
    await c2cStore.fetchMyListings({ page: 1, status: activeStatus.value || undefined })
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.myListings.operationFailed'))
  }
}

const onClose = async (item: C2CListing) => {
  const ok = await confirm({ title: t('c2c.myListings.closeTitle'), message: t('c2c.myListings.closeConfirm', { listingNo: item.listing_no }), variant: 'danger' })
  if (!ok) return
  try {
    await c2cAPI.closeListing(item.id)
    toast.success(t('c2c.myListings.closed'))
    await c2cStore.fetchMyListings({ page: 1, status: activeStatus.value || undefined })
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.myListings.operationFailed'))
  }
}

onMounted(() => {
  void c2cStore.fetchMyListings({ page: 1 })
})
</script>
