<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AdminWithdrawal } from '@/api/admin'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatDate } from '@/utils/format'
import ComplianceGuardWrapper from '@/components/ComplianceGuardWrapper.vue'
import { adminUrl } from '@/utils/adminBase'
import { useRouter } from 'vue-router'

const { t } = useI18n()
const router = useRouter()
const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const withdrawals = ref<AdminWithdrawal[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const filters = reactive({
  withdrawalNo: '',
  userId: '',
  status: '__all__',
  startDate: '',
  endDate: '',
})

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

const fetchWithdrawals = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const response = await adminAPI.getWalletWithdrawals({
      page,
      page_size: pagination.value.page_size,
      withdrawal_no: filters.withdrawalNo || undefined,
      user_id: filters.userId || undefined,
      status: normalizeFilterValue(filters.status) || undefined,
      start_date: filters.startDate || undefined,
      end_date: filters.endDate || undefined,
    })
    withdrawals.value = response.data.data?.list || response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch {
    if (!options.preserveRows) withdrawals.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => {
  fetchWithdrawals(1)
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const refresh = () => {
  refreshList(() => fetchWithdrawals(pagination.value.page, { preserveRows: true }))
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchWithdrawals(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchWithdrawals(1)
}

const userLink = (userID: number) => adminUrl(`/users/${userID}`)

const statusClass = (status?: string) => {
  switch (status) {
    case 'pending': return 'border-warning/30 bg-warning/10 text-warning'
    case 'approved': return 'border-info/30 bg-info/10 text-info'
    case 'processing': return 'border-primary/30 bg-primary/10 text-primary'
    case 'completed': return 'border-success/30 bg-success/10 text-success'
    case 'rejected': return 'border-destructive/30 bg-destructive/10 text-destructive'
    case 'canceled': return 'border-border bg-muted text-muted-foreground'
    default: return 'border-border bg-muted text-muted-foreground'
  }
}

const statusLabel = (status?: string) => {
  const map: Record<string, string> = {
    pending: t('admin.walletWithdrawals.status.pending'),
    approved: t('admin.walletWithdrawals.status.approved'),
    processing: t('admin.walletWithdrawals.status.processing'),
    completed: t('admin.walletWithdrawals.status.completed'),
    rejected: t('admin.walletWithdrawals.status.rejected'),
    canceled: t('admin.walletWithdrawals.status.canceled'),
  }
  return status ? (map[status] || status) : '-'
}

const goDetail = (id: number) => {
  router.push(`/wallet-withdrawals/${id}`)
}

onMounted(() => {
  fetchWithdrawals()
})
</script>

<template>
  <ComplianceGuardWrapper>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h1 class="text-2xl font-semibold">{{ t('admin.walletWithdrawals.title') }}</h1>
      </div>

      <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
          <div class="w-full md:w-48">
            <Input v-model="filters.withdrawalNo" :placeholder="t('admin.walletWithdrawals.filterWithdrawalNo')" @update:modelValue="debouncedSearch" />
          </div>
          <div class="w-full md:w-32">
            <Input v-model="filters.userId" :placeholder="t('admin.walletWithdrawals.filterUserId')" @update:modelValue="debouncedSearch" />
          </div>
          <div class="w-full md:w-40">
            <Select v-model="filters.status" @update:modelValue="handleSearch">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.walletWithdrawals.filterStatusAll')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">{{ t('admin.walletWithdrawals.filterStatusAll') }}</SelectItem>
                <SelectItem value="pending">{{ t('admin.walletWithdrawals.status.pending') }}</SelectItem>
                <SelectItem value="approved">{{ t('admin.walletWithdrawals.status.approved') }}</SelectItem>
                <SelectItem value="processing">{{ t('admin.walletWithdrawals.status.processing') }}</SelectItem>
                <SelectItem value="completed">{{ t('admin.walletWithdrawals.status.completed') }}</SelectItem>
                <SelectItem value="rejected">{{ t('admin.walletWithdrawals.status.rejected') }}</SelectItem>
                <SelectItem value="canceled">{{ t('admin.walletWithdrawals.status.canceled') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
            <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('admin.walletWithdrawals.filterDateRange') }}</span>
            <Input
              v-model="filters.startDate"
              type="date"
              class="h-9 w-full md:w-auto"
              @update:modelValue="handleSearch"
            />
            <span class="hidden text-muted-foreground md:inline">-</span>
            <Input
              v-model="filters.endDate"
              type="date"
              class="h-9 w-full md:w-auto"
              @update:modelValue="handleSearch"
            />
          </div>
          <div class="hidden flex-1 sm:block"></div>
          <Button size="sm" variant="outline" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">{{ t('admin.common.refresh') }}</Button>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card overflow-x-auto">
        <Table class="min-w-[1000px]">
          <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
            <TableRow>
              <TableHead class="px-6 py-3">{{ t('admin.walletWithdrawals.table.id') }}</TableHead>
              <TableHead class="min-w-[160px] px-6 py-3">{{ t('admin.walletWithdrawals.table.withdrawalNo') }}</TableHead>
              <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.walletWithdrawals.table.user') }}</TableHead>
              <TableHead class="min-w-[100px] px-6 py-3">{{ t('admin.walletWithdrawals.table.network') }}</TableHead>
              <TableHead class="min-w-[180px] px-6 py-3">{{ t('admin.walletWithdrawals.table.address') }}</TableHead>
              <TableHead class="min-w-[120px] px-6 py-3">{{ t('admin.walletWithdrawals.table.amount') }}</TableHead>
              <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.walletWithdrawals.table.status') }}</TableHead>
              <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.walletWithdrawals.table.createdAt') }}</TableHead>
              <TableHead class="min-w-[80px] px-6 py-3">{{ t('admin.walletWithdrawals.table.actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border">
            <TableRow v-if="loading">
              <TableCell :colspan="9" class="p-0">
                <TableSkeleton :columns="9" :rows="5" />
              </TableCell>
            </TableRow>
            <TableRow v-else-if="withdrawals.length === 0">
              <TableCell colspan="9" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.walletWithdrawals.empty') }}</TableCell>
            </TableRow>
            <TableRow v-for="item in withdrawals" :key="item.id" class="hover:bg-muted/30 cursor-pointer" @click="goDetail(item.id)">
              <TableCell class="px-6 py-4">
                <IdCell :value="item.id" />
              </TableCell>
              <TableCell class="min-w-[160px] px-6 py-4 text-foreground font-mono text-xs">
                <div class="break-all">{{ item.withdrawal_no }}</div>
              </TableCell>
              <TableCell class="min-w-[140px] px-6 py-4 text-xs text-muted-foreground">
                <div class="text-foreground">
                  <a v-if="item.user_id" :href="userLink(item.user_id)" target="_blank" rel="noopener" @click.stop class="text-primary underline-offset-4 hover:underline">
                    #{{ item.user_id }}
                  </a>
                </div>
                <div v-if="item.user?.email" class="mt-0.5 break-all">{{ item.user.email }}</div>
              </TableCell>
              <TableCell class="min-w-[100px] px-6 py-4 text-xs text-foreground">{{ item.network }}</TableCell>
              <TableCell class="min-w-[180px] px-6 py-4 text-xs text-muted-foreground">
                <div class="break-all font-mono">{{ item.address }}</div>
              </TableCell>
              <TableCell class="min-w-[120px] px-6 py-4 text-xs">
                <div class="font-mono text-foreground">{{ item.request_amount }} USDT</div>
                <div class="mt-0.5 text-muted-foreground">Fee: {{ item.fee_amount }} USDT</div>
                <div class="mt-0.5 font-mono text-primary">Net: {{ item.net_amount }} USDT</div>
              </TableCell>
              <TableCell class="min-w-[90px] px-6 py-4 text-xs">
                <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.status)">
                  {{ statusLabel(item.status) }}
                </span>
              </TableCell>
              <TableCell class="min-w-[140px] px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
              <TableCell class="min-w-[80px] px-6 py-4 text-xs">
                <Button size="sm" variant="outline" @click.stop="goDetail(item.id)">
                  {{ t('admin.walletWithdrawals.detail') }}
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>

        <ListPagination
          :page="pagination.page"
          :total-page="pagination.total_page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          :page-size-options="pageSizeOptions"
          @change-page="changePage"
          @change-page-size="changePageSize"
        />
      </div>
    </div>
  </ComplianceGuardWrapper>
</template>
