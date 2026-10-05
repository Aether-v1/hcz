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

const emptyForm = (): HomeEntry => ({
  key: '',
  title: '',
  subtitle: '',
  icon: 'recharge',
  action_type: 'internal',
  action_target: 'recharge',
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
  formError.value = ''
  showModal.value = true
}

const openEdit = (entry: HomeEntry) => {
  isEditing.value = true
  Object.assign(form, {
    id: entry.id,
    key: entry.key || '',
    title: entry.title || '',
    subtitle: entry.subtitle || '',
    icon: entry.icon || 'recharge',
    action_type: (entry.action_type === 'external' ? 'external' : 'internal') as 'internal' | 'external',
    action_target: entry.action_target || '',
    badge: entry.badge || '',
    recommended: Boolean(entry.recommended),
    enabled: Boolean(entry.enabled),
    sort_order: Number(entry.sort_order || 0),
  })
  formError.value = ''
  showModal.value = true
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
      <p class="text-sm text-muted-foreground">配置前台首页九宫格/功能入口，支持排序与启停。</p>
      <Button @click="openCreate">新建入口</Button>
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
            <TableCell class="px-4 py-3 text-xs">{{ entry.icon }}</TableCell>
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
                  <input type="radio" value="internal" v-model="form.action_type" /> 内部路由
                </Label>
                <Label class="flex items-center gap-2 text-sm cursor-pointer">
                  <input type="radio" value="external" v-model="form.action_type" /> 外部链接
                </Label>
              </div>
            </div>

            <div class="space-y-2 md:col-span-2">
              <Label class="text-xs font-medium text-muted-foreground">跳转目标</Label>
              <Select v-if="form.action_type === 'internal'" v-model="form.action_target">
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
