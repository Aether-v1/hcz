<template>
  <div class="pb-8 text-foreground">
    <!-- Header -->
    <div class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 class="text-xl font-bold tracking-tight md:text-2xl">{{ t('support.my_tickets') }}</h1>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('support.list_subtitle') }}</p>
      </div>
      <Button @click="goCreate">
        <CirclePlus class="h-4 w-4" />
        {{ t('support.create_ticket') }}
      </Button>
    </div>

    <!-- 工单列表：点击工单进入独立详情页 -->
    <div class="mt-4 overflow-hidden rounded-2xl border bg-card shadow-sm">
      <!-- 状态过滤页签 -->
      <div class="flex flex-wrap gap-1 border-b p-2">
        <button
          v-for="f in TICKET_FILTERS"
          :key="f"
          type="button"
          class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
          :class="filter === f ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
          @click="setFilter(f)"
        >
          {{ t(`support.tab_${f}`) }}
        </button>
      </div>

      <!-- 加载骨架 -->
      <div v-if="loading && items.length === 0" class="space-y-2 p-4">
        <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-xl bg-muted/60" />
      </div>

      <!-- 空态 -->
      <EmptyState
        v-else-if="items.length === 0"
        icon="inbox"
        size="sm"
        class="m-4"
        :title="t('support.no_tickets')"
      />

      <!-- 列表 -->
      <TicketList v-else :items="items" :selected-id="null" @select="onSelect" />

      <!-- 加载更多 -->
      <div v-if="hasMore" class="border-t p-3 text-center">
        <Button variant="outline" size="sm" :disabled="loadingMore" @click="loadMore">
          <Loader2 v-if="loadingMore" class="h-4 w-4 animate-spin" />
          {{ loadingMore ? t('support.loading') : t('support.load_more') }}
        </Button>
      </div>
    </div>

    <!-- 相关问题 FAQ -->
    <section class="mt-4">
      <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
        <div class="border-b px-5 py-3">
          <p class="text-sm font-semibold text-foreground">{{ t('support.faq') }}</p>
        </div>
        <div class="divide-y divide-border">
          <details v-for="(q, i) in faqList" :key="i" class="group px-5 py-3">
            <summary class="flex cursor-pointer items-center justify-between text-sm font-medium text-foreground">
              {{ q.q }}
              <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50 transition-transform group-open:rotate-90" />
            </summary>
            <p class="mt-2 text-sm text-muted-foreground">{{ q.a }}</p>
          </details>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ChevronRight, CirclePlus, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import TicketList from '../../components/support/TicketList.vue'
import { useTicketList, TICKET_FILTERS } from '../../composables/useTicketList'
import type { SupportTicketSummary } from '../../types/support'

const { t } = useI18n()
const router = useRouter()

const faqList = computed(() => [
  { q: t('support.faq_q1'), a: t('support.faq_a1') },
  { q: t('support.faq_q2'), a: t('support.faq_a2') },
  { q: t('support.faq_q3'), a: t('support.faq_a3') },
])

const {
  items,
  loading,
  loadingMore,
  filter,
  hasMore,
  fetchList,
  setFilter,
  loadMore,
} = useTicketList()

onMounted(() => {
  void fetchList(1)
})

const onSelect = (item: SupportTicketSummary) => {
  void router.push(`/support/tickets/${item.id}`)
}

const goCreate = () => {
  void router.push('/support/tickets/new')
}
</script>
