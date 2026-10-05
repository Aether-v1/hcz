<template>
  <section class="home-entry" aria-labelledby="home-entry-title">
    <div class="home-entry__head">
      <h2 id="home-entry-title">{{ heading }}</h2>
    </div>
    <div class="home-entry__grid">
      <template v-for="entry in entries" :key="entry.id">
        <!-- 外部链接 -->
        <a
          v-if="entry.external"
          :href="entry.href"
          target="_blank"
          rel="noopener noreferrer"
          class="home-entry__card"
          :class="{ 'home-entry__card--recommended': entry.recommended }"
        >
          <span class="home-entry__icon">
            <component :is="iconFor(entry.icon)" :size="24" :stroke-width="1.9" aria-hidden="true" />
          </span>
          <strong>{{ entry.title }}</strong>
          <span v-if="entry.subtitle" class="home-entry__desc">{{ entry.subtitle }}</span>
          <span v-if="entry.badge" class="home-entry__badge">{{ entry.badge }}</span>
        </a>
        <!-- 内部路由 -->
        <RouterLink
          v-else
          :to="entry.href"
          class="home-entry__card"
          :class="{ 'home-entry__card--recommended': entry.recommended }"
        >
          <span class="home-entry__icon">
            <component :is="iconFor(entry.icon)" :size="24" :stroke-width="1.9" aria-hidden="true" />
          </span>
          <strong>{{ entry.title }}</strong>
          <span v-if="entry.subtitle" class="home-entry__desc">{{ entry.subtitle }}</span>
          <span v-if="entry.badge" class="home-entry__badge">{{ entry.badge }}</span>
        </RouterLink>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSiteConfig } from '../../composables/useSiteConfig'
import { resolveHomeEntryIcon } from './homeEntryIcons'

defineProps<{ heading?: string }>()

const { homeEntries } = useSiteConfig()
const entries = computed(() => homeEntries.value)
const iconFor = (key: string) => resolveHomeEntryIcon(key)
</script>

<style scoped>
.home-entry {
  margin-bottom: var(--ui-section-gap, 2rem);
}
.home-entry__head {
  margin-bottom: 0.9rem;
}
.home-entry__head h2 {
  font-size: 1.1rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--ui-text-primary);
}
.home-entry__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}
@media (min-width: 640px) {
  .home-entry__grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}
.home-entry__card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 1rem;
  border-radius: var(--ui-radius-lg);
  border: 1px solid var(--ui-border);
  background: var(--ui-bg-elevated);
  box-shadow: var(--ui-shadow-soft);
  text-decoration: none;
  color: var(--ui-text-primary);
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}
.home-entry__card:hover {
  transform: translateY(-2px);
  border-color: color-mix(in oklab, var(--brand-primary) 45%, transparent);
  box-shadow: var(--ui-shadow-card);
}
.home-entry__icon {
  display: grid;
  place-items: center;
  width: 2.6rem;
  height: 2.6rem;
  border-radius: var(--ui-radius-md);
  background: var(--brand-primary-light);
  color: var(--brand-primary);
}
.home-entry__card strong {
  font-size: 0.92rem;
  font-weight: 700;
}
.home-entry__desc {
  font-size: 0.75rem;
  color: var(--ui-text-muted);
  line-height: 1.3;
}
.home-entry__card--recommended {
  border-color: color-mix(in oklab, var(--brand-primary) 55%, transparent);
  background: color-mix(in oklab, var(--brand-primary-light) 60%, var(--ui-bg-elevated));
}
.home-entry__badge {
  position: absolute;
  top: 0.6rem;
  right: 0.6rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  font-size: 0.66rem;
  font-weight: 700;
  color: #fff;
  background: var(--brand-primary);
}
</style>
