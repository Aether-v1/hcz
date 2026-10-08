<template>
  <div class="pb-8 text-foreground">
    <router-link
      to="/support/tickets"
      class="inline-flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft class="h-4 w-4" />
      {{ t('support.my_tickets') }}
    </router-link>

    <!-- 加载骨架 -->
    <div v-if="loading" class="mt-4 space-y-2">
      <div v-for="i in 4" :key="i" class="h-16 animate-pulse rounded-2xl border bg-card" />
    </div>

    <!-- 加载失败 -->
    <EmptyState
      v-else-if="!ticket"
      icon="alert"
      size="md"
      class="mt-4"
      :title="t('support.load_failed')"
    />

    <!-- 详情卡片：头部 + 可滚动会话 + 回复框 -->
    <div
      v-else
      class="mt-4 flex h-[calc(100vh-176px)] min-h-[520px] flex-col overflow-hidden rounded-2xl border bg-card shadow-sm"
    >
      <!-- 头部 -->
      <div class="border-b px-5 py-4">
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="font-mono text-base font-semibold">{{ ticket.ticket_no }}</h1>
              <StatusBadge kind="status" :value="ticket.status" />
              <StatusBadge kind="priority" :value="ticket.priority" />
            </div>
            <div class="mt-1 truncate text-xs text-muted-foreground">
              {{ ticket.subject }} · {{ t('support.created_at') }}
              {{ formatDateTime(ticket.created_at, locale) }}
            </div>
          </div>
          <div class="flex flex-none items-center gap-2">
            <!-- resolved：允许时可重新打开 -->
            <Button
              v-if="ticket.status === 'resolved'"
              size="sm"
              variant="outline"
              :disabled="acting || reopenExpired"
              :title="reopenExpired ? t('support.reopen_expired') : undefined"
              @click="handleReopen"
            >
              <RotateCcw class="h-4 w-4" />
              {{ t('support.reopen') }}
            </Button>
            <!-- 未关闭：可主动关闭 -->
            <Button
              v-if="ticket.status !== 'closed'"
              size="sm"
              variant="outline"
              :disabled="acting"
              @click="handleClose"
            >
              {{ t('support.close') }}
            </Button>
          </div>
        </div>
        <BizLinkCard
          v-if="ticket.biz_type && ticket.biz_id"
          :biz-type="ticket.biz_type"
          :biz-id="ticket.biz_id"
        />
        <p v-if="ticket.status === 'resolved' && reopenExpired" class="mt-2 text-[11px] text-muted-foreground">
          {{ t('support.reopen_expired') }}
        </p>
      </div>

      <!-- 会话时间线 -->
      <TicketConversation :messages="messages" :attachments="attachments" />

      <!-- 关闭后提示 -->
      <div v-if="ticket.status === 'closed'" class="border-t bg-muted/40 px-4 py-2 text-center text-xs text-muted-foreground">
        {{ t('support.ticket_closed_notice') }}
      </div>
      <!-- resolved 状态下提示先 reopen 才能回复 -->
      <div
        v-else-if="ticket.status === 'resolved'"
        class="border-t bg-muted/30 px-4 py-2 text-center text-xs text-muted-foreground"
      >
        <Button size="xs" variant="ghost" :disabled="acting || reopenExpired" :title="reopenExpired ? t('support.reopen_expired') : undefined" @click="handleReopen">
          <RotateCcw class="h-3 w-3" />
          {{ t('support.reopen_to_reply') }}
        </Button>
      </div>

      <!-- 回复框 -->
      <TicketReplyComposer
        :disabled="ticket.status === 'closed'"
        :sending="sending"
        @send="handleSend"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ArrowLeft, RotateCcw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import TicketConversation from '../../components/support/TicketConversation.vue'
import TicketReplyComposer from '../../components/support/TicketReplyComposer.vue'
import StatusBadge from '../../components/support/StatusBadge.vue'
import BizLinkCard from '../../components/support/BizLinkCard.vue'
import { useSupportTicket } from '../../composables/useSupportTicket'
import { formatDateTime } from '../../utils/datetime'
import { useAppStore } from '../../stores/app'
import { toast } from '../../composables/useToast'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const locale = appStore.locale

const {
  ticket,
  messages,
  attachments,
  loading,
  sending,
  acting,
  reopenExpired,
  fetchDetail,
  sendReply,
  closeTicket,
  reopenTicket,
} = useSupportTicket(() => Number(route.params.id))

onMounted(() => {
  void fetchDetail().catch(() => undefined)
})

const handleSend = async (bodyText: string, attachmentIds: number[]) => {
  try {
    await sendReply(bodyText, attachmentIds)
    toast.success(t('support.send_success'))
  } catch (err: any) {
    toast.error(err?.message || t('support.send_failed'))
  }
}

const handleClose = async () => {
  try {
    await closeTicket()
    toast.success(t('support.closed_success'))
  } catch (err: any) {
    toast.error(err?.message || t('support.close_failed'))
  }
}

const handleReopen = async () => {
  try {
    await reopenTicket()
    toast.success(t('support.reopened_success'))
  } catch (err: any) {
    toast.error(err?.message || t('support.reopen_failed'))
  }
}
</script>
