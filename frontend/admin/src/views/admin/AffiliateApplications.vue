<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AffiliateApplication } from '@/api/admin'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatDate } from '@/utils/format'
import { confirmAction } from '@/utils/confirm'
import { notifyError, notifySuccess } from '@/utils/notify'
import { adminUrl } from '@/utils/adminBase'

const { t } = useI18n()
const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const rows = ref<AffiliateApplication[]>([])
const operatingId = ref<number | null>(null)
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const filters = reactive({
  status: '__all__',
  userId: '',
  keyword: '',
})

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)
const userDetailLink = (userId: number) => adminUrl(`/users/${userId}`)

// Detail dialog
const showDetail = ref(false)
const detailLoading = ref(false)
const detailApplication = ref<AffiliateApplication | null>(null)

// Reject dialog
const showReject = ref(false)
const rejectReason = ref('')
const rejectSubmitting = ref(false)
const REJECT_REASON_MAX = 500

const rejectReasonError = computed(() => {
  const value = rejectReason.value.trim()
  if (!value) return t('admin.affiliatesApplications.reject.required')
  if (value.length > REJECT_REASON_MAX) return t('admin.affiliatesApplications.reject.tooLong')
  return ''
})

const canSubmitReject = computed(() => rejectReasonError.value === '')

const fetchRows = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const params: Record<string, unknown> = {
      page,
      page_size: pagination.value.page_size,
    }
    const status = normalizeFilterValue(filters.status)
    if (status) params.status = status
    if (filters.userId.trim()) params.user_id = filters.userId.trim()
    if (filters.keyword.trim()) params.keyword = filters.keyword.trim()
    const response = await adminAPI.getAffiliateApplications(params)
    rows.value = (response.data.data?.list as AffiliateApplication[]) || []
    pagination.value = response.data.data?.pagination || pagination.value
  } catch {
    if (!options.preserveRows) {
      rows.value = []
    }
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => {
  fetchRows(1, { preserveRows: true })
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const reloadCurrentPage = () => fetchRows(pagination.value.page, { preserveRows: true })

const refreshCurrentPage = () => {
  refreshList(reloadCurrentPage)
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchRows(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchRows(1)
}

const statusLabel = (status?: string) => {
  if (status === 'pending') return t('admin.affiliatesApplications.status.pending')
  if (status === 'approved') return t('admin.affiliatesApplications.status.approved')
  if (status === 'rejected') return t('admin.affiliatesApplications.status.rejected')
  return status || '-'
}

const statusClass = (status?: string) => {
  if (status === 'pending') return 'border-amber-200 bg-amber-50 text-amber-700'
  if (status === 'approved') return 'border-emerald-200 bg-emerald-50 text-emerald-700'
  if (status === 'rejected') return 'border-red-200 bg-red-50 text-red-700'
  return 'border-border bg-muted/30 text-muted-foreground'
}

const isPending = (row: AffiliateApplication | null) => row?.status === 'pending'

const openDetail = async (row: AffiliateApplication) => {
  showDetail.value = true
  detailApplication.value = row
  detailLoading.value = true
  try {
    const response = await adminAPI.getAffiliateApplication(row.id)
    detailApplication.value = (response.data.data as AffiliateApplication) || row
  } catch {
    // 详情拉取失败时保留列表数据，错误提示由 client 统一发出
  } finally {
    detailLoading.value = false
  }
}

const closeDetail = () => {
  showDetail.value = false
  detailApplication.value = null
}

const runApprove = async (application: AffiliateApplication) => {
  const confirmed = await confirmAction({
    title: t('admin.affiliatesApplications.approve.title'),
    description: t('admin.affiliatesApplications.approve.confirm', {
      id: application.id,
      user: application.user?.email || application.user?.username || `#${application.user_id}`,
    }),
    confirmText: t('admin.affiliatesApplications.approve.confirmText'),
  })
  if (!confirmed) return

  operatingId.value = application.id
  try {
    await adminAPI.approveAffiliateApplication(application.id)
    notifySuccess(t('admin.affiliatesApplications.approve.success', { id: application.id }))
    closeDetail()
    await reloadCurrentPage()
  } catch (err: any) {
    notifyError(err?.message || t('admin.affiliatesApplications.approve.failed'))
  } finally {
    operatingId.value = null
  }
}

const openReject = (row: AffiliateApplication) => {
  rejectReason.value = ''
  rejectSubmitting.value = false
  // 复用 detailApplication 作为目标，便于关闭详情弹窗后感知状态变化
  detailApplication.value = row
  showReject.value = true
}

const submitReject = async () => {
  if (!canSubmitReject.value || !detailApplication.value) return
  const target = detailApplication.value
  rejectSubmitting.value = true
  try {
    await adminAPI.rejectAffiliateApplication(target.id, { reason: rejectReason.value.trim() })
    notifySuccess(t('admin.affiliatesApplications.reject.success', { id: target.id }))
    showReject.value = false
    closeDetail()
    await reloadCurrentPage()
  } catch (err: any) {
    notifyError(err?.message || t('admin.affiliatesApplications.reject.failed'))
  } finally {
    rejectSubmitting.value = false
  }
}

onMounted(() => {
  fetchRows()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.affiliatesApplications.title') }}</h1>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-wrap items-center gap-3">
        <div class="w-full md:w-56">
          <Input
            v-model="filters.keyword"
            :placeholder="t('admin.affiliatesApplications.filters.keyword')"
            @update:modelValue="debouncedSearch"
            @keyup.enter="handleSearch"
          />
        </div>
        <div class="w-full md:w-40">
          <Input
            v-model="filters.userId"
            :placeholder="t('admin.affiliatesApplications.filters.userId')"
            @keyup.enter="handleSearch"
          />
        </div>
        <div class="w-full md:w-44">
          <Select v-model="filters.status" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.affiliatesApplications.filters.statusAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.affiliatesApplications.filters.statusAll') }}</SelectItem>
              <SelectItem value="pending">{{ t('admin.affiliatesApplications.status.pending') }}</SelectItem>
              <SelectItem value="approved">{{ t('admin.affiliatesApplications.status.approved') }}</SelectItem>
              <SelectItem value="rejected">{{ t('admin.affiliatesApplications.status.rejected') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button size="sm" variant="outline" @click="handleSearch">
          {{ t('admin.affiliatesApplications.filters.search') }}
        </Button>
        <div class="flex-1"></div>
        <Button size="sm" variant="outline" :disabled="refreshing" @click="refreshCurrentPage">{{ t('admin.common.refresh') }}</Button>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[1100px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.affiliatesApplications.table.id') }}</TableHead>
            <TableHead class="min-w-[180px] px-6 py-3">{{ t('admin.affiliatesApplications.table.user') }}</TableHead>
            <TableHead class="min-w-[120px] px-6 py-3">{{ t('admin.affiliatesApplications.table.status') }}</TableHead>
            <TableHead class="min-w-[220px] px-6 py-3">{{ t('admin.affiliatesApplications.table.reason') }}</TableHead>
            <TableHead class="min-w-[150px] px-6 py-3">{{ t('admin.affiliatesApplications.table.createdAt') }}</TableHead>
            <TableHead class="min-w-[150px] px-6 py-3">{{ t('admin.affiliatesApplications.table.reviewedAt') }}</TableHead>
            <TableHead class="min-w-[100px] px-6 py-3">{{ t('admin.affiliatesApplications.table.reviewedBy') }}</TableHead>
            <TableHead class="min-w-[200px] px-6 py-3 text-right">{{ t('admin.affiliatesApplications.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="8" class="p-0">
              <TableSkeleton :columns="8" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="rows.length === 0">
            <TableCell colspan="8" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.affiliatesApplications.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="item in rows" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <IdCell :value="item.id" />
            </TableCell>
            <TableCell class="min-w-[180px] px-6 py-4 text-xs text-muted-foreground">
              <div>
                <a
                  :href="userDetailLink(item.user_id)"
                  target="_blank"
                  rel="noopener"
                  class="font-mono text-primary underline-offset-4 hover:underline"
                >
                  #{{ item.user_id }}
                </a>
              </div>
              <div v-if="item.user?.username" class="mt-0.5 break-words text-foreground">{{ item.user.username }}</div>
              <div v-if="item.user?.email" class="mt-0.5 break-all">{{ item.user.email }}</div>
            </TableCell>
            <TableCell class="min-w-[120px] px-6 py-4 text-xs">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.status)">
                {{ statusLabel(item.status) }}
              </span>
            </TableCell>
            <TableCell class="min-w-[220px] px-6 py-4">
              <p class="max-w-[280px] truncate text-xs text-foreground" :title="item.reason">{{ item.reason || '-' }}</p>
            </TableCell>
            <TableCell class="min-w-[150px] px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            <TableCell class="min-w-[150px] px-6 py-4 text-xs text-muted-foreground">{{ item.reviewed_at ? formatDate(item.reviewed_at) : '-' }}</TableCell>
            <TableCell class="min-w-[100px] px-6 py-4 text-xs text-muted-foreground">
              <span v-if="item.reviewed_by">{{ item.reviewed_by }}</span>
              <span v-else>-</span>
            </TableCell>
            <TableCell class="min-w-[200px] px-6 py-4">
              <div class="flex flex-wrap justify-end gap-1">
                <Button size="xs" variant="outline" @click="openDetail(item)">
                  {{ t('admin.affiliatesApplications.actions.detail') }}
                </Button>
                <Button
                  v-if="isPending(item)"
                  size="xs"
                  variant="default"
                  :disabled="operatingId === item.id"
                  @click="runApprove(item)"
                >
                  {{ t('admin.affiliatesApplications.actions.approve') }}
                </Button>
                <Button
                  v-if="isPending(item)"
                  size="xs"
                  variant="destructive"
                  :disabled="operatingId === item.id"
                  @click="openReject(item)"
                >
                  {{ t('admin.affiliatesApplications.actions.reject') }}
                </Button>
              </div>
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

    <!-- Detail Dialog -->
    <Dialog v-model:open="showDetail">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-lg p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{{ t('admin.affiliatesApplications.detail.title') }}</DialogTitle>
        </DialogHeader>
        <div v-if="detailLoading" class="py-8 text-center text-sm text-muted-foreground">
          {{ t('admin.affiliatesApplications.detail.loading') }}
        </div>
        <div v-else-if="detailApplication" class="space-y-3 text-sm">
          <div class="grid grid-cols-1 gap-y-2 sm:grid-cols-[120px_1fr]">
            <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.table.id') }}</span>
            <span><IdCell :value="detailApplication.id" /></span>

            <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.table.user') }}</span>
            <div>
              <a
                v-if="detailApplication.user_id"
                :href="userDetailLink(detailApplication.user_id)"
                target="_blank"
                rel="noopener"
                class="font-mono text-primary underline-offset-4 hover:underline"
              >
                #{{ detailApplication.user_id }}
              </a>
              <div v-if="detailApplication.user?.username" class="mt-0.5 break-words text-foreground">{{ detailApplication.user.username }}</div>
              <div v-if="detailApplication.user?.email" class="mt-0.5 break-all">{{ detailApplication.user.email }}</div>
            </div>

            <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.table.status') }}</span>
            <span class="inline-flex w-fit rounded-full border px-2.5 py-1 text-xs" :class="statusClass(detailApplication.status)">
              {{ statusLabel(detailApplication.status) }}
            </span>

            <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.table.reason') }}</span>
            <p class="break-words rounded-md border border-border bg-muted/30 p-2 text-xs">{{ detailApplication.reason || '-' }}</p>

            <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.table.createdAt') }}</span>
            <span>{{ formatDate(detailApplication.created_at) }}</span>

            <template v-if="detailApplication.reviewed_at">
              <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.table.reviewedAt') }}</span>
              <span>{{ formatDate(detailApplication.reviewed_at) }}</span>
            </template>

            <template v-if="detailApplication.reviewed_by">
              <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.table.reviewedBy') }}</span>
              <span>{{ detailApplication.reviewed_by }}</span>
            </template>

            <template v-if="detailApplication.review_note">
              <span class="text-muted-foreground">{{ t('admin.affiliatesApplications.detail.reviewNote') }}</span>
              <p class="break-words rounded-md border border-border bg-muted/30 p-2 text-xs text-destructive">{{ detailApplication.review_note }}</p>
            </template>
          </div>

          <div v-if="isPending(detailApplication)" class="flex flex-col-reverse gap-2 border-t border-border pt-4 sm:flex-row sm:justify-end">
            <Button variant="outline" @click="closeDetail">{{ t('admin.common.cancel') }}</Button>
            <Button variant="destructive" :disabled="operatingId === detailApplication.id" @click="openReject(detailApplication)">
              {{ t('admin.affiliatesApplications.actions.reject') }}
            </Button>
            <Button :disabled="operatingId === detailApplication.id" @click="runApprove(detailApplication)">
              {{ t('admin.affiliatesApplications.actions.approve') }}
            </Button>
          </div>
          <div v-else class="flex justify-end border-t border-border pt-4">
            <Button variant="outline" @click="closeDetail">{{ t('admin.common.close') }}</Button>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>

    <!-- Reject Dialog -->
    <Dialog v-model:open="showReject">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-md p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{{ t('admin.affiliatesApplications.reject.title') }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-4">
          <div class="space-y-2">
            <Label>{{ t('admin.affiliatesApplications.reject.reasonLabel') }}</Label>
            <Textarea
              v-model="rejectReason"
              :maxlength="REJECT_REASON_MAX"
              :placeholder="t('admin.affiliatesApplications.reject.reasonPlaceholder')"
              class="min-h-[100px]"
            />
            <div class="flex items-center justify-between text-xs">
              <span v-if="rejectReasonError" class="text-destructive">{{ rejectReasonError }}</span>
              <span v-else></span>
              <span class="text-muted-foreground">{{ rejectReason.length }} / {{ REJECT_REASON_MAX }}</span>
            </div>
          </div>
          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button variant="outline" :disabled="rejectSubmitting" @click="showReject = false">
              {{ t('admin.common.cancel') }}
            </Button>
            <Button variant="destructive" :disabled="!canSubmitReject || rejectSubmitting" @click="submitReject">
              {{ rejectSubmitting ? t('admin.affiliatesApplications.reject.submitting') : t('admin.affiliatesApplications.actions.reject') }}
            </Button>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
