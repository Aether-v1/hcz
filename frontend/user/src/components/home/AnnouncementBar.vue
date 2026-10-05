<template>
  <div v-if="announcementTitle" class="home-announce" role="status">
    <span class="home-announce__dot" aria-hidden="true" />
    <button
      v-if="announcement?.link"
      type="button"
      class="home-announce__text"
      @click="onClick"
    >
      <span class="home-announce__label">公告</span>
      {{ announcementTitle }}
    </button>
    <span v-else class="home-announce__text">
      <span class="home-announce__label">公告</span>
      {{ announcementTitle }}
    </span>
    <button type="button" class="home-announce__close" :aria-label="'close'" @click="dismiss">×</button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSiteConfig } from '../../composables/useSiteConfig'

const router = useRouter()
const { announcement } = useSiteConfig()
const dismissed = ref(false)

const announcementTitle = computed(() => {
  if (dismissed.value) return ''
  const a = announcement.value
  if (!a) return ''
  return typeof a.title === 'string' ? a.title : ''
})

const onClick = () => {
  const link = String(announcement.value?.link || '').trim()
  if (!link) return
  if (/^https?:\/\//i.test(link)) {
    window.open(link, '_blank', 'noopener,noreferrer')
    return
  }
  if (link.startsWith('/')) void router.push(link)
}

const dismiss = () => {
  dismissed.value = true
}
</script>

<style scoped>
.home-announce {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 1.25rem;
  padding: 0.55rem 0.9rem;
  border-radius: var(--ui-radius-md);
  background: var(--brand-primary-light);
  border: 1px solid color-mix(in oklab, var(--brand-primary) 22%, transparent);
  font-size: 0.82rem;
}
.home-announce__dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 999px;
  background: var(--brand-primary);
  flex: none;
}
.home-announce__text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ui-text-primary);
  text-align: left;
  background: none;
  border: none;
  padding: 0;
  font: inherit;
  cursor: pointer;
}
button.home-announce__text {
  cursor: pointer;
}
.home-announce__label {
  font-weight: 700;
  color: var(--brand-primary);
  margin-right: 0.4rem;
}
.home-announce__close {
  flex: none;
  width: 1.4rem;
  height: 1.4rem;
  line-height: 1;
  border-radius: 999px;
  border: none;
  background: transparent;
  color: var(--ui-text-muted);
  font-size: 1.1rem;
  cursor: pointer;
}
.home-announce__close:hover {
  background: var(--ui-bg-muted);
  color: var(--ui-text-primary);
}
</style>
