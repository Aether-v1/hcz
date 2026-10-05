<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { supportAPI } from '@/api/support'
import { adminAPI } from '@/api/admin'
import { useAdminTicketList } from '@/composables/useAdminTicketList'
import type { SupportCategory, SupportAdminUser } from '@/types/support'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import StatusBadge from '@/components/support/StatusBadge.vue'
import { formatDate } from '@/utils/format'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const router = useRouter()

const myAdminId = ref(0)
const categories = ref<SupportCategory[]>([])
const admins = ref<SupportAdminUser[]>([])

const {
  loading,
  list,
  total,
  page,
  pageSize,
  filters,
  search,
  changePage,
  changePageSize,
  claim,
} = useAdminTicketList(myAdminId)

const debouncedSearch = useDebounceFn(search, 300)

onMounted(async () => {
  try {
    const me = await adminAPI.getAuthzMe()
    myAdminId.value = Number(me.data?.data?.admin_id || 0)
  } catch { /* ignore */ }
  try {
    const cat = await supportAPI.getCategories()
    categories.value = (cat.data?.data as SupportCategory[]) || []
  } catch { categories.value = [] }
  try {
    const adm = await adminAPI.listAuthzAdmins()
    admins.value = (adm.data?.data as SupportAdminUser[]) || []
  } catch { admins.value = [] }
})

const pageSizeOptions = [10, 20, 50, 100]
const totalPage = computedTotalPage()

function computedTotalPage() {
  return Math.max(1, Math.ceil(total.value / Math.max(1, pageSize.value)))
}

const openDetail = (id: number) => router.push(`/support/tickets/${id}`)
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold">{{ t('admin.support.tickets') }}</h1>

    <!-- 筛选栏 -->
    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-40">
          <Select v-model="filters.status" @update:model-value="search">
            <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.support.filter_status')" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.support.all') }}</SelectItem>
              <SelectItem value="open">{{ t('admin.support.status_open') }}</SelectItem>
              <SelectItem value="waiting_user">{{ t('admin.support.status_waiting_user') }}</SelectItem>
              <SelectItem value="waiting_support">{{ t('admin.support.status_waiting_support') }}</SelectItem>
              <SelectItem value="resolved">{{ t('admin.support.status_resolved') }}</SelectItem>
              <SelectItem value="closed">{{ t('admin.support.status_closed') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.category_id" @update:model-value="search">
            <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.support.filter_category')" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.support.all') }}</SelectItem>
              <SelectItem v-for="c in categories" :key="c.id" :value="String(c.id)">{{ c.name }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-36">
          <Select v-model="filters.priority" @update:model-value="search">
            <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.support.filter_priority')" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.support.all') }}</SelectItem>
              <SelectItem value="low">{{ t('admin.support.priority_low') }}</SelectItem>
              <SelectItem value="normal">{{ t('admin.support.priority_normal') }}</SelectItem>
              <SelectItem value="high">{{ t('admin.support.priority_high') }}</SelectItem>
              <SelectItem value="urgent">{{ t('admin.support.priority_urgent') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-44">
          <Select v-model="filters.assigned" @update:model-value="search">
            <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.support.filter_assigned')" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.support.all') }}</SelectItem>
              <SelectItem value="unassigned">{{ t('admin.support.unassigned') }}</SelectItem>
              <SelectItem value="me">{{ t('admin.support.my_tickets') }}</SelectItem>
              <SelectItem v-for="a in admins" :key="a.id" :value="String(a.id)">{{ a.username || a.email || `#${a.id}` }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <label class="flex items-center gap-2 text-xs text-muted-foreground">
          <Checkbox v-model="filters.unread_only" @update:model-value="search" />
          {{ t('admin.support.unread_only') }}
        </label>
        <div class="w-full md:w-52">
          <Input v-model="filters.search" :placeholder="t('admin.support.search')" @update:model-value="debouncedSearch" />
        </div>
      </div>
    </div>

    <!-- 表格 -->
    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[1080px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.support.ticket_no') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.user') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.category') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.subject') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.status') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.priority') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.assigned_to') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.unread') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.last_reply') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.created_at') }}</TableHead>
            <TableHead class="px-6 py-3 text-right">{{ t('admin.common.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading"><TableCell :colspan="11" class="p-0"><TableSkeleton :columns="11" :rows="6" /></TableCell></TableRow>
          <TableRow v-else-if="list.length === 0"><TableCell colspan="11" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.support.no_tickets') }}</TableCell></TableRow>
          <TableRow v-for="item in list" :key="item.id" class="hover:bg-muted/30 cursor-pointer" @click="openDetail(item.id)">
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.ticket_no }}</TableCell>
            <TableCell class="px-6 py-4 text-xs">
              <div>{{ item.user_email || '-' }}</div>
              <div class="text-muted-foreground">{{ item.user_name || `#${item.user_id}` }}</div>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs">{{ item.category_name || '-' }}</TableCell>
            <TableCell class="px-6 py-4 text-sm">{{ item.subject }}</TableCell>
            <TableCell class="px-6 py-4"><StatusBadge :status="item.status" /></TableCell>
            <TableCell class="px-6 py-4 text-xs">{{ item.priority }}</TableCell>
            <TableCell class="px-6 py-4 text-xs">{{ item.assigned_admin_name || (item.assigned_admin_id ? `#${item.assigned_admin_id}` : t('admin.support.unassigned')) }}</TableCell>
            <TableCell class="px-6 py-4">
              <span v-if="item.admin_unread_count > 0" class="inline-flex h-2 w-2 rounded-full bg-destructive" :title="String(item.admin_unread_count)" />
              <span v-else class="text-muted-foreground text-xs">-</span>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.last_replied_at || item.created_at) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            <TableCell class="px-6 py-4 text-right" @click.stop>
              <Button size="sm" variant="outline" @click="openDetail(item.id)">{{ t('admin.common.view') }}</Button>
              <Button v-if="!item.assigned_admin_id" size="sm" class="ml-2" @click="claim(item)">{{ t('admin.support.claim') }}</Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <ListPagination
        :page="page"
        :total-page="totalPage"
        :total="total"
        :page-size="pageSize"
        :page-size-options="pageSizeOptions"
        @change-page="changePage"
        @change-page-size="changePageSize"
      />
    </div>
  </div>
</template>
