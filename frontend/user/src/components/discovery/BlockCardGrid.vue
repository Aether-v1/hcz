<template>
  <div v-if="cards.length" class="db-cards">
    <component
      v-for="(card, index) in cards"
      :key="index"
      :is="card.action.href ? (card.action.external ? 'a' : 'RouterLink') : 'div'"
      v-bind="card.action.href ? card.linkProps : {}"
      class="db-card"
    >
      <img v-if="card.image" :src="card.image" :alt="card.title" loading="lazy" class="db-card__img" />
      <div class="db-card__body">
        <h4 v-if="card.title">{{ card.title }}</h4>
        <p v-if="card.description">{{ card.description }}</p>
      </div>
    </component>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getImageUrl } from '../../utils/image'
import { pickText } from '../../utils/siteConfig'
import { resolveBlockAction } from './shared'

const props = defineProps<{ config?: Record<string, any> }>()
const { locale } = useI18n()

const pick = (v: unknown): string => pickText(v, locale.value)

const cards = computed(() => {
  const raw = props.config?.cards
  if (!Array.isArray(raw)) return []
  return raw
    .map((c: any) => {
      const action = resolveBlockAction(c?.action_type, c?.action_target)
      return {
        title: pick(c?.title),
        description: pick(c?.description),
        image: getImageUrl(c?.image),
        action,
        linkProps: action.external
          ? { href: action.href, target: '_blank', rel: 'noopener noreferrer' }
          : { to: action.href },
      }
    })
    .filter((c) => c.title || c.image)
})
</script>

<style scoped>
.db-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}
@media (min-width: 768px) {
  .db-cards {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
.db-card {
  display: block;
  border-radius: var(--ui-radius-lg);
  overflow: hidden;
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-elevated);
  text-decoration: none;
  color: inherit;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}
.db-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--ui-shadow-card);
}
.db-card__img {
  width: 100%;
  height: 8rem;
  object-fit: cover;
  display: block;
}
.db-card__body {
  padding: 0.75rem 0.9rem;
}
.db-card__body h4 {
  font-size: 0.9rem;
  font-weight: 700;
  color: var(--ui-text-primary);
}
.db-card__body p {
  font-size: 0.76rem;
  color: var(--ui-text-muted);
  margin-top: 0.25rem;
}
</style>
