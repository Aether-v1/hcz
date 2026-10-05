<template>
  <div class="divide-y divide-border">
    <button
      v-for="ticket in items"
      :key="ticket.id"
      type="button"
      class="w-full px-4 py-3 text-left transition-colors"
      :class="selectedId === ticket.id ? 'bg-primary/5' : 'hover:bg-muted/50'"
      @click="emit('select', ticket)"
    >
      <div class="flex items-start justify-between gap-2">
        <div class="flex min-w-0 items-center gap-2">
          <span v-if="ticket.user_unread_count > 0" class="mt-1 h-2 w-2 flex-none rounded-full bg-primary" />
          <h3 class="truncate text-sm" :class="ticket.user_unread_count > 0 ? 'font-semibold text-foreground' : 'font-medium text-foreground/90'">
            {{ ticket.subject }}
          </h3>
        </div>
        <StatusBadge kind="status" :value="ticket.status" class="flex-none" />
      </div>
      <div class="mt-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span class="truncate font-mono">{{ ticket.ticket_no }} · {{ ticket.category_name }}</span>
        <span class="flex-none">{{ formatShortTime(ticket.last_replied_at || ticket.created_at, locale) }}</span>
      </div>
      <div class="mt-1 flex items-center gap-2 text-[11px] text-muted-foreground/80">
        <StatusBadge kind="priority" :value="ticket.priority" />
        <span v-if="ticket.user_unread_count > 0" class="text-primary">{{ t('support.unread') }} {{ ticket.user_unread_count }}</span>
      </div>
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import StatusBadge from './StatusBadge.vue'
import type { SupportTicketSummary } from '../../types/support'
import { formatShortTime } from '../../utils/datetime'
import { useAppStore } from '../../stores/app'

/**
 * 工单列表侧栏（PC 主从布局的左侧 / 移动端整列）。
 * 纯展示组件，数据与过滤逻辑在 useTicketList 中。
 */
defineProps<{
  items: SupportTicketSummary[]
  selectedId: number | null
}>()

const emit = defineEmits<{
  select: [ticket: SupportTicketSummary]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const locale = appStore.locale
</script>
