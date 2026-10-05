<template>
  <RouterLink to="/c2c" class="flex min-h-20 items-center gap-4 rounded-xl border bg-card px-4 py-3 text-foreground transition-colors hover:border-primary/40 hover:text-primary">
    <span class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-emerald-500/10 text-emerald-600"><ArrowLeftRight :size="22" aria-hidden="true" /></span>
    <span class="min-w-0 flex-1">
      <strong class="block text-sm">{{ t('homeV2.usdtTitle') }}</strong>
      <span class="block text-xs text-muted-foreground">{{ t('homeV2.usdtDescription') }}</span>
    </span>
    <ChevronRight :size="18" class="shrink-0 text-muted-foreground" aria-hidden="true" />
  </RouterLink>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeftRight, ChevronRight } from 'lucide-vue-next'

const { t } = useI18n()
</script>
