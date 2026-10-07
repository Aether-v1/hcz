<template>
  <div class="home-v2">
    <div class="home-v2__container">
      <!-- ===== Hero: Banner + User Summary Card (desktop split) ===== -->
      <div class="home-hero">
        <div class="home-hero__banner">
          <!-- ===== Banner Carousel ===== -->
          <section
            v-if="bannerCount > 0"
            class="home-banner"
            aria-label="Banner"
            @touchstart.passive="onTouchStart"
            @touchend="onTouchEnd"
          >
        <div
          class="home-banner__track"
          :style="{ transform: `translateX(-${currentBannerIndex * 100}%)` }"
        >
          <component
            v-for="(banner, idx) in banners"
            :key="banner.id || idx"
            :is="bannerLink(banner) ? 'a' : 'div'"
            :href="bannerLink(banner) || undefined"
            :target="isExternalLink(banner) ? '_blank' : undefined"
            :rel="isExternalLink(banner) ? 'noopener noreferrer' : undefined"
            class="home-banner__slide"
            @click.prevent="handleBannerClick(banner)"
          >
            <img
              :src="bannerImage(banner)"
              :alt="bannerText(banner.title)"
              class="home-banner__image"
              loading="eager"
              @error="onBannerImageError(idx)"
            />
          </component>
        </div>
        <div v-if="bannerCount > 1" class="home-banner__dots">
          <button
            v-for="(banner, idx) in banners"
            :key="`dot-${banner.id || idx}`"
            type="button"
            class="home-banner__dot"
            :class="{ 'is-active': idx === currentBannerIndex }"
            :aria-label="`Banner ${idx + 1}`"
            @click.prevent="selectBanner(idx)"
          />
        </div>
      </section>

      <!-- Banner Skeleton -->
      <div v-else-if="configLoading" class="home-banner home-banner--skeleton" aria-hidden="true">
        <div class="home-banner__skeleton-block" />
      </div>
        </div><!-- /.home-hero__banner -->

        <!-- User Summary Card (desktop only, hidden <1024px via CSS) -->
        <UserSummaryCard class="home-hero__card" />
      </div><!-- /.home-hero -->

      <!-- ===== Core Entries ===== -->
      <section class="home-section">

        <!-- Skeleton -->
        <div v-if="configLoading" class="home-entries" aria-hidden="true">
          <div v-for="i in 4" :key="`skel-entry-${i}`" class="home-entry home-entry--skeleton">
            <div class="home-entry__icon-skeleton" />
            <div class="home-entry__label-skeleton" />
          </div>
        </div>

        <!-- Entries -->
        <div v-else-if="coreEntries.length" class="home-entries">
          <component
            v-for="entry in coreEntries"
            :key="entry.id"
            :is="entry.external ? 'a' : RouterLink"
            :href="entry.external ? entry.href : undefined"
            :target="entry.external ? '_blank' : undefined"
            :rel="entry.external ? 'noopener noreferrer' : undefined"
            :to="entry.external ? undefined : entry.href"
            class="home-entry"
          >
            <span class="home-entry__icon">
              <component :is="resolveHomeEntryIcon(entry.icon)" :size="26" :stroke-width="1.8" aria-hidden="true" />
            </span>
            <span class="home-entry__label">{{ entry.title }}</span>
          </component>
        </div>

        <!-- Empty -->
        <div v-else class="home-empty">
          <p>{{ t('homeV2.noEntries') }}</p>
        </div>
      </section>

      <!-- ===== Featured Categories ===== -->
      <section v-if="featuredCategories.length > 0 || configLoading" class="home-section" aria-labelledby="home-featured-title">
        <div class="home-section__head">
          <h2 id="home-featured-title" class="home-section__title">{{ t('homeV2.featuredTitle') }}</h2>
          <RouterLink v-if="featuredCategories.length > 0" to="/products" class="home-section__more">
            {{ t('homeV2.allServices') }}
          </RouterLink>
        </div>

        <!-- Skeleton -->
        <div v-if="configLoading" class="home-featured" aria-hidden="true">
          <div v-for="i in 6" :key="`skel-cat-${i}`" class="home-featured__card home-featured__card--skeleton">
            <div class="home-featured__icon-skeleton" />
            <div class="home-featured__label-skeleton" />
          </div>
        </div>

        <!-- Categories -->
        <div v-else class="home-featured">
          <RouterLink
            v-for="cat in featuredCategories"
            :key="cat.id"
            :to="{ name: 'category-products', params: { slug: cat.slug } }"
            class="home-featured__card"
          >
            <span class="home-featured__icon">
              <img v-if="cat.icon" :src="getImageUrl(cat.icon)" :alt="cat.name" class="home-featured__icon-img" />
              <component v-else :is="getCategoryFallbackIcon(cat.slug)" :size="28" :stroke-width="1.6" aria-hidden="true" />
            </span>
            <span class="home-featured__body">
              <span class="home-featured__title">{{ cat.name }}</span>
              <span v-if="cat.description" class="home-featured__desc">{{ cat.description }}</span>
            </span>
          </RouterLink>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import {
  Smartphone, Wifi, Gamepad2, HousePlug, PlayCircle,
  ShoppingBag, Plane, CreditCard, Gift, Globe,
} from 'lucide-vue-next'
import { useSiteConfig } from '../../composables/useSiteConfig'
import { useAppStore } from '../../stores/app'
import { usePageSeo } from '../../composables/usePageSeo'
import { resolveHomeEntryIcon } from './homeEntryIcons'
import { getImageUrl } from '../../utils/image'
import type { BannerItem } from '../../types/siteConfig'
import UserSummaryCard from './UserSummaryCard.vue'

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()
const { banners, homeEntries, featuredCategories } = useSiteConfig()
usePageSeo({ canonicalPath: () => '/' })

const configLoading = computed(() => appStore.loading)

// ===== Banner Carousel =====
const currentBannerIndex = ref(0)
const bannerImageErrors = ref<Set<number>>(new Set())
let autoPlayTimer: ReturnType<typeof setInterval> | null = null

const bannerCount = computed(() => banners.value.length)

const bannerText = (value: unknown): string => {
  if (typeof value === 'string') return value
  if (value && typeof value === 'object') {
    const record = value as Record<string, string>
    return record[appStore.locale] || record['zh-CN'] || record['en-US'] || Object.values(record)[0] || ''
  }
  return ''
}

const bannerImage = (banner: BannerItem): string => {
  const isMobile = typeof window !== 'undefined' && window.innerWidth < 768
  if (isMobile && banner.mobile_image) return getImageUrl(banner.mobile_image)
  return getImageUrl(banner.image || banner.mobile_image || '')
}

const bannerLink = (banner: BannerItem): string => {
  if (banner.link_type === 'none') return ''
  return banner.link_value || ''
}

const isExternalLink = (banner: BannerItem): boolean => {
  const link = bannerLink(banner)
  return /^https?:\/\//i.test(link) || Boolean(banner.open_in_new_tab)
}

const handleBannerClick = (banner: BannerItem) => {
  const link = bannerLink(banner)
  if (!link) return
  if (isExternalLink(banner)) {
    window.open(link, banner.open_in_new_tab ? '_blank' : '_self')
    return
  }
  // internal link: use router push
  if (link.startsWith('/')) {
    router.push(link)
  }
}

const onBannerImageError = (idx: number) => {
  bannerImageErrors.value.add(idx)
}

const selectBanner = (idx: number) => {
  if (bannerCount.value === 0) return
  currentBannerIndex.value = ((idx % bannerCount.value) + bannerCount.value) % bannerCount.value
  restartAutoPlay()
}

const nextBanner = () => {
  if (bannerCount.value <= 1) return
  currentBannerIndex.value = (currentBannerIndex.value + 1) % bannerCount.value
}

const stopAutoPlay = () => {
  if (autoPlayTimer) {
    clearInterval(autoPlayTimer)
    autoPlayTimer = null
  }
}

const startAutoPlay = () => {
  stopAutoPlay()
  if (bannerCount.value <= 1) return
  autoPlayTimer = setInterval(nextBanner, 5000)
}

const restartAutoPlay = () => {
  stopAutoPlay()
  startAutoPlay()
}

// Touch swipe
let touchStartX = 0
const onTouchStart = (e: TouchEvent) => {
  touchStartX = e.touches[0]?.clientX ?? 0
}
const onTouchEnd = (e: TouchEvent) => {
  const diff = touchStartX - (e.changedTouches[0]?.clientX ?? 0)
  if (Math.abs(diff) > 50) {
    if (diff > 0) {
      currentBannerIndex.value = (currentBannerIndex.value + 1) % bannerCount.value
    } else {
      currentBannerIndex.value = (currentBannerIndex.value - 1 + bannerCount.value) % bannerCount.value
    }
    restartAutoPlay()
  }
}

// ===== Core Entries: enabled + sort + top 4 =====
const coreEntries = computed(() => {
  return homeEntries.value
    .filter((e) => e.href && (e.external ? /^https?:\/\//i.test(e.href) : e.href.startsWith('/')))
    .slice(0, 4)
})

// ===== Category fallback icon mapping =====
const categoryIconMap: Record<string, typeof Smartphone> = {
  phone: Smartphone,
  data: Wifi,
  game: Gamepad2,
  bills: HousePlug,
  video: PlayCircle,
  shopping: ShoppingBag,
  travel: Plane,
  credit: CreditCard,
  gift: Gift,
  global: Globe,
}

const getCategoryFallbackIcon = (slug: string) => {
  const lower = slug.toLowerCase()
  for (const [key, icon] of Object.entries(categoryIconMap)) {
    if (lower.includes(key)) return icon
  }
  return Gift
}

onMounted(() => {
  startAutoPlay()
})

onUnmounted(() => {
  stopAutoPlay()
})
</script>

<style scoped>
.home-v2 {
  min-height: 100%;
}

.home-v2__container {
  padding-top: 0;
  padding-bottom: calc(40px + env(safe-area-inset-bottom, 0px));
}

@media (min-width: 768px) {
  .home-v2__container {
    padding-top: 0;
    padding-bottom: 40px;
  }
}

/* ===== Hero Split (Banner + User Card) ===== */
.home-hero {
  display: flex;
  flex-direction: column;
  gap: 0;
  margin-bottom: 12px;
  min-width: 0;
}

.home-hero__banner {
  min-width: 0;
}

.home-hero__card {
  display: none;
}

@media (min-width: 1024px) {
  .home-hero {
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(300px, 0.8fr);
    gap: 20px;
    margin-bottom: 20px;
    align-items: stretch;
  }

  .home-hero__card {
    display: block;
  }

  /* Banner inside hero: remove standalone bottom margin, match card radius */
  .home-hero .home-banner {
    margin-bottom: 0;
    height: 100%;
    aspect-ratio: auto;
  }
}

/* ===== Banner ===== */
.home-banner {
  position: relative;
  border-radius: 16px;
  overflow: hidden;
  margin-bottom: 24px;
  aspect-ratio: 16 / 7;
  background: var(--secondary, #1a1a24);
}

@media (min-width: 768px) {
  .home-banner {
    aspect-ratio: 21 / 8;
    border-radius: 20px;
    margin-bottom: 32px;
  }
}

.home-banner__track {
  display: flex;
  width: 100%;
  height: 100%;
  transition: transform 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.home-banner__slide {
  flex: 0 0 100%;
  width: 100%;
  height: 100%;
  display: block;
  position: relative;
}

.home-banner__image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.home-banner__dots {
  position: absolute;
  bottom: 10px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 6px;
  z-index: 2;
}

.home-banner__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  border: none;
  padding: 0;
  background: rgba(255, 255, 255, 0.4);
  cursor: pointer;
  transition: all 0.3s ease;
}

.home-banner__dot.is-active {
  width: 18px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.9);
}

.home-banner--skeleton {
  background: var(--secondary, #1a1a24);
}

.home-banner__skeleton-block {
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255,255,255,0.05), transparent);
  animation: shimmer 1.5s infinite;
}

@keyframes shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

/* ===== Section ===== */
.home-section {
  margin-bottom: 28px;
}

.home-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.home-section__title {
  font-size: 17px;
  font-weight: 700;
  color: var(--foreground, #f5f5f7);
  margin: 0;
  letter-spacing: -0.01em;
}

@media (min-width: 768px) {
  .home-section__title {
    font-size: 20px;
  }
}

.home-section__more {
  font-size: 13px;
  color: var(--muted-foreground, #86868b);
  text-decoration: none;
  font-weight: 500;
  transition: color 0.2s;
}

.home-section__more:hover {
  color: var(--primary, #4F46E5);
}

/* ===== Core Entries (4 columns) ===== */
.home-entries {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

@media (min-width: 768px) {
  .home-entries {
    gap: 16px;
  }
}

.home-entry {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 12px 4px;
  border-radius: 14px;
  text-decoration: none;
  transition: transform 0.2s, background 0.2s;
}

.home-entry:hover {
  background: var(--accent, rgba(255,255,255,0.05));
  transform: translateY(-2px);
}

.home-entry:active {
  transform: translateY(0);
}

.home-entry__icon {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--primary, #4F46E5);
  color: #fff;
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.25);
}

@media (min-width: 768px) {
  .home-entry__icon {
    width: 56px;
    height: 56px;
    border-radius: 16px;
  }
}

.home-entry__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--foreground, #f5f5f7);
  text-align: center;
  line-height: 1.3;
}

@media (min-width: 768px) {
  .home-entry__label {
    font-size: 14px;
  }
}

/* Entry skeleton */
.home-entry--skeleton {
  pointer-events: none;
}

.home-entry__icon-skeleton {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: var(--secondary, #1a1a24);
  animation: pulse 1.5s ease-in-out infinite;
}

.home-entry__label-skeleton {
  width: 70%;
  height: 12px;
  border-radius: 4px;
  background: var(--secondary, #1a1a24);
  animation: pulse 1.5s ease-in-out infinite 0.1s;
}

@keyframes pulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

/* ===== Featured Categories: 2-column grid ===== */
.home-featured {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.home-featured__card {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 16px;
  background: var(--card, #16161e);
  border: 1px solid var(--border, rgba(255,255,255,0.06));
  text-decoration: none;
  transition: transform 0.2s, border-color 0.2s, box-shadow 0.2s;
}

.home-featured__icon {
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: var(--muted, rgba(255,255,255,0.06));
  color: var(--primary, #4F46E5);
}

.home-featured__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.home-featured__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--foreground, #fff);
  line-height: 1.3;
}

.home-featured__desc {
  font-size: 12px;
  color: var(--muted-foreground, rgba(255,255,255,0.5));
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.home-featured__card:hover {
  transform: translateY(-3px);
  border-color: var(--primary, #4F46E5);
  box-shadow: 0 8px 24px rgba(79, 70, 229, 0.15);
}

.home-featured__icon-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 8px;
}

/* Category skeleton */
.home-featured__card--skeleton {
  pointer-events: none;
  border-color: transparent;
}

.home-featured__icon-skeleton {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: var(--secondary, #1a1a24);
  animation: pulse 1.5s ease-in-out infinite;
}

.home-featured__label-skeleton {
  width: 65%;
  height: 12px;
  border-radius: 4px;
  background: var(--secondary, #1a1a24);
  animation: pulse 1.5s ease-in-out infinite 0.1s;
}

/* ===== Empty ===== */
.home-empty {
  text-align: center;
  padding: 24px;
  color: var(--muted-foreground, #86868b);
  font-size: 14px;
}
</style>
