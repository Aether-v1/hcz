<template>
  <Card
    class="group flex h-full cursor-pointer flex-col rounded-xl p-4 transition-colors hover:border-primary/40 hover:bg-primary/5 theme-slide-up"
    :class="isSoldOut(product) ? 'opacity-60' : ''"
    :style="{ animationDelay: `${index * animationStep}ms` }"
    @click="$emit('click', product.slug)"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-primary/10 text-primary">
        <img v-if="serviceIcon" :src="serviceIcon" :alt="getLocalizedText(product.title)" class="h-9 w-9 object-contain" loading="lazy" />
        <component :is="getServiceIcon(product.category)" v-else class="h-6 w-6" aria-hidden="true" />
      </div>
      <span class="rounded-full bg-secondary px-2 py-1 text-[11px] text-muted-foreground">
        {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
      </span>
    </div>
    <div class="mt-4 min-w-0 flex-1">
      <p v-if="product.category?.name" class="truncate text-xs text-muted-foreground">{{ getLocalizedText(product.category.name) }}</p>
      <h3 class="mt-1 line-clamp-2 text-base font-bold">{{ getLocalizedText(product.title) }}</h3>
      <p class="mt-2 line-clamp-2 min-h-10 text-sm leading-5 text-muted-foreground">{{ getLocalizedText(product.description) }}</p>
    </div>
    <div class="mt-4 flex items-end justify-between gap-2 border-t pt-3">
      <div>
        <p class="text-xs text-muted-foreground">{{ t('products.price') }}</p>
        <p class="mt-1 font-semibold tabular-nums">{{ formatPrice(hasPromotionPrice(product) ? getPromotionPriceAmount(product) : product.price_amount, siteCurrency) }}{{ t('products.startingAt') }}</p>
      </div>
      <span class="inline-flex items-center gap-1 text-sm font-semibold text-primary">
        {{ t('products.enterService') }} <ArrowRight class="h-4 w-4 transition-transform group-hover:translate-x-1" />
      </span>
    </div>
  </Card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../utils/image'
import { getServiceIcon } from '../utils/serviceIcon'
import { useLocalized, useProductLabels } from '../composables/useProduct'
import { Card } from '@/components/ui/card'

const props = withDefaults(defineProps<{ product: any; index?: number; maxTags?: number; animationStep?: number }>(), {
  index: 0,
  maxTags: 2,
  animationStep: 50,
})

defineEmits<{ click: [slug: string]; quickBuy: [product: any] }>()
const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { getFulfillmentTypeLabel, isSoldOut, hasPromotionPrice, getPromotionPriceAmount } = useProductLabels()
const serviceIcon = computed(() => {
  const icon = props.product?.category?.icon
  return icon ? getImageUrl(icon) : getFirstImageUrl(props.product?.images)
})
</script>
