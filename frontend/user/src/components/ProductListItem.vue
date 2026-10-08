<template>
  <Card
    class="group flex cursor-pointer items-center gap-4 rounded-xl p-3 transition-colors hover:border-primary/40 hover:bg-primary/5 theme-slide-up"
    :class="[isSoldOut(product) ? 'opacity-60' : '', compact ? 'max-sm:flex-wrap max-sm:gap-2' : '']"
    :style="{ animationDelay: `${index * animationStep}ms` }"
    @click="$emit('click', product.slug)"
  >
    <div class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-primary/10 text-primary" :class="compact ? 'max-sm:h-9 max-sm:w-9' : ''">
      <img v-if="serviceIcon" :src="serviceIcon" :alt="getLocalizedText(product.title)" class="h-9 w-9 object-contain" loading="lazy" />
      <component :is="getServiceIcon(product.category)" v-else class="h-5 w-5" aria-hidden="true" />
    </div>
    <div class="min-w-0 flex-1">
      <p v-if="product.category?.name" class="truncate text-xs text-muted-foreground">{{ getLocalizedText(product.category.name) }}</p>
      <h3 class="truncate text-sm font-semibold">{{ getLocalizedText(product.title) }}</h3>
      <p class="truncate text-xs text-muted-foreground">{{ getLocalizedText(product.description) }}</p>
    </div>
    <div class="shrink-0 text-right" :class="compact ? 'max-sm:w-full' : ''">
      <p class="text-sm font-semibold tabular-nums">{{ formatPrice(hasPromotionPrice(product) ? getPromotionPriceAmount(product) : product.price_amount, siteCurrency) }}{{ t('products.startingAt') }}</p>
    </div>
  </Card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getFirstImageUrl, getImageUrl } from '../utils/image'
import { getServiceIcon } from '../utils/serviceIcon'
import { useLocalized, useProductLabels } from '../composables/useProduct'
import { Card } from '@/components/ui/card'

const props = withDefaults(defineProps<{ product: any; index?: number; animationStep?: number; compact?: boolean }>(), {
  index: 0,
  animationStep: 30,
  compact: false,
})
defineEmits<{ click: [slug: string]; quickBuy: [product: any] }>()
const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { isSoldOut, hasPromotionPrice, getPromotionPriceAmount } = useProductLabels()
const serviceIcon = computed(() => {
  const icon = props.product?.category?.icon
  return icon ? getImageUrl(icon) : getFirstImageUrl(props.product?.images)
})
</script>
