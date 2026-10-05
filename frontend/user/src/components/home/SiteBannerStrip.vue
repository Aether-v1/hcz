<template>
  <section v-if="current" class="site-banner" aria-label="banner">
    <component
      :is="href ? 'a' : 'div'"
      :href="href || undefined"
      :target="href && openInNew ? '_blank' : undefined"
      :rel="href ? 'noopener noreferrer' : undefined"
      class="site-banner__inner"
    >
      <img
        v-if="image && !failed"
        :src="image"
        :alt="title"
        loading="lazy"
        class="site-banner__img"
        @error="failed = true"
      />
      <div class="site-banner__text">
        <h3 v-if="title">{{ title }}</h3>
        <p v-if="subtitle">{{ subtitle }}</p>
      </div>
    </component>
    <div v-if="items.length > 1" class="site-banner__dots">
      <button
        v-for="(_, index) in items"
        :key="index"
        type="button"
        class="site-banner__dot"
        :class="{ 'is-active': index === currentIndex }"
        :aria-label="`banner ${index + 1}`"
        @click="go(index)"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSiteConfig } from '../../composables/useSiteConfig'
import { getImageUrl } from '../../utils/image'
import type { BannerItem } from '../../types/siteConfig'

const { locale } = useI18n()
const { banners } = useSiteConfig()
const items = computed<BannerItem[]>(() => banners.value)

const currentIndex = ref(0)
const failed = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const current = computed(() => items.value[currentIndex.value] || items.value[0] || null)

const title = computed(() => {
  const t = current.value?.title
  if (typeof t === 'string') return t
  if (t && typeof t === 'object') return String(t[locale.value] || t['zh-CN'] || '')
  return ''
})
const subtitle = computed(() => {
  const s = current.value?.subtitle
  if (typeof s === 'string') return s
  if (s && typeof s === 'object') return String(s[locale.value] || s['zh-CN'] || '')
  return ''
})
const image = computed(() => {
  if (typeof window !== 'undefined' && window.innerWidth < 768 && current.value?.mobile_image) {
    return getImageUrl(current.value.mobile_image)
  }
  return getImageUrl(current.value?.image || current.value?.mobile_image || '')
})
const href = computed(() => {
  const b = current.value
  if (!b || String(b.link_type || '').toLowerCase() === 'none') return ''
  const raw = String(b.link_value || '').trim()
  if (!raw) return ''
  const type = String(b.link_type || '').toLowerCase()
  if (type === 'internal' && raw.startsWith('/') && !raw.startsWith('//')) return raw
  if (type === 'external' || type === 'url') return /^https?:\/\//i.test(raw) ? raw : ''
  // 未知类型：http(s) 当外链，/ 当内链
  if (/^https?:\/\//i.test(raw)) return raw
  if (raw.startsWith('/') && !raw.startsWith('//')) return raw
  return ''
})
const openInNew = computed(() => Boolean(current.value?.open_in_new_tab))

const go = (index: number) => {
  if (items.value.length === 0) return
  currentIndex.value = ((index % items.value.length) + items.value.length) % items.value.length
}

watch(items, () => {
  currentIndex.value = 0
  failed.value = false
})

onMounted(() => {
  if (items.value.length > 1) {
    timer = setInterval(() => go(currentIndex.value + 1), 5000)
  }
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.site-banner {
  margin-bottom: var(--ui-section-gap, 2rem);
}
.site-banner__inner {
  position: relative;
  display: block;
  border-radius: var(--ui-radius-lg);
  overflow: hidden;
  background: var(--ui-bg-elevated);
  border: 1px solid var(--ui-border);
  text-decoration: none;
  color: var(--ui-text-primary);
  min-height: 7rem;
}
.site-banner__img {
  width: 100%;
  height: auto;
  display: block;
  object-fit: cover;
}
.site-banner__text {
  position: absolute;
  left: 1rem;
  bottom: 0.9rem;
  max-width: 70%;
}
.site-banner__text h3 {
  font-size: 1rem;
  font-weight: 700;
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.35);
  color: #fff;
}
.site-banner__text p {
  font-size: 0.78rem;
  color: rgba(255, 255, 255, 0.9);
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.35);
}
.site-banner__dots {
  display: flex;
  justify-content: center;
  gap: 0.4rem;
  margin-top: 0.5rem;
}
.site-banner__dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 999px;
  border: none;
  background: var(--ui-border-strong);
  cursor: pointer;
}
.site-banner__dot.is-active {
  background: var(--brand-primary);
}
</style>
