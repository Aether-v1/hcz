<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { adminAPI } from '@/api/admin'
import { isHttpUrl } from '@/api/site-builder'
import { notifyError, notifySuccess } from '@/utils/notify'
import FooterLinkRow from './FooterLinkRow.vue'

const loading = ref(false)
const saving = ref(false)

const form = reactive({
  footer_links: [] as any[],
})

const fetchConfig = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getSettings({ key: 'site_config' })
    const data = (res.data?.data || {}) as Record<string, any>
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

const addFooterLink = () => form.footer_links.push({ name: '', url: '' })
const removeFooterLink = (i: number) => form.footer_links.splice(i, 1)

const save = async () => {
  for (const link of form.footer_links) {
    if (link.url && !isHttpUrl(link.url)) {
      notifyError(`Footer 链接「${link.name || ''}」必须以 http:// 或 https:// 开头`)
      return
    }
  }
  saving.value = true
  try {
    // 读取现有 site_config，仅覆盖 footer_links，保留其他字段
    const res = await adminAPI.getSettings({ key: 'site_config' })
    const current = (res.data?.data || {}) as Record<string, any>
    const payload = { ...current }
    // nav_config 是独立 settings key（Storage A），不再嵌套写入 site_config（Storage C 已废弃）。
    delete payload.nav_config
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
    <!-- P1-5: 顶部导航唯一编辑入口在 设置 → 导航，此处仅做跳转提示 -->
    <Alert>
      <AlertTitle>顶部导航维护位置已迁移</AlertTitle>
      <AlertDescription>
        顶部导航请在
        <router-link to="/settings" class="font-medium text-primary underline underline-offset-2">设置 → 导航</router-link>
        中维护，此处仅保留 Footer 链接编辑。
      </AlertDescription>
    </Alert>

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
