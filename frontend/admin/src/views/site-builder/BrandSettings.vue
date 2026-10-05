<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import SocialLinkRow from './SocialLinkRow.vue'
import { adminAPI } from '@/api/admin'
import {
  getBrand,
  updateBrand,
  SOCIAL_LINK_KEYS,
  isHttpUrl,
  isEmail,
  isHexColor,
  type BrandConfig,
  type SocialLink,
} from '@/api/site-builder'
import { notifyError, notifySuccess } from '@/utils/notify'
import { getImageUrl } from '@/utils/image'

const loading = ref(false)
const saving = ref(false)
const iconUploading = ref(false)
const iconFileInput = ref<HTMLInputElement | null>(null)

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name:'English' },
] as const
type Lang = (typeof languages)[number]['code']

const currentLang = ref<Lang>('zh-CN')

const SOCIAL_LABELS: Record<string, string> = {
  telegram: 'Telegram',
  whatsapp: 'WhatsApp',
  x: 'X (Twitter)',
  discord: 'Discord',
  email: 'Email',
  custom: '自定义链接',
}

const form = reactive({
  site_name: '',
  site_logo: '',
  site_icon: '',
  primary_color: '#4F46E5',
  copyright: '',
  site_description: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
  social_links: SOCIAL_LINK_KEYS.map((key) => ({
    key,
    label: SOCIAL_LABELS[key] || key,
    value: '',
    enabled: false,
  })) as any[],
})

const colorError = ref('')

const normalizeSocialLinks = (raw: unknown): SocialLink[] => {
  const map = new Map<string, SocialLink>()
  SOCIAL_LINK_KEYS.forEach((key) => {
    map.set(key, { key, label: SOCIAL_LABELS[key] || key, value: '', enabled: false })
  })
  if (Array.isArray(raw)) {
    raw.forEach((item) => {
      const rec = item as Record<string, any>
      const key = String(rec?.key || '')
      if (map.has(key)) {
        map.set(key, {
          key,
          label: String(rec.label || SOCIAL_LABELS[key] || key),
          value: String(rec.value || ''),
          enabled: Boolean(rec.enabled),
        })
      }
    })
  }
  return Array.from(map.values())
}

const fetchBrand = async () => {
  loading.value = true
  try {
    const res = await getBrand()
    const data = (res.data?.data || {}) as Partial<BrandConfig>
    form.site_name = String(data.site_name || '')
    form.site_logo = String(data.site_logo || '')
    form.site_icon = String(data.site_icon || '')
    form.primary_color = isHexColor(String(data.primary_color || '')) ? String(data.primary_color) : '#4F46E5'
    form.copyright = String(data.copyright || '')
    const desc = (data.site_description || {}) as Record<string, string>
    languages.forEach((lang) => {
      form.site_description[lang.code] = String(desc[lang.code] || '')
    })
    form.social_links.splice(0, form.social_links.length, ...normalizeSocialLinks(data.social_links))
  } catch {
    /* 接口未就绪时保持默认值 */
  } finally {
    loading.value = false
  }
}

const pickIcon = () => iconFileInput.value?.click()

const handleIconFile = async (e: Event) => {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  iconUploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    const res = await adminAPI.upload(fd, 'common')
    const url = (res.data?.data as any)?.url
    if (url) form.site_icon = String(url)
  } catch {
    notifyError('favicon 上传失败')
  } finally {
    iconUploading.value = false
    input.value = ''
  }
}

const validate = (): boolean => {
  colorError.value = ''
  if (!isHexColor(form.primary_color)) {
    colorError.value = '主色必须是 #RRGGBB 或 #RGB 格式的 HEX 值'
    return false
  }
  for (const link of form.social_links) {
    if (!link.enabled || !link.value.trim()) continue
    if (link.key === 'email') {
      if (!isEmail(link.value)) {
        notifyError(`${SOCIAL_LABELS[link.key]} 不是合法的邮箱地址`)
        return false
      }
    } else if (!isHttpUrl(link.value)) {
      notifyError(`${SOCIAL_LABELS[link.key]} 链接必须以 http:// 或 https:// 开头`)
      return false
    }
  }
  return true
}

const save = async () => {
  if (!validate()) return
  saving.value = true
  try {
    await updateBrand({ ...form, social_links: form.social_links.map((l) => ({ ...l })) })
    notifySuccess('已保存')
  } catch {
    notifyError('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(fetchBrand)
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-xl border border-border bg-card" v-if="!loading">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">品牌基础</h2>
        <p class="mt-1 text-xs text-muted-foreground">前台站点名称、Logo、图标与主色</p>
      </div>

      <div class="grid grid-cols-1 gap-5 p-6 md:grid-cols-2">
        <div class="space-y-2">
          <Label class="text-sm font-medium">站点名称</Label>
          <Input v-model="form.site_name" placeholder="例如：HCZ Store" />
        </div>

        <div class="space-y-2">
          <Label class="text-sm font-medium">版权信息 (Copyright)</Label>
          <Input v-model="form.copyright" placeholder="例如：© 2026 HCZ. All rights reserved." />
        </div>

        <div class="space-y-2 md:col-span-2">
          <Label class="text-sm font-medium">站点 Logo</Label>
          <MediaPicker v-model="form.site_logo" scene="common" />
        </div>

        <div class="space-y-2">
          <Label class="text-sm font-medium">Favicon（浏览器图标，支持 .ico）</Label>
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center overflow-hidden rounded border border-border bg-muted">
              <img v-if="form.site_icon" :src="getImageUrl(form.site_icon)" class="h-full w-full object-contain" />
              <span v-else class="text-[10px] text-muted-foreground">无</span>
            </div>
            <Button type="button" variant="outline" size="sm" :disabled="iconUploading" @click="pickIcon">
              {{ iconUploading ? '上传中…' : '选择文件' }}
            </Button>
            <Button v-if="form.site_icon" type="button" variant="ghost" size="sm" @click="form.site_icon = ''">移除</Button>
            <input ref="iconFileInput" type="file" accept="image/*,.ico" class="hidden" @change="handleIconFile" />
          </div>
        </div>

        <div class="space-y-2">
          <Label class="text-sm font-medium">主色调 (Primary Color)</Label>
          <div class="flex items-center gap-3">
            <input
              type="color"
              v-model="form.primary_color"
              class="h-10 w-14 cursor-pointer rounded border border-border bg-background p-1"
            />
            <Input v-model="form.primary_color" class="max-w-[160px] font-mono" placeholder="#4F46E5" />
          </div>
          <p v-if="colorError" class="text-xs text-destructive">{{ colorError }}</p>
          <p v-else class="text-xs text-muted-foreground">HEX 格式，默认 #4F46E5</p>
        </div>

        <div class="space-y-2 md:col-span-2">
          <div class="flex items-center gap-2">
            <Label class="text-sm font-medium">站点描述</Label>
            <span class="rounded bg-muted px-2 py-0.5 text-xs text-muted-foreground">{{ currentLang }}</span>
          </div>
          <div class="mb-2 flex gap-2 border-b border-border">
            <button
              v-for="lang in languages"
              :key="lang.code"
              type="button"
              class="border-b-2 px-3 py-1.5 text-xs font-medium"
              :class="currentLang === lang.code ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground'"
              @click="currentLang = lang.code"
            >
              {{ lang.name }}
            </button>
          </div>
          <Textarea v-model="form.site_description[currentLang]" rows="3" placeholder="一句话描述站点" />
        </div>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">社交链接</h2>
        <p class="mt-1 text-xs text-muted-foreground">开启后将展示在前台。外链需 http/https，邮箱需合法格式</p>
      </div>
      <div class="space-y-3 p-6">
        <SocialLinkRow
          v-for="(social, socialIdx) in form.social_links"
          :key="`social-${socialIdx}`"
          :item="social"
        />
      </div>
    </div>

    <div class="flex justify-end">
      <Button class="min-w-[120px]" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</Button>
    </div>
  </div>
</template>
