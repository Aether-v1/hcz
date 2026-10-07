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
import { adminAPI } from '@/api/admin'
import type { AdminProduct, AdminCategory } from '@/api/types'
import { getLocalizedText } from '@/utils/format'
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

// ── 各类型 config 的初始值，严格对齐后端 discovery_schema.go ──
// banner: { image, link_type(none/internal/external), link_value, open_in_new_tab }
const emptyBannerConfig = () => ({
  image: '',
  link_type: 'none' as 'none' | 'internal' | 'external',
  link_value: '',
  open_in_new_tab: false,
})

// card_grid: { cards: [{ title, subtitle, image, action_type(internal/external), action_target }] }
interface CardGridCard {
  title: string
  subtitle: string
  image: string
  action_type: 'internal' | 'external'
  action_target: string
}
const emptyCard = (): CardGridCard => ({
  title: '',
  subtitle: '',
  image: '',
  action_type: 'internal',
  action_target: HOME_ENTRY_ROUTES[0],
})
const emptyCardGridConfig = () => ({ cards: [emptyCard()] as CardGridCard[] })

// business_recommend: { title, product_ids[]uint }
const emptyBusinessRecommendConfig = () => ({ title: '', product_ids: [] as number[] })

// announcement: { text, link_type, link_value }
const emptyAnnouncementConfig = () => ({
  text: '',
  link_type: 'none' as 'none' | 'internal' | 'external',
  link_value: '',
})

// external_link: { label, url, icon }
const emptyExternalLinkConfig = () => ({ label: '', url: '', icon: '' })

// category_entry: { category_ids[]uint }
const emptyCategoryEntryConfig = () => ({ category_ids: [] as number[] })

const emptyConfig = (type: DiscoveryBlockType): Record<string, any> => {
  switch (type) {
    case 'banner':
      return emptyBannerConfig()
    case 'card_grid':
      return emptyCardGridConfig()
    case 'business_recommend':
      return emptyBusinessRecommendConfig()
    case 'announcement':
      return emptyAnnouncementConfig()
    case 'external_link':
      return emptyExternalLinkConfig()
    case 'category_entry':
      return emptyCategoryEntryConfig()
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
  // 以空配置为基底合并后端返回的 config，保证字段完整、类型正确
  const base = emptyConfig(block.type)
  form.config = { ...base, ...JSON.parse(JSON.stringify(block.config || {})) }
  if (block.type === 'card_grid') {
    if (!Array.isArray(form.config.cards) || form.config.cards.length === 0) form.config.cards = [emptyCard()]
    form.config.cards = (form.config.cards as any[]).map((c) => ({ ...emptyCard(), ...c }))
  }
  if (block.type === 'business_recommend') {
    if (!Array.isArray(form.config.product_ids)) form.config.product_ids = []
  }
  if (block.type === 'category_entry') {
    if (!Array.isArray(form.config.category_ids)) form.config.category_ids = []
  }
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
}

const changeType = (type: DiscoveryBlockType) => {
  if (isEditing.value) return
  resetForm(type)
}

// ── 商品 / 分类选项（供 business_recommend / category_entry 多选） ──
const productOptions = ref<AdminProduct[]>([])
const categoryOptions = ref<AdminCategory[]>([])

const fetchOptions = async () => {
  const [pRes, cRes] = await Promise.allSettled([
    adminAPI.getProducts({ page: 1, page_size: 100, is_active: 1 }),
    adminAPI.getCategories(),
  ])
  if (pRes.status === 'fulfilled') {
    productOptions.value = ((pRes.value.data?.data || []) as AdminProduct[]).filter(
      (p) => p.is_active && p.slug,
    )
  }
  if (cRes.status === 'fulfilled') {
    categoryOptions.value = ((cRes.value.data?.data || []) as AdminCategory[]).filter((c) => c.is_active)
  }
}

// ── card_grid 动态卡片 ──
const addCard = () => {
  if (((form.config.cards as any[]) || []).length >= 12) return
  ;(form.config.cards as any[]).push(emptyCard())
}
const removeCard = (i: number) => (form.config.cards as any[]).splice(i, 1)

// ── business_recommend / category_entry 多选 ──
const toggleProduct = (id: number) => {
  const arr = form.config.product_ids as number[]
  const idx = arr.indexOf(id)
  if (idx >= 0) arr.splice(idx, 1)
  else if (arr.length < 20) arr.push(id)
}
const toggleCategory = (id: number) => {
  const arr = form.config.category_ids as number[]
  const idx = arr.indexOf(id)
  if (idx >= 0) arr.splice(idx, 1)
  else if (arr.length < 20) arr.push(id)
}

const validate = (): boolean => {
  formError.value = ''
  if (!form.title.trim()) {
    formError.value = '区块标题必填'
    return false
  }
  const cfg = form.config
  // link_type = internal/external 时的 link_value 校验（对齐后端 validateLink）
  const checkLink = (linkType: string, linkValue: string, label: string): boolean => {
    if (linkType === 'internal') {
      if (!String(linkValue || '').trim()) {
        formError.value = `${label}：内部路由必选`
        return false
      }
      return true
    }
    if (linkType === 'external') {
      if (!isHttpUrl(String(linkValue || ''))) {
        formError.value = `${label}：外链必须以 http:// 或 https:// 开头`
        return false
      }
      return true
    }
    return true
  }
  switch (form.type) {
    case 'banner':
      if (!String(cfg.image || '').trim()) {
        formError.value = '横幅图片必填'
        return false
      }
      return checkLink(String(cfg.link_type || 'none'), String(cfg.link_value || ''), '横幅跳转')
    case 'card_grid': {
      const cards = (Array.isArray(cfg.cards) ? cfg.cards : []) as CardGridCard[]
      if (cards.length === 0) {
        formError.value = '至少添加 1 张卡片'
        return false
      }
      for (let i = 0; i < cards.length; i++) {
        const c = cards[i]!
        if (!String(c.title || '').trim()) {
          formError.value = `卡片 ${i + 1}：标题必填`
          return false
        }
        if (c.action_type === 'external' && !isHttpUrl(String(c.action_target || ''))) {
          formError.value = `卡片 ${i + 1}：外链必须以 http:// 或 https:// 开头`
          return false
        }
      }
      return true
    }
    case 'business_recommend':
      if (!Array.isArray(cfg.product_ids) || (cfg.product_ids as number[]).length === 0) {
        formError.value = '请至少选择 1 个推荐商品'
        return false
      }
      return true
    case 'announcement':
      if (!String(cfg.text || '').trim()) {
        formError.value = '公告文本必填'
        return false
      }
      return checkLink(String(cfg.link_type || 'none'), String(cfg.link_value || ''), '公告链接')
    case 'external_link':
      if (!String(cfg.label || '').trim()) {
        formError.value = '链接名称必填'
        return false
      }
      if (!isHttpUrl(String(cfg.url || ''))) {
        formError.value = '链接地址必须以 http:// 或 https:// 开头'
        return false
      }
      return true
    case 'category_entry':
      if (!Array.isArray(cfg.category_ids) || (cfg.category_ids as number[]).length === 0) {
        formError.value = '请至少选择 1 个分类'
        return false
      }
      return true
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

onMounted(() => {
  void fetchBlocks()
  void fetchOptions()
})
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

          <!-- banner: { image, link_type, link_value, open_in_new_tab } -->
          <div v-if="form.type === 'banner'" class="space-y-4 rounded-lg border border-border p-4">
            <div class="space-y-2"><Label class="text-xs font-medium text-muted-foreground">横幅图片 *</Label><MediaPicker v-model="form.config.image" scene="banner" /></div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">跳转类型</Label>
              <Select v-model="form.config.link_type">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">不跳转</SelectItem>
                  <SelectItem value="internal">内部路由</SelectItem>
                  <SelectItem value="external">外部链接</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div v-if="form.config.link_type === 'internal'" class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">内部路由 *</Label>
              <Select v-model="form.config.link_value">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem v-for="r in HOME_ENTRY_ROUTES" :key="r" :value="r">{{ r }}</SelectItem></SelectContent>
              </Select>
            </div>
            <div v-else-if="form.config.link_type === 'external'" class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">外部链接 *</Label>
              <Input v-model="form.config.link_value" placeholder="https://example.com" />
            </div>
            <div v-if="form.config.link_type && form.config.link_type !== 'none'" class="flex items-center gap-2">
              <Switch v-model="form.config.open_in_new_tab" />
              <Label class="text-sm font-normal">在新标签页打开</Label>
            </div>
          </div>

          <!-- card_grid: { cards: [{ title, subtitle, image, action_type, action_target }] } -->
          <div v-else-if="form.type === 'card_grid'" class="space-y-4 rounded-lg border border-border p-4">
            <div v-for="(card, i) in form.config.cards" :key="i" class="space-y-3 rounded-lg border border-border bg-muted/10 p-3">
              <div class="flex justify-between"><span class="text-sm font-medium">卡片 {{ Number(i) + 1 }}</span><Button size="sm" variant="destructive" type="button" @click="removeCard(Number(i))">移除</Button></div>
              <MediaPicker v-model="card.image" scene="common" />
              <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Input v-model="card.title" placeholder="标题 *" />
                <Input v-model="card.subtitle" placeholder="副标题" />
              </div>
              <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
                <div class="space-y-1">
                  <Label class="text-xs text-muted-foreground">跳转类型</Label>
                  <Select v-model="card.action_type">
                    <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="internal">内部路由</SelectItem>
                      <SelectItem value="external">外部链接</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div class="space-y-1">
                  <Label class="text-xs text-muted-foreground">跳转目标</Label>
                  <Select v-if="card.action_type === 'internal'" v-model="card.action_target">
                    <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem v-for="r in HOME_ENTRY_ROUTES" :key="r" :value="r">{{ r }}</SelectItem></SelectContent>
                  </Select>
                  <Input v-else v-model="card.action_target" placeholder="https://example.com" />
                </div>
              </div>
            </div>
            <Button type="button" variant="outline" size="sm" :disabled="(form.config.cards || []).length >= 12" @click="addCard">+ 添加卡片</Button>
          </div>

          <!-- business_recommend: { title, product_ids[]uint } -->
          <div v-else-if="form.type === 'business_recommend'" class="space-y-3 rounded-lg border border-border p-4">
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">区块副文案（可选）</Label>
              <Input v-model="form.config.title" placeholder="留空则展示区块标题" />
            </div>
            <Label class="text-xs font-medium text-muted-foreground">选择推荐商品（可多选，最多 20）*</Label>
            <div class="flex max-h-60 flex-wrap gap-2 overflow-y-auto">
              <Label v-for="p in productOptions" :key="p.id" class="flex items-center gap-2 rounded border border-border px-3 py-1.5 text-sm cursor-pointer" :class="(form.config.product_ids || []).includes(p.id) ? 'border-primary bg-primary/5 text-primary' : ''">
                <input type="checkbox" :checked="(form.config.product_ids || []).includes(p.id)" @change="toggleProduct(p.id)" />
                {{ getLocalizedText(p.title) }} (#{{ p.id }})
              </Label>
            </div>
          </div>

          <!-- announcement: { text, link_type, link_value } -->
          <div v-else-if="form.type === 'announcement'" class="space-y-4 rounded-lg border border-border p-4">
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">公告文本 *</Label>
              <Input v-model="form.config.text" placeholder="例如：全站充值限时 9 折" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">跳转类型</Label>
              <Select v-model="form.config.link_type">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">不跳转</SelectItem>
                  <SelectItem value="internal">内部路由</SelectItem>
                  <SelectItem value="external">外部链接</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div v-if="form.config.link_type === 'internal'" class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">内部路由 *</Label>
              <Select v-model="form.config.link_value">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem v-for="r in HOME_ENTRY_ROUTES" :key="r" :value="r">{{ r }}</SelectItem></SelectContent>
              </Select>
            </div>
            <div v-else-if="form.config.link_type === 'external'" class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">外部链接 *</Label>
              <Input v-model="form.config.link_value" placeholder="https://example.com" />
            </div>
          </div>

          <!-- external_link: { label, url, icon } -->
          <div v-else-if="form.type === 'external_link'" class="space-y-4 rounded-lg border border-border p-4">
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">链接名称 *</Label>
              <Input v-model="form.config.label" placeholder="例如：Telegram 频道" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">链接地址 (http/https) *</Label>
              <Input v-model="form.config.url" placeholder="https://example.com" />
            </div>
            <div class="space-y-2">
              <Label class="text-xs font-medium text-muted-foreground">图标（可选）</Label>
              <Select v-model="form.config.icon">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem v-for="ic in HOME_ENTRY_ICONS" :key="ic" :value="ic">{{ ic }}</SelectItem></SelectContent>
              </Select>
            </div>
          </div>

          <!-- category_entry: { category_ids[]uint } -->
          <div v-else-if="form.type === 'category_entry'" class="space-y-3 rounded-lg border border-border p-4">
            <Label class="text-xs font-medium text-muted-foreground">选择分类（可多选，最多 20）*</Label>
            <div class="flex max-h-60 flex-wrap gap-2 overflow-y-auto">
              <Label v-for="c in categoryOptions" :key="c.id" class="flex items-center gap-2 rounded border border-border px-3 py-1.5 text-sm cursor-pointer" :class="(form.config.category_ids || []).includes(c.id) ? 'border-primary bg-primary/5 text-primary' : ''">
                <input type="checkbox" :checked="(form.config.category_ids || []).includes(c.id)" @change="toggleCategory(c.id)" />
                {{ getLocalizedText(c.name) }} (#{{ c.id }})
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
