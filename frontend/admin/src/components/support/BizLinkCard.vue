<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type { SupportTicketDetail } from '@/types/support'

const props = defineProps<{
  ticket: SupportTicketDetail
}>()

const { t } = useI18n()

const link = computed(() => {
  if (!props.ticket.biz_type || !props.ticket.biz_id) return ''
  const id = props.ticket.biz_id
  switch (props.ticket.biz_type) {
    case 'order': return `/orders/${id}`
    case 'withdrawal': return `/wallet/withdrawals/${id}`
    case 'c2c': return `/c2c/trades/${id}`
    case 'recharge': return `/wallet/recharges/${id}`
    default: return ''
  }
})

const typeLabel = computed(() => {
  switch (props.ticket.biz_type) {
    case 'order': return t('admin.support.related_order')
    case 'withdrawal': return t('admin.support.biz_withdrawal')
    case 'c2c': return t('admin.support.biz_c2c')
    case 'recharge': return t('admin.support.biz_recharge')
    default: return props.ticket.biz_type || ''
  }
})
</script>

<template>
  <div v-if="ticket.biz_type && ticket.biz_id" class="rounded-xl border border-border bg-card p-4">
    <h3 class="mb-2 text-sm font-semibold">{{ t('admin.support.biz_link') }}</h3>
    <div class="flex items-center justify-between text-sm">
      <span class="text-muted-foreground">{{ typeLabel }} #{{ ticket.biz_id }}</span>
      <RouterLink v-if="link" :to="link" class="text-primary hover:underline text-xs">
        {{ t('admin.support.view_detail') }} →
      </RouterLink>
    </div>
  </div>
</template>
