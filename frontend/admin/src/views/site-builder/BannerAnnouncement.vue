<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import Banners from '@/views/admin/Banners.vue'
import { adminAPI } from '@/api/admin'
import { notifyError, notifySuccess } from '@/utils/notify'

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
] as const
type Lang = (typeof languages)[number]['code']
const currentLang = ref<Lang>('zh-CN')

const loading = ref(false)
const saving = ref(false)

const form = reactive({
  enabled: false,
  title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<Lang, string>,
  content: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<Lang, string>,
})

const fetchAnnouncement = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getHomeAnnouncement()
    const data = res.data?.data as Record<string, any> | undefined
    if (data && typeof data === 'object') {
      form.enabled = Boolean(data.enabled)
      const title = (data.title || {}) as Record<string, string>
      const content = (data.content || {}) as Record<string, string>
      languages.forEach((lang) => {
        form.title[lang.code] = title[lang.code] || ''
        form.content[lang.code] = content[lang.code] || ''
      })
    }
  } catch {
    /* 首次使用无数据 */
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    await adminAPI.updateHomeAnnouncement({
      enabled: form.enabled,
      title: { ...form.title },
      content: { ...form.content },
    })
    notifySuccess('已保存')
  } catch {
    notifyError('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(fetchAnnouncement)
</script>

<template>
  <div class="space-y-8">
    <!-- Banner 管理：复用现有完整页面 -->
    <section>
      <Banners />
    </section>

    <!-- 首页公告 -->
    <section class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">首页公告</h2>
        <p class="mt-1 text-xs text-muted-foreground">前台首页顶部滚动公告，内容为多语言文本</p>
      </div>

      <div v-if="!loading" class="space-y-5 p-6">
        <div class="flex items-center justify-between">
          <div>
            <Label class="text-sm font-medium">启用公告</Label>
            <p class="mt-1 text-xs text-muted-foreground">关闭后前台不展示</p>
          </div>
          <Switch v-model="form.enabled" />
        </div>

        <div>
          <div class="mb-2 flex items-center gap-2">
            <Label class="text-sm font-medium">标题</Label>
            <div class="flex gap-2 border-b border-border">
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
          </div>
          <Input v-model="form.title[currentLang]" placeholder="公告标题" />
        </div>

        <div>
          <div class="mb-2 flex items-center gap-2">
            <Label class="text-sm font-medium">内容</Label>
            <span class="rounded bg-muted px-2 py-0.5 text-xs text-muted-foreground">{{ currentLang }}</span>
          </div>
          <Textarea v-model="form.content[currentLang]" rows="4" placeholder="公告正文" />
        </div>

        <div class="flex justify-end">
          <Button :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存公告' }}</Button>
        </div>
      </div>
    </section>
  </div>
</template>
