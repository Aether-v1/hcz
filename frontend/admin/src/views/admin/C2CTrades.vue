<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { c2cAPI, type C2CTrade } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { formatDate, toRFC3339 } from '@/utils/format'

const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const trades = ref<C2CTrade[]>([])
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const filters = reactive({
  status: '__all__',
  buyerUserId: '',
  sellerUserId: '',
  createdFrom: '',
  createdTo: '',
})

const fetchList = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const res = await c2cAPI.getTrades({
      page,
      page_size: pagination.value.page_size,
      status: filters.status === '__all__' ? undefined : filters.status,
      buyer_user_id: filters.buyerUserId || undefined,
      seller_user_id: filters.sellerUserId || undefined,
      created_from: toRFC3339(filters.createdFrom),
      created_to: toRFC3339(filters.createdTo),
    })
    trades.value = res.data.data || []
    pagination.value = res.data.pagination || pagination.value
  } catch {
    trades.value = []
  } finally {
    loading.value = false
  }
}

const handleSearch = () => fetchList(1)
const debouncedSearch = useDebounceFn(handleSearch, 300)
const refresh = () => refreshList(() => fetchList(pagination.value.page, { preserveRows: true }))

const changePage = (page: number) => fetchList(page)
const changePageSize = (size: number) => {
  pagination.value.page_size = size
  fetchList(1)
}
const pageSizeOptions = [10, 20, 50, 100]

const statusClass = (status?: string) => {
  switch (status) {
    case 'pending_payment': return 'border-warning/30 bg-warning/10 text-warning'
    case 'paid': return 'border-info/30 bg-info/10 text-info'
    case 'completed': return 'border-success/30 bg-success/10 text-success'
    case 'canceled':
    case 'expired': return 'border-border bg-muted text-muted-foreground'
    case 'disputed': return 'border-destructive/30 bg-destructive/10 text-destructive'
    default: return 'border-border bg-muted text-muted-foreground'
  }
}
const statusLabel = (status?: string) => {
  const map: Record<string, string> = {
    pending_payment: '待付款',
    paid: '已付款',
    completed: '已完成',
    canceled: '已取消',
    expired: '已过期',
    disputed: '争议中',
  }
  return status ? (map[status] || status) : '-'
}

onMounted(() => fetchList())
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold">交易管理</h1>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-40">
          <Select v-model="filters.status" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full"><SelectValue placeholder="全部状态" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">全部状态</SelectItem>
              <SelectItem value="pending_payment">待付款</SelectItem>
              <SelectItem value="paid">已付款</SelectItem>
              <SelectItem value="completed">已完成</SelectItem>
              <SelectItem value="canceled">已取消</SelectItem>
              <SelectItem value="expired">已过期</SelectItem>
              <SelectItem value="disputed">争议中</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-36">
          <Input v-model="filters.buyerUserId" placeholder="买家ID" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-36">
          <Input v-model="filters.sellerUserId" placeholder="卖家ID" @update:modelValue="debouncedSearch" />
        </div>
        <div class="flex w-full flex-col gap-2 md:w-auto md:flex-row md:items-center">
          <span class="text-xs text-muted-foreground whitespace-nowrap">创建时间</span>
          <Input v-model="filters.createdFrom" type="datetime-local" class="h-9 w-full md:w-auto" @update:modelValue="handleSearch" />
          <span class="hidden text-muted-foreground md:inline">-</span>
          <Input v-model="filters.createdTo" type="datetime-local" class="h-9 w-full md:w-auto" @update:modelValue="handleSearch" />
        </div>
        <div class="hidden flex-1 sm:block"></div>
        <Button size="sm" variant="outline" :disabled="refreshing" @click="refresh">刷新</Button>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[960px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">交易ID</TableHead>
            <TableHead class="px-6 py-3">挂单ID</TableHead>
            <TableHead class="px-6 py-3">买家ID</TableHead>
            <TableHead class="px-6 py-3">卖家ID</TableHead>
            <TableHead class="px-6 py-3">数量 (USDT)</TableHead>
            <TableHead class="px-6 py-3">法币金额</TableHead>
            <TableHead class="px-6 py-3">状态</TableHead>
            <TableHead class="px-6 py-3">创建时间</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="8" class="p-0"><TableSkeleton :columns="8" :rows="5" /></TableCell>
          </TableRow>
          <TableRow v-else-if="trades.length === 0">
            <TableCell colspan="8" class="px-6 py-8 text-center text-muted-foreground">暂无交易</TableCell>
          </TableRow>
          <TableRow v-for="item in trades" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.id }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.listing_id }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.buyer_user_id }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.seller_user_id }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.amount_usdt }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.fiat_amount }} {{ item.fiat_currency }}</TableCell>
            <TableCell class="px-6 py-4">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
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
</template>
