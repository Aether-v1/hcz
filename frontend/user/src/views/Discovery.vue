<template>
  <div class="hcz-shell-container discovery-page">
    <header class="discovery-page__head">
      <h1>{{ heading }}</h1>
    </header>

    <template v-if="discoveryBlocks.length">
      <BlockRenderer v-for="block in discoveryBlocks" :key="block.id" :block="block" />
    </template>
    <div v-else class="discovery-page__empty">
      <Compass :size="28" aria-hidden="true" />
      <p>暂无内容</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Compass } from 'lucide-vue-next'
import { useSiteConfig } from '../composables/useSiteConfig'
import BlockRenderer from '../components/discovery/BlockRenderer.vue'
import { usePageSeo } from '../composables/usePageSeo'

const { t } = useI18n()
const { discoveryBlocks } = useSiteConfig()
usePageSeo({ canonicalPath: () => '/discovery' })

const heading = t('coreNav.discover') || '发现'
</script>

<style scoped>
.discovery-page {
  padding-top: 1.5rem;
  padding-bottom: 2rem;
}
.discovery-page__head h1 {
  font-size: 1.4rem;
  font-weight: 800;
  margin-bottom: 1.2rem;
}
.discovery-page__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 4rem 1rem;
  color: var(--ui-text-muted);
}
</style>
