<template>
  <div class="pb-8 text-foreground">
    <div class="mx-auto max-w-3xl rounded-2xl border bg-card p-8 text-center shadow-sm md:p-12">
      <Badge variant="neutral" class="rounded-full gap-1.5 px-3 py-1 text-xs">
        <Info class="h-3.5 w-3.5 opacity-60" />
        {{ brandSiteName }}
      </Badge>
      <div class="my-4 flex justify-center">
        <Frown class="h-20 w-20 text-muted-foreground opacity-30 md:h-24 md:w-24" :stroke-width="1" />
      </div>
      <p class="text-6xl font-black tracking-tight text-foreground md:text-7xl">404</p>
      <h1 class="mt-3 text-xl font-extrabold text-foreground md:text-2xl">
        {{ t('notFoundPage.title') }}
      </h1>
      <p class="mt-3 text-sm leading-relaxed text-muted-foreground md:text-base">
        {{ t('notFoundPage.description', { site: brandSiteName }) }}
      </p>

      <div class="mt-6 rounded-2xl border bg-muted/30 p-4 text-left">
        <p class="mb-3 text-[11px] font-semibold uppercase tracking-widest text-muted-foreground">
          {{ t('notFoundPage.quickLinksTitle') }}
        </p>
        <div class="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" as-child>
            <RouterLink to="/products">
              <ShoppingBag class="opacity-70" />
              {{ t('nav.products') }}
            </RouterLink>
          </Button>
          <Button variant="secondary" size="sm" as-child>
            <RouterLink to="/notice">
              <Bell class="opacity-70" />
              {{ t('nav.notice') }}
            </RouterLink>
          </Button>
        </div>
      </div>

      <div class="mt-6 flex flex-col items-center justify-center gap-3 sm:flex-row">
        <Button as-child class="h-11 min-w-[160px]">
          <RouterLink to="/">
            <Home />
            {{ t('notFoundPage.backHome') }}
          </RouterLink>
        </Button>
        <Button type="button" variant="secondary" class="h-11 min-w-[160px]" @click="goBack">
          <ArrowLeft />
          {{ t('notFoundPage.backPrevious') }}
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Bell, Frown, Home, Info, ShoppingBag } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useNotFound } from '../composables/useNotFound'

const { t } = useI18n()

const { brandSiteName, goBack } = useNotFound()
</script>

