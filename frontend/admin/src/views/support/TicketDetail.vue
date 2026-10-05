<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { adminAPI } from '@/api/admin'
import { useAdminTicket } from '@/composables/useAdminTicket'
import type { SupportAdminUser } from '@/types/support'
import { Button } from '@/components/ui/button'
import StatusBadge from '@/components/support/StatusBadge.vue'
import TicketConversation from '@/components/support/TicketConversation.vue'
import AdminReplyComposer from '@/components/support/AdminReplyComposer.vue'
import TicketInfoPanel from '@/components/support/TicketInfoPanel.vue'
import AuditTimeline from '@/components/support/AuditTimeline.vue'
import BizLinkCard from '@/components/support/BizLinkCard.vue'
import { formatDate } from '@/utils/format'
import { useI18n } from 'vue-i18n'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const id = computed(() => Number(route.params.id))

const { loading, detail, acting, reply, assign, changePriority, resolve, close, reopen } = useAdminTicket(id.value)

const admins = ref<SupportAdminUser[]>([])

onMounted(async () => {
  try {
    const adm = await adminAPI.listAuthzAdmins()
    admins.value = (adm.data?.data as SupportAdminUser[]) || []
  } catch { admins.value = [] }
})

const ticket = computed(() => detail.value?.ticket || null)

const onSendReply = async (payload: { body: string; attachment_ids: number[] }) => {
  await reply(payload.body, payload.attachment_ids)
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center gap-3">
      <Button variant="outline" size="sm" @click="router.push('/support/tickets')">← {{ t('admin.support.backToList') }}</Button>
      <h1 class="text-2xl font-semibold">{{ t('admin.support.ticket_detail') }}</h1>
    </div>

    <div v-if="loading" class="rounded-xl border border-border bg-card p-8 text-center text-muted-foreground">加载中…</div>

    <template v-else-if="ticket && detail">
      <!-- Header -->
      <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <span class="font-mono text-sm">{{ ticket.ticket_no }}</span>
              <StatusBadge :status="ticket.status" />
            </div>
            <div class="mt-1 text-base font-semibold">{{ ticket.subject }}</div>
          </div>
          <div class="text-xs text-muted-foreground">{{ formatDate(ticket.created_at) }}</div>
        </div>
        <div class="mt-3 grid grid-cols-2 gap-3 text-sm md:grid-cols-4">
          <div>
            <div class="text-xs text-muted-foreground">{{ t('admin.support.user') }}</div>
            <div class="mt-1">
              <RouterLink v-if="ticket.user_id" :to="`/users/${ticket.user_id}`" class="text-primary hover:underline">
                {{ detail.user?.email || ticket.user_email || `#${ticket.user_id}` }}
              </RouterLink>
              <span v-else>-</span>
            </div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">{{ t('admin.support.category') }}</div>
            <div class="mt-1">{{ detail.category?.name || '-' }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">{{ t('admin.support.priority') }}</div>
            <div class="mt-1">{{ ticket.priority }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">{{ t('admin.support.assigned_to') }}</div>
            <div class="mt-1">{{ ticket.assigned_admin_name || (ticket.assigned_admin_id ? `#${ticket.assigned_admin_id}` : t('admin.support.unassigned')) }}</div>
          </div>
        </div>
      </div>

      <!-- Two columns -->
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-5">
        <!-- Left: conversation -->
        <div class="rounded-xl border border-border bg-card p-4 shadow-sm lg:col-span-3">
          <TicketConversation :messages="detail.messages" :attachments="detail.attachments" />
          <AdminReplyComposer :sending="acting" @send="onSendReply" />
        </div>

        <!-- Right: actions -->
        <div class="space-y-4 lg:col-span-2">
          <TicketInfoPanel
            :ticket="ticket"
            :admins="admins"
            :acting="acting"
            @claim="assign()"
            @assign="(adminId) => assign(adminId)"
            @change-priority="(p) => changePriority(p)"
            @resolve="(r) => resolve(r)"
            @close="(r) => close(r)"
            @reopen="(r) => reopen(r)"
          />
          <BizLinkCard :ticket="ticket" />
          <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
            <AuditTimeline :audits="detail.audits" />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
