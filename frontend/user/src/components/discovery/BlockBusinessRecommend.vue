<template>
  <div v-if="matched.length" class="db-recommend">
    <template v-for="entry in matched" :key="entry.id">
      <a
        v-if="entry.external"
        :href="entry.href"
        target="_blank"
        rel="noopener noreferrer"
        class="db-recommend__item"
      >
        <span class="db-recommend__icon">
          <component :is="iconFor(entry.icon)" :size="20" aria-hidden="true" />
        </span>
        <span class="db-recommend__text">
          <strong>{{ entry.title }}</strong>
          <small v-if="entry.subtitle">{{ entry.subtitle }}</small>
        </span>
      </a>
      <RouterLink v-else :to="entry.href" class="db-recommend__item">
        <span class="db-recommend__icon">
          <component :is="iconFor(entry.icon)" :size="20" aria-hidden="true" />
        </span>
        <span class="db-recommend__text">
          <strong>{{ entry.title }}</strong>
          <small v-if="entry.subtitle">{{ entry.subtitle }}</small>
        </span>
      </RouterLink>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSiteConfig } from '../../composables/useSiteConfig'
import { resolveHomeEntryIcon } from '../home/homeEntryIcons'

const props = defineProps<{ config?: Record<string, any> }>()
const { homeEntries } = useSiteConfig()

const keys = computed<string[]>(() => {
  const raw = props.config?.business_keys
  return Array.isArray(raw) ? raw.map((k) => String(k)) : []
})

const matched = computed(() => {
  if (!keys.value.length) return []
  return keys.value
    .map((key) => homeEntries.value.find((e) => e.key === key))
    .filter((e): e is NonNullable<typeof e> => !!e)
})

const iconFor = (key: string) => resolveHomeEntryIcon(key)
</script>

<style scoped>
.db-recommend {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
@media (min-width: 768px) {
  .db-recommend {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
.db-recommend__item {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.85rem;
  border-radius: var(--ui-radius-md);
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-elevated);
  text-decoration: none;
  color: inherit;
}
.db-recommend__icon {
  display: grid;
  place-items: center;
  width: 2.2rem;
  height: 2.2rem;
  border-radius: var(--ui-radius-sm);
  background: var(--brand-primary-light);
  color: var(--brand-primary);
  flex: none;
}
.db-recommend__text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.db-recommend__text strong {
  font-size: 0.86rem;
}
.db-recommend__text small {
  font-size: 0.72rem;
  color: var(--ui-text-muted);
}
</style>
