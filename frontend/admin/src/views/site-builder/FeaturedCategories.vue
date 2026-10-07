<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import TableSkeleton from '@/components/TableSkeleton.vue'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import { adminAPI } from '@/api/admin'
import type { AdminCategory } from '@/api/types'
import { getImageUrl } from '@/utils/image'
import {
  getFeaturedCategories,
  createFeaturedCategory,
  updateFeaturedCategory,
  deleteFeaturedCategory,
  toggleFeaturedCategory,
  reorderFeaturedCategories,
  type FeaturedCategory,
} from '@/api/site-builder'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'

const MAX_FEATURED = 6

const loading = ref(false)
const submitting = ref(false)
const showModal = ref(false)
const isEditing = ref(false)

const items = ref<FeaturedCategory[]>([])
const categories = ref<AdminCategory[]>([])
const categoriesLoading = ref(false)

const fetchCategories = async () => {
  categoriesLoading.value = true
  try {
    const res = await adminAPI.getCategories({ page_size: 100 })
    const list = (res.data?.data || []) as AdminCategory[]
    categories.value = list.filter((c) => c.is_active)
  } catch {
    categories.value = []
    notifyError('分类加载失败')
  } finally {
    categoriesLoading.value = false
  }
}

const emptyForm = (): FeaturedCategory => ({
  category_id: 0,
  alias: '',
  icon_override: '',
  enabled: true,
  sort_order: 0,
})

const form = reactive<FeaturedCategory>(emptyForm())
const formError = ref('')

const fetchItems = async () => {
  loading.value = true
  try {
    const res = await getFeaturedCategories()
    const list = (res.data?.data || []) as FeaturedCategory[]
    items.value = [...list].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

const categoryName = (cat: AdminCategory) =>
  cat.name?.['zh-CN'] || cat.name?.['en-US'] || cat.slug

const categoryByID = (id: number) => categories.value.find((c) => c.id === id)

const openCreate = () => {
  isEditing.value = false
  Object.assign(form, emptyForm())
  formError.value = ''
  showModal.value = true
  void fetchCategories()
}

const openEdit = (item: FeaturedCategory) => {
  isEditing.value = true
  Object.assign(form, {
    id: item.id,
    category_id: item.category_id || 0,
    alias: item.alias || '',
    icon_override: item.icon_override || '',
    enabled: Boolean(item.enabled),
    sort_order: Number(item.sort_order || 0),
  })
  formError.value = ''
  showModal.value = true
  void fetchCategories()
}

const closeModal = () => {
  showModal.value = false
}

const validate = (): boolean => {
  formError.value = ''
  if (!form.category_id) {
    formError.value = '请选择业务分类'
    return false
  }
  return true
}

const submit = async () => {
  if (!validate()) return
  submitting.value = true
  try {
    if (isEditing.value && form.id) {
      await updateFeaturedCategory(form.id, { ...form })
    } else {
      await createFeaturedCategory({ ...form })
    }
    showModal.value = false
    notifySuccess('已保存')
    fetchItems()
  } catch (e: any) {
    const msg = e?.response?.data?.message || e?.message || '保存失败'
    notifyError(msg)
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (item: FeaturedCategory) => {
  const cat = categoryByID(item.category_id)
  const ok = await confirmAction({
    description: `确认移除热门推荐「${cat ? categoryName(cat) : '#' + item.category_id}」？`,
    confirmText: '移除',
    variant: 'destructive',
  })
  if (!ok || !item.id) return
  try {
    await deleteFeaturedCategory(item.id)
    notifySuccess('已移除')
    fetchItems()
  } catch {
    notifyError('操作失败')
  }
}

const handleToggle = async (item: FeaturedCategory) => {
  if (!item.id) return
  const next = !item.enabled
  item.enabled = next
  try {
    await toggleFeaturedCategory(item.id, next)
  } catch {
    item.enabled = !next
    notifyError('操作失败')
  }
}

const move = async (index: number, direction: -1 | 1) => {
  const target = index + direction
  if (target < 0 || target >= items.value.length) return
  const current = items.value[index]
  const next = items.value[target]
  if (!current?.id || !next?.id) return
  const tmp = items.value[index]!
  items.value[index] = items.value[target]!
  items.value[target] = tmp
  try {
    await reorderFeaturedCategories(items.value.map((e) => e.id!).filter((x) => typeof x === 'number'))
  } catch {
    const tmp2 = items.value[target]!
    items.value[target] = items.value[index]!
    items.value[index] = tmp2
    notifyError('排序失败')
  }
}

onMounted(fetchItems)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <p class="text-sm text-muted-foreground">
        选择正式业务分类作为首页热门推荐，最多 {{ MAX_FEATURED }} 个，按排序展示。仅引用分类 ID，不复制分类数据。
      </p>
      <Button @click="openCreate" :disabled="items.length >= MAX_FEATURED">
        {{ items.length >= MAX_FEATURED ? '已达上限' : '添加推荐' }}
      </Button>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[800px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-4 py-3">排序</TableHead>
            <TableHead class="px-4 py-3">分类</TableHead>
            <TableHead class="px-4 py-3">首页别名</TableHead>
            <TableHead class="px-4 py-3">图标覆盖</TableHead>
            <TableHead class="px-4 py-3">启用</TableHead>
            <TableHead class="px-4 py-3 text-right">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="6" class="p-0"><TableSkeleton :columns="6" :rows="5" /></TableCell>
          </TableRow>
          <TableRow v-else-if="items.length === 0">
            <TableCell colspan="6" class="px-4 py-10 text-center text-muted-foreground">
              暂无热门推荐，点击右上角「添加推荐」选择正式业务分类
            </TableCell>
          </TableRow>
          <TableRow v-for="(item, index) in items" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-4 py-3">
              <div class="flex items-center gap-1">
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === 0" @click="move(index, -1)">↑</Button>
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === items.length - 1" @click="move(index, 1)">↓</Button>
                <span class="ml-1 text-xs text-muted-foreground">{{ item.sort_order }}</span>
              </div>
            </TableCell>
            <TableCell class="px-4 py-3">
              <div class="flex items-center gap-2">
                <img
                  v-if="categoryByID(item.category_id)?.icon"
                  :src="getImageUrl(categoryByID(item.category_id)!.icon)"
                  alt=""
                  class="h-8 w-8 rounded object-cover"
                />
                <div>
                  <div class="font-medium">{{ categoryByID(item.category_id) ? categoryName(categoryByID(item.category_id)!) : '#' + item.category_id }}</div>
                  <div class="text-[10px] text-muted-foreground/70">ID: {{ item.category_id }}</div>
                </div>
              </div>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs">{{ item.alias || '—' }}</TableCell>
            <TableCell class="px-4 py-3">
              <img v-if="item.icon_override" :src="getImageUrl(item.icon_override)" alt="" class="h-8 w-8 rounded object-cover" />
              <span v-else class="text-xs text-muted-foreground">—</span>
            </TableCell>
            <TableCell class="px-4 py-3"><Switch :checked="item.enabled" @update:checked="handleToggle(item)" /></TableCell>
            <TableCell class="px-4 py-3 text-right">
              <div class="flex justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEdit(item)">编辑</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(item)">移除</Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <Dialog v-model:open="showModal" @update:open="(v) => { if (!v) closeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6">
        <DialogHeader><DialogTitle>{{ isEditing ? '编辑热门推荐' : '添加热门推荐' }}</DialogTitle></DialogHeader>

        <form class="space-y-5" @submit.prevent="submit">
          <div class="space-y-2">
            <Label class="text-xs font-medium text-muted-foreground">业务分类 *</Label>
            <Select v-model="form.category_id" :disabled="categoriesLoading">
              <SelectTrigger class="h-9 w-full"><SelectValue placeholder="选择已启用的正式业务分类" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="cat in categories" :key="cat.id" :value="cat.id">
                  {{ categoryName(cat) }} · {{ cat.slug }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p v-if="categoriesLoading" class="text-[10px] text-muted-foreground">分类加载中…</p>
          </div>

          <div class="space-y-2">
            <Label class="text-xs font-medium text-muted-foreground">首页别名（可选，覆盖分类原名称）</Label>
            <Input v-model="form.alias" placeholder="留空则使用分类原名称" />
          </div>

          <div class="space-y-2">
            <Label class="text-xs font-medium text-muted-foreground">首页图标覆盖（可选，覆盖分类原图标）</Label>
            <MediaPicker v-model="form.icon_override" scene="common" />
          </div>

          <div class="flex items-center gap-6">
            <Label class="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
              <Switch v-model="form.enabled" /> 启用
            </Label>
            <div class="flex items-center gap-2">
              <Label class="text-xs text-muted-foreground">排序权重</Label>
              <Input v-model.number="form.sort_order" type="number" class="w-24 h-9" />
            </div>
          </div>

          <p v-if="formError" class="text-xs text-destructive">{{ formError }}</p>

          <div class="flex justify-end gap-3 border-t border-border pt-5">
            <Button type="button" variant="outline" @click="closeModal">取消</Button>
            <Button type="submit" :disabled="submitting">{{ submitting ? '保存中…' : '保存' }}</Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
