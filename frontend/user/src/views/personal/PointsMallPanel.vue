<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('personalCenter.points.mallTitle') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('personalCenter.points.mallSubtitle') }}</p>
    </div>

    <!-- 可用积分提示（与现有余额卡同一语言，仅一行） -->
    <div class="flex items-center justify-between rounded-2xl border bg-card px-5 py-3 shadow-sm">
      <span class="flex items-center gap-2 text-sm text-muted-foreground">
        <Coins :size="16" :stroke-width="1.8" />
        {{ t('personalCenter.points.balance') }}
      </span>
      <span class="text-sm font-bold tabular-nums text-foreground">{{ balance }}</span>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <div v-for="i in 4" :key="i" class="h-32 animate-pulse rounded-2xl bg-muted/60"></div>
    </div>

    <!-- Empty -->
    <EmptyState v-else-if="products.length === 0" icon="package" :description="t('personalCenter.points.mallEmpty')" />

    <!-- 商品卡：复用现有服务列表卡（同 ProductListItem / 订单列表卡语言） -->
    <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <button
        v-for="product in products"
        :key="product.id"
        type="button"
        class="group flex min-w-0 cursor-pointer items-center gap-4 rounded-xl border bg-card p-3 text-left shadow-sm transition-colors hover:border-primary/40 hover:bg-primary/5"
        @click="openDetail(product.id)"
      >
        <div class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-primary/10 text-primary">
          <img v-if="coverUrl(product)" :src="coverUrl(product)" :alt="product.name" class="h-9 w-9 object-contain" loading="lazy" />
          <ShoppingBag v-else class="h-5 w-5" aria-hidden="true" />
        </div>
        <div class="min-w-0 flex-1">
          <h3 class="truncate text-sm font-semibold text-foreground">{{ product.name }}</h3>
          <p class="truncate text-xs text-muted-foreground">{{ product.subtitle || product.description }}</p>
          <p class="mt-1 text-xs font-medium text-primary">{{ pointsText(product.points_price) }}</p>
        </div>
        <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50 transition-transform group-hover:translate-x-0.5" />
      </button>
    </div>

    <PaginationNav
      :current-page="pagination.page"
      :total-pages="pagination.total_page"
      :loading="loading"
      :scroll-top="false"
      @change-page="loadProducts"
    />

    <!-- 详情弹层：复用现有确认弹窗视觉（bg-black/40 + rounded-2xl bg-card） -->
    <Teleport to="body">
      <div
        v-if="detail"
        class="fixed inset-0 z-50 flex items-end justify-center p-0 md:items-center md:p-4"
        @click.self="closeDetail"
      >
        <div class="fixed inset-0 bg-black/40 backdrop-blur-sm" aria-hidden="true" @click="closeDetail" />
        <div role="dialog" aria-modal="true" class="relative z-10 w-full max-w-md rounded-t-2xl border bg-card p-6 shadow-2xl md:rounded-2xl">
          <div class="flex items-start gap-3">
            <div class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-primary/10 text-primary">
              <img v-if="coverUrl(detail.product)" :src="coverUrl(detail.product)" :alt="detail.product.name" class="h-8 w-8 object-contain" />
              <ShoppingBag v-else class="h-5 w-5" aria-hidden="true" />
            </div>
            <div class="min-w-0 flex-1">
              <h3 class="truncate text-base font-bold text-foreground">{{ detail.product.name }}</h3>
              <p v-if="detail.product.subtitle" class="mt-0.5 truncate text-sm text-muted-foreground">{{ detail.product.subtitle }}</p>
            </div>
          </div>

          <div class="mt-4 space-y-2 rounded-xl bg-muted/50 p-4 text-sm">
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ t('personalCenter.points.price') }}</span>
              <span class="font-mono font-semibold tabular-nums text-foreground">{{ pointsText(detail.product.points_price) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ t('personalCenter.points.stock') }}</span>
              <span class="font-medium tabular-nums text-foreground">{{ stockText(detail.product) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ t('personalCenter.points.myRedeemed') }}</span>
              <span class="font-medium tabular-nums text-foreground">{{ detail.user_redeemed_count }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ t('personalCenter.points.balance') }}</span>
              <span class="font-medium tabular-nums text-foreground">{{ balance }}</span>
            </div>
          </div>

          <p v-if="detail.product.description" class="mt-4 text-sm leading-6 text-muted-foreground">{{ detail.product.description }}</p>
          <div v-if="detail.product.instructions" class="mt-3 rounded-xl border px-4 py-3">
            <p class="text-xs font-semibold text-muted-foreground">{{ t('personalCenter.points.instructionsTitle') }}</p>
            <p class="mt-1.5 whitespace-pre-line text-sm leading-6 text-foreground">{{ detail.product.instructions }}</p>
          </div>

          <p v-if="!detail.can_redeem && reasonLabel" class="mt-4 rounded-xl bg-warning/10 px-4 py-2.5 text-sm font-medium text-warning">{{ reasonLabel }}</p>

          <div class="mt-5 flex gap-3">
            <Button variant="outline" class="flex-1" @click="closeDetail">{{ t('common.cancel') }}</Button>
            <Button class="flex-1" :disabled="!detail.can_redeem || submitting" @click="confirmExchange">
              {{ submitting ? t('personalCenter.points.exchanging') : t('personalCenter.points.exchangeButton') }}
            </Button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Coins, ShoppingBag } from 'lucide-vue-next'
import { pointsMallAPI, pointsAPI, genPointsIdempotencyKey, type PointsProduct, type PointsProductDetail } from '../../api'
import { pointsReasonCodeLabel } from '../../utils/status'
import { getImageUrl } from '../../utils/image'
import { toast } from '../../composables/useToast'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()
const { confirm } = useConfirmDialog()

const products = ref<PointsProduct[]>([])
const loading = ref(true)
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const balance = ref(0)

const detail = ref<PointsProductDetail | null>(null)
const submitting = ref(false)
// 一次兑换会话共用同一幂等键；失败重试复用，成功后重置。
let exchangeKey = ''

const reasonLabel = computed(() => pointsReasonCodeLabel(t, detail.value?.reason_code))

const coverUrl = (product: PointsProduct) => getImageUrl(product.cover)

const pointsText = (price: number) => t('personalCenter.points.pointsUnit', { points: price })

const stockText = (product: PointsProduct) =>
  product.unlimited_stock ? t('personalCenter.points.unlimitedStock') : String(product.stock)

const loadBalance = async () => {
  try {
    const response = await pointsAPI.account()
    balance.value = response.data.data?.balance ?? 0
  } catch {
    /* 静默 */
  }
}

const loadProducts = async (page = 1, silent = false) => {
  if (!silent) loading.value = true
  try {
    const response = await pointsMallAPI.products({ page, page_size: pagination.value.page_size })
    products.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch {
    // 静默刷新失败时保留现有列表，避免覆盖已成功的兑换结果
    if (!silent) products.value = []
  } finally {
    if (!silent) loading.value = false
  }
}

const loadProductDetail = async (id: number, silent = false): Promise<boolean> => {
  try {
    const response = await pointsMallAPI.productDetail(id)
    detail.value = response.data.data || null
    return true
  } catch (err: any) {
    if (!silent) toast.error(err?.message || t('personalCenter.common.loadFailed'))
    return false
  }
}

const openDetail = async (id: number) => {
  if (await loadProductDetail(id)) exchangeKey = ''
}

// 兑换成功后的数据对齐：列表 + （若仍打开）当前详情。
// 两个分支都走静默模式，失败只保留旧数据，不会把结果回退成「兑换失败」。
const refreshAfterExchange = async (productId: number) => {
  await loadProducts(pagination.value.page, true)
  if (detail.value) await loadProductDetail(productId, true)
}

const closeDetail = () => {
  detail.value = null
  exchangeKey = ''
}

const confirmExchange = async () => {
  const product = detail.value?.product
  if (!product || !detail.value?.can_redeem || submitting.value) return
  const ok = await confirm({
    title: t('personalCenter.points.exchangeConfirmTitle'),
    message: t('personalCenter.points.exchangeConfirm', { name: product.name, points: product.points_price }),
  })
  if (!ok) return

  submitting.value = true
  try {
    if (!exchangeKey) exchangeKey = genPointsIdempotencyKey()
    const response = await pointsMallAPI.createExchange(product.id, exchangeKey)
    const result = response.data.data
    balance.value = result?.current_balance ?? balance.value
    exchangeKey = ''
    toast.success(t('personalCenter.points.exchangeSuccess', { points: result?.order?.total_points ?? product.points_price }))
    // 余额与成功提示已按兑换响应落定；刷新期间沿用 submitting 挡住重复提交
    await refreshAfterExchange(product.id)
  } catch (err: any) {
    toast.error(err?.message || t('personalCenter.points.exchangeFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void Promise.all([loadProducts(1), loadBalance()])
})
</script>
