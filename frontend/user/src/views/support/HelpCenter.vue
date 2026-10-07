<template>
  <div class="pb-8 text-foreground">
    <!-- 标题 -->
    <div class="mb-4">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('support.help_center') }}</h1>
    </div>

    <!-- 资讯入口 -->
    <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
      <router-link
        to="/discovery"
        class="flex min-h-[52px] items-center gap-3 px-5 py-3 transition-colors hover:bg-accent/40"
      >
        <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
          <Newspaper :size="18" :stroke-width="1.8" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-foreground">{{ t('support.news_info') }}</p>
          <p class="text-xs text-muted-foreground">{{ t('support.news_info_hint') }}</p>
        </div>
        <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight, Newspaper } from 'lucide-vue-next'

const { t } = useI18n()
</script>
