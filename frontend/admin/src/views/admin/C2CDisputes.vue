<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { c2cAPI, type C2CDispute } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { formatDate } from '@/utils/format'

const router = useRouter()
const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const activeTab = ref<'open' | 'resolved' | 'all'>('open')
const disputes = ref<C2CDispute[]>([])
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const fetchList = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const res = await c2cAPI.getDisputes({
      page,
      page_size: pagination.value.page_size,
      status: activeTab.value === 'all' ? undefined : activeTab.value,
    })
    disputes.value = res.data.data || []
    pagination.value = res.data.pagination || pagination.value
  } catch {
    disputes.value = []
  } finally {
    loading.value = false
  }
}

const handleTabChange = () => fetchList(1)
const refresh = () => refreshList(() => fetchList(pagination.value.page, { preserveRows: true }))
const changePage = (page: number) => fetchList(page)
const changePageSize = (size: number) => {
  pagination.value.page_size = size
  fetchList(1)
}
const pageSizeOptions = [10, 20, 50, 100]

const statusClass = (status?: string) =>
  status === 'resolved'
    ? 'border-success/30 bg-success/10 text-success'
    : 'border-destructive/30 bg-destructive/10 text-destructive'

const statusLabel = (status?: string) =>
  status === 'resolved' ? '已处理' : '待处理'

watch(activeTab, handleTabChange)

onMounted(() => fetchList())
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold">申诉仲裁</h1>

    <Tabs v-model="activeTab" class="space-y-4">
      <div class="flex items-center justify-between">
        <TabsList>
          <TabsTrigger value="open">待处理</TabsTrigger>
          <TabsTrigger value="resolved">已处理</TabsTrigger>
          <TabsTrigger value="all">全部</TabsTrigger>
        </TabsList>
        <Button size="sm" variant="outline" :disabled="refreshing" @click="refresh">刷新</Button>
      </div>

      <TabsContent :value="activeTab" class="mt-0">
        <div class="rounded-xl border border-border bg-card overflow-x-auto">
          <Table class="min-w-[860px]">
            <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
              <TableRow>
                <TableHead class="px-6 py-3">申诉ID</TableHead>
                <TableHead class="px-6 py-3">交易ID</TableHead>
                <TableHead class="px-6 py-3">发起人ID</TableHead>
                <TableHead class="px-6 py-3">原因</TableHead>
                <TableHead class="px-6 py-3">状态</TableHead>
                <TableHead class="px-6 py-3">创建时间</TableHead>
                <TableHead class="px-6 py-3">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border">
              <TableRow v-if="loading">
                <TableCell :colspan="7" class="p-0"><TableSkeleton :columns="7" :rows="5" /></TableCell>
              </TableRow>
              <TableRow v-else-if="disputes.length === 0">
                <TableCell colspan="7" class="px-6 py-8 text-center text-muted-foreground">暂无申诉</TableCell>
              </TableRow>
              <TableRow v-for="item in disputes" :key="item.id" class="hover:bg-muted/30">
                <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.id }}</TableCell>
                <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.trade_id }}</TableCell>
                <TableCell class="px-6 py-4 font-mono text-xs">#{{ item.initiator_user_id }}</TableCell>
                <TableCell class="px-6 py-4 text-xs">{{ item.reason }}</TableCell>
                <TableCell class="px-6 py-4">
                  <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
                </TableCell>
                <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
                <TableCell class="px-6 py-4">
                  <Button size="sm" variant="outline" @click="router.push(`/c2c/disputes/${item.id}`)">
                    {{ item.status === 'resolved' ? '查看详情' : '查看 / 仲裁' }}
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
      </TabsContent>
    </Tabs>
  </div>
</template>
