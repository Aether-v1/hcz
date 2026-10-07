<template>
  <div class="discovery-page">
    <div class="hcz-shell-container discovery-page__inner">
      <!-- 页头 -->
      <header class="disc-hero" aria-labelledby="disc-title">
        <span class="disc-hero__eyebrow">HCZ · {{ t('discover.title') }}</span>
        <h1 id="disc-title">{{ t('discover.title') }}</h1>
        <p>{{ t('discover.subtitle') }}</p>
      </header>

      <!-- 装修驱动：按后端 discovery_blocks 动态渲染 -->
      <template v-if="hasDynamicBlocks">
        <section
          v-for="block in discoveryBlocks"
          :key="String(block.id)"
          class="disc-section"
        >
          <div v-if="block.title" class="disc-section__head">
            <div><h2>{{ block.title }}</h2></div>
          </div>

          <!-- banner: image + link_type/link_value -->
          <template v-if="block.type === 'banner'">
            <a
              v-if="resolveBlockLink(block.config?.link_type, block.config?.link_value).external"
              :href="String(block.config?.link_value)"
              target="_blank"
              rel="noopener"
              class="disc-banner"
            >
              <img :src="getImageUrl(block.config?.image)" :alt="block.title || 'banner'" />
            </a>
            <RouterLink
              v-else-if="resolveBlockLink(block.config?.link_type, block.config?.link_value).href"
              :to="resolveBlockLink(block.config?.link_type, block.config?.link_value).href"
              class="disc-banner"
            >
              <img :src="getImageUrl(block.config?.image)" :alt="block.title || 'banner'" />
            </RouterLink>
            <div v-else class="disc-banner">
              <img :src="getImageUrl(block.config?.image)" :alt="block.title || 'banner'" />
            </div>
          </template>

          <!-- card_grid: cards[] { title, subtitle, image, action_type, action_target } -->
          <div v-else-if="block.type === 'card_grid'" class="disc-card-grid">
            <template v-for="(card, ci) in (block.config?.cards || [])" :key="ci">
              <RouterLink
                v-if="card.action_type !== 'external'"
                :to="resolveInternalPath(card.action_target)"
                class="disc-mini-card"
              >
                <img v-if="card.image" :src="getImageUrl(card.image)" :alt="card.title" />
                <div class="disc-mini-card__body">
                  <span class="disc-mini-card__title">{{ card.title }}</span>
                  <span v-if="card.subtitle" class="disc-mini-card__subtitle">{{ card.subtitle }}</span>
                </div>
              </RouterLink>
              <a
                v-else
                :href="card.action_target"
                target="_blank"
                rel="noopener"
                class="disc-mini-card"
              >
                <img v-if="card.image" :src="getImageUrl(card.image)" :alt="card.title" />
                <div class="disc-mini-card__body">
                  <span class="disc-mini-card__title">{{ card.title }}</span>
                  <span v-if="card.subtitle" class="disc-mini-card__subtitle">{{ card.subtitle }}</span>
                </div>
              </a>
            </template>
          </div>

          <!-- business_recommend: product_ids → 商品列表 -->
          <div v-else-if="block.type === 'business_recommend'" class="disc-card-grid">
            <HomeServiceCard v-for="product in productsForBlock(block)" :key="product.id" :product="product" />
          </div>

          <!-- announcement: text + link_type/link_value -->
          <div v-else-if="block.type === 'announcement'" class="disc-notice-box">
            <a
              v-if="resolveBlockLink(block.config?.link_type, block.config?.link_value).external"
              :href="String(block.config?.link_value)"
              target="_blank"
              rel="noopener"
              class="disc-notice-item"
            >
              <span class="disc-notice-item__title">{{ block.config?.text }}</span>
              <ArrowUpRight :size="16" aria-hidden="true" />
            </a>
            <RouterLink
              v-else-if="resolveBlockLink(block.config?.link_type, block.config?.link_value).href"
              :to="resolveBlockLink(block.config?.link_type, block.config?.link_value).href"
              class="disc-notice-item"
            >
              <span class="disc-notice-item__title">{{ block.config?.text }}</span>
              <ArrowUpRight :size="16" aria-hidden="true" />
            </RouterLink>
            <div v-else class="disc-notice-item">
              <span class="disc-notice-item__title">{{ block.config?.text }}</span>
            </div>
          </div>

          <!-- external_link: label + url + icon -->
          <div v-else-if="block.type === 'external_link'" class="disc-card-grid">
            <a :href="String(block.config?.url)" target="_blank" rel="noopener" class="disc-ext-card">
              <span class="disc-ext-card__icon">
                <component :is="resolveHomeEntryIcon(block.config?.icon)" :size="20" aria-hidden="true" />
              </span>
              <span class="disc-ext-card__label">{{ block.config?.label }}</span>
              <ArrowUpRight :size="16" class="disc-ext-card__arrow" aria-hidden="true" />
            </a>
          </div>

          <!-- category_entry: category_ids → 分类卡片 -->
          <div v-else-if="block.type === 'category_entry'" class="disc-cat-grid">
            <RouterLink
              v-for="category in categoriesForBlock(block)"
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
        </section>
      </template>

      <!-- Fallback：未配置 discovery_blocks 时展示既有 4 个硬编码区块 -->
      <template v-else>
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
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ArrowUpRight, Flame, LayoutGrid, Megaphone, Sparkles } from 'lucide-vue-next'
import { categoryAPI, postAPI, productAPI } from '../api'
import { useLocalized } from '../composables/useProduct'
import { usePageSeo } from '../composables/usePageSeo'
import { useSiteConfig } from '../composables/useSiteConfig'
import { getServiceIcon } from '../utils/serviceIcon'
import { getImageUrl } from '../utils/image'
import { HOME_ENTRY_ROUTE_MAP, resolveHomeEntryIcon } from '../components/home/homeEntryIcons'
import type { PublicCategory } from '../utils/category'
import type { DiscoveryBlock } from '../types/siteConfig'
import HomeServiceCard from '../components/home/HomeServiceCard.vue'

const { t } = useI18n()
const { getLocalizedText } = useLocalized()
usePageSeo({ canonicalPath: () => '/discovery' })

const localizedText = (value: unknown): string => typeof value === 'string' ? value : getLocalizedText(value)
const categoryLabel = (category: PublicCategory) => localizedText(category.name) || category.slug || ''

// ── 装修驱动的发现页区块（来自 /public/config 的 discovery_blocks） ──
const { discoveryBlocks } = useSiteConfig()
const hasDynamicBlocks = computed(() => discoveryBlocks.value.length > 0)

/** internal 路由白名单 key → 站内路径；已是路径则原样返回 */
const resolveInternalPath = (key: string): string => {
  const v = String(key || '').trim()
  if (!v) return '/'
  if (v.startsWith('/')) return v
  return HOME_ENTRY_ROUTE_MAP[v] || '/'
}

interface ResolvedLink { external: boolean; href: string }
const resolveBlockLink = (linkType: string, linkValue: string): ResolvedLink => {
  const lt = String(linkType || 'none')
  const lv = String(linkValue || '').trim()
  if (lt === 'external' && lv) return { external: true, href: lv }
  if (lt === 'internal' && lv) return { external: false, href: resolveInternalPath(lv) }
  return { external: false, href: '' }
}

// business_recommend 用：拉取商品列表后按 product_ids 过滤（无 ids 过滤接口，前端过滤）
const dynamicProducts = ref<any[]>([])
const loadDynamicProducts = async () => {
  try {
    const response = await productAPI.list({ page: 1, page_size: 100 })
    dynamicProducts.value = (Array.isArray(response.data.data) ? response.data.data : []).filter((p: any) => p?.slug)
  } catch {
    dynamicProducts.value = []
  }
}
const productsForBlock = (block: DiscoveryBlock): any[] => {
  const ids: number[] = Array.isArray(block.config?.product_ids) ? block.config.product_ids : []
  const byId = new Map(dynamicProducts.value.map((p: any) => [Number(p.id), p]))
  return ids.map((id) => byId.get(Number(id))).filter((p): p is any => Boolean(p))
}

// category_entry 用：按 category_ids 过滤已加载的分类
const categoriesForBlock = (block: DiscoveryBlock): PublicCategory[] => {
  const ids: number[] = Array.isArray(block.config?.category_ids) ? block.config.category_ids : []
  const idSet = new Set(ids.map(Number))
  return categories.value.filter((c) => idSet.has(Number(c.id)))
}

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
  // 分类在两种模式下都需要（fallback 分类区 + 动态 category_entry）
  void loadCategories()
  if (hasDynamicBlocks.value) {
    void loadDynamicProducts()
  } else {
    void Promise.allSettled([loadHot(), loadBenefits(), loadNotices()])
  }
})

// config 异步到达后从 fallback 切换为动态区块时，补拉商品数据
watch(hasDynamicBlocks, (v) => {
  if (v) void loadDynamicProducts()
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

/* ===== 动态区块（discovery_blocks）：banner / 迷你卡片 / 外链卡片 ===== */
.disc-banner { display: block; overflow: hidden; border: 1px solid var(--ui-border); border-radius: var(--ui-radius-xl); background: var(--ui-bg-elevated); }
.disc-banner img { display: block; width: 100%; height: auto; object-fit: cover; }
.disc-mini-card { display: flex; flex-direction: column; overflow: hidden; border: 1px solid var(--ui-border); border-radius: var(--ui-radius-lg); background: var(--ui-bg-elevated); color: var(--ui-text-primary); transition: border-color 180ms ease, transform 180ms ease; }
.disc-mini-card:hover, .disc-mini-card:focus-visible { border-color: var(--ui-accent); transform: translateY(-2px); }
.disc-mini-card img { width: 100%; height: 120px; object-fit: cover; }
.disc-mini-card__body { display: flex; flex-direction: column; gap: 4px; padding: 12px 14px; }
.disc-mini-card__title { font-size: 14px; font-weight: 700; }
.disc-mini-card__subtitle { font-size: 12px; color: var(--ui-text-secondary); }
.disc-ext-card { display: flex; align-items: center; gap: 12px; border: 1px solid var(--ui-border); border-radius: var(--ui-radius-lg); padding: 14px; background: var(--ui-bg-elevated); color: var(--ui-text-primary); transition: border-color 180ms ease, transform 180ms ease; }
.disc-ext-card:hover, .disc-ext-card:focus-visible { border-color: var(--ui-accent); transform: translateY(-2px); }
.disc-ext-card__icon { display: grid; width: 42px; height: 42px; flex: none; place-items: center; border-radius: 13px; background: var(--ui-accent-soft); color: var(--ui-accent); }
.disc-ext-card__label { min-width: 0; flex: 1; overflow: hidden; font-size: 13px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.disc-ext-card__arrow { flex: none; color: var(--ui-text-muted); }

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
