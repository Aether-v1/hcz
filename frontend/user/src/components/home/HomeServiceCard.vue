<template>
  <RouterLink :to="{ name: 'product-detail', params: { slug: product.slug } }" class="home-service-card">
    <div class="home-service-card__top">
      <span class="home-service-card__icon">
        <img v-if="imageUrl && !imageFailed" :src="imageUrl" :alt="title" loading="lazy" @error="imageFailed = true" />
        <component :is="getServiceIcon(product.category)" v-else :size="26" :stroke-width="1.9" aria-hidden="true" />
      </span>
      <span v-if="hasPromotionPrice(product)" class="home-service-card__tag">{{ t('homeV2.offer') }}</span>
    </div>
    <div class="home-service-card__body">
      <p v-if="categoryName" class="home-service-card__category">{{ categoryName }}</p>
      <h3>{{ title }}</h3>
      <p v-if="description" class="home-service-card__description">{{ description }}</p>
    </div>
    <div class="home-service-card__bottom">
      <div v-if="priceAmount !== null" class="min-w-0">
        <span class="home-service-card__price">{{ formatPrice(priceAmount, siteCurrency) }}</span>
        <span v-if="hasPromotionPrice(product)" class="home-service-card__original">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
      </div>
      <span v-else class="home-service-card__details">{{ t('homeV2.viewDetails') }}</span>
      <ArrowUpRight :size="18" aria-hidden="true" />
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight } from 'lucide-vue-next'
import { useLocalized, useProductLabels } from '../../composables/useProduct'
import { getFirstImageUrl, getImageUrl } from '../../utils/image'
import { getServiceIcon } from '../../utils/serviceIcon'

const props = defineProps<{ product: any }>()
const { t } = useI18n()
const { getLocalizedText, formatPrice, siteCurrency } = useLocalized()
const { hasPromotionPrice, getPromotionPriceAmount } = useProductLabels()
const imageFailed = ref(false)

watch(() => props.product?.id, () => { imageFailed.value = false })

const imageUrl = computed(() => {
  const categoryIcon = props.product?.category?.icon
  return categoryIcon ? getImageUrl(categoryIcon) : getFirstImageUrl(props.product?.images)
})
const title = computed(() => getLocalizedText(props.product?.title) || props.product?.slug || '')
const description = computed(() => getLocalizedText(props.product?.description))
const categoryName = computed(() => getLocalizedText(props.product?.category?.name))
const priceAmount = computed(() => {
  const value = hasPromotionPrice(props.product) ? getPromotionPriceAmount(props.product) : props.product?.price_amount
  return value === undefined || value === null || value === '' ? null : value
})
</script>

<style scoped>
.home-service-card {
  display: flex;
  min-width: 0;
  min-height: 235px;
  flex-direction: column;
  padding: 18px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-xl);
  background: var(--ui-bg-elevated);
  box-shadow: var(--ui-shadow-soft);
  color: var(--ui-text-primary);
  transition: border-color 180ms ease, transform 180ms ease, box-shadow 180ms ease;
}
.home-service-card:hover,
.home-service-card:focus-visible {
  transform: translateY(-2px);
  border-color: var(--ui-accent);
  box-shadow: var(--ui-shadow-card);
}
.home-service-card__top,
.home-service-card__bottom { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.home-service-card__icon { display: grid; width: 54px; height: 54px; flex: none; place-items: center; overflow: hidden; border-radius: 16px; background: var(--ui-accent-soft); color: var(--ui-accent); }
.home-service-card__icon img { width: 38px; height: 38px; object-fit: contain; }
.home-service-card__tag { flex: none; border-radius: 999px; padding: 4px 8px; background: var(--ui-danger-soft); color: var(--ui-danger); font-size: 11px; font-weight: 700; }
.home-service-card__body { min-width: 0; flex: 1; padding: 16px 0 18px; }
.home-service-card__category { overflow: hidden; color: var(--ui-text-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.home-service-card__body h3 { display: -webkit-box; overflow: hidden; margin-top: 4px; font-size: 16px; font-weight: 750; line-height: 1.35; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.home-service-card__description { display: -webkit-box; overflow: hidden; margin-top: 6px; color: var(--ui-text-secondary); font-size: 12px; line-height: 1.5; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.home-service-card__bottom { min-height: 38px; padding-top: 12px; border-top: 1px solid var(--ui-border); color: var(--ui-accent); }
.home-service-card__price { display: block; color: var(--ui-text-primary); font-size: 15px; font-weight: 800; white-space: nowrap; }
.home-service-card__original { color: var(--ui-text-muted); font-size: 11px; text-decoration: line-through; }
.home-service-card__details { color: var(--ui-accent); font-size: 12px; font-weight: 700; }
@media (max-width: 480px) {
  .home-service-card { min-height: 222px; padding: 14px; }
  .home-service-card__icon { width: 46px; height: 46px; border-radius: 14px; }
  .home-service-card__icon img { width: 34px; height: 34px; }
  .home-service-card__body h3 { font-size: 14px; }
  .home-service-card__price { font-size: 13px; }
}
</style>
