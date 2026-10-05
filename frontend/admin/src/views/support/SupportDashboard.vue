<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { supportAPI } from '@/api/support'
import type { SupportOverview, SupportTicketListItem } from '@/types/support'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import StatusBadge from '@/components/support/StatusBadge.vue'
import { formatDate } from '@/utils/format'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const router = useRouter()
const loading = ref(true)
const overview = ref<SupportOverview | null>(null)
const recent = ref<SupportTicketListItem[]>([])

const fetchData = async () => {
  loading.value = true
  try {
    const [ov, list] = await Promise.all([
      supportAPI.getOverview(),
      supportAPI.getTickets({ page: 1, page_size: 10 }),
    ])
    overview.value = (ov.data?.data as SupportOverview) || null
    recent.value = (list.data?.data?.items as SupportTicketListItem[]) || []
  } catch {
    overview.value = null
    recent.value = []
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)

const cards = () => [
  { key: 'total', value: overview.value?.total ?? 0 },
  { key: 'waiting_support', value: overview.value?.waiting_support ?? 0 },
  { key: 'waiting_user', value: overview.value?.waiting_user ?? 0 },
  { key: 'resolved', value: overview.value?.resolved ?? 0 },
  { key: 'closed', value: overview.value?.closed ?? 0 },
  { key: 'unassigned', value: overview.value?.unassigned ?? 0 },
  { key: 'my_tickets', value: overview.value?.my_assigned ?? 0 },
]
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.support.dashboard') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('admin.support.dashboardSubtitle') }}</p>
      </div>
      <div class="flex gap-2">
        <RouterLink to="/support/tickets"><Button size="sm" variant="outline">{{ t('admin.support.tickets') }}</Button></RouterLink>
        <RouterLink to="/support/categories"><Button size="sm">{{ t('admin.support.categories') }}</Button></RouterLink>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7">
      <div v-for="card in cards()" :key="card.key" class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <div class="text-xs text-muted-foreground">{{ t(`admin.support.${card.key}`) }}</div>
        <div class="mt-2 text-2xl font-semibold">{{ card.value }}</div>
      </div>
    </div>

    <!-- 最近工单 -->
    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <div class="px-6 py-4 text-sm font-semibold border-b border-border">{{ t('admin.support.recentTickets') }}</div>
      <Table class="min-w-[860px]">
        <TableHeader class="bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.support.ticket_no') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.user') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.subject') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.status') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.priority') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.created_at') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading"><TableCell :colspan="6" class="p-0"><TableSkeleton :columns="6" :rows="6" /></TableCell></TableRow>
          <TableRow v-else-if="recent.length === 0"><TableCell colspan="6" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.support.no_tickets') }}</TableCell></TableRow>
          <TableRow v-for="item in recent" :key="item.id" class="hover:bg-muted/30 cursor-pointer" @click="router.push(`/support/tickets/${item.id}`)">
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.ticket_no }}</TableCell>
            <TableCell class="px-6 py-4 text-xs">{{ item.user_email || item.user_name || `#${item.user_id}` }}</TableCell>
            <TableCell class="px-6 py-4 text-sm">{{ item.subject }}</TableCell>
            <TableCell class="px-6 py-4"><StatusBadge :status="item.status" /></TableCell>
            <TableCell class="px-6 py-4 text-xs">{{ item.priority }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
