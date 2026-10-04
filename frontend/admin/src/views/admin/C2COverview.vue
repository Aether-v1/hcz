<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { c2cAPI, type C2COverview as C2COverviewData, type C2CDispute, type C2CTrade } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { formatDate } from '@/utils/format'

const router = useRouter()
const loading = ref(true)
const overview = ref<C2COverviewData | null>(null)
const openDisputes = ref<C2CDispute[]>([])
const recentTrades = ref<C2CTrade[]>([])

const statCards = computed(() => [
  { label: '活跃挂单数', value: overview.value?.active_listings ?? '-', to: '/c2c/listings' },
  { label: '进行中交易数', value: overview.value?.pending_trades ?? '-', to: '/c2c/trades' },
  { label: '待仲裁申诉数', value: overview.value?.open_disputes ?? '-', to: '/c2c/disputes' },
  { label: '今日成交额 (USDT)', value: overview.value?.today_volume_usdt ?? '-', to: '/c2c/trades' },
  { label: '24h 交易笔数', value: overview.value?.total_trades_24h ?? '-', to: '/c2c/trades' },
])

const tradeStatusClass = (status?: string) => {
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

const tradeStatusLabel = (status?: string) => {
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

const fetchData = async () => {
  loading.value = true
  try {
    const [ovRes, disputeRes, tradeRes] = await Promise.all([
      c2cAPI.getOverview(),
      c2cAPI.getDisputes({ status: 'open', page: 1, page_size: 5 }),
      c2cAPI.getTrades({ page: 1, page_size: 5 }),
    ])
    overview.value = ovRes.data.data
    openDisputes.value = disputeRes.data.data || []
    recentTrades.value = tradeRes.data.data || []
  } catch {
    // 错误已由 client 统一提示
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">C2C 概览</h1>
      <Button size="sm" variant="outline" :disabled="loading" @click="fetchData">刷新</Button>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
      <Card
        v-for="card in statCards"
        :key="card.label"
        class="cursor-pointer"
        @click="router.push(card.to)"
      >
        <CardHeader class="pb-2">
          <CardTitle class="text-xs font-normal text-muted-foreground">{{ card.label }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-semibold font-mono">{{ card.value }}</div>
        </CardContent>
      </Card>
    </div>

    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader class="flex flex-row items-center justify-between space-y-0">
          <CardTitle class="text-base">最近待处理申诉</CardTitle>
          <Button size="sm" variant="ghost" @click="router.push('/c2c/disputes')">查看全部</Button>
        </CardHeader>
        <CardContent class="p-0">
          <Table v-if="!loading">
            <TableHeader>
              <TableRow>
                <TableHead class="px-4 py-2 text-xs">申诉ID</TableHead>
                <TableHead class="px-4 py-2 text-xs">交易号</TableHead>
                <TableHead class="px-4 py-2 text-xs">发起人</TableHead>
                <TableHead class="px-4 py-2 text-xs">原因</TableHead>
                <TableHead class="px-4 py-2 text-xs">时间</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border">
              <TableRow v-if="openDisputes.length === 0">
                <TableCell colspan="5" class="px-4 py-8 text-center text-muted-foreground text-sm">暂无待处理申诉</TableCell>
              </TableRow>
              <TableRow
                v-for="item in openDisputes"
                :key="item.id"
                class="cursor-pointer hover:bg-muted/30"
                @click="router.push(`/c2c/disputes/${item.id}`)"
              >
                <TableCell class="px-4 py-3 text-xs font-mono">#{{ item.id }}</TableCell>
                <TableCell class="px-4 py-3 text-xs font-mono">#{{ item.trade_id }}</TableCell>
                <TableCell class="px-4 py-3 text-xs">#{{ item.initiator_user_id }}</TableCell>
                <TableCell class="px-4 py-3 text-xs">{{ item.reason }}</TableCell>
                <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <div v-else class="p-4"><TableSkeleton :columns="5" :rows="3" /></div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="flex flex-row items-center justify-between space-y-0">
          <CardTitle class="text-base">最近交易</CardTitle>
          <Button size="sm" variant="ghost" @click="router.push('/c2c/trades')">查看全部</Button>
        </CardHeader>
        <CardContent class="p-0">
          <Table v-if="!loading">
            <TableHeader>
              <TableRow>
                <TableHead class="px-4 py-2 text-xs">交易号</TableHead>
                <TableHead class="px-4 py-2 text-xs">买家</TableHead>
                <TableHead class="px-4 py-2 text-xs">卖家</TableHead>
                <TableHead class="px-4 py-2 text-xs">数量</TableHead>
                <TableHead class="px-4 py-2 text-xs">状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border">
              <TableRow v-if="recentTrades.length === 0">
                <TableCell colspan="5" class="px-4 py-8 text-center text-muted-foreground text-sm">暂无交易</TableCell>
              </TableRow>
              <TableRow v-for="item in recentTrades" :key="item.id" class="hover:bg-muted/30">
                <TableCell class="px-4 py-3 text-xs font-mono">#{{ item.id }}</TableCell>
                <TableCell class="px-4 py-3 text-xs">#{{ item.buyer_user_id }}</TableCell>
                <TableCell class="px-4 py-3 text-xs">#{{ item.seller_user_id }}</TableCell>
                <TableCell class="px-4 py-3 text-xs font-mono">{{ item.amount_usdt }} USDT</TableCell>
                <TableCell class="px-4 py-3 text-xs">
                  <span class="inline-flex rounded-full border px-2 py-0.5" :class="tradeStatusClass(item.status)">
                    {{ tradeStatusLabel(item.status) }}
                  </span>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <div v-else class="p-4"><TableSkeleton :columns="5" :rows="3" /></div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
