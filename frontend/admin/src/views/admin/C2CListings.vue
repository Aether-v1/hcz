<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { c2cAPI, type C2CListing } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { formatDate } from '@/utils/format'
import { confirmAction } from '@/utils/confirm'
import { notifySuccess } from '@/utils/notify'

const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const listings = ref<C2CListing[]>([])
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const filters = reactive({
  sellerUserId: '',
  status: '__all__',
  fiatCurrency: '',
})

const fetchList = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const res = await c2cAPI.getListings({
      page,
      page_size: pagination.value.page_size,
      seller_user_id: filters.sellerUserId || undefined,
      status: filters.status === '__all__' ? undefined : filters.status,
      fiat_currency: filters.fiatCurrency || undefined,
    })
    listings.value = res.data.data || []
    pagination.value = res.data.pagination || pagination.value
  } catch {
    listings.value = []
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
    case 'active': return 'border-success/30 bg-success/10 text-success'
    case 'paused': return 'border-warning/30 bg-warning/10 text-warning'
    case 'closed': return 'border-border bg-muted text-muted-foreground'
    default: return 'border-border bg-muted text-muted-foreground'
  }
}
const statusLabel = (status?: string) => {
  const map: Record<string, string> = { active: '进行中', paused: '已暂停', closed: '已关闭' }
  return status ? (map[status] || status) : '-'
}

const canClose = (item: C2CListing) => item.status === 'active' || item.status === 'paused'

const doClose = async (item: C2CListing) => {
  const confirmed = await confirmAction({
    title: '强制关闭挂单',
    description: `确认关闭挂单 #${item.id}（卖家 #${item.seller_user_id}）？关闭后该挂单不可再被交易。`,
    confirmText: '确认关闭',
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await c2cAPI.closeListing(item.id)
    notifySuccess('挂单已关闭')
    await fetchList(pagination.value.page)
  } catch {
    // 错误已统一提示
  }
}

onMounted(() => fetchList())
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold">挂单管理</h1>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-40">
          <Input v-model="filters.sellerUserId" placeholder="卖家用户ID" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.status" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full"><SelectValue placeholder="全部状态" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">全部状态</SelectItem>
              <SelectItem value="active">进行中</SelectItem>
              <SelectItem value="paused">已暂停</SelectItem>
              <SelectItem value="closed">已关闭</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-40">
          <Input v-model="filters.fiatCurrency" placeholder="法币币种 (如 CNY)" @update:modelValue="debouncedSearch" />
        </div>
        <div class="hidden flex-1 sm:block"></div>
        <Button size="sm" variant="outline" :disabled="refreshing" @click="refresh">刷新</Button>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[960px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">挂单ID</TableHead>
            <TableHead class="px-6 py-3">卖家ID</TableHead>
            <TableHead class="px-6 py-3">法币</TableHead>
            <TableHead class="px-6 py-3">价格</TableHead>
            <TableHead class="px-6 py-3">总量 (USDT)</TableHead>
            <TableHead class="px-6 py-3">剩余 (USDT)</TableHead>
            <TableHead class="px-6 py-3">状态</TableHead>
            <TableHead class="px-6 py-3">创建时间</TableHead>
            <TableHead class="px-6 py-3">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="9" class="p-0"><TableSkeleton :columns="9" :rows="5" /></TableCell>
          </TableRow>
          <TableRow v-else-if="listings.length === 0">
            <TableCell colspan="9" class="px-6 py-8 text-center text-muted-foreground">暂无挂单</TableCell>
          </TableRow>
          <TableRow v-for="item in listings" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.id }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.seller_user_id }}</TableCell>
            <TableCell class="px-6 py-4 text-xs">{{ item.fiat_currency }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.price }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.total_amount }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.remaining_amount }}</TableCell>
            <TableCell class="px-6 py-4">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            <TableCell class="px-6 py-4">
              <Button
                v-if="canClose(item)"
                size="sm"
                variant="destructive"
                @click="doClose(item)"
              >关闭</Button>
              <span v-else class="text-xs text-muted-foreground">-</span>
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
</template>
