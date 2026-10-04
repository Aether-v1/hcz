<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { c2cAPI, type C2CRiskSignal } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { formatDate } from '@/utils/format'

const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const signals = ref<C2CRiskSignal[]>([])
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const filters = reactive({
  userId: '',
  signalType: '__all__',
})

const signalTypeOptions = [
  { value: 'self_trade_attempt', label: '自交易尝试' },
  { value: 'high_cancel_rate', label: '高取消率' },
  { value: 'repeated_counterparty', label: '重复对手方' },
  { value: 'new_account_large_trade', label: '新户大额交易' },
  { value: 'daily_volume_exceeded', label: '日交易额超限' },
]

const fetchList = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const res = await c2cAPI.getRiskSignals({
      page,
      page_size: pagination.value.page_size,
      user_id: filters.userId || undefined,
      signal_type: filters.signalType === '__all__' ? undefined : filters.signalType,
    })
    signals.value = res.data.data || []
    pagination.value = res.data.pagination || pagination.value
  } catch {
    signals.value = []
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

const typeLabel = (value?: string) => {
  const found = signalTypeOptions.find((o) => o.value === value)
  return found ? found.label : (value || '-')
}

const typeClass = (value?: string) => {
  switch (value) {
    case 'self_trade_attempt': return 'border-destructive/30 bg-destructive/10 text-destructive'
    case 'high_cancel_rate': return 'border-warning/30 bg-warning/10 text-warning'
    case 'repeated_counterparty': return 'border-warning/30 bg-warning/10 text-warning'
    case 'new_account_large_trade': return 'border-info/30 bg-info/10 text-info'
    case 'daily_volume_exceeded': return 'border-destructive/30 bg-destructive/10 text-destructive'
    default: return 'border-border bg-muted text-muted-foreground'
  }
}

const formatMeta = (raw?: string | Record<string, unknown>) => {
  if (raw === undefined || raw === null || raw === '') return '-'
  if (typeof raw !== 'string') return JSON.stringify(raw)
  try {
    return JSON.stringify(JSON.parse(raw))
  } catch {
    return raw
  }
}

onMounted(() => fetchList())
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold">风控信号</h1>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-40">
          <Input v-model="filters.userId" placeholder="用户ID" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-56">
          <Select v-model="filters.signalType" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full"><SelectValue placeholder="全部信号类型" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">全部信号类型</SelectItem>
              <SelectItem v-for="opt in signalTypeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="hidden flex-1 sm:block"></div>
        <Button size="sm" variant="outline" :disabled="refreshing" @click="refresh">刷新</Button>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[860px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">信号ID</TableHead>
            <TableHead class="px-6 py-3">用户ID</TableHead>
            <TableHead class="px-6 py-3">信号类型</TableHead>
            <TableHead class="px-6 py-3">关联交易ID</TableHead>
            <TableHead class="px-6 py-3">元数据</TableHead>
            <TableHead class="px-6 py-3">创建时间</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="6" class="p-0"><TableSkeleton :columns="6" :rows="5" /></TableCell>
          </TableRow>
          <TableRow v-else-if="signals.length === 0">
            <TableCell colspan="6" class="px-6 py-8 text-center text-muted-foreground">暂无风控信号</TableCell>
          </TableRow>
          <TableRow v-for="item in signals" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.id }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.user_id }}</TableCell>
            <TableCell class="px-6 py-4">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="typeClass(item.signal_type)">{{ typeLabel(item.signal_type) }}</span>
            </TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.trade_id ? `#${item.trade_id}` : '-' }}</TableCell>
            <TableCell class="px-6 py-4 max-w-[280px]">
              <div class="truncate font-mono text-xs text-muted-foreground" :title="formatMeta(item.metadata)">{{ formatMeta(item.metadata) }}</div>
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
