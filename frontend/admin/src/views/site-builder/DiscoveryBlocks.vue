<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import {
  getDiscoveryBlocks,
  createDiscoveryBlock,
  updateDiscoveryBlock,
  deleteDiscoveryBlock,
  toggleDiscoveryBlock,
  reorderDiscoveryBlocks,
  DISCOVERY_BLOCK_TYPES,
  HOME_ENTRY_ICONS,
  HOME_ENTRY_ROUTES,
  isHttpUrl,
  type DiscoveryBlock,
  type DiscoveryBlockType,
} from '@/api/site-builder'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'

const BLOCK_TYPE_LABELS: Record<DiscoveryBlockType, string> = {
  banner: '横幅 Banner',
  card_grid: '卡片网格',
  business_recommend: '业务推荐',
  announcement: '公告（全局）',
  external_link: '外部链接',
  category_entry: '分类入口',
}

interface ActionLink {
  action_type: 'internal' | 'external'
  action_target: string
}

const emptyActionLink = (): ActionLink => ({ action_type: 'internal', action_target: HOME_ENTRY_ROUTES[0] })

const emptyConfig = (type: DiscoveryBlockType): Record<string, any> => {
  switch (type) {
    case 'banner':
      return { image: '', title: '', subtitle: '', ...emptyActionLink() }
    case 'card_grid':
      return { cards: [] as any[] }
    case 'business_recommend':
      return { business_keys: [] as string[] }
    case 'announcement':
      return {}
    case 'external_link':
      return { title: '', subtitle: '', image: '', url: '' }
    case 'category_entry':
      return { entries: [] as any[] }
  }
}

const loading = ref(false)
const submitting = ref(false)
const showModal = ref(false)
const isEditing = ref(false)
const formError = ref('')

const blocks = ref<DiscoveryBlock[]>([])

const form = reactive({
  id: 0 as number,
  type: 'banner' as DiscoveryBlockType,
  title: '',
  enabled: true,
  sort_order: 0,
  config: {} as Record<string, any>,
})

const fetchBlocks = async () => {
  loading.value = true
  try {
    const res = await getDiscoveryBlocks()
    const list = (res.data?.data || []) as DiscoveryBlock[]
    blocks.value = [...list].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
  } catch {
    blocks.value = []
  } finally {
    loading.value = false
  }
}

const resetForm = (type: DiscoveryBlockType) => {
  form.id = 0
  form.type = type
  form.title = ''
  form.enabled = true
  form.sort_order = 0
  form.config = emptyConfig(type)
}

const openCreate = () => {
  isEditing.value = false
  formError.value = ''
  resetForm('banner')
  showModal.value = true
}

const openEdit = (block: DiscoveryBlock) => {
  isEditing.value = true
  formError.value = ''
  form.id = Number(block.id)
  form.type = block.type
  form.title = block.title || ''
  form.enabled = Boolean(block.enabled)
  form.sort_order = Number(block.sort_order || 0)
  form.config = JSON.parse(JSON.stringify(block.config || emptyConfig(block.type)))
  // 保证 config 结构完整
  const base = emptyConfig(block.type)
  form.config = { ...base, ...form.config }
  if (block.type === 'card_grid' && !Array.isArray(form.config.cards)) form.config.cards = []
  if (block.type === 'category_entry' && !Array.isArray(form.config.entries)) form.config.entries = []
  if (block.type === 'business_recommend' && !Array.isArray(form.config.business_keys)) form.config.business_keys = []
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
}

const changeType = (type: DiscoveryBlockType) => {
  if (isEditing.value) return
  resetForm(type)
}

// ── card_grid / category_entry 动态数组操作 ──
const addCard = () => {
  form.config.cards.push({ image: '', title: '', description: '', ...emptyActionLink() })
}
const removeCard = (i: number) => form.config.cards.splice(i, 1)
const addEntry = () => {
  form.config.entries.push({ name: '', route: HOME_ENTRY_ROUTES[0], icon: HOME_ENTRY_ICONS[0] })
}
const removeEntry = (i: number) => form.config.entries.splice(i, 1)

const toggleBusinessKey = (key: string) => {
  const arr = form.config.business_keys as string[]
  const idx = arr.indexOf(key)
  if (idx >= 0) arr.splice(idx, 1)
  else arr.push(key)
}

const validate = (): boolean => {
  formError.value = ''
  if (!form.title.trim()) {
    formError.value = '区块标题必填'
    return false
  }
  const cfg = form.config
  const checkUrl = (v: string, label: string) => {
    if (v && !isHttpUrl(v)) {
      formError.value = `${label}：外链必须以 http:// 或 https:// 开头`
      return false
    }
    return true
  }
  if (form.type === 'banner') {
    if (cfg.action_type === 'external' && !checkUrl(cfg.action_target, '横幅跳转')) return false
  }
  if (form.type === 'card_grid') {
    for (let i = 0; i < (cfg.cards || []).length; i++) {
      const c = cfg.cards[i]
      if (c.action_type === 'external' && !checkUrl(c.action_target, `卡片 ${i + 1} 跳转`)) return false
    }
  }
  if (form.type === 'external_link') {
    if (!checkUrl(cfg.url, '链接地址')) return false
  }
  return true
}

const submit = async () => {
  if (!validate()) return
  submitting.value = true
  try {
    const payload: DiscoveryBlock = {
      type: form.type,
      title: form.title,
      enabled: form.enabled,
      sort_order: Number(form.sort_order || 0),
      config: form.config,
    }
    if (isEditing.value && form.id) {
      await updateDiscoveryBlock(form.id, payload)
    } else {
      await createDiscoveryBlock(payload)
    }
    showModal.value = false
    notifySuccess('已保存')
    fetchBlocks()
  } catch {
    notifyError('保存失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (block: DiscoveryBlock) => {
  const ok = await confirmAction({
    description: `确认删除区块「${block.title || block.type}」？此操作不可撤销。`,
    confirmText: '删除',
    variant: 'destructive',
  })
  if (!ok || !block.id) return
  try {
    await deleteDiscoveryBlock(block.id)
    notifySuccess('已删除')
    fetchBlocks()
  } catch {
    notifyError('删除失败')
  }
}

const handleToggle = async (block: DiscoveryBlock) => {
  if (!block.id) return
  block.enabled = !block.enabled
  try {
    await toggleDiscoveryBlock(block.id)
  } catch {
    block.enabled = !block.enabled
    notifyError('操作失败')
  }
}

const swap = (a: number, b: number) => {
  const t = blocks.value[a]!
  blocks.value[a] = blocks.value[b]!
  blocks.value[b] = t
}

const move = async (index: number, direction: -1 | 1) => {
  const target = index + direction
  if (target < 0 || target >= blocks.value.length) return
  swap(index, target)
  try {
    await reorderDiscoveryBlocks(blocks.value.map((b) => Number(b.id)).filter((x) => Number.isFinite(x)))
  } catch {
    swap(target, index)
    notifyError('排序失败')
  }
}

onMounted(fetchBlocks)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <p class="text-sm text-muted-foreground">配置前台「发现」页自上而下的区块，全部通过表单维护，不暴露原始 JSON。</p>
      <Button @click="openCreate">新建区块</Button>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[860px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-4 py-3">排序</TableHead>
            <TableHead class="px-4 py-3">类型</TableHead>
            <TableHead class="px-4 py-3">标题</TableHead>
            <TableHead class="px-4 py-3">启用</TableHead>
            <TableHead class="px-4 py-3 text-right">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="5" class="p-0"><TableSkeleton :columns="5" :rows="5" /></TableCell>
          </TableRow>
          <TableRow v-else-if="blocks.length === 0">
            <TableCell colspan="5" class="px-4 py-10 text-center text-muted-foreground">暂无区块</TableCell>
          </TableRow>
          <TableRow v-for="(block, index) in blocks" :key="block.id" class="hover:bg-muted/30">
            <TableCell class="px-4 py-3">
              <div class="flex items-center gap-1">
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === 0" @click="move(index, -1)">↑</Button>
                <Button size="icon-sm" variant="ghost" class="h-6 w-6" :disabled="index === blocks.length - 1" @click="move(index, 1)">↓</Button>
              </div>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs">{{ BLOCK_TYPE_LABELS[block.type] }}</TableCell>
            <TableCell class="px-4 py-3 font-medium">{{ block.title }}</TableCell>
            <TableCell class="px-4 py-3"><Switch :checked="block.enabled" @update:checked="handleToggle(block)" /></TableCell>
            <TableCell class="px-4 py-3 text-right">
              <div class="flex justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEdit(block)">编辑</Button>
                <Button size="sm" variant="destructive" @click="handleDelete(block)">删除</Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <Dialog v-model:open="showModal" @update:open="(v) => { if (!v) closeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-3xl p-4 sm:p-6">
        <DialogHeader><DialogTitle>{{ isEditing ? '编辑区块' : '新建区块' }}</DialogTitle></DialogHeader>

        <form class="space-y-5" @submit.prevent="submit">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">区块类型</Label>
              <Select :model-value="form.type" @update:model-value="(v) => changeType(v as DiscoveryBlockType)" :disabled="isEditing">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="t in DISCOVERY_BLOCK_TYPES" :key="t" :value="t">{{ BLOCK_TYPE_LABELS[t] }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">区块标题 *</Label>
              <Input v-model="form.title" placeholder="展示在区块顶部" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">排序权重</Label>
              <Input v-model.number="form.sort_order" type="number" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">启用</Label>
              <div>
                <Switch v-model="form.enabled" />
              </div>
            </div>
          </div>

          <!-- banner -->
          <div v-if="form.type === 'banner'" class="space-y-4 rounded-lg border border-border p-4">
            <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">图片</Label><MediaPicker v-model="form.config.image" scene="banner" /></div>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">标题</Label><Input v-model="form.config.title" /></div>
              <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">副标题</Label><Input v-model="form.config.subtitle" /></div>
            </div>
            <div class="flex gap-4">
              <Label class="flex items-center gap-2 text-sm cursor-pointer"><input type="radio" value="internal" v-model="form.config.action_type" /> 内部路由</Label>
              <Label class="flex items-center gap-2 text-sm cursor-pointer"><input type="radio" value="external" v-model="form.config.action_type" /> 外部链接</Label>
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">跳转目标</Label>
              <Select v-if="form.config.action_type === 'internal'" v-model="form.config.action_target">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem v-for="r in HOME_ENTRY_ROUTES" :key="r" :value="r">{{ r }}</SelectItem></SelectContent>
              </Select>
              <Input v-else v-model="form.config.action_target" placeholder="https://example.com" />
            </div>
          </div>

          <!-- card_grid -->
          <div v-else-if="form.type === 'card_grid'" class="space-y-4 rounded-lg border border-border p-4">
            <div v-for="(card, i) in form.config.cards" :key="i" class="space-y-3 rounded-lg border border-border bg-muted/10 p-3">
              <div class="flex justify-between"><span class="text-sm font-medium">卡片 {{ Number(i) + 1 }}</span><Button size="sm" variant="destructive" type="button" @click="removeCard(Number(i))">移除</Button></div>
              <MediaPicker v-model="card.image" scene="common" />
              <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Input v-model="card.title" placeholder="标题" />
                <Input v-model="card.description" placeholder="描述" />
              </div>
              <div class="flex gap-4">
                <Label class="flex items-center gap-2 text-sm cursor-pointer"><input type="radio" value="internal" v-model="card.action_type" /> 内部</Label>
                <Label class="flex items-center gap-2 text-sm cursor-pointer"><input type="radio" value="external" v-model="card.action_type" /> 外链</Label>
              </div>
              <Select v-if="card.action_type === 'internal'" v-model="card.action_target">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem v-for="r in HOME_ENTRY_ROUTES" :key="r" :value="r">{{ r }}</SelectItem></SelectContent>
              </Select>
              <Input v-else v-model="card.action_target" placeholder="https://example.com" />
            </div>
            <Button type="button" variant="outline" size="sm" @click="addCard">+ 添加卡片</Button>
          </div>

          <!-- business_recommend -->
          <div v-else-if="form.type === 'business_recommend'" class="space-y-3 rounded-lg border border-border p-4">
            <Label class="text-xs font-medium text-muted-foreground">选择要推荐的业务（多选）</Label>
            <div class="flex flex-wrap gap-3">
              <Label v-for="key in HOME_ENTRY_ROUTES" :key="key" class="flex items-center gap-2 rounded border border-border px-3 py-1.5 text-sm cursor-pointer" :class="form.config.business_keys.includes(key) ? 'border-primary bg-primary/5 text-primary' : ''">
                <input type="checkbox" :checked="form.config.business_keys.includes(key)" @change="toggleBusinessKey(key)" />
                {{ key }}
              </Label>
            </div>
          </div>

          <!-- announcement -->
          <div v-else-if="form.type === 'announcement'" class="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
            此区块直接使用「Banner / 公告」Tab 中配置的全局首页公告，无需额外字段。
          </div>

          <!-- external_link -->
          <div v-else-if="form.type === 'external_link'" class="space-y-4 rounded-lg border border-border p-4">
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">标题</Label><Input v-model="form.config.title" /></div>
              <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">副标题</Label><Input v-model="form.config.subtitle" /></div>
            </div>
            <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">图片</Label><MediaPicker v-model="form.config.image" scene="common" /></div>
            <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">链接地址 (http/https)</Label><Input v-model="form.config.url" placeholder="https://example.com" /></div>
          </div>

          <!-- category_entry -->
          <div v-else-if="form.type === 'category_entry'" class="space-y-4 rounded-lg border border-border p-4">
            <div v-for="(entry, i) in form.config.entries" :key="i" class="space-y-3 rounded-lg border border-border bg-muted/10 p-3">
              <div class="flex justify-between"><span class="text-sm font-medium">入口 {{ Number(i) + 1 }}</span><Button size="sm" variant="destructive" type="button" @click="removeEntry(Number(i))">移除</Button></div>
              <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
                <div class="space-y-1"><Label class="text-xs text-muted-foreground">名称</Label><Input v-model="entry.name" /></div>
                <div class="space-y-1">
                  <Label class="text-xs text-muted-foreground">路由</Label>
                  <Select v-model="entry.route">
                    <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="r in HOME_ENTRY_ROUTES" :key="r" :value="r">{{ r }}</SelectItem></SelectContent>
                  </Select>
                </div>
                <div class="space-y-1">
                  <Label class="text-xs text-muted-foreground">图标</Label>
                  <Select v-model="entry.icon">
                    <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="ic in HOME_ENTRY_ICONS" :key="ic" :value="ic">{{ ic }}</SelectItem></SelectContent>
                  </Select>
                </div>
              </div>
            </div>
            <Button type="button" variant="outline" size="sm" @click="addEntry">+ 添加入口</Button>
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
