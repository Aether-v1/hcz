<template>
  <div class="min-h-screen bg-background pb-16 pt-20 text-foreground">
    <div class="container mx-auto max-w-7xl px-4">
      <!-- Header -->
      <div class="mt-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold tracking-tight md:text-3xl">{{ t('support.my_tickets') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('support.list_subtitle') }}</p>
        </div>
        <Button @click="goCreate">
          <CirclePlus class="h-4 w-4" />
          {{ t('support.create_ticket') }}
        </Button>
      </div>

      <div class="mt-6 grid gap-4 lg:grid-cols-[400px,minmax(0,1fr)]">
        <!-- 左：状态过滤 + 工单列表 -->
        <div class="overflow-hidden rounded-2xl border bg-card/60 shadow-sm">
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
          <TicketList v-else :items="items" :selected-id="selectedId" @select="onSelect" />

          <!-- 加载更多 -->
          <div v-if="hasMore" class="border-t p-3 text-center">
            <Button variant="outline" size="sm" :disabled="loadingMore" @click="loadMore">
              <Loader2 v-if="loadingMore" class="h-4 w-4 animate-spin" />
              {{ loadingMore ? t('support.loading') : t('support.load_more') }}
            </Button>
          </div>
        </div>

        <!-- 右：PC 主从会话面板（移动端隐藏，点击列表项跳详情页） -->
        <div class="hidden lg:block">
          <div
            v-if="!selectedId"
            class="flex min-h-[480px] items-center justify-center rounded-2xl border border-dashed text-sm text-muted-foreground"
          >
            {{ t('support.select_ticket_hint') }}
          </div>

          <div
            v-else-if="ticket"
            class="flex h-[calc(100vh-180px)] min-h-[560px] flex-col overflow-hidden rounded-2xl border bg-card shadow-sm"
          >
            <!-- 详情头部 -->
            <div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="font-mono text-sm font-medium">{{ ticket.ticket_no }}</span>
                  <StatusBadge kind="status" :value="ticket.status" />
                  <StatusBadge kind="priority" :value="ticket.priority" />
                </div>
                <div class="mt-0.5 truncate text-xs text-muted-foreground">{{ ticket.subject }}</div>
              </div>
              <Button variant="outline" size="sm" @click="goDetail">
                {{ t('support.ticket_detail') }}
              </Button>
            </div>

            <!-- 会话 -->
            <TicketConversation :messages="messages" :attachments="attachments" />

            <!-- 关闭提示 -->
            <div v-if="ticket.status === 'closed'" class="border-t bg-muted/40 px-4 py-2 text-center text-xs text-muted-foreground">
              {{ t('support.ticket_closed_notice') }}
            </div>

            <!-- 回复框 -->
            <TicketReplyComposer
              :disabled="ticket.status === 'closed'"
              :sending="sending"
              @send="handleSend"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { CirclePlus, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import TicketList from '../../components/support/TicketList.vue'
import TicketConversation from '../../components/support/TicketConversation.vue'
import TicketReplyComposer from '../../components/support/TicketReplyComposer.vue'
import StatusBadge from '../../components/support/StatusBadge.vue'
import { useTicketList, TICKET_FILTERS } from '../../composables/useTicketList'
import { useSupportTicket } from '../../composables/useSupportTicket'
import type { SupportTicketSummary } from '../../types/support'
import { toast } from '../../composables/useToast'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const {
  items,
  loading,
  loadingMore,
  filter,
  selectedId,
  hasMore,
  fetchList,
  setFilter,
  loadMore,
  selectTicket,
} = useTicketList()

// PC 主从布局右侧会话面板（复用详情逻辑与轮询）
const detail = useSupportTicket(() => selectedId.value ?? 0)
const { ticket, messages, attachments, sending } = detail

watch(selectedId, (id) => {
  if (id) void detail.fetchDetail()
})

// 直接进入 /support/tickets?id=xxx 时预选中
onMounted(() => {
  void fetchList(1)
  const preset = Number(route.query.id)
  if (preset && window.innerWidth >= 1024) {
    selectTicket(preset)
  }
})

const onSelect = (item: SupportTicketSummary) => {
  if (window.innerWidth < 1024) {
    void router.push(`/support/tickets/${item.id}`)
  } else {
    selectTicket(item.id)
  }
}

const goCreate = () => {
  void router.push('/support/tickets/new')
}

const goDetail = () => {
  if (selectedId.value) void router.push(`/support/tickets/${selectedId.value}`)
}

const handleSend = async (bodyText: string, attachmentIds: number[]) => {
  try {
    await detail.sendReply(bodyText, attachmentIds)
    toast.success(t('support.send_success'))
  } catch (err: any) {
    toast.error(err?.message || t('support.send_failed'))
  }
}
</script>
