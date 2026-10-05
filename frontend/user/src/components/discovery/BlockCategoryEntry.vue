<template>
  <div v-if="entries.length" class="db-cat">
    <RouterLink
      v-for="(entry, index) in entries"
      :key="index"
      :to="entry.route"
      class="db-cat__item"
    >
      <span class="db-cat__icon">
        <component :is="iconFor(entry.icon)" :size="20" aria-hidden="true" />
      </span>
      <span>{{ entry.name }}</span>
    </RouterLink>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { pickText } from '../../utils/siteConfig'
import { resolveHomeEntryIcon } from '../home/homeEntryIcons'

const props = defineProps<{ config?: Record<string, any> }>()
const { locale } = useI18n()

const pick = (v: unknown): string => pickText(v, locale.value)

const entries = computed(() => {
  const raw = props.config?.entries
  if (!Array.isArray(raw)) return []
  return raw
    .map((e: any) => ({
      name: pick(e?.name),
      route: String(e?.route || '').startsWith('/') ? String(e.route) : '',
      icon: String(e?.icon || ''),
    }))
    .filter((e) => e.name && e.route)
})

const iconFor = (key: string) => resolveHomeEntryIcon(key)
</script>

<style scoped>
.db-cat {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}
@media (min-width: 768px) {
  .db-cat {
    grid-template-columns: repeat(6, minmax(0, 1fr));
  }
}
.db-cat__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.45rem;
  padding: 0.9rem 0.5rem;
  border-radius: var(--ui-radius-md);
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-elevated);
  text-decoration: none;
  color: var(--ui-text-primary);
  font-size: 0.76rem;
  font-weight: 600;
  text-align: center;
}
.db-cat__icon {
  display: grid;
  place-items: center;
  width: 2.4rem;
  height: 2.4rem;
  border-radius: 999px;
  background: var(--brand-primary-light);
  color: var(--brand-primary);
}
</style>
