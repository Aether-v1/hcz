<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { supportAPI } from '@/api/support'
import type { SupportCategory } from '@/types/support'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import CategoryFormDialog from '@/components/support/CategoryFormDialog.vue'
import { formatDate } from '@/utils/format'
import { notifySuccess, notifyError } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const loading = ref(true)
const categories = ref<SupportCategory[]>([])
const dialogOpen = ref(false)
const editing = ref<SupportCategory | null>(null)
const submitting = ref(false)

const fetchList = async () => {
  loading.value = true
  try {
    const res = await supportAPI.getCategories()
    categories.value = (res.data?.data as SupportCategory[]) || []
  } catch {
    categories.value = []
  } finally {
    loading.value = false
  }
}

onMounted(fetchList)

const openCreate = () => {
  editing.value = null
  dialogOpen.value = true
}

const openEdit = (cat: SupportCategory) => {
  editing.value = cat
  dialogOpen.value = true
}

const onSubmit = async (payload: { code?: string; name: string; enabled: boolean; sort_order: number; default_priority: string }) => {
  submitting.value = true
  try {
    if (editing.value) {
      await supportAPI.updateCategory(editing.value.id, payload)
    } else {
      await supportAPI.createCategory(payload)
    }
    notifySuccess(t('admin.common.operationSuccess'))
    dialogOpen.value = false
    await fetchList()
  } catch (err: any) {
    notifyError(err?.message)
  } finally {
    submitting.value = false
  }
}

const toggleEnabled = async (cat: SupportCategory) => {
  const next = !cat.enabled
  try {
    cat.enabled = next
    await supportAPI.updateCategory(cat.id, {
      name: cat.name,
      enabled: next,
      sort_order: cat.sort_order,
      default_priority: cat.default_priority,
    })
  } catch (err: any) {
    cat.enabled = !next
    notifyError(err?.message)
  }
}

const handleDelete = async (cat: SupportCategory) => {
  const ok = await confirmAction({
    description: t('admin.support.deleteConfirm', { name: cat.name }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!ok) return
  try {
    await supportAPI.deleteCategory(cat.id)
    notifySuccess(t('admin.common.operationSuccess'))
    await fetchList()
  } catch (err: any) {
    notifyError(err?.message)
  }
}

const move = async (index: number, direction: -1 | 1) => {
  const target = categories.value[index + direction]
  const current = categories.value[index]
  if (!target || !current) return
  try {
    await supportAPI.updateCategory(current.id, {
      name: current.name,
      enabled: current.enabled,
      sort_order: target.sort_order,
      default_priority: current.default_priority,
    })
    await supportAPI.updateCategory(target.id, {
      name: target.name,
      enabled: target.enabled,
      sort_order: current.sort_order,
      default_priority: target.default_priority,
    })
    await fetchList()
  } catch (err: any) {
    notifyError(err?.message)
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.support.categories') }}</h1>
      <Button class="w-full sm:w-auto" @click="openCreate">{{ t('admin.support.add_category') }}</Button>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[860px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.support.category_code') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.category_name') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.category_default_priority') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.category_sort_order') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.category_enabled') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.support.created_at') }}</TableHead>
            <TableHead class="px-6 py-3 text-right">{{ t('admin.common.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading"><TableCell :colspan="7" class="p-0"><TableSkeleton :columns="7" :rows="5" /></TableCell></TableRow>
          <TableRow v-else-if="categories.length === 0"><TableCell colspan="7" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.support.no_categories') }}</TableCell></TableRow>
          <TableRow v-for="(item, index) in categories" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4 font-mono text-xs">{{ item.code }}</TableCell>
            <TableCell class="px-6 py-4 text-sm">{{ item.name }}</TableCell>
            <TableCell class="px-6 py-4 text-xs">{{ item.default_priority }}</TableCell>
            <TableCell class="px-6 py-4">
              <div class="flex items-center gap-1">
                <span class="font-mono text-xs">{{ item.sort_order }}</span>
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === 0" @click="move(index, -1)">↑</Button>
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === categories.length - 1" @click="move(index, 1)">↓</Button>
              </div>
            </TableCell>
            <TableCell class="px-6 py-4">
              <Switch :model-value="item.enabled" @update:model-value="() => toggleEnabled(item)" />
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            <TableCell class="px-6 py-4 text-right">
              <div class="flex items-center justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEdit(item)">{{ t('admin.support.edit_category') }}</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(item)">{{ t('admin.support.delete_category') }}</Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <CategoryFormDialog
      :open="dialogOpen"
      :editing="editing"
      @update:open="dialogOpen = $event"
      @submit="onSubmit"
    />
  </div>
</template>
