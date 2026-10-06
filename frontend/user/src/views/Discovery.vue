<template>
  <div class="discovery-page">
    <div class="hcz-shell-container discovery-page__inner">
      <!-- 页头 -->
      <header class="disc-hero" aria-labelledby="disc-title">
        <span class="disc-hero__eyebrow">HCZ · {{ t('discover.title') }}</span>
        <h1 id="disc-title">{{ t('discover.title') }}</h1>
        <p>{{ t('discover.subtitle') }}</p>
      </header>

      <!-- 热门服务（productAPI，服务卡片） -->
      <section class="disc-section" aria-labelledby="disc-hot">
        <div class="disc-section__head">
          <div>
            <span class="disc-section__eyebrow">{{ t('discover.hotEyebrow') }}</span>
            <h2 id="disc-hot">{{ t('discover.hotTitle') }}</h2>
          </div>
          <RouterLink to="/products" class="disc-section__more">
            {{ t('discover.allServices') }} <ArrowRight :size="15" aria-hidden="true" />
          </RouterLink>
        </div>

        <div v-if="hotLoading" class="disc-card-grid" role="status" :aria-label="t('discover.loading')">
          <div v-for="index in 4" :key="index" class="disc-skeleton disc-skeleton--card"></div>
        </div>
        <div v-else-if="hotError" class="disc-state" role="alert">
          <Flame :size="28" aria-hidden="true" />
          <p>{{ t('discover.hotError') }}</p>
          <button type="button" @click="loadHot">{{ t('discover.retry') }}</button>
        </div>
        <div v-else-if="hotProducts.length" class="disc-card-grid">
          <HomeServiceCard v-for="product in hotProducts" :key="product.id" :product="product" />
        </div>
        <div v-else class="disc-state">
          <Flame :size="28" aria-hidden="true" />
          <p>{{ t('discover.hotEmpty') }}</p>
        </div>
      </section>

      <!-- 服务分类入口（categoryAPI） -->
      <section class="disc-section" aria-labelledby="disc-cat">
        <div class="disc-section__head">
          <div>
            <span class="disc-section__eyebrow">{{ t('discover.categoryEyebrow') }}</span>
            <h2 id="disc-cat">{{ t('discover.categoryTitle') }}</h2>
          </div>
        </div>

        <div v-if="categoriesLoading" class="disc-cat-grid" role="status" :aria-label="t('discover.loading')">
          <div v-for="index in 6" :key="index" class="disc-skeleton disc-skeleton--cat"></div>
        </div>
        <div v-else-if="categoriesError" class="disc-state" role="alert">
          <LayoutGrid :size="28" aria-hidden="true" />
          <p>{{ t('discover.categoryError') }}</p>
          <button type="button" @click="loadCategories">{{ t('discover.retry') }}</button>
        </div>
        <div v-else-if="categories.length" class="disc-cat-grid">
          <RouterLink
            v-for="category in categories"
            :key="category.id"
            :to="{ name: 'category-products', params: { slug: category.slug } }"
            class="disc-cat-card"
          >
            <span class="disc-cat-card__icon">
              <component :is="getServiceIcon(category)" :size="20" :stroke-width="1.9" aria-hidden="true" />
            </span>
            <span class="disc-cat-card__label">{{ categoryLabel(category) }}</span>
            <ArrowUpRight :size="14" class="disc-cat-card__arrow" aria-hidden="true" />
          </RouterLink>
        </div>
        <div v-else class="disc-state">
          <LayoutGrid :size="28" aria-hidden="true" />
          <p>{{ t('discover.categoryEmpty') }}</p>
        </div>
      </section>

      <!-- 平台公告 / 活动（postAPI type=notice） -->
      <section class="disc-section" aria-labelledby="disc-notice">
        <div class="disc-section__head">
          <div>
            <span class="disc-section__eyebrow">{{ t('discover.noticeEyebrow') }}</span>
            <h2 id="disc-notice">{{ t('discover.noticeTitle') }}</h2>
          </div>
          <RouterLink to="/notice" class="disc-section__more">
            {{ t('discover.viewAll') }} <ArrowRight :size="15" aria-hidden="true" />
          </RouterLink>
        </div>

        <div v-if="noticesLoading" class="disc-notice-box" role="status" :aria-label="t('discover.loading')">
          <div v-for="index in 3" :key="index" class="disc-skeleton disc-skeleton--row"></div>
        </div>
        <div v-else-if="noticesError" class="disc-state" role="alert">
          <Megaphone :size="28" aria-hidden="true" />
          <p>{{ t('discover.noticeError') }}</p>
          <button type="button" @click="loadNotices">{{ t('discover.retry') }}</button>
        </div>
        <div v-else-if="notices.length" class="disc-notice-box">
          <RouterLink
            v-for="post in notices"
            :key="post.id"
            :to="{ name: 'blog-detail', params: { slug: post.slug } }"
            class="disc-notice-item"
          >
            <span class="disc-notice-item__title">{{ localizedText(post.title) }}</span>
            <ArrowUpRight :size="16" aria-hidden="true" />
          </RouterLink>
        </div>
        <div v-else class="disc-state">
          <Megaphone :size="28" aria-hidden="true" />
          <p>{{ t('discover.noticeEmpty') }}</p>
        </div>
      </section>

      <!-- 数字权益 / 精选推荐（productAPI 下一页真实数据） -->
      <section class="disc-section" aria-labelledby="disc-benefit">
        <div class="disc-section__head">
          <div>
            <span class="disc-section__eyebrow">{{ t('discover.benefitEyebrow') }}</span>
            <h2 id="disc-benefit">{{ t('discover.benefitTitle') }}</h2>
          </div>
          <RouterLink to="/products" class="disc-section__more">
            {{ t('discover.allServices') }} <ArrowRight :size="15" aria-hidden="true" />
          </RouterLink>
        </div>

        <div v-if="benefitsLoading" class="disc-card-grid" role="status" :aria-label="t('discover.loading')">
          <div v-for="index in 4" :key="index" class="disc-skeleton disc-skeleton--card"></div>
        </div>
        <div v-else-if="benefitsError" class="disc-state" role="alert">
          <Sparkles :size="28" aria-hidden="true" />
          <p>{{ t('discover.benefitError') }}</p>
          <button type="button" @click="loadBenefits">{{ t('discover.retry') }}</button>
        </div>
        <div v-else-if="benefits.length" class="disc-card-grid">
          <HomeServiceCard v-for="product in benefits" :key="product.id" :product="product" />
        </div>
        <div v-else class="disc-state">
          <Sparkles :size="28" aria-hidden="true" />
          <p>{{ t('discover.benefitEmpty') }}</p>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ArrowUpRight, Flame, LayoutGrid, Megaphone, Sparkles } from 'lucide-vue-next'
import { categoryAPI, postAPI, productAPI } from '../api'
import { useLocalized } from '../composables/useProduct'
import { usePageSeo } from '../composables/usePageSeo'
import { getServiceIcon } from '../utils/serviceIcon'
import type { PublicCategory } from '../utils/category'
import HomeServiceCard from '../components/home/HomeServiceCard.vue'

const { t } = useI18n()
const { getLocalizedText } = useLocalized()
usePageSeo({ canonicalPath: () => '/discovery' })

const localizedText = (value: unknown): string => typeof value === 'string' ? value : getLocalizedText(value)
const categoryLabel = (category: PublicCategory) => localizedText(category.name) || category.slug || ''

// 热门服务
const hotProducts = ref<any[]>([])
const hotLoading = ref(true)
const hotError = ref(false)

// 服务分类
const categories = ref<PublicCategory[]>([])
const categoriesLoading = ref(true)
const categoriesError = ref(false)

// 平台公告
const notices = ref<any[]>([])
const noticesLoading = ref(true)
const noticesError = ref(false)

// 精选推荐（商品列表第 2 页真实数据）
const benefits = ref<any[]>([])
const benefitsLoading = ref(true)
const benefitsError = ref(false)

const loadHot = async () => {
  hotLoading.value = true
  hotError.value = false
  try {
    const response = await productAPI.list({ page: 1, page_size: 8 })
    hotProducts.value = (Array.isArray(response.data.data) ? response.data.data : []).filter((p: any) => p?.slug)
  } catch {
    hotError.value = true
  } finally {
    hotLoading.value = false
  }
}

const loadBenefits = async () => {
  benefitsLoading.value = true
  benefitsError.value = false
  try {
    const response = await productAPI.list({ page: 2, page_size: 8 })
    benefits.value = (Array.isArray(response.data.data) ? response.data.data : []).filter((p: any) => p?.slug)
  } catch {
    benefitsError.value = true
  } finally {
    benefitsLoading.value = false
  }
}

const loadCategories = async () => {
  categoriesLoading.value = true
  categoriesError.value = false
  try {
    const response = await categoryAPI.list()
    categories.value = (Array.isArray(response.data.data) ? response.data.data : []).filter(
      (c: PublicCategory) => typeof c.slug === 'string' && c.slug.length > 0,
    )
  } catch {
    categoriesError.value = true
  } finally {
    categoriesLoading.value = false
  }
}

const loadNotices = async () => {
  noticesLoading.value = true
  noticesError.value = false
  try {
    const response = await postAPI.list({ type: 'notice', page: 1, page_size: 5 })
    notices.value = (Array.isArray(response.data.data) ? response.data.data : []).filter((p: any) => p?.slug)
  } catch {
    noticesError.value = true
  } finally {
    noticesLoading.value = false
  }
}

onMounted(() => {
  void Promise.allSettled([loadHot(), loadBenefits(), loadCategories(), loadNotices()])
})
</script>

<style scoped>
.discovery-page {
  min-width: 0;
  padding-top: 104px;
  padding-bottom: 56px;
  background: var(--ui-bg-page);
  color: var(--ui-text-primary);
}

/* ===== 页头 ===== */
.disc-hero { padding: clamp(8px, 2vw, 16px) 0 4px; }
.disc-hero__eyebrow {
  display: inline-block;
  border-radius: 999px;
  padding: 5px 12px;
  background: var(--ui-accent-soft);
  color: var(--ui-accent);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: .08em;
}
.disc-hero h1 { margin-top: 14px; font-size: clamp(26px, 3.4vw, 38px); font-weight: 800; letter-spacing: -.025em; line-height: 1.15; }
.disc-hero p { margin-top: 10px; max-width: 560px; color: var(--ui-text-secondary); font-size: 14px; line-height: 1.6; }

/* ===== Section 通用 ===== */
.disc-section { min-width: 0; margin-top: 48px; }
.disc-section__head { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.disc-section__eyebrow { color: var(--ui-accent); font-size: 11px; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }
.disc-section__head h2 { margin-top: 5px; font-size: clamp(20px, 2vw, 26px); font-weight: 800; letter-spacing: -.02em; }
.disc-section__more { display: inline-flex; flex: none; align-items: center; gap: 5px; padding-bottom: 3px; color: var(--ui-text-secondary); font-size: 13px; font-weight: 700; }
.disc-section__more:hover { color: var(--ui-accent); }

/* ===== 服务卡片网格：Mobile 单列 / Tablet 2列 / Desktop 3-4列 ===== */
.disc-card-grid { display: grid; grid-template-columns: 1fr; gap: 14px; }
@media (min-width: 768px) {
  .disc-card-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (min-width: 1024px) {
  .disc-card-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (min-width: 1280px) {
  .disc-card-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}

/* ===== 分类入口 ===== */
.disc-cat-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
@media (min-width: 768px) {
  .disc-cat-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (min-width: 1024px) {
  .disc-cat-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}
.disc-cat-card {
  display: flex;
  min-width: 0;
  min-height: 84px;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  padding: 14px;
  background: var(--ui-bg-elevated);
  color: var(--ui-text-primary);
  transition: border-color 180ms ease, transform 180ms ease;
}
.disc-cat-card:hover,
.disc-cat-card:focus-visible { border-color: var(--ui-accent); transform: translateY(-2px); }
.disc-cat-card__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: none;
  place-items: center;
  border-radius: 13px;
  background: var(--ui-accent-soft);
  color: var(--ui-accent);
}
.disc-cat-card__label { min-width: 0; flex: 1; overflow: hidden; font-size: 13px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.disc-cat-card__arrow { flex: none; color: var(--ui-text-muted); }

/* ===== 公告列表 ===== */
.disc-notice-box { overflow: hidden; border: 1px solid var(--ui-border); border-radius: var(--ui-radius-xl); background: var(--ui-bg-elevated); }
.disc-notice-item { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 58px; padding: 14px 20px; color: var(--ui-text-primary); font-size: 13px; font-weight: 650; }
.disc-notice-item + .disc-notice-item { border-top: 1px solid var(--ui-border); }
.disc-notice-item__title { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.disc-notice-item svg { flex: none; color: var(--ui-text-muted); }
.disc-notice-item:hover { color: var(--ui-accent); }

/* ===== 三态：骨架屏 / 空 / 错误 ===== */
.disc-skeleton { background: var(--ui-bg-muted); animation: theme-shimmer 1.4s ease infinite; }
.disc-skeleton--card { height: 235px; border-radius: var(--ui-radius-xl); }
.disc-skeleton--cat { height: 84px; border-radius: var(--ui-radius-lg); }
.disc-skeleton--row { height: 58px; }

.disc-state {
  display: flex;
  min-height: 150px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 11px;
  border: 1px dashed var(--ui-border-strong);
  border-radius: var(--ui-radius-xl);
  background: var(--ui-bg-elevated);
  color: var(--ui-text-muted);
  font-size: 13px;
  text-align: center;
}
.disc-state button { border-radius: 10px; padding: 8px 16px; background: var(--ui-accent-soft); color: var(--ui-accent); font-weight: 700; }

@media (max-width: 767px) {
  .discovery-page { padding-top: 88px; padding-bottom: 40px; }
  .disc-section { margin-top: 36px; }
  .disc-section__head { margin-bottom: 14px; }
  .disc-cat-grid { gap: 10px; }
}
</style>
