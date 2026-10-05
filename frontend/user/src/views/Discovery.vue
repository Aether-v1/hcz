<template>
  <div class="hcz-shell-container discovery-page">
    <section class="discovery-page__card" aria-labelledby="discovery-title">
      <span class="discovery-page__icon"><Compass :size="30" :stroke-width="1.7" aria-hidden="true" /></span>
      <h1 id="discovery-title">{{ t('coreNav.discover') }}</h1>
      <strong>{{ t('discover.comingSoon') }}</strong>
      <p>{{ t('discover.description') }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Compass } from 'lucide-vue-next'
import { usePageSeo } from '../composables/usePageSeo'

const { t } = useI18n()
usePageSeo({ canonicalPath: () => '/discovery' })
</script>

<style scoped>
.discovery-page { display: grid; min-height: min(70vh, 640px); place-items: center; padding-top: 2rem; padding-bottom: 4rem; }
.discovery-page__card { width: min(100%, 580px); border: 1px solid var(--ui-border); border-radius: var(--ui-radius-xl); padding: clamp(32px, 6vw, 68px) 24px; background: var(--ui-bg-elevated); box-shadow: var(--ui-shadow-soft); text-align: center; }
.discovery-page__icon { display: grid; width: 68px; height: 68px; place-items: center; margin: 0 auto 22px; border-radius: 21px; background: var(--ui-accent-soft); color: var(--ui-accent); }
.discovery-page h1 { font-size: clamp(25px, 3vw, 32px); font-weight: 800; }
.discovery-page strong { display: block; margin-top: 12px; color: var(--ui-accent); font-size: 16px; }
.discovery-page p { margin-top: 9px; color: var(--ui-text-muted); font-size: 14px; line-height: 1.6; }
</style>
