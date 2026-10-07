<template>
  <div class="pb-8 text-foreground">
    <!-- Page Header -->
    <div class="mb-4">
      <h1 class="text-xl font-bold tracking-tight md:text-2xl">{{ t('nav.notice') }}</h1>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('notice.subtitle') }}</p>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-2xl border bg-muted"></div>
    </div>

    <!-- Notices List -->
    <div v-else-if="notices.length > 0" class="space-y-3">
      <article
        v-for="notice in notices"
        :key="notice.id"
        class="flex cursor-pointer items-center gap-3 rounded-2xl border bg-card p-4 shadow-sm transition-colors hover:bg-accent/40"
        @click="goToNotice(notice.slug)"
      >
        <!-- Icon Column -->
        <div class="grid h-12 w-12 shrink-0 place-items-center overflow-hidden rounded-xl bg-accent text-muted-foreground">
          <img v-if="notice.thumbnail" :src="getImageUrl(notice.thumbnail)" :alt="getLocalizedText(notice.title)" loading="lazy" class="h-full w-full object-cover">
          <Bell v-else :size="20" :stroke-width="1.8" />
        </div>

        <!-- Content -->
        <div class="min-w-0 flex-1">
          <div class="mb-1 flex items-center gap-2">
            <time class="text-xs text-muted-foreground">{{ formatDate(notice.published_at) }}</time>
          </div>
          <h2 class="truncate text-sm font-semibold text-foreground">
            {{ getLocalizedText(notice.title) }}
          </h2>
          <p class="mt-0.5 truncate text-xs text-muted-foreground">
            {{ getLocalizedText(notice.summary) }}
          </p>
        </div>

        <!-- Arrow -->
        <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
      </article>

      <!-- Pagination -->
      <PaginationNav
        :current-page="currentPage"
        :total-pages="totalPages"
        @change-page="changePage"
      />
    </div>

    <!-- Empty State -->
    <EmptyState v-else variant="soft" size="lg" :title="t('notice.empty')">
      <template #icon>
        <Bell class="h-16 w-16 text-muted-foreground opacity-50" :stroke-width="1.5" />
      </template>
    </EmptyState>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronRight } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import EmptyState from '../components/EmptyState.vue'
import PaginationNav from '../components/PaginationNav.vue'
import { usePostList } from '../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts: notices, currentPage, totalPages,
  getLocalizedText, formatDate, goToPost: goToNotice, changePage,
} = usePostList('notice', { title: () => t('nav.notice'), canonicalPath: '/notice' })
</script>

