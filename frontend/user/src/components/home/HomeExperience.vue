<template>
  <div class="hcz-home">
    <div class="hcz-shell-container hcz-home__content">
      <!-- 公告条（config.announcement，无则不渲染） -->
      <AnnouncementBar />

      <!-- 轮播 Banner（后端 GET /public/banners，position=home_hero） -->
      <section
        v-if="bannerCount > 0"
        class="home-hero-carousel"
        :aria-label="t('homeV2.brand')"
        @touchstart.passive="onBannerTouchStart"
        @touchend="onBannerTouchEnd"
      >
        <component
          :is="hasHeroLink ? 'a' : 'div'"
          :href="hasHeroLink ? heroLink : undefined"
          :target="hasHeroLink && heroOpenInNewTab ? '_blank' : undefined"
          :rel="hasHeroLink && heroOpenInNewTab ? 'noopener noreferrer' : undefined"
          class="home-hero-carousel__slide"
        >
          <img
            v-if="heroImage && !bannerImageFailed"
            :src="heroImage"
            :alt="heroTitle"
            class="home-hero-carousel__image"
            loading="eager"
            @error="bannerImageFailed = true"
          />
          <div class="home-hero-carousel__overlay">
            <span class="home-hero-carousel__badge">{{ heroBadge }}</span>
            <h1 class="home-hero-carousel__title">{{ heroTitle }}</h1>
            <p v-if="heroSubtitle" class="home-hero-carousel__subtitle">{{ heroSubtitle }}</p>
            <span v-if="hasHeroLink" class="home-hero-carousel__cta">
              {{ heroPrimaryButtonText }} <ArrowRight :size="15" aria-hidden="true" />
            </span>
          </div>
        </component>
        <button
          v-if="bannerCount > 1"
          type="button"
          class="home-hero-carousel__arrow home-hero-carousel__arrow--prev"
          :aria-label="t('common.previous')"
          @click.prevent="handlePrevHeroBanner"
        >
          <ChevronLeft :size="20" aria-hidden="true" />
        </button>
        <button
          v-if="bannerCount > 1"
          type="button"
          class="home-hero-carousel__arrow home-hero-carousel__arrow--next"
          :aria-label="t('common.next')"
          @click.prevent="handleNextHeroBanner"
        >
          <ChevronRight :size="20" aria-hidden="true" />
        </button>
        <div v-if="bannerCount > 1" class="home-hero-carousel__dots">
          <button
            v-for="(banner, index) in banners"
            :key="(banner.id as number) || index"
            type="button"
            class="home-hero-carousel__dot"
            :class="{ 'home-hero-carousel__dot--active': index === currentBannerIndex }"
            :aria-label="`Banner ${index + 1}`"
            :aria-current="index === currentBannerIndex"
            @click.prevent="selectHeroBanner(index)"
          ></button>
        </div>
      </section>

      <!-- Fallback：无 banner 时的简洁品牌展示 -->
      <section v-else class="home-hero-fallback" :aria-labelledby="'home-title'">
        <span class="home-hero-fallback__eyebrow">HCZ · {{ t('homeV2.brand') }}</span>
        <h1 id="home-title">{{ t('homeV2.title') }}</h1>
        <p>{{ t('homeV2.subtitle') }}</p>
      </section>

      <div v-if="!online" class="home-alert" role="status">
        <WifiOff :size="18" aria-hidden="true" /> {{ t('homeV2.offline') }}
      </div>

      <!-- 装修 Banner（config.banners，无则不渲染） -->
      <SiteBannerStrip />

      <!-- 常用服务入口（话费/流量/游戏/生活缴费） -->
      <section class="home-section" :aria-labelledby="'home-core-title'">
        <div class="home-section__head">
          <div>
            <span class="home-section__eyebrow">{{ t('homeV2.quickEyebrow') }}</span>
            <h2 id="home-core-title">{{ t('homeV2.quickTitle') }}</h2>
          </div>
          <RouterLink to="/products" class="home-section__more">{{ t('homeV2.allServices') }} <ArrowRight :size="16" aria-hidden="true" /></RouterLink>
        </div>
        <div class="home-core-grid">
          <template v-for="entry in coreEntries" :key="entry.key">
            <RouterLink v-if="entry.category" :to="{ name: 'category-products', params: { slug: entry.category.slug } }" class="home-core-card">
              <span class="home-core-card__icon"><component :is="entry.icon" :size="24" :stroke-width="1.9" aria-hidden="true" /></span>
              <strong>{{ entry.title }}</strong>
            </RouterLink>
            <div v-else class="home-core-card home-core-card--disabled" aria-disabled="true">
              <span class="home-core-card__icon"><component :is="entry.icon" :size="24" :stroke-width="1.9" aria-hidden="true" /></span>
              <strong>{{ entry.title }}</strong>
              <span class="home-core-card__desc">{{ categoriesLoading ? t('common.loading') : t('homeV2.unavailable') }}</span>
            </div>
          </template>
        </div>
        <div v-if="categoriesLoading" class="home-inline-state" role="status">{{ t('homeV2.loadingCategories') }}</div>
        <div v-else-if="categoriesError" class="home-inline-state" role="alert">
          {{ t('homeV2.categoriesError') }}
          <button type="button" @click="loadCategories">{{ t('homeV2.retry') }}</button>
        </div>
      </section>

      <section class="home-section" :aria-labelledby="'home-featured-title'">
        <div class="home-section__head">
          <div>
            <span class="home-section__eyebrow">{{ t('homeV2.featuredEyebrow') }}</span>
            <h2 id="home-featured-title">{{ t('homeV2.featuredTitle') }}</h2>
          </div>
          <RouterLink to="/products" class="home-section__more">{{ t('homeV2.allServices') }} <ArrowRight :size="16" aria-hidden="true" /></RouterLink>
        </div>
        <div v-if="productsLoading" class="home-recommend-grid" role="status" :aria-label="t('common.loading')">
          <div v-for="index in 4" :key="index" class="home-recommend-skeleton"></div>
        </div>
        <div v-else-if="productsError" class="home-state" role="alert">
          <PackageOpen :size="30" aria-hidden="true" />
          <p>{{ t('homeV2.productsError') }}</p>
          <button type="button" @click="loadProducts">{{ t('homeV2.retry') }}</button>
        </div>
        <div v-else-if="featuredProducts.length" class="home-recommend-grid">
          <HomeServiceCard v-for="product in featuredProducts" :key="product.id" :product="product" />
        </div>
        <div v-else class="home-state"><PackageOpen :size="30" aria-hidden="true" /><p>{{ t('homeV2.noProducts') }}</p></div>
      </section>

      <section v-if="moreCategories.length" class="home-section" :aria-labelledby="'home-more-title'">
        <div class="home-section__head">
          <div>
            <span class="home-section__eyebrow">{{ t('homeV2.moreEyebrow') }}</span>
            <h2 id="home-more-title">{{ t('homeV2.moreTitle') }}</h2>
          </div>
        </div>
        <div class="home-category-grid">
          <RouterLink v-for="category in moreCategories" :key="category.id" :to="{ name: 'category-products', params: { slug: category.slug } }" class="home-category-card">
            <span class="home-category-card__icon"><component :is="getServiceIcon(category)" :size="22" :stroke-width="1.9" aria-hidden="true" /></span>
            <span>{{ categoryLabel(category) }}</span>
            <ArrowUpRight :size="15" aria-hidden="true" />
          </RouterLink>
        </div>
      </section>

      <section v-if="newsPosts.length" class="home-section home-news" :aria-labelledby="'home-news-title'">
        <div class="home-section__head">
          <div><span class="home-section__eyebrow">{{ t('homeV2.newsEyebrow') }}</span><h2 id="home-news-title">{{ t('homeV2.newsTitle') }}</h2></div>
          <RouterLink :to="newsType === 'notice' ? '/notice' : '/blog'" class="home-section__more">{{ t('homeV2.viewAll') }} <ArrowRight :size="16" aria-hidden="true" /></RouterLink>
        </div>
        <div class="home-news__list">
          <RouterLink v-for="post in newsPosts" :key="post.id" :to="{ name: 'blog-detail', params: { slug: post.slug } }" class="home-news__item">
            <span>{{ localizedText(post.title) }}</span><ArrowUpRight :size="17" aria-hidden="true" />
          </RouterLink>
        </div>
      </section>
    </div>

    <AnnouncementModal v-if="activeAnnouncement" :announcement="activeAnnouncement" :visible="announcementVisible" @update:visible="announcementVisible = $event" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ArrowUpRight, ChevronLeft, ChevronRight, Gamepad2, HousePlug, PackageOpen, Smartphone, Wifi, WifiOff } from 'lucide-vue-next'
import { categoryAPI, postAPI, productAPI } from '../../api'
import { useAppStore } from '../../stores/app'
import { useLocalized } from '../../composables/useProduct'
import { useAnnouncement, type HomeAnnouncement } from '../../composables/useAnnouncement'
import { useBannerCarousel } from '../../composables/useBannerCarousel'
import { usePageSeo } from '../../composables/usePageSeo'
import { getServiceIcon } from '../../utils/serviceIcon'
import type { PublicCategory } from '../../utils/category'
import AnnouncementModal from '../AnnouncementModal.vue'
import HomeServiceCard from './HomeServiceCard.vue'
import AnnouncementBar from './AnnouncementBar.vue'
import SiteBannerStrip from './SiteBannerStrip.vue'
import './home.css'

const { t } = useI18n()
const appStore = useAppStore()
const { getLocalizedText } = useLocalized()
const { shouldShow } = useAnnouncement()
usePageSeo({ canonicalPath: () => '/' })

const categories = ref<PublicCategory[]>([])
const products = ref<any[]>([])
const categoriesLoading = ref(true)
const categoriesError = ref(false)
const productsLoading = ref(true)
const productsError = ref(false)
const online = ref(true)
const newsPosts = ref<any[]>([])
const activeAnnouncement = ref<HomeAnnouncement | null>(null)
const announcementVisible = ref(false)
const bannerImageFailed = ref(false)

const localizedText = (value: unknown): string => typeof value === 'string' ? value : getLocalizedText(value)
const categoryLabel = (category: PublicCategory) => localizedText(category.name) || category.slug || ''
const categoryCandidates = computed(() => {
  const result = [...categories.value]
  const seen = new Set(result.map(category => category.id))
  for (const product of products.value) {
    const category = product?.category as PublicCategory | undefined
    if (category?.id && !seen.has(category.id)) {
      result.push(category)
      seen.add(category.id)
    }
  }
  return result.filter(category => typeof category.slug === 'string' && category.slug.length > 0)
})
const categorySearchText = (category: PublicCategory) => {
  const names = category.name && typeof category.name === 'object' ? Object.values(category.name) : [category.name]
  return `${category.slug || ''} ${names.join(' ')}`.toLowerCase()
}
const coreDefinitions = [
  { key: 'phone', icon: Smartphone, matches: /话费|話費|airtime|phone.?recharge|mobile.?top.?up/ },
  { key: 'data', icon: Wifi, matches: /流量|數據|数据|traffic|data.?plan|mobile.?data/ },
  { key: 'game', icon: Gamepad2, matches: /游戏|遊戲|game/ },
  { key: 'bills', icon: HousePlug, matches: /生活缴费|生活繳費|utility|utilities|bill.?pay|生活服务|生活服務/ },
] as const
const coreEntries = computed(() => coreDefinitions.map(definition => ({
  key: definition.key,
  icon: definition.icon,
  title: t(`homeV2.core.${definition.key}.title`),
  category: categoryCandidates.value.find(category => definition.matches.test(categorySearchText(category))) || null,
})))
const moreCategories = computed(() => {
  const usedIds = new Set(coreEntries.value.map(entry => entry.category?.id).filter(Boolean))
  return categoryCandidates.value.filter(category => !usedIds.has(category.id)).slice(0, 12)
})
const featuredProducts = computed(() => products.value.filter(product => product?.slug && localizedText(product.title)).slice(0, 8))

const {
  banners,
  bannerCount,
  currentBannerIndex,
  heroImage,
  heroBadge,
  heroTitle,
  heroSubtitle,
  heroLink,
  hasHeroLink,
  heroPrimaryButtonText,
  heroOpenInNewTab,
  loadBanners,
  handleNextHeroBanner,
  handlePrevHeroBanner,
  selectHeroBanner,
  onBannerTouchStart,
  onBannerTouchEnd,
  stopHeroAutoPlay,
} = useBannerCarousel()
watch(heroImage, () => { bannerImageFailed.value = false })

const newsType = computed<'notice' | 'blog'>(() => appStore.config?.nav_config?.builtin?.notice !== false ? 'notice' : 'blog')
const newsEnabled = computed(() => appStore.config?.nav_config?.builtin?.notice !== false || appStore.config?.nav_config?.builtin?.blog !== false)

const loadCategories = async () => {
  categoriesLoading.value = true
  categoriesError.value = false
  try {
    const response = await categoryAPI.list()
    categories.value = Array.isArray(response.data.data) ? response.data.data : []
  } catch {
    categoriesError.value = true
  } finally {
    categoriesLoading.value = false
  }
}
const loadProducts = async () => {
  productsLoading.value = true
  productsError.value = false
  try {
    const response = await productAPI.list({ page: 1, page_size: 8 })
    products.value = Array.isArray(response.data.data) ? response.data.data : []
  } catch {
    productsError.value = true
  } finally {
    productsLoading.value = false
  }
}
const loadHomeBanners = async () => {
  // loadBanners 内部已调用 startHeroAutoPlay()，此处不再 stop
  await loadBanners()
}
const loadNews = async () => {
  if (!newsEnabled.value) return
  try {
    const response = await postAPI.list({ type: newsType.value, page: 1, page_size: 2 })
    newsPosts.value = Array.isArray(response.data.data) ? response.data.data.filter((post: any) => post?.slug) : []
  } catch {
    newsPosts.value = []
  }
}
const updateOnline = () => {
  const wasOffline = !online.value
  online.value = navigator.onLine
  if (wasOffline && navigator.onLine) {
    void Promise.allSettled([loadCategories(), loadProducts(), loadHomeBanners(), loadNews()])
  }
}

onMounted(() => {
  updateOnline()
  window.addEventListener('online', updateOnline)
  window.addEventListener('offline', updateOnline)
  const announcement = appStore.config?.announcement as HomeAnnouncement | undefined
  if (announcement && shouldShow(announcement)) {
    activeAnnouncement.value = announcement
    announcementVisible.value = true
  }
  void Promise.allSettled([loadCategories(), loadProducts(), loadHomeBanners(), loadNews()])
})
onUnmounted(() => {
  window.removeEventListener('online', updateOnline)
  window.removeEventListener('offline', updateOnline)
  stopHeroAutoPlay()
})
</script>
