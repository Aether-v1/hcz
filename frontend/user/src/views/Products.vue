<template>
  <div class="min-h-screen bg-background pb-16 pt-20 text-foreground">
    <div class="container mx-auto px-4">
      <section class="mt-7 overflow-hidden rounded-3xl bg-slate-950 text-white">
        <div class="grid gap-8 p-7 md:p-10 lg:grid-cols-[minmax(0,1fr)_370px] lg:items-end">
          <div>
            <p class="text-xs font-bold uppercase tracking-[0.2em] text-sky-300">HCZ / {{ t('nav.products') }}</p>
            <h1 class="mt-4 text-3xl font-bold tracking-tight md:text-4xl">{{ t('products.serviceHeroTitle') }}</h1>
            <p class="mt-3 max-w-2xl text-sm leading-7 text-slate-300">{{ t('products.serviceHeroDescription') }}</p>
          </div>
          <div class="grid grid-cols-3 gap-2 border-t border-white/15 pt-5 text-xs text-slate-300 lg:border-l lg:border-t-0 lg:pl-7 lg:pt-0">
            <div><span class="mb-2 block text-lg font-bold text-sky-300">01</span>{{ t('products.stepChoose') }}</div>
            <div><span class="mb-2 block text-lg font-bold text-sky-300">02</span>{{ t('products.stepDetails') }}</div>
            <div><span class="mb-2 block text-lg font-bold text-sky-300">03</span>{{ t('products.stepSubmit') }}</div>
          </div>
        </div>
      </section>

      <div class="mt-9 grid grid-cols-[104px_minmax(0,1fr)] items-start gap-3 sm:grid-cols-[200px_minmax(0,1fr)] sm:gap-6 lg:grid-cols-[240px_minmax(0,1fr)]">
      <aside class="sticky top-20 min-w-0 rounded-2xl border bg-card p-2 sm:top-24 sm:p-4" :aria-label="t('products.categories')">
        <h2 class="mb-3 px-1 text-sm font-bold sm:text-lg">{{ t('products.chooseCategory') }}</h2>
        <nav class="max-h-[calc(100vh-7rem)] space-y-1 overflow-y-auto">
          <button type="button" class="w-full rounded-lg px-2 py-2.5 text-left text-xs font-semibold transition-colors sm:px-3 sm:text-sm"
            :class="selectedCategory === null ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
            @click="selectCategory(null)">{{ t('products.allCategories') }}</button>
          <div v-for="group in categoryGroups" :key="group.id">
            <div class="flex min-w-0 items-center">
              <button type="button" class="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-2 py-2.5 text-left text-xs font-semibold transition-colors sm:px-3 sm:text-sm"
                :class="selectedCategory === group.id ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
                @click="selectCategory(group.id)">
                <img v-if="group.icon" :src="getImageUrl(group.icon)" alt="" class="hidden h-5 w-5 shrink-0 object-contain sm:block" />
                <component :is="getServiceIcon(group)" v-else class="hidden h-4 w-4 shrink-0 sm:block" aria-hidden="true" />
                <span class="min-w-0 break-words">{{ getLocalizedText(group.name) }}</span>
              </button>
              <button v-if="group.children.length" type="button" class="hidden h-8 w-8 shrink-0 items-center justify-center rounded-lg hover:bg-secondary sm:flex"
                :aria-label="getLocalizedText(group.name)" :aria-expanded="expandedParentIds.includes(group.id)" @click="toggleParentCategory(group.id)">
                <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': expandedParentIds.includes(group.id) }" />
              </button>
            </div>
            <div v-if="group.children.length && (expandedParentIds.includes(group.id) || selectedCategory === group.id)" class="ml-1 border-l pl-1 sm:ml-4 sm:pl-2">
              <button v-for="child in group.children" :key="child.id" type="button" class="w-full rounded-lg px-2 py-2 text-left text-xs transition-colors sm:text-sm"
                :class="selectedCategory === child.id ? 'bg-primary/10 font-semibold text-primary' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
                @click="selectCategory(child.id)">{{ getLocalizedText(child.name) }}</button>
            </div>
          </div>
        </nav>
      </aside>

      <section class="min-w-0">
        <div class="mb-5 flex flex-col gap-4 border-b pb-5 md:flex-row md:items-end md:justify-between">
          <div>
            <h2 class="text-xl font-bold">{{ t('products.availableServices') }}</h2>
            <p class="mt-1 text-sm text-muted-foreground">{{ t('products.listHint') }}</p>
          </div>
          <div class="flex w-full items-center gap-2 md:max-w-xs">
            <Search class="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <input v-model="searchQuery" type="search" class="h-10 min-w-0 flex-1 rounded-lg border bg-card px-3 text-sm outline-none focus:border-primary"
              :aria-label="t('products.searchLabel')" :placeholder="t('products.searchPlaceholder')" />
            <button v-if="searchQuery" type="button" class="text-xs font-semibold text-primary" @click="clearSearch">{{ t('products.clearFilters') }}</button>
          </div>
        </div>

        <div v-if="loading" class="grid gap-3 lg:grid-cols-2">
          <div v-for="i in 6" :key="i" class="h-24 animate-pulse rounded-xl border bg-card" />
        </div>
        <div v-else-if="products.length" class="grid gap-3 lg:grid-cols-2">
          <ProductListItem v-for="(product, index) in products" :key="product.id" :product="product" :index="index" compact
            @click="goToProduct" />
        </div>
        <EmptyState v-else variant="soft" icon="search"
          :title="(searchQuery || selectedCategory) ? t('products.emptyFiltered') : t('products.empty')">
          <template v-if="searchQuery || selectedCategory" #action>
            <Button variant="secondary" @click="clearSearch(); selectCategory(null)">{{ t('products.clearFilters') }}</Button>
          </template>
        </EmptyState>

        <PaginationNav :current-page="currentPage" :total-pages="totalPages" :loading="loading" @change-page="changePage" />
      </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Search } from 'lucide-vue-next'
import { useProductList } from '../composables/useProductList'
import { usePageSeo } from '../composables/usePageSeo'
import { useLocalized } from '../composables/useProduct'
import { getImageUrl } from '../utils/image'
import { getServiceIcon } from '../utils/serviceIcon'
import ProductListItem from '../components/ProductListItem.vue'
import PaginationNav from '../components/PaginationNav.vue'
import EmptyState from '../components/EmptyState.vue'
import { Button } from '@/components/ui/button'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const {
  loading, products, selectedCategory, searchQuery, currentPage, totalPages,
  categoryGroups, categoryMap, expandedParentIds, selectCategory, toggleParentCategory, changePage, clearSearch, initialize, cleanup,
} = useProductList({ pageSize: 12, homeRouteName: 'products' })

const seoCategoryName = computed(() => {
  if (!selectedCategory.value) return ''
  const category = categoryMap.value.get(selectedCategory.value)
  return category ? getLocalizedText(category.name) : ''
})
usePageSeo({
  canonicalPath: () => route.path,
  title: () => route.name === 'category-products' ? seoCategoryName.value || t('nav.products') : t('nav.products'),
})

const goToProduct = (slug: string) => router.push(`/products/${slug}`)
onMounted(() => { void initialize() })
onUnmounted(cleanup)
</script>
