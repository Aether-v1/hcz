<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
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
import type { AdminProduct } from '@/api/types'
import { getImageUrl } from '@/utils/image'
import {
  getHomeEntries,
  createHomeEntry,
  updateHomeEntry,
  deleteHomeEntry,
  toggleHomeEntry,
  reorderHomeEntries,
  HOME_ENTRY_ICONS,
  HOME_ENTRY_ROUTES,
  isHttpUrl,
  type HomeEntry,
} from '@/api/site-builder'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'

const loading = ref(false)
const submitting = ref(false)
const showModal = ref(false)
const isEditing = ref(false)

const entries = ref<HomeEntry[]>([])
const publishedProducts = ref<AdminProduct[]>([])
const productSearch = ref('')
const productPage = ref(1)
const productTotalPages = ref(1)
const productsLoading = ref(false)

const fetchPublishedProducts = async (page = 1) => {
  productsLoading.value = true
  try {
    const res = await adminAPI.getProducts({ page, page_size: 20, is_active: 1, search: productSearch.value || undefined })
    publishedProducts.value = (res.data.data || []).filter((product: AdminProduct) => product.is_active && product.slug)
    productPage.value = page
    productTotalPages.value = res.data.pagination?.total_page || 1
  } catch {
    publishedProducts.value = []
    notifyError('已上架商品加载失败')
  } finally {
    productsLoading.value = false
  }
}
watch(productSearch, useDebounceFn(() => { void fetchPublishedProducts(1) }, 300))

const emptyForm = (): HomeEntry => ({
  key: '',
  title: '',
  subtitle: '',
  icon: 'recharge',
  image: '',
  action_type: 'product',
  action_target: '',
  badge: '',
  recommended: false,
  enabled: true,
  sort_order: 0,
})

const form = reactive<HomeEntry>(emptyForm())
const formError = ref('')

const fetchEntries = async () => {
  loading.value = true
  try {
    const res = await getHomeEntries()
    const list = (res.data?.data || []) as HomeEntry[]
    entries.value = [...list].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
  } catch {
    entries.value = []
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  isEditing.value = false
  Object.assign(form, emptyForm())
  productSearch.value = ''
  formError.value = ''
  showModal.value = true
  void fetchPublishedProducts(1)
}

const openEdit = (entry: HomeEntry) => {
  isEditing.value = true
  productSearch.value = ''
  Object.assign(form, {
    id: entry.id,
    key: entry.key || '',
    title: entry.title || '',
    subtitle: entry.subtitle || '',
    icon: entry.icon || 'recharge',
    image: entry.image || '',
    action_type: entry.action_type === 'product' ? 'product' : entry.action_type === 'external' ? 'external' : 'internal',
    action_target: entry.action_target || '',
    badge: entry.badge || '',
    recommended: Boolean(entry.recommended),
    enabled: Boolean(entry.enabled),
    sort_order: Number(entry.sort_order || 0),
  })
  formError.value = ''
  showModal.value = true
  void fetchPublishedProducts(1)
}

const closeModal = () => {
  showModal.value = false
}

const validate = (): boolean => {
  formError.value = ''
  if (!form.title.trim()) {
    formError.value = '标题必填'
    return false
  }
  if (!form.key.trim()) {
    formError.value = '标识 key 必填（如 entry_recharge）'
    return false
  }
  if (form.action_type === 'external' && !isHttpUrl(form.action_target)) {
    formError.value = '外部链接必须以 http:// 或 https:// 开头'
    return false
  }
  if (form.action_type === 'product' && !form.action_target.trim()) {
    formError.value = '请选择已上架商品'
    return false
  }
  return true
}

const submit = async () => {
  if (!validate()) return
  submitting.value = true
  try {
    if (isEditing.value && form.id) {
      await updateHomeEntry(form.id, { ...form })
    } else {
      await createHomeEntry({ ...form })
    }
    showModal.value = false
    notifySuccess('已保存')
    fetchEntries()
  } catch {
    notifyError('保存失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (entry: HomeEntry) => {
  const ok = await confirmAction({
    description: `确认删除入口「${entry.title || entry.key}」？此操作不可撤销。`,
    confirmText: '删除',
    variant: 'destructive',
  })
  if (!ok || !entry.id) return
  try {
    await deleteHomeEntry(entry.id)
    notifySuccess('已删除')
    fetchEntries()
  } catch {
    notifyError('删除失败')
  }
}

const handleToggle = async (entry: HomeEntry) => {
  if (!entry.id) return
  // 乐观更新
  entry.enabled = !entry.enabled
  try {
    await toggleHomeEntry(entry.id)
  } catch {
    entry.enabled = !entry.enabled
    notifyError('操作失败')
  }
}

const move = async (index: number, direction: -1 | 1) => {
  const target = index + direction
  if (target < 0 || target >= entries.value.length) return
  const current = entries.value[index]
  const next = entries.value[target]
  if (!current?.id || !next?.id) return
  const tmp = entries.value[index]!
  entries.value[index] = entries.value[target]!
  entries.value[target] = tmp
  try {
    await reorderHomeEntries(entries.value.map((e) => e.id!).filter((x) => typeof x === 'number'))
  } catch {
    const tmp2 = entries.value[target]!
    entries.value[target] = entries.value[index]!
    entries.value[index] = tmp2
    notifyError('排序失败')
  }
}

onMounted(fetchEntries)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <p class="text-sm text-muted-foreground">手动配置首页核心入口。首页按排序展示前四个启用入口，最多配置 4 个。</p>
      <Button @click="openCreate" :disabled="entries.length >= 4">
        {{ entries.length >= 4 ? '已达上限（4个）' : '新建入口' }}
      </Button>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[960px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-4 py-3">排序</TableHead>
            <TableHead class="px-4 py-3">图标</TableHead>
            <TableHead class="px-4 py-3">标题</TableHead>
            <TableHead class="px-4 py-3">动作</TableHead>
            <TableHead class="px-4 py-3">角标</TableHead>
            <TableHead class="px-4 py-3">推荐</TableHead>
            <TableHead class="px-4 py-3">启用</TableHead>
            <TableHead class="px-4 py-3 text-right">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="8" class="p-0"><TableSkeleton :columns="8" :rows="5" /></TableCell>
          </TableRow>
          <TableRow v-else-if="entries.length === 0">
            <TableCell colspan="8" class="px-4 py-10 text-center text-muted-foreground">暂无入口，点击右上角「新建入口」创建</TableCell>
          </TableRow>
          <TableRow v-for="(entry, index) in entries" :key="entry.id" class="hover:bg-muted/30">
            <TableCell class="px-4 py-3">
              <div class="flex items-center gap-1">
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === 0" @click="move(index, -1)">↑</Button>
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === entries.length - 1" @click="move(index, 1)">↓</Button>
                <span class="ml-1 text-xs text-muted-foreground">{{ entry.sort_order }}</span>
              </div>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs"><img v-if="entry.image" :src="getImageUrl(entry.image)" alt="" class="h-9 w-9 rounded object-cover" /><span v-else>{{ entry.icon }}</span></TableCell>
            <TableCell class="px-4 py-3">
              <div class="font-medium">{{ entry.title }}</div>
              <div class="text-xs text-muted-foreground">{{ entry.subtitle }}</div>
              <div class="text-[10px] text-muted-foreground/70">key: {{ entry.key }}</div>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs">
              <div>{{ entry.action_type === 'external' ? '外链' : '内部路由' }}</div>
              <div class="text-muted-foreground">{{ entry.action_target }}</div>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs">{{ entry.badge || '—' }}</TableCell>
            <TableCell class="px-4 py-3">
              <span v-if="entry.recommended" class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] text-amber-700">推荐</span>
              <span v-else class="text-xs text-muted-foreground">—</span>
            </TableCell>
            <TableCell class="px-4 py-3"><Switch :checked="entry.enabled" @update:checked="handleToggle(entry)" /></TableCell>
            <TableCell class="px-4 py-3 text-right">
              <div class="flex justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEdit(entry)">编辑</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(entry)">删除</Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <Dialog v-model:open="showModal" @update:open="(v) => { if (!v) closeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-2xl p-4 sm:p-6">
        <DialogHeader><DialogTitle>{{ isEditing ? '编辑入口' : '新建入口' }}</DialogTitle></DialogHeader>

        <form class="space-y-5" @submit.prevent="submit">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">标识 key（唯一）</Label>
              <Input v-model="form.key" placeholder="entry_recharge" :disabled="isEditing" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">图标</Label>
              <Select v-model="form.icon">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="ic in HOME_ENTRY_ICONS" :key="ic" :value="ic">{{ ic }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="space-y-2 md:col-span-2">
              <Label class="text-xs font-medium text-muted-foreground">入口图片（单独配置，留空时使用图标）</Label>
              <MediaPicker v-model="form.image" scene="common" />
            </div>

            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">标题 *</Label>
              <Input v-model="form.title" placeholder="例如：充值" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">副标题</Label>
              <Input v-model="form.subtitle" placeholder="例如：快速到账" />
            </div>

            <div class="space-y-2 md:col-span-2">
              <Label class="text-xs font-medium text-muted-foreground">跳转类型</Label>
              <div class="flex gap-4">
                <Label class="flex items-center gap-2 text-sm cursor-pointer">
                  <input type="radio" value="product" v-model="form.action_type" @change="form.action_target = ''" /> 已上架商品
                </Label>
                <Label class="flex items-center gap-2 text-sm cursor-pointer">
                  <input type="radio" value="internal" v-model="form.action_type" @change="form.action_target = 'recharge'" /> 内部路由
                </Label>
                <Label class="flex items-center gap-2 text-sm cursor-pointer">
                  <input type="radio" value="external" v-model="form.action_type" @change="form.action_target = ''" /> 外部链接
                </Label>
              </div>
            </div>

            <div class="space-y-2 md:col-span-2">
              <Label class="text-xs font-medium text-muted-foreground">跳转目标</Label>
              <div v-if="form.action_type === 'product'" class="space-y-2">
                <Input v-model="productSearch" placeholder="搜索已上架商品" />
                <div class="max-h-44 overflow-y-auto rounded border border-border">
                  <button v-for="product in publishedProducts" :key="product.id" type="button" class="block w-full px-3 py-2 text-left text-sm hover:bg-muted" :class="{ 'bg-muted font-semibold': form.action_target === product.slug }" @click="form.action_target = product.slug">
                    {{ product.title?.['zh-CN'] || product.title?.['en-US'] || product.slug }} · {{ product.slug }}
                  </button>
                  <p v-if="!productsLoading && !publishedProducts.length" class="px-3 py-3 text-sm text-muted-foreground">没有符合条件的已上架商品</p>
                </div>
                <p v-if="form.action_target" class="text-xs text-muted-foreground">已选择：{{ form.action_target }}</p>
                <div v-if="productTotalPages > 1" class="flex items-center gap-2 text-xs"><Button type="button" size="sm" variant="outline" :disabled="productPage <= 1 || productsLoading" @click="fetchPublishedProducts(productPage - 1)">上一页</Button><span>{{ productPage }} / {{ productTotalPages }}</span><Button type="button" size="sm" variant="outline" :disabled="productPage >= productTotalPages || productsLoading" @click="fetchPublishedProducts(productPage + 1)">下一页</Button></div>
              </div>
              <Select v-else-if="form.action_type === 'internal'" v-model="form.action_target">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="r in HOME_ENTRY_ROUTES" :key="r" :value="r">{{ r }}</SelectItem>
                </SelectContent>
              </Select>
              <Input v-else v-model="form.action_target" placeholder="https://example.com/page" />
            </div>

            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">角标（如 新 / 热）</Label>
              <Input v-model="form.badge" placeholder="可留空" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">排序权重</Label>
              <Input v-model.number="form.sort_order" type="number" />
            </div>

            <div class="flex gap-6">
              <Label class="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
                <Switch v-model="form.recommended" /> 推荐
              </Label>
              <Label class="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
                <Switch v-model="form.enabled" /> 启用
              </Label>
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
