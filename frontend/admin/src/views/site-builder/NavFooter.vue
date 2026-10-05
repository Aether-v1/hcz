<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { adminAPI } from '@/api/admin'
import { HOME_ENTRY_ICONS, isHttpUrl } from '@/api/site-builder'
import { notifyError, notifySuccess } from '@/utils/notify'
import NavItemRow from './NavItemRow.vue'
import FooterLinkRow from './FooterLinkRow.vue'

type LocalizedText = Record<string, string>
const LANGS = ['zh-CN', 'zh-TW', 'en-US'] as const
const currentLang = ref<(typeof LANGS)[number]>('zh-CN')

interface CustomNavItem {
  title: LocalizedText
  link_type: 'internal' | 'external'
  url: string
  target: '_self' | '_blank'
  icon: string
  enabled: boolean
  sort_order: number
}

const loading = ref(false)
const saving = ref(false)

const form = reactive({
  builtin: { blog: false, notice: false, about: false },
  custom_items: [] as any[],
  footer_links: [] as any[],
})

const emptyNavItem = (): CustomNavItem => ({
  title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
  link_type: 'internal',
  url: '',
  target: '_self',
  icon: HOME_ENTRY_ICONS[0],
  enabled: true,
  sort_order: 0,
})

const normalizeNavItem = (raw: any): CustomNavItem => {
  const title: LocalizedText = { 'zh-CN': '', 'zh-TW': '', 'en-US': '' }
  if (raw?.title && typeof raw.title === 'object') {
    LANGS.forEach((l) => { title[l] = String(raw.title[l] || '') })
  }
  return {
    title,
    link_type: raw?.link_type === 'external' ? 'external' : 'internal',
    url: String(raw?.url || raw?.name || ''),
    target: raw?.target === '_blank' ? '_blank' : '_self',
    icon: String(raw?.icon || HOME_ENTRY_ICONS[0]),
    enabled: raw?.enabled !== false,
    sort_order: Number(raw?.sort_order || 0),
  }
}

const fetchConfig = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getSettings({ key: 'site_config' })
    const data = (res.data?.data || {}) as Record<string, any>
    const nav = (data.nav_config || {}) as Record<string, any>
    const builtin = (nav.builtin || {}) as Record<string, any>
    form.builtin.blog = Boolean(builtin.blog)
    form.builtin.notice = Boolean(builtin.notice)
    form.builtin.about = Boolean(builtin.about)
    const rawItems = Array.isArray(nav.custom_items) ? nav.custom_items : []
    form.custom_items.splice(0, form.custom_items.length, ...rawItems.map(normalizeNavItem))
    const rawFooter = Array.isArray(data.footer_links) ? data.footer_links : []
    form.footer_links.splice(
      0,
      form.footer_links.length,
      ...rawFooter.map((f: any) => ({ name: String(f?.name || ''), url: String(f?.url || '') }))
    )
  } catch {
    /* 首次使用 */
  } finally {
    loading.value = false
  }
}

const addNavItem = () => form.custom_items.push(emptyNavItem())
const removeNavItem = (i: number) => form.custom_items.splice(i, 1)
const addFooterLink = () => form.footer_links.push({ name: '', url: '' })
const removeFooterLink = (i: number) => form.footer_links.splice(i, 1)

const save = async () => {
  // 校验外链
  for (const item of form.custom_items) {
    if (item.link_type === 'external' && item.url && !isHttpUrl(item.url)) {
      notifyError(`自定义导航「${item.title['zh-CN'] || ''}」外链必须以 http:// 或 https:// 开头`)
      return
    }
  }
  for (const link of form.footer_links) {
    if (link.url && !isHttpUrl(link.url)) {
      notifyError(`Footer 链接「${link.name || ''}」必须以 http:// 或 https:// 开头`)
      return
    }
  }
  saving.value = true
  try {
    // 读取现有 site_config，仅覆盖 nav_config 与 footer_links，保留其他字段
    const res = await adminAPI.getSettings({ key: 'site_config' })
    const current = (res.data?.data || {}) as Record<string, any>
    const payload = { ...current }
    payload.nav_config = {
      builtin: { ...form.builtin },
      custom_items: form.custom_items.map((item) => ({
        title: { ...item.title },
        link_type: item.link_type,
        url: item.url,
        target: item.target,
        icon: item.icon,
        enabled: item.enabled,
        sort_order: Number(item.sort_order || 0),
      })),
    }
    payload.footer_links = form.footer_links.map((l) => ({ name: l.name, url: l.url }))
    await adminAPI.updateSettings({ key: 'site_config', value: payload })
    notifySuccess('已保存')
  } catch {
    notifyError('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(fetchConfig)
</script>

<template>
  <div v-if="!loading" class="space-y-6">
    <!-- 导航配置 -->
    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">顶部导航</h2>
        <p class="mt-1 text-xs text-muted-foreground">控制前台内置导航入口与自定义导航项</p>
      </div>
      <div class="space-y-5 p-6">
        <div class="flex flex-wrap gap-6">
          <Label class="flex items-center gap-2 text-sm cursor-pointer"><Switch v-model="form.builtin.blog" /> 博客 (Blog)</Label>
          <Label class="flex items-center gap-2 text-sm cursor-pointer"><Switch v-model="form.builtin.notice" /> 公告 (Notice)</Label>
          <Label class="flex items-center gap-2 text-sm cursor-pointer"><Switch v-model="form.builtin.about" /> 关于我们 (About)</Label>
        </div>

        <div class="border-t border-border pt-4">
          <div class="mb-3 flex items-center justify-between">
            <h3 class="text-sm font-medium">自定义导航项</h3>
            <Button size="sm" variant="outline" @click="addNavItem">+ 添加</Button>
          </div>
          <div class="mb-3 flex gap-2 border-b border-border">
            <button v-for="l in LANGS" :key="l" type="button"
              class="border-b-2 px-3 py-1.5 text-xs font-medium"
              :class="currentLang === l ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground'"
              @click="currentLang = l">{{ l }}</button>
          </div>
          <div class="space-y-3">
            <NavItemRow
              v-for="(navItem, navIdx) in form.custom_items"
              :key="navIdx"
              :item="navItem"
              :index="navIdx"
              :current-lang="currentLang"
              @remove="removeNavItem"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Footer 配置 -->
    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">Footer 链接</h2>
        <p class="mt-1 text-xs text-muted-foreground">前台底部友情链接列表。版权信息请在「品牌设置」中维护。</p>
      </div>
      <div class="space-y-3 p-6">
        <div class="flex justify-end"><Button size="sm" variant="outline" @click="addFooterLink">+ 添加链接</Button></div>
        <FooterLinkRow
          v-for="(footLink, footIdx) in form.footer_links"
          :key="`footer-link-${footIdx}`"
          :item="footLink"
          :index="footIdx"
          @remove="removeFooterLink"
        />
      </div>
    </div>

    <div class="flex justify-end">
      <Button :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</Button>
    </div>
  </div>
</template>
