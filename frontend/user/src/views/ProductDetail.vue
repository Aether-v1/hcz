<template>
  <div class="space-y-4 pb-8">
      <!-- Loading Skeleton -->
      <div v-if="loading" class="space-y-4">
        <div class="h-5 w-48 rounded theme-skeleton"></div>
        <div class="rounded-2xl border bg-card p-5 shadow-sm">
          <div class="flex items-start gap-4">
            <div class="h-16 w-16 shrink-0 rounded-xl theme-skeleton"></div>
            <div class="flex-1 space-y-3">
              <div class="h-3 w-24 rounded theme-skeleton"></div>
              <div class="h-7 w-3/4 rounded theme-skeleton"></div>
              <div class="flex gap-2">
                <div class="h-6 w-16 rounded-full theme-skeleton"></div>
                <div class="h-6 w-16 rounded-full theme-skeleton"></div>
              </div>
            </div>
          </div>
          <div class="mt-5 h-4 w-full rounded theme-skeleton"></div>
          <div class="mt-2 h-4 w-2/3 rounded theme-skeleton"></div>
        </div>
        <div class="space-y-4 rounded-2xl border bg-card p-5 shadow-sm">
          <div class="h-5 w-40 rounded theme-skeleton"></div>
          <div class="h-10 w-48 rounded theme-skeleton"></div>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <div class="h-12 rounded-xl theme-skeleton"></div>
            <div class="h-12 rounded-xl theme-skeleton"></div>
          </div>
          <div class="h-12 w-full rounded-xl theme-skeleton"></div>
        </div>
      </div>

      <!-- Product Content -->
      <div v-else-if="product" class="space-y-4">
        <BreadcrumbNav
          class="mb-1"
          :items="[
            { label: t('nav.home'), to: '/' },
            { label: t('nav.products'), to: '/products' },
            { label: getLocalizedText(product.title) },
          ]"
        />

        <!-- Summary Card -->
        <div class="rounded-2xl border bg-card p-5 shadow-sm">
          <div class="flex items-start gap-4">
            <div class="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-xl border bg-muted p-3">
              <img v-if="product.category?.icon || images[0]" :src="product.category?.icon ? getImageUrl(product.category.icon) : images[0]" :alt="getLocalizedText(product.title)" class="h-full w-full object-contain" />
              <component :is="getServiceIcon(product.category)" v-else class="h-8 w-8 text-muted-foreground" aria-hidden="true" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{{ t('productDetail.serviceLabel') }}</p>
              <h1 class="mt-1 text-xl font-bold tracking-tight text-foreground md:text-2xl">
                {{ getLocalizedText(product.title) }}
              </h1>
              <p v-if="categoryName" class="mt-1 text-xs uppercase tracking-wider text-muted-foreground">
                {{ t('productDetail.categoryLabel') }} · {{ categoryName }}
              </p>
            </div>
          </div>

          <div v-if="product.tags && product.tags.length > 0" class="mt-4 flex flex-wrap gap-2">
            <Badge
              v-for="(tag, index) in product.tags"
              :key="index"
              variant="neutral"
            >
              {{ tag }}
            </Badge>
          </div>

          <p class="mt-4 text-sm leading-relaxed text-muted-foreground">
            {{ getLocalizedText(product.description) }}
          </p>
        </div>

        <!-- Purchase Card -->
        <div class="rounded-2xl border bg-card p-5 shadow-sm">
          <p class="text-sm font-semibold text-foreground">{{ t('productDetail.rechargeOptions') }}</p>

          <div class="mb-6 mt-4 border-b pb-5" ref="priceSection">
            <div class="mb-3 flex flex-wrap items-center gap-2">
              <span class="text-sm text-muted-foreground">{{ t('products.price') }}</span>
              <Badge v-if="(selectedSku && hasSkuPromotionPrice(selectedSku)) || (!selectedSku && hasPromotionPrice(product))" variant="danger">
                {{ t('products.promotionTag') }}
              </Badge>
              <Badge v-if="showSelectedSkuMemberBadge" variant="warning">
                {{ t('products.memberPriceTag') }}
              </Badge>
              <Badge v-if="hasSelectedSkuWholesalePrice" variant="success">
                {{ t('products.wholesaleTag') }}
              </Badge>
            </div>
            <div v-if="selectedSku && hasSelectedSkuWholesalePrice" class="space-y-2">
              <div class="flex flex-wrap items-end gap-4">
                <span
                  class="text-3xl font-bold tracking-tight tabular-nums"
                  :class="selectedSkuWholesaleFinalIsMember ? 'text-amber-600 dark:text-amber-300' : 'text-emerald-600 dark:text-emerald-300'"
                >
                  {{ formatPrice(selectedSkuWholesaleFinalPrice!, siteCurrency) }}
                </span>
                <span class="theme-price-original">
                  {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
                </span>
              </div>
              <p
                class="text-sm font-medium"
                :class="selectedSkuWholesaleFinalIsMember ? 'text-amber-600 dark:text-amber-300' : 'text-emerald-600 dark:text-emerald-300'"
              >
                {{ selectedSkuWholesaleFinalIsMember ? t('products.memberPriceTag') : t('products.wholesaleTag') }} ·
                {{ t('products.saveAmount') }} {{ formatPrice(Number(selectedSku.price_amount) - Number(selectedSkuWholesaleFinalPrice), siteCurrency) }}
              </p>
            </div>
            <!-- 选中 SKU 且有促销价 -->
            <div v-else-if="selectedSku && hasSkuPromotionPrice(selectedSku)" class="space-y-2">
              <div class="flex flex-wrap items-end gap-4">
                <span v-if="selectedSkuPromotionFinalIsMember" class="text-3xl font-bold tracking-tight tabular-nums text-amber-600 dark:text-amber-300">
                  {{ formatPrice(selectedSkuPromotionFinalPrice!, siteCurrency) }}
                </span>
                <span v-else class="text-3xl font-bold tracking-tight tabular-nums text-rose-600 dark:text-rose-300">
                  {{ formatPrice(selectedSkuPromotionPrice!, siteCurrency) }}
                </span>
                <span class="theme-price-original">
                  {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
                </span>
              </div>
              <p v-if="selectedSkuPromotionFinalIsMember" class="text-sm font-medium text-amber-600 dark:text-amber-300">
                {{ t('products.memberPriceTag') }} · {{ t('products.saveAmount') }} {{ formatPrice(Number(selectedSku.price_amount) - Number(selectedSkuPromotionFinalPrice), siteCurrency) }}
              </p>
              <p v-else class="text-sm font-medium text-rose-500 dark:text-rose-300">
                {{ t('products.saveAmount') }} {{ formatPrice(getSkuPromotionSaveAmount(selectedSku), siteCurrency) }}
              </p>
            </div>
            <!-- 选中 SKU 有会员价但无促销价 -->
            <div v-else-if="selectedSku && hasMemberPrice" class="space-y-2">
              <div class="flex flex-wrap items-end gap-4">
                <span class="text-3xl font-bold tracking-tight tabular-nums text-amber-600 dark:text-amber-300">
                  {{ formatPrice(selectedSkuMemberPrice!, siteCurrency) }}
                </span>
                <span class="theme-price-original">
                  {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
                </span>
              </div>
              <p class="text-sm font-medium text-amber-600 dark:text-amber-300">
                {{ t('products.memberPriceTag') }} · {{ t('products.saveAmount') }} {{ formatPrice(Number(selectedSku.price_amount) - selectedSkuMemberPrice!, siteCurrency) }}
              </p>
            </div>
            <!-- 选中 SKU 但无促销价也无会员价 -->
            <div v-else-if="selectedSku" class="flex items-end gap-4">
              <span class="text-3xl font-bold tracking-tight tabular-nums text-foreground">
                {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
              </span>
            </div>
            <!-- 未选 SKU，产品级有促销价 -->
            <div v-else-if="hasPromotionPrice(product)" class="space-y-2">
              <div class="flex flex-wrap items-end gap-4">
                <span class="text-3xl font-bold tracking-tight tabular-nums text-rose-600 dark:text-rose-300">
                  {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
                </span>
                <span class="theme-price-original">
                  {{ formatPrice(product.price_amount, siteCurrency) }}
                </span>
              </div>
              <p class="text-sm font-medium text-rose-500 dark:text-rose-300">
                {{ t('products.saveAmount') }} {{ formatPrice(getPromotionSaveAmount(product), siteCurrency) }}
              </p>
            </div>
            <!-- 未选 SKU，无促销 -->
            <div v-else class="flex items-end gap-4">
              <span class="text-3xl font-bold tracking-tight tabular-nums text-foreground">
                {{ formatPrice(product.price_amount, siteCurrency) }}
              </span>
            </div>
          </div>

          <!-- 批发价规则展示 -->
          <div v-if="selectedSkuWholesaleRules.length" class="mb-5 rounded-xl border border-emerald-200 bg-emerald-50/50 px-4 py-3 dark:border-emerald-800/50 dark:bg-emerald-950/20">
            <h2 class="mb-2 flex items-center gap-1.5 text-sm font-bold text-emerald-700 dark:text-emerald-300">
              {{ t('products.wholesaleRulesTitle') }}
            </h2>
            <div class="flex flex-wrap gap-2">
              <Badge v-for="tier in selectedSkuWholesaleRules" :key="`${tier.sku_id || tier.sku_code || 'all'}-${tier.min_quantity}`" variant="success" class="rounded-full px-3 py-1 font-medium">
                {{ formatWholesaleTier(tier) }}
              </Badge>
            </div>
          </div>

          <!-- 活动规则展示 -->
          <div v-if="hasPromotionRules(product)" class="mb-5 rounded-xl border border-orange-200 dark:border-orange-800/50 bg-orange-50/50 dark:bg-orange-950/20 px-4 py-3">
            <h2 class="mb-2 text-sm font-bold text-orange-700 dark:text-orange-300 flex items-center gap-1.5">
              <Tag class="w-4 h-4" />
              {{ t('products.promotionRulesTitle') }}
            </h2>
            <ul class="space-y-1">
              <li v-for="rule in getPromotionRules(product)" :key="rule.id" class="text-sm text-orange-600 dark:text-orange-300/90 flex items-center gap-1.5">
                <span class="w-1 h-1 rounded-full bg-orange-400 dark:bg-orange-500 shrink-0"></span>
                <span>{{ formatPromotionRule(rule) }}</span>
              </li>
            </ul>
          </div>

          <div v-if="activeSkus.length" class="mb-5">
            <h2 class="mb-3 text-sm font-semibold text-foreground">
              {{ t('productDetail.skuTitle') }}
            </h2>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <button
                v-for="sku in activeSkus"
                :key="sku.id"
                type="button"
                class="flex flex-col items-start rounded-xl border px-3 py-2 text-sm transition-all min-h-[44px]"
                :class="[
                  normalizeSkuId(sku.id) === selectedSkuId ? 'border-primary/40 bg-primary/10 ring-1 ring-primary/30' : 'border bg-secondary text-foreground',
                  isSkuPurchasable(sku) ? 'hover:-translate-y-0.5' : 'cursor-not-allowed opacity-55 border-dashed',
                ]"
                :disabled="!isSkuPurchasable(sku)"
                @click="selectedSkuId = normalizeSkuId(sku.id)"
              >
                <span class="font-semibold leading-tight">{{ skuDisplayText(sku) }}</span>
              </button>
            </div>
            <p v-if="requiresSKUSelection" class="mt-2 text-xs text-amber-500">
              {{ t('productDetail.skuRequired') }}
            </p>
          </div>

          <!-- Quantity Selector -->
          <div class="mb-5">
            <h2 class="mb-3 text-sm font-semibold text-foreground">
              {{ t('productDetail.quantity') }}
            </h2>
            <div class="flex items-center rounded-lg border overflow-hidden w-fit">
              <button
                type="button"
                class="w-10 h-10 flex items-center justify-center text-muted-foreground hover:bg-muted transition-colors disabled:opacity-30"
                :disabled="quantity <= quantityEffectiveMin"
                @click="quantity = Math.max(quantityEffectiveMin, quantity - 1)"
              >
                <Minus class="w-4 h-4" :stroke-width="2.5" />
              </button>
              <input
                type="text"
                inputmode="numeric"
                class="w-14 h-10 text-center text-sm font-semibold text-foreground border-x bg-transparent outline-none tabular-nums [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                :value="quantity"
                @change="handleQuantityInput($event)"
                @keydown.enter.prevent="($event.target as HTMLInputElement)?.blur()"
              />
              <button
                type="button"
                class="w-10 h-10 flex items-center justify-center text-muted-foreground hover:bg-muted transition-colors disabled:opacity-30"
                :disabled="quantityEffectiveLimit !== null && quantity >= quantityEffectiveLimit"
                @click="quantity = quantity + 1"
              >
                <Plus class="w-4 h-4" :stroke-width="2.5" />
              </button>
            </div>
          </div>

          <!-- Purchase Actions -->
          <div ref="purchaseActionsRef" class="space-y-4 border-t pt-5">
            <Alert v-if="cannotPurchaseReason" variant="destructive">
              <AlertDescription class="font-semibold">{{ cannotPurchaseReason }}</AlertDescription>
            </Alert>
            <Alert v-if="purchaseWarning" class="border-warning/40 text-warning">
              <AlertDescription class="font-semibold text-warning">{{ purchaseWarning }}</AlertDescription>
            </Alert>

            <div class="space-y-3">
              <Button v-if="requiresLogin" class="w-full h-12 font-bold" @click="goLogin">
                {{ t('productDetail.loginToBuy') }}
              </Button>
              <Button v-else class="w-full h-12 font-bold" :disabled="!canPurchase" @click="buyNow">
                {{ t('productDetail.buyNow') }}
              </Button>
              <p class="text-xs leading-5 text-muted-foreground">{{ t('productDetail.accountAtCheckout') }}</p>
            </div>
          </div>
        </div>

        <!-- Details Content Card -->
        <div v-if="product.content"
          class="rounded-2xl border bg-card p-5 shadow-sm">
          <h2 class="mb-3 text-sm font-semibold text-foreground">
            {{ t('productDetail.details') }}
          </h2>
          <div v-html="sanitizeRichHtml(getLocalizedText(product.content))"
            class="prose prose-gray dark:prose-invert max-w-none theme-prose">
          </div>
        </div>

        <!-- Related Posts -->
        <section v-if="relatedPosts.length"
          class="rounded-2xl border bg-card p-5 shadow-sm">
          <h2 class="mb-3 text-sm font-semibold text-foreground">
            {{ t('productDetail.relatedPosts') }}
          </h2>
          <div class="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
            <router-link
              v-for="rp in relatedPosts"
              :key="rp.id"
              :to="`/news/${rp.slug}`"
              class="group flex flex-col overflow-hidden rounded-xl border bg-card transition-colors hover:border-primary/40"
            >
              <div v-if="rp.thumbnail" class="h-32 overflow-hidden relative">
                <img :src="getImageUrl(rp.thumbnail)" :alt="getLocalizedText(rp.title)" loading="lazy"
                  class="h-full w-full object-cover transition-transform group-hover:scale-110" />
              </div>
              <div class="p-4 flex flex-col flex-1">
                <h3 class="font-semibold text-foreground line-clamp-2 mb-2">{{ getLocalizedText(rp.title) }}</h3>
                <p v-if="rp.summary" class="text-sm text-muted-foreground line-clamp-2 leading-relaxed mb-3">
                  {{ getLocalizedText(rp.summary) }}
                </p>
                <time class="mt-auto text-xs text-muted-foreground font-mono">
                  {{ formatRelatedPostDate(rp.published_at) }}
                </time>
              </div>
            </router-link>
          </div>
        </section>

        <!-- Back Button -->
        <div class="text-center">
          <router-link to="/products"
            class="inline-flex items-center space-x-2 text-muted-foreground hover:text-foreground transition-colors border-b border-transparent hover:border-current pb-1">
            <ArrowLeft class="w-5 h-5" />
            <span>{{ t('productDetail.backToProducts') }}</span>
          </router-link>
        </div>

        <!-- Mobile Fixed Purchase Bar -->
        <ProductMobileBar
          :visible="showMobileBar && !!product && !loading"
          :requires-login="requiresLogin"
          :can-purchase="canPurchase"
          :show-member-price="mobileBarShowMemberPrice"
          :member-price-display="mobileBarMemberPriceDisplay"
          :show-sku-promotion-price="mobileBarShowSkuPromotionPrice"
          :sku-promotion-price-display="mobileBarSkuPromotionPriceDisplay"
          :show-sku-price="mobileBarShowSkuPrice"
          :sku-price-display="mobileBarSkuPriceDisplay"
          :show-product-promotion-price="mobileBarShowProductPromotionPrice"
          :product-promotion-price-display="mobileBarProductPromotionPriceDisplay"
          :product-price-display="mobileBarProductPriceDisplay"
          @buy-now="buyNow"
          @go-login="goLogin"
        />
      </div>

      <!-- Error State -->
      <EmptyState
        v-else
        size="lg"
        icon="alert"
        :title="t('productDetail.notFound')"
      >
        <template #action>
          <Button class="rounded-full h-10" @click="loadProduct">
            <RotateCw />
            {{ t('errorBoundary.retry') }}
          </Button>
          <Button variant="secondary" as-child class="rounded-full h-10">
            <router-link to="/products">{{ t('productDetail.backToProducts') }}</router-link>
          </Button>
        </template>
      </EmptyState>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Minus, Plus, RotateCw, Tag } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { getServiceIcon } from '../utils/serviceIcon'
import { sanitizeRichHtml } from '../utils/content'
import { useProductDetail } from '../composables/useProductDetail'
import ProductMobileBar from '../components/product/ProductMobileBar.vue'
import BreadcrumbNav from '../components/BreadcrumbNav.vue'
import EmptyState from '../components/EmptyState.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

// 视图专属：移动端固定购买条（IntersectionObserver 监听桌面购买区是否出屏）
const purchaseActionsRef = ref<HTMLElement | null>(null)
const showMobileBar = ref(false)
let observer: IntersectionObserver | null = null

const setupMobileBarObserver = () => {
  if (observer) observer.disconnect()
  if (!purchaseActionsRef.value) return
  observer = new IntersectionObserver(
    (entries) => {
      const entry = entries[0]
      if (entry) {
        showMobileBar.value = !entry.isIntersecting
      }
    },
    { threshold: 0.1 }
  )
  observer.observe(purchaseActionsRef.value)
}

// 全部业务逻辑由 useProductDetail 提供（与 vault 模板共用，保证功能一致）
const {
  getLocalizedText, siteCurrency, formatPrice,
  hasPromotionPrice, getPromotionPriceAmount, getPromotionSaveAmount,
  hasSkuPromotionPrice, getSkuPromotionSaveAmount,
  hasPromotionRules, getPromotionRules,
  formatPromotionRule, formatWholesaleTier, formatRelatedPostDate, normalizeSkuId,
  loading, product, relatedPosts, selectedSkuId, quantity, purchaseWarning,
  activeSkus, selectedSku,
  selectedSkuMemberPrice, hasMemberPrice,
  hasSelectedSkuWholesalePrice, selectedSkuWholesaleFinalIsMember, selectedSkuWholesaleFinalPrice,
  selectedSkuWholesaleRules,
  selectedSkuPromotionPrice, selectedSkuPromotionFinalIsMember, selectedSkuPromotionFinalPrice,
  showSelectedSkuMemberBadge,
  isSkuPurchasable, skuDisplayText,
  quantityEffectiveLimit, quantityEffectiveMin, handleQuantityInput,
  requiresLogin, requiresSKUSelection, canPurchase, cannotPurchaseReason,
  categoryName, images,
  buyNow, goLogin, loadProduct,
  mobileBarShowMemberPrice, mobileBarMemberPriceDisplay,
  mobileBarShowSkuPromotionPrice, mobileBarSkuPromotionPriceDisplay,
  mobileBarShowSkuPrice, mobileBarSkuPriceDisplay,
  mobileBarShowProductPromotionPrice, mobileBarProductPromotionPriceDisplay, mobileBarProductPriceDisplay,
} = useProductDetail({ onLoaded: () => setupMobileBarObserver() })

onUnmounted(() => {
  if (observer) {
    observer.disconnect()
    observer = null
  }
})
</script>
