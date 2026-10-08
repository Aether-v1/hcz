<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('personalCenter.points.history') }}</h1>
    </div>

    <!-- 积分流水列表 -->
    <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
      <div v-if="loading" class="space-y-3 px-5 py-4">
        <div v-for="i in 6" :key="i" class="h-12 animate-pulse rounded-xl bg-muted/60"></div>
      </div>

      <div v-else-if="ledger.length === 0" class="px-5 py-8 text-center text-sm text-muted-foreground">
        {{ t('personalCenter.points.noRecords') }}
      </div>

      <div v-else class="divide-y divide-border">
        <div v-for="entry in ledger" :key="entry.id" class="flex items-center justify-between px-5 py-3">
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium text-foreground">{{ pointsLedgerLabel(t, entry.action_type, entry.reason) }}</p>
            <p class="mt-0.5 text-xs text-muted-foreground">{{ formatDate(entry.created_at) }}</p>
          </div>
          <span
            class="shrink-0 font-mono text-sm font-semibold tabular-nums"
            :class="entry.amount >= 0 ? 'text-success' : 'text-muted-foreground'"
          >
            {{ entry.amount >= 0 ? '+' : '' }}{{ entry.amount }}
          </span>
        </div>
      </div>

      <div v-if="!loading && pagination.total_page > 1" class="border-t px-5 py-2.5">
        <PaginationNav
          :current-page="pagination.page"
          :total-pages="pagination.total_page"
          :loading="loading"
          :scroll-top="false"
          @change-page="loadLedger"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { pointsAPI, type PointsLedgerEntry } from '../../api'
import { pointsLedgerLabel } from '../../utils/status'
import PaginationNav from '../../components/PaginationNav.vue'

const { t } = useI18n()

const ledger = ref<PointsLedgerEntry[]>([])
const loading = ref(true)
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const loadLedger = async (page = 1) => {
  loading.value = true
  try {
    const response = await pointsAPI.ledger({ page, page_size: pagination.value.page_size })
    ledger.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch {
    ledger.value = []
  } finally {
    loading.value = false
  }
}

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

onMounted(() => {
  void loadLedger(1)
})
</script>
