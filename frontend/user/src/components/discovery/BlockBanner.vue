<template>
  <component
    :is="action.href ? (action.external ? 'a' : 'RouterLink') : 'div'"
    v-bind="action.href ? linkProps : {}"
    class="db-banner"
  >
    <img v-if="image && !failed" :src="image" :alt="title" loading="lazy" class="db-banner__img" @error="failed = true" />
    <div class="db-banner__overlay">
      <h3 v-if="title">{{ title }}</h3>
      <p v-if="subtitle">{{ subtitle }}</p>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getImageUrl } from '../../utils/image'
import { pickText } from '../../utils/siteConfig'
import { resolveBlockAction } from './shared'

const props = defineProps<{ config?: Record<string, any> }>()
const { locale } = useI18n()

const failed = ref(false)
const pick = (v: unknown): string => pickText(v, locale.value)

const cfg = computed(() => props.config || {})
const title = computed(() => pick(cfg.value.title))
const subtitle = computed(() => pick(cfg.value.subtitle))
const image = computed(() => getImageUrl(cfg.value.image))
const action = computed(() => resolveBlockAction(cfg.value.action_type, cfg.value.action_target))
const linkProps = computed(() => {
  if (action.value.external) {
    return { href: action.value.href, target: '_blank', rel: 'noopener noreferrer' }
  }
  return { to: action.value.href }
})
</script>

<style scoped>
.db-banner {
  display: block;
  position: relative;
  border-radius: var(--ui-radius-lg);
  overflow: hidden;
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-elevated);
  text-decoration: none;
  color: inherit;
  min-height: 9rem;
}
.db-banner__img {
  width: 100%;
  display: block;
  object-fit: cover;
}
.db-banner__overlay {
  position: absolute;
  inset: auto 0 0 0;
  padding: 1rem;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.55), transparent);
}
.db-banner__overlay h3 {
  color: #fff;
  font-size: 1.05rem;
  font-weight: 700;
}
.db-banner__overlay p {
  color: rgba(255, 255, 255, 0.9);
  font-size: 0.8rem;
}
</style>
