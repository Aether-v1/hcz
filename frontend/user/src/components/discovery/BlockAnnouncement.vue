<template>
  <div v-if="text" class="db-announce">
    <span class="db-announce__badge">公告</span>
    <span class="db-announce__text">{{ text }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSiteConfig } from '../../composables/useSiteConfig'

defineProps<{ config?: Record<string, any> }>()
const { announcement } = useSiteConfig()

const text = computed(() => {
  const a = announcement.value
  if (!a) return ''
  return typeof a.title === 'string' ? a.title : ''
})
</script>

<style scoped>
.db-announce {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.7rem 0.9rem;
  border-radius: var(--ui-radius-md);
  background: var(--brand-primary-light);
  border: 1px solid color-mix(in oklab, var(--brand-primary) 22%, transparent);
  font-size: 0.82rem;
}
.db-announce__badge {
  flex: none;
  font-size: 0.68rem;
  font-weight: 700;
  color: #fff;
  background: var(--brand-primary);
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
}
.db-announce__text {
  color: var(--ui-text-primary);
  min-width: 0;
}
</style>
