<template>
  <a
    v-if="url"
    :href="url"
    target="_blank"
    rel="noopener noreferrer"
    class="db-ext"
  >
    <img v-if="image" :src="image" :alt="title" loading="lazy" class="db-ext__img" />
    <div class="db-ext__body">
      <h4 v-if="title">{{ title }}</h4>
      <p v-if="subtitle">{{ subtitle }}</p>
    </div>
    <span class="db-ext__arrow">↗</span>
  </a>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getImageUrl } from '../../utils/image'
import { pickText } from '../../utils/siteConfig'

const props = defineProps<{ config?: Record<string, any> }>()
const { locale } = useI18n()

const pick = (v: unknown): string => pickText(v, locale.value)

const cfg = computed(() => props.config || {})
const title = computed(() => pick(cfg.value.title))
const subtitle = computed(() => pick(cfg.value.subtitle))
const image = computed(() => getImageUrl(cfg.value.image))
const url = computed(() => {
  const u = String(cfg.value.url || '').trim()
  return /^https?:\/\//i.test(u) ? u : ''
})
</script>

<style scoped>
.db-ext {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 0.75rem;
  border-radius: var(--ui-radius-lg);
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-elevated);
  text-decoration: none;
  color: inherit;
}
.db-ext__img {
  width: 3.2rem;
  height: 3.2rem;
  border-radius: var(--ui-radius-md);
  object-fit: cover;
  flex: none;
}
.db-ext__body {
  flex: 1;
  min-width: 0;
}
.db-ext__body h4 {
  font-size: 0.9rem;
  font-weight: 700;
}
.db-ext__body p {
  font-size: 0.76rem;
  color: var(--ui-text-muted);
}
.db-ext__arrow {
  color: var(--brand-primary);
  font-size: 1.1rem;
}
</style>
